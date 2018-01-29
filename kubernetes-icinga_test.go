package main

import (
	"fmt"
	"testing"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// Create a *KubeIcingaConfig with fake Kubernetes and fake Icinga that can be used for unit tests.
func initTests() *KubeIcingaConfig {
	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})

	return &KubeIcingaConfig{
		Kube:   fake.NewSimpleClientset(),
		Icinga: icinga2.NewMockClient(),
	}
}

// Return true if our configuration has this host..
func (c *KubeIcingaConfig) hasHost(name string) bool {
	hosts, _ := c.Icinga.ListHosts()

	for _, h := range hosts {
		if h.Name == name {
			return true
		}
	}

	return false
}

// Return true if our configuration has this hostgroup.
func (c *KubeIcingaConfig) hasHostGroup(name string) bool {
	hostGroups, _ := c.Icinga.ListHostGroups()

	for _, hg := range hostGroups {
		if hg.Name == name {
			return true
		}
	}

	return false
}

func (c *KubeIcingaConfig) numHosts() int {
	hosts, _ := c.Icinga.ListHosts()

	return len(hosts)
}

func (c *KubeIcingaConfig) numHostGroups() int {
	hostGroups, _ := c.Icinga.ListHostGroups()

	return len(hostGroups)
}

func (c *KubeIcingaConfig) validateHost(host icinga2.Host) error {
	if host.Name == "" {
		return fmt.Errorf("empty host name")
	}
	if host.CheckCommand != "dummy" {
		return fmt.Errorf("check command for host is not 'dummy'")
	}

	return nil
}

// validate certain attributes of a host
func (c *KubeIcingaConfig) validateHostAttribs(host, hostGroup, checkCommand string) error {
	h, _ := c.Icinga.GetHost(host)

	if len(h.Name) == 0 {
		return fmt.Errorf("host %s not found", host)
	}

	if len(h.Groups) != 1 {
		return fmt.Errorf("host is not in exactly 1 groups")
	}

	if h.Groups[0] != hostGroup {
		return fmt.Errorf("host is supposed to be in group %s, but is in %s", hostGroup, h.Groups[0])
	}

	if h.CheckCommand != checkCommand {
		return fmt.Errorf("check command is supposed to be %s, but is %s", checkCommand, h.CheckCommand)
	}

	return nil
}

func (c *KubeIcingaConfig) validateHostGroup(hostGroup icinga2.HostGroup) error {
	if hostGroup.Name == "" {
		return fmt.Errorf("empty host group")
	}
	return nil
}

// Every namespace is created as an icinga hostgroup.
func TestNamespace(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	tests1 := []corev1.Namespace{
		{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "ns1"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "ns2"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "ns3"}},
		{ObjectMeta: metav1.ObjectMeta{
			Name:        "dev",
			Annotations: map[string]string{AnnMonitor: "false"}}},
	}

	for _, ns := range tests1 {
		c.Kube.CoreV1().Namespaces().Create(&ns)
	}

	err := c.sync()

	a.Nil(err)

	for _, name := range []string{"kube-system", "default", "ns1", "ns2", "ns3"} {
		a.True(c.hasHostGroup(name), "should have hostgroup "+name)
	}
	a.False(c.hasHostGroup("dev"), "the dev namespace should not be monitored")

	a.Equal(7, c.numHostGroups()) // includes nodes/infrastructure

	if hostGroups, err := c.Icinga.ListHostGroups(); err == nil {
		for _, hostGroup := range hostGroups {
			a.Nil(c.validateHostGroup(hostGroup))
		}
	}

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ns4"},
	})

	err = c.sync()

	a.Nil(err)

	for _, ns := range []string{"kube-system", "default", "ns1", "ns2", "ns3", "ns4"} {
		a.True(c.hasHostGroup(ns))
	}
	a.Equal(8, c.numHostGroups()) // includes nodes/infrastructure

	if hostGroups, err := c.Icinga.ListHostGroups(); err == nil {
		for _, hostGroup := range hostGroups {
			a.Nil(c.validateHostGroup(hostGroup))
		}
	}

	c.Kube.CoreV1().Namespaces().Delete("ns1", &metav1.DeleteOptions{})
	c.Kube.CoreV1().Namespaces().Delete("ns2", &metav1.DeleteOptions{})

	err = c.sync()

	a.Nil(err)

	for _, ns := range []string{"kube-system", "default", "ns3", "ns4"} {
		a.True(c.hasHostGroup(ns))
	}
	a.Equal(6, c.numHostGroups()) // includes nodes/infrastructure

	if hostGroups, err := c.Icinga.ListHostGroups(); err == nil {
		for _, hostGroup := range hostGroups {
			a.Nil(c.validateHostGroup(hostGroup))
		}
	}

}

