package main

import (
	"bytes"
	"fmt"
	"os"
	"text/template"
	"time"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	log "github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	//appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubeIcingaConfig struct {
	Kube   kubernetes.Interface
	Icinga icinga2.Client

	RefreshInterval int

	NamespaceTemplate string
	WorkloadTemplate  string

	ClusterName string

	templates struct {
		namespace *template.Template
		workload  *template.Template
	}
}

type TemplateParams struct {
	Namespace string
	Workload  string
}

const (
	defaultNamespaceTemplate = "{{.Namespace}}"
	defaultWorkloadTemplate  = "{{.Namespace}}.{{.Workload}}"

	defaultClusterName = "kubernetes"

	// These are Vars used on icinga2 objects

	// The kubernetes cluster name, used in shared icinga instances.
	VarCluster = "k8si.cluster"

	// The object type to monitor.
	VarType = "kubernetes_type"

	// The object name to monitor.
	VarName = "kubernetes_name"

	// The namespace containing the monitored object.
	VarNamespace = "kubernetes_namespace"

	// The following are annotations on kubernetes objects

	// This annotation enables or disables monitoring
	AnnMonitor = "com.nexinto/k8si-monitor"

	// This annotation will be used for Host/Service notes
	AnnNotes = "com.nexinto/k8si-notes"

	// This annotation will be used for Host/Service notes url
	AnnNotesURL = "com.nexinto/k8si-notes-url"
)

func main() {
	if e := os.Getenv("LOG_LEVEL"); e != "" {
		if l, err := log.ParseLevel(e); err == nil {
			log.SetLevel(l)
		} else {
			log.SetLevel(log.WarnLevel)
			log.Warnf("unkown log level %s, setting to 'warn'", e)
		}
	}

	var kubeconfig string

	if c := os.Getenv("KUBECONFIG"); c != "" {
		kubeconfig = c
	}

	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err.Error())
	}

	icingaClient, err := icinga2.New(icinga2.WebClient{
		URL:      os.Getenv("ICINGA_URL"),
		Username: os.Getenv("ICINGA_USER"),
		Password: os.Getenv("ICINGA_PASSWORD"),
		//Debug:       true,
		InsecureTLS: true})

	if err != nil {
		panic(err.Error())
	}

	// the configuration object
	config := &KubeIcingaConfig{
		Kube:   clientset,
		Icinga: icingaClient,
	}

	if c := os.Getenv("REFRESH_INTERVAL"); c != "" {
		fmt.Sscanf(c, "%d", &config.RefreshInterval)
	} else {
		config.RefreshInterval = 0
	}

	for {
		log.Infof("Running at %s", time.Now().Local())

		err = config.sync()

		if err != nil {
			log.Errorf("error synchronizing workload: %s", err.Error())
		}

		if config.RefreshInterval <= 0 {
			break
		} else {
			log.Debugf("Sleeping for %d seconds", config.RefreshInterval)
			time.Sleep(time.Duration(config.RefreshInterval) * time.Second)
		}
	}

	log.Debugf("Exiting.")
	os.Exit(0)

}

// Create the template objects.
func (c *KubeIcingaConfig) parseTemplates() error {
	if len(c.NamespaceTemplate) == 0 {
		c.NamespaceTemplate = defaultNamespaceTemplate
	}
	if len(c.WorkloadTemplate) == 0 {
		c.WorkloadTemplate = defaultWorkloadTemplate
	}

	if t, err := template.New("namespace").Parse(c.NamespaceTemplate); err == nil {
		c.templates.namespace = t
	} else {
		return err
	}

	if t, err := template.New("workload").Parse(c.WorkloadTemplate); err == nil {
		c.templates.workload = t
	} else {
		return err
	}

	return nil
}

// Returns the list of hosts that are managed by us.
func (c *KubeIcingaConfig) managedHosts() (hosts []icinga2.Host, err error) {
	lhosts, err := c.Icinga.ListHosts()
	if err != nil {
		return []icinga2.Host{}, err
	}

	for _, host := range lhosts {
		if host.Vars != nil && host.Vars[VarCluster] != nil && host.Vars[VarCluster] == c.ClusterName {
			hosts = append(hosts, host)
		}
	}

	return hosts, nil
}

// Returns the list of hostgroups that are managed by us.
func (c *KubeIcingaConfig) managedHostGroups() (hostGroups []icinga2.HostGroup, err error) {
	lhostGroups, err := c.Icinga.ListHostGroups()
	if err != nil {
		return []icinga2.HostGroup{}, err
	}

	for _, hostGroup := range lhostGroups {
		if hostGroup.Vars != nil && hostGroup.Vars[VarCluster] != nil && hostGroup.Vars[VarCluster] == c.ClusterName {
			hostGroups = append(hostGroups, hostGroup)
		}
	}

	return hostGroups, nil
}

