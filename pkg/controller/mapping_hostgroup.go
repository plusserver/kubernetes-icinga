package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
)

type HostGroupMapping struct{}

func (m *HostGroupMapping) MonitorCluster(c *Controller) error {
	return nil
}

func (m *HostGroupMapping) MonitorNamespace(c *Controller, namespace *corev1.Namespace) error {
	return c.reconcileHostGroup(
		&icingav1.HostGroup{
			ObjectMeta: MakeObjectMeta(namespace, "Namespace", "v1", "", true),
			Spec: icingav1.HostGroupSpec{
				Name: namespace.GetName(),
				Vars: c.MakeVars(namespace, "namespace", false),
			},
		},
	)
}

func (m *HostGroupMapping) UnmonitorNamespace(c *Controller, namespace *corev1.Namespace) error {
	return c.deleteHostGroup("kube-system", namespace.Name)
}

func (m *HostGroupMapping) MonitorNodesGroup(c *Controller) error {
	kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting kube-system namespace: %s", err.Error())
	}

	newHg := &icingav1.HostGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nodes",
			Namespace:       "kube-system",
			OwnerReferences: MakeOwnerRef(kubeSystem, "Namespace", "v1"),
		},
		Spec: icingav1.HostGroupSpec{
			Name: "nodes",
			Vars: map[string]string{VarCluster: c.Tag},
		},
	}

	hg, err := c.IcingaClient.IcingaV1().HostGroups("kube-system").Get("nodes", metav1.GetOptions{})

	if err == nil {
		if hg.Spec.Name != newHg.Spec.Name {
			log.Infof("updating default hostgroup 'kube-system/%s'", "nodes")
			_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Update(newHg)
			if err != nil {
				return fmt.Errorf("error updating hostgroup 'kube-system/%s': %s", "nodes", err.Error())
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating default hostgroup 'kube-system/%s'", "nodes")
		_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Create(newHg)
		if err != nil {
			return fmt.Errorf("error creating hostgroup 'kube-system/%s': %s", "nodes", err.Error())
		}
	} else {
		return err
	}

	return nil
}

func (m *HostGroupMapping) MonitorInfrastructureGroup(c *Controller) error {
	kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting kube-system namespace: %s", err.Error())
	}

	newHg := &icingav1.HostGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "infrastructure",
			Namespace:       "kube-system",
			OwnerReferences: MakeOwnerRef(kubeSystem, "Namespace", "v1"),
		},
		Spec: icingav1.HostGroupSpec{
			Name: "infrastructure",
			Vars: map[string]string{VarCluster: c.Tag},
		},
	}

	hg, err := c.IcingaClient.IcingaV1().HostGroups("kube-system").Get("infrastructure", metav1.GetOptions{})

	if err == nil {
		if hg.Spec.Name != newHg.Spec.Name {
			log.Infof("updating default hostgroup 'kube-system/%s'", "infrastructure")
			_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Update(newHg)
			if err != nil {
				return fmt.Errorf("error updating hostgroup 'kube-system/%s': %s", "infrastructure", err.Error())
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating default hostgroup 'kube-system/%s'", "infrastructure")
		_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Create(newHg)
		if err != nil {
			return fmt.Errorf("error creating hostgroup 'kube-system/%s': %s", "infrastructure", err.Error())
		}
	} else {
		return err
	}

	return nil
}

func (m *HostGroupMapping) MonitorNode(c *Controller, node *corev1.Node) error {
	return c.reconcileHost(&icingav1.Host{
		ObjectMeta: MakeObjectMeta(node, "Node", "v1", "", true),
		Spec: icingav1.HostSpec{
			CheckCommand: "check_kubernetes",
			Name:         "nodes." + node.Name,
			Hostgroups:   []string{"nodes"},
			Vars:         c.MakeVars(node, "node", false)},
	})
}

func (m *HostGroupMapping) UnmonitorNode(c *Controller, node *corev1.Node) error {
	return c.deleteHost("kube-system", node.Name)
}

func (m *HostGroupMapping) MonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error {
	return c.reconcileHost(
		&icingav1.Host{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cs-" + cs.Name,
				Namespace: "kube-system",
			},
			Spec: icingav1.HostSpec{
				Name:         "infrastructure.cs-" + cs.Name,
				Hostgroups:   []string{"infrastructure"},
				CheckCommand: "check_kubernetes",
				Vars:         c.MakeVars(cs, "componentstatus", false),
			},
		})
}

func (m *HostGroupMapping) UnmonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error {
	return c.deleteHost("kube-system", cs.Name)
}

func (m *HostGroupMapping) MonitorWorkload(c *Controller, o metav1.Object, abbrev, typ, kind, apiVersion string) error {
	h := &icingav1.Host{
		ObjectMeta: MakeObjectMeta(o, kind, apiVersion, abbrev, false),
		Spec: icingav1.HostSpec{
			Name:         fmt.Sprintf("%s.%s-%s", o.GetNamespace(), abbrev, o.GetName()),
			Hostgroups:   []string{o.GetNamespace()},
			CheckCommand: "check_kubernetes",
			Vars:         c.MakeVars(o, typ, true),
		},
	}

	if a, ok := o.GetAnnotations()[AnnNotes]; ok && a != "" {
		h.Spec.Notes = a
	}

	if a, ok := o.GetAnnotations()[AnnNotesURL]; ok && a != "" {
		h.Spec.NotesURL = a
	}

	return c.reconcileHost(h)
}

func (m *HostGroupMapping) UnmonitorWorkload(c *Controller, o metav1.Object, abbrev string) error {
	return c.deleteHost(o.GetNamespace(), fmt.Sprintf("%s-%s", abbrev, o.GetName()))
}