// Standalone pods are monitored as icinga hosts.
func TestPod(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	// Create a standalone pod and a pod that looks like it was created by a Deployment.
	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone"}})

	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployed",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "Invisible Deployment",
			}},
		}})

	err := c.sync()

	a.Nil(err)

	a.True(c.hasHostGroup("default"))
	a.Nil(c.validateHostAttribs("default.standalone", "default", "check_kubernetes"))
	a.False(c.hasHost("default.deployed"))
	a.Equal(1, c.numHosts())

	// Add a second standalone pod
	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone2"}})

	err = c.sync()

	a.Nil(err)

	a.True(c.hasHostGroup("default"))
	a.Nil(c.validateHostAttribs("default.standalone", "default", "check_kubernetes"))
	a.Nil(c.validateHostAttribs("default.standalone2", "default", "check_kubernetes"))
	a.False(c.hasHost("default.deployed"))
	a.Equal(2, c.numHosts())

	// Delete the first pod
	c.Kube.CoreV1().Pods("default").Delete("standalone", &metav1.DeleteOptions{})

	err = c.sync()

	a.Nil(err)

	a.True(c.hasHostGroup("default"))
	a.False(c.hasHost("default.standalone"))
	a.Nil(c.validateHostAttribs("default.standalone2", "default", "check_kubernetes"))
	a.False(c.hasHost("default.deployed"))
	a.Equal(1, c.numHosts())
}

// Multiple namespaces with pods in them. Assert that annotated objects or objects
// in annotated namespaces are not monitored.
func TestPodsAndNamespacesAndAnnotations(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})
	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "prod"}})
	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "dev",
			Annotations: map[string]string{AnnMonitor: "false"}}})

	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod1"}})
	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod2",
			Annotations: map[string]string{AnnMonitor: "false"},
		}})

	c.Kube.CoreV1().Pods("prod").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod3",
			Annotations: map[string]string{
				AnnNotes:    "This is important",
				AnnNotesURL: "docs.mysite.com"},
		}})
	c.Kube.CoreV1().Pods("prod").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod4",
			Annotations: map[string]string{AnnMonitor: "false"},
		}})

	c.Kube.CoreV1().Pods("dev").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod5"}})
	c.Kube.CoreV1().Pods("dev").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "pod6",
			Annotations: map[string]string{AnnMonitor: "false"},
		}})

	err := c.sync()

	a.Nil(err)

	a.True(c.hasHostGroup("default"))
	a.True(c.hasHostGroup("prod"))

	a.Equal(4, c.numHostGroups()) // includes nodes/infrastructure
	a.Equal(2, c.numHosts())

	a.Nil(c.validateHostAttribs("default.pod1", "default", "check_kubernetes"))
	a.Nil(c.validateHostAttribs("prod.pod3", "prod", "check_kubernetes"))

	pod3, err := c.Icinga.GetHost("prod.pod3")
	a.Nil(err)
	a.NotNil(pod3)
	a.Equal("This is important", pod3.Notes)
	a.Equal("docs.mysite.com", pod3.NotesURL)
}

// Test configuring different naming templates
func TestNamingThings(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.NamespaceTemplate = "my namespace is called {{.Namespace}}"
	c.WorkloadTemplate = "guess what it's {{.Workload}} in {{.Namespace}}"

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "mypod"}})

	err := c.sync()

	a.Nil(err)

	hgs, err := c.Icinga.ListHostGroups()
	a.Nil(err)
	a.NotNil(hgs)
	a.Equal(3, len(hgs))

	hg, err := c.Icinga.GetHostGroup("my namespace is called default")
	a.Nil(err)

	a.Equal("my namespace is called default", hg.Name)

	a.True(c.hasHostGroup("my namespace is called default"))
	a.Nil(c.validateHostAttribs("guess what it's mypod in default",
		"my namespace is called default",
		"check_kubernetes"))
}