// Returns true if an object should be monitored.
func (c *KubeIcingaConfig) isMonitored(m metav1.ObjectMeta) bool {
	if len(m.OwnerReferences) > 0 {
		// not standalone, so it should be monitored by whatever controls it
		return false
	}
	return m.Annotations[AnnMonitor] != "false"
}

// Returns the list of namespaces that are to be monitored.
func (c *KubeIcingaConfig) Namespaces() ([]corev1.Namespace, error) {
	var namespaces []corev1.Namespace

	nse, err := c.Kube.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return []corev1.Namespace{}, err
	}

	for _, namespace := range nse.Items {
		if c.isMonitored(namespace.ObjectMeta) {
			namespaces = append(namespaces, namespace)
		}
	}

	return namespaces, nil
}

// Sync whole configuration
func (c *KubeIcingaConfig) sync() error {

	log.SetLevel(log.DebugLevel)

	if len(c.ClusterName) == 0 {
		c.ClusterName = defaultClusterName
	}

	if err := c.parseTemplates(); err != nil {
		return err
	}

	if err := c.syncHostGroups(); err != nil {
		return err
	}

	if err := c.syncHosts(); err != nil {
		return err
	}

	return nil
}

func execTemplate(t *template.Template, p TemplateParams) string {
	var buffer bytes.Buffer

	if t == nil {
		panic("t is nil")
	}

	err := t.Execute(&buffer, p)
	if err != nil {
		panic(err)
	}

	return buffer.String()
}

// Constructs a list of icinga2 hostgroups that represent the namespaces and infrastructure elements.
func (c *KubeIcingaConfig) wantedHostGroups() (hostGroups []icinga2.HostGroup, err error) {

	namespaces, err := c.Namespaces()
	if err != nil {
		return []icinga2.HostGroup{}, err
	}

	hostGroups = []icinga2.HostGroup{
		{
			Name: "nodes", // TODO: template
			Vars: icinga2.Vars{VarCluster: c.ClusterName},
		},
		{
			Name: "infrastructure", // TODO: template
			Vars: icinga2.Vars{VarCluster: c.ClusterName},
		},
	}

	for _, namespace := range namespaces {
		if !c.isMonitored(namespace.ObjectMeta) {
			continue
		}

		tp := TemplateParams{Namespace: namespace.Name}

		hostGroups = append(hostGroups,
			icinga2.HostGroup{
				Name: execTemplate(c.templates.namespace, tp),
				Vars: icinga2.Vars{VarCluster: c.ClusterName},
			})
	}

	return
}

// Constructs a list of icinga2 hosts that represent all workload objects and infrastructure elements.
func (c *KubeIcingaConfig) wantedHosts() (hosts []icinga2.Host, err error) {

	namespaces, err := c.Namespaces()
	if err != nil {
		return
	}

	for _, namespace := range namespaces {
		pods, err := c.Kube.CoreV1().Pods(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return []icinga2.Host{}, err
		}

		for _, pod := range pods.Items {
			if !c.isMonitored(pod.ObjectMeta) {
				continue
			}

			hosts = append(hosts, c.mkHost(&pod.ObjectMeta, "pod"))
		}

		replicasets, err := c.Kube.AppsV1beta2().ReplicaSets(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return []icinga2.Host{}, err
		}

		for _, rs := range replicasets.Items {
			if !c.isMonitored(rs.ObjectMeta) {
				continue
			}

			hosts = append(hosts, c.mkHost(&rs.ObjectMeta, "replicaset"))
		}

		statefulsets, err := c.Kube.AppsV1beta2().StatefulSets(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return []icinga2.Host{}, err
		}

		for _, ss := range statefulsets.Items {
			if !c.isMonitored(ss.ObjectMeta) {
				continue
			}

			hosts = append(hosts, c.mkHost(&ss.ObjectMeta, "statefulset"))
		}

		daemonsets, err := c.Kube.AppsV1beta2().DaemonSets(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return []icinga2.Host{}, err
		}

		for _, ds := range daemonsets.Items {
			if !c.isMonitored(ds.ObjectMeta) {
				continue
			}

			hosts = append(hosts, c.mkHost(&ds.ObjectMeta, "daemonset"))
		}

		deployments, err := c.Kube.AppsV1beta2().Deployments(namespace.Name).List(metav1.ListOptions{})
		if err != nil {
			return []icinga2.Host{}, err
		}

		for _, dd := range deployments.Items {
			if !c.isMonitored(dd.ObjectMeta) {
				continue
			}

			hosts = append(hosts, c.mkHost(&dd.ObjectMeta, "deployment"))
		}
	}

	nodes, err := c.Kube.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, node := range nodes.Items {
		hosts = append(hosts, c.mkHost(&node.ObjectMeta, "node"))
	}

	componentstatuses, err := c.Kube.CoreV1().ComponentStatuses().List(metav1.ListOptions{})
	if err != nil {
		return
	}

	for _, cs := range componentstatuses.Items {
		hosts = append(hosts, c.mkHost(&cs.ObjectMeta, "componentstatus"))
	}

	return
}