// Test that we do not overwrite any objects in a shared icinga instance.
func TestSharedIcinga(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	// These objects are created outside of kubernetes-icinga and should not be touched.
	c.Icinga.CreateHostGroup(icinga2.HostGroup{
		Name: "database servers",
	})
	c.Icinga.CreateHostGroup(icinga2.HostGroup{
		Name: "kubecluster1",
		Vars: icinga2.Vars{VarCluster: "kubecluster1"},
	})

	c.Icinga.CreateHost(icinga2.Host{
		Name:   "dbserver",
		Groups: []string{"database servers"},
	})
	c.Icinga.CreateHost(icinga2.Host{
		Name:   "kubecluster1.node1",
		Groups: []string{"kubecluster1"},
		Vars:   icinga2.Vars{VarCluster: "kubecluster1"},
	})
	c.Icinga.CreateHost(icinga2.Host{
		Name:   "kubecluster1.somepod",
		Groups: []string{"kubecluster1"},
		Vars:   icinga2.Vars{VarCluster: "kubecluster1"},
	})

	a.Equal(2, c.numHostGroups())
	a.Equal(3, c.numHosts())

	a.True(c.hasHostGroup("database servers"))
	a.True(c.hasHostGroup("kubecluster1"))

	a.Nil(c.validateHostAttribs("dbserver", "database servers", ""))
	a.Nil(c.validateHostAttribs("kubecluster1.node1", "kubecluster1", ""))
	a.Nil(c.validateHostAttribs("kubecluster1.somepod", "kubecluster1", ""))

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "default"}})

	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "mypod"}})

	err := c.sync()

	a.Nil(err)

	a.Equal(5, c.numHostGroups()) // includes nodes/infrastructure
	a.Equal(4, c.numHosts())

	a.True(c.hasHostGroup("database servers"))
	a.True(c.hasHostGroup("kubecluster1"))
	a.True(c.hasHostGroup("default"))

	a.Nil(c.validateHostAttribs("dbserver", "database servers", ""))
	a.Nil(c.validateHostAttribs("kubecluster1.node1", "kubecluster1", ""))
	a.Nil(c.validateHostAttribs("kubecluster1.somepod", "kubecluster1", ""))
}

// Test that the notes and notes-url annotations can be changed.
func TestChangeNotes(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})

	c.Kube.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod",
			Annotations: map[string]string{
				AnnNotes:    "just testing",
				AnnNotesURL: "www.mysite.com/havingfun"},
		}})

	err := c.sync()

	a.Nil(err)

	pod3, err := c.Icinga.GetHost("default.pod")

	a.Equal("just testing", pod3.Notes)
	a.Equal("www.mysite.com/havingfun", pod3.NotesURL)

	// suddenly it's in production

	c.Kube.CoreV1().Pods("default").Update(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pod",
			Annotations: map[string]string{
				AnnNotes:    "this is very important",
				AnnNotesURL: "docs.mysite.com/404.html"},
		}})

	err = c.sync()

	a.Nil(err)

	pod3, err = c.Icinga.GetHost("default.pod")

	a.Equal("this is very important", pod3.Notes)
	a.Equal("docs.mysite.com/404.html", pod3.NotesURL)
}

// Test ReplicaSet
func TestReplicaSet(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})

	c.Kube.AppsV1beta2().ReplicaSets("default").Create(&appsv1beta2.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "standalone",
			Annotations: map[string]string{
				AnnNotes: "a great replicaset",
			},
		},
	})

	c.Kube.AppsV1beta2().ReplicaSets("default").Create(&appsv1beta2.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "controlled",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "Invisible Deployment",
			}},
		},
	})

	c.Kube.AppsV1beta2().ReplicaSets("default").Create(&appsv1beta2.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmonitored",
			Annotations: map[string]string{
				AnnMonitor: "false",
			},
		},
	})

	err := c.sync()

	a.Nil(err)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(1, len(hosts))

	host, err := c.Icinga.GetHost("default.standalone")
	a.Nil(err)
	a.Equal("a great replicaset", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("standalone", host.Vars[VarName])
	a.Equal("replicaset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
}

// Test StatefulSet
func TestStatefulSet(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})

	c.Kube.AppsV1beta2().StatefulSets("default").Create(&appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myset",
			Annotations: map[string]string{
				AnnNotes: "an even greater statefulset",
			},
		},
	})

	c.Kube.AppsV1beta2().StatefulSets("default").Create(&appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmonitored",
			Annotations: map[string]string{
				AnnMonitor: "false",
			},
		},
	})

	err := c.sync()

	a.Nil(err)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(1, len(hosts))

	host, err := c.Icinga.GetHost("default.myset")
	a.Nil(err)
	a.Equal("an even greater statefulset", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("myset", host.Vars[VarName])
	a.Equal("statefulset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
}

// Test DaemonSet
func TestDaemonSet(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})

	c.Kube.AppsV1beta2().DaemonSets("default").Create(&appsv1beta2.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myset",
			Annotations: map[string]string{
				AnnNotes: "a little daemonset",
			},
		},
	})

	c.Kube.AppsV1beta2().DaemonSets("default").Create(&appsv1beta2.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmonitored",
			Annotations: map[string]string{
				AnnMonitor: "false",
			},
		},
	})

	err := c.sync()

	a.Nil(err)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(1, len(hosts))

	host, err := c.Icinga.GetHost("default.myset")
	a.Nil(err)
	a.Equal("a little daemonset", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("myset", host.Vars[VarName])
	a.Equal("daemonset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
}

// Test Deployment
func TestDeployment(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default"}})

	c.Kube.AppsV1beta2().Deployments("default").Create(&appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mydepl",
			Annotations: map[string]string{
				AnnNotes: "a nice deployment",
			},
		},
	})

	c.Kube.AppsV1beta2().Deployments("default").Create(&appsv1beta2.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmonitored",
			Annotations: map[string]string{
				AnnMonitor: "false",
			},
		},
	})

	err := c.sync()

	a.Nil(err)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(1, len(hosts))

	host, err := c.Icinga.GetHost("default.mydepl")
	a.Nil(err)
	a.Equal("a nice deployment", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("mydepl", host.Vars[VarName])
	a.Equal("deployment", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
}

// Test Nodes
func TestNodes(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().Nodes().Create(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node1"}})
	c.Kube.CoreV1().Nodes().Create(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node2"}})
	c.Kube.CoreV1().Nodes().Create(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node3"}})

	err := c.sync()

	a.Nil(err)

	hostgroups, err := c.Icinga.ListHostGroups()
	a.Nil(err)
	a.Equal(2, len(hostgroups))

	hostgroup, err := c.Icinga.GetHostGroup("nodes")
	a.Nil(err)
	a.Equal("nodes", hostgroup.Name)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(3, len(hosts))

	for _, i := range []string{"1", "2", "3"} {
		host, err := c.Icinga.GetHost("nodes.node" + i)
		a.Nil(err)
		a.NotNil(host)
		a.Equal("nodes.node"+i, host.Name)
		a.Equal(1, len(host.Groups))
		a.Equal("nodes", host.Groups[0])
		a.NotNil(host.Vars)
		a.Equal("node"+i, host.Vars[VarName])
		a.Equal("node", host.Vars[VarType])
	}
}

// Test component statuses
func TestComponentstatuses(t *testing.T) {
	a := assert.New(t)
	c := initTests()

	c.Kube.CoreV1().ComponentStatuses().Create(&corev1.ComponentStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "comp-a"}})
	c.Kube.CoreV1().ComponentStatuses().Create(&corev1.ComponentStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "comp-b"}})
	c.Kube.CoreV1().ComponentStatuses().Create(&corev1.ComponentStatus{
		ObjectMeta: metav1.ObjectMeta{Name: "comp-c"}})

	err := c.sync()

	a.Nil(err)

	hostgroups, err := c.Icinga.ListHostGroups()
	a.Nil(err)
	a.Equal(2, len(hostgroups))

	hostgroup, err := c.Icinga.GetHostGroup("infrastructure")
	a.Nil(err)
	a.Equal("infrastructure", hostgroup.Name)

	hosts, err := c.Icinga.ListHosts()
	a.Nil(err)
	a.Equal(3, len(hosts))

	for _, i := range []string{"a", "b", "c"} {
		host, err := c.Icinga.GetHost("infrastructure.comp-" + i)
		a.Nil(err)
		a.Equal("infrastructure.comp-"+i, host.Name)
		a.Equal(1, len(host.Groups))
		a.Equal("infrastructure", host.Groups[0])
		a.NotNil(host.Vars)
		a.Equal("comp-"+i, host.Vars[VarName])
		a.Equal("componentstatus", host.Vars[VarType])
	}
}