func (c *KubeIcingaConfig) mkHost(m *metav1.ObjectMeta, objectType string) icinga2.Host {
	namespace := m.Namespace

	switch objectType {
	case "node":
		namespace = "nodes"
	case "componentstatus":
		namespace = "infrastructure"
	}

	tp := TemplateParams{
		Namespace: namespace,
		Workload:  m.Name,
	}

	return icinga2.Host{
		Name:         execTemplate(c.templates.workload, tp),
		Groups:       []string{execTemplate(c.templates.namespace, tp)},
		CheckCommand: "check_kubernetes",
		Vars: icinga2.Vars{
			VarCluster:   c.ClusterName,
			VarType:      objectType,
			VarName:      m.Name,
			VarNamespace: namespace,
		},
		Notes:    m.Annotations[AnnNotes],
		NotesURL: m.Annotations[AnnNotesURL],
	}
}

// Sync Kubernetes namespaces and infrastructure groups with Icinga hostgroups
func (c *KubeIcingaConfig) syncHostGroups() error {
	wanted, err := c.wantedHostGroups()
	if err != nil {
		return err
	}

	current, err := c.managedHostGroups()
	if err != nil {
		return err
	}

	var createHostGroups, updateHostGroups []icinga2.HostGroup
	var deleteHostGroups []string

	for _, w := range wanted {
		found := false

		for _, c := range current {
			if w.Name == c.Name { // TODO: use metadata
				found = true
			}
		}

		if !found {
			createHostGroups = append(createHostGroups, w)
		}
	}

	for _, c := range current {
		found := false

		for _, w := range wanted {
			if w.Name == c.Name { // TODO: use metadata
				found = true
			}
		}

		if !found {
			deleteHostGroups = append(deleteHostGroups, c.Name)
		}
	}

	for _, g := range createHostGroups {
		log.Infof("creating hostgroup %s", g.Name)
		c.Icinga.CreateHostGroup(g)
	}

	for _, g := range updateHostGroups {
		log.Infof("updating hostgroup %s", g.Name)
		c.Icinga.UpdateHostGroup(g)
	}

	for _, g := range deleteHostGroups {
		log.Infof("deleting hostgroup %s", g)
		c.Icinga.DeleteHostGroup(g)
	}

	return nil
}

// Sync all Kubernetes objects are monitored with Icinga hosts.
func (c *KubeIcingaConfig) syncHosts() error {
	wanted, err := c.wantedHosts()
	if err != nil {
		return err
	}

	current, err := c.managedHosts()
	if err != nil {
		return err
	}

	var createHosts, updateHosts []icinga2.Host
	var deleteHosts []string

	for _, h := range wanted {
		found := false
		update := false

		for _, ch := range current {
			if h.Name == ch.Name { // TODO: use metadata
				found = true

				for k, v := range h.Vars {
					if ch.Vars[k] != v {
						log.Infof("Updating host %s: var %s: %s -> %s", h.Name, k, ch.Vars[k], v)
						update = true
					}
				}

				if ch.Notes != h.Notes {
					log.Infof("Updating host %s: notes %s -> %s", h.Name, ch.Notes, h.Notes)
					update = true
				}

				if ch.NotesURL != h.NotesURL {
					log.Infof("Updating host %s: notes url %s -> %s", h.Name, ch.NotesURL, h.NotesURL)
					update = true
				}
			}
		}

		if update {
			updateHosts = append(updateHosts, h)
		}

		if !found {
			createHosts = append(createHosts, h)
		}
	}

	for _, ch := range current {
		found := false

		for _, h := range wanted {
			if h.Name == ch.Name { // TODO: use metadata
				found = true
			}
		}

		if !found {
			deleteHosts = append(deleteHosts, ch.Name)
		}
	}

	for _, host := range createHosts {
		log.Infof("creating host %s", host.Name)
		c.Icinga.CreateHost(host)
	}

	for _, host := range updateHosts {
		log.Infof("updating host %s", host.Name)
		c.Icinga.UpdateHost(host)
	}

	for _, host := range deleteHosts {
		log.Infof("deleting host %s", host)
		c.Icinga.DeleteHost(host)
	}

	return nil
}
