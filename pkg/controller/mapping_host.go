package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"fmt"
	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
)

type HostMapping struct{}

func (m *HostMapping) Name() string {
	return "host"
}

func (m *HostMapping) MonitorCluster(c *Controller) error {
	kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting kube-system namespace: %s", err.Error())
	}

	return c.reconcileHostGroup(
		&icingav1.HostGroup{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "cluster." + c.Tag,
				Namespace:       "kube-system",
				OwnerReferences: MakeOwnerRef(kubeSystem, "Namespace", "v1"),
			},
			Spec: icingav1.HostGroupSpec{
				Name: EMPTY,
				Vars: c.MakeVars(kubeSystem, "namespace", false),
			},
		},
	)
}

func (m *HostMapping) MonitorNamespace(c *Controller, namespace *corev1.Namespace) error {
	return c.reconcileHost(
		&icingav1.Host{
			ObjectMeta: MakeObjectMeta(namespace, "Namespace", "v1", "", true),
			Spec: icingav1.HostSpec{
				Name:         namespace.GetName(),
				Hostgroups:   []string{EMPTY},
				CheckCommand: "dummy",
				Vars:         c.MakeVars(namespace, "namespace", false),
			},
		},
	)
}

func (m *HostMapping) UnmonitorNamespace(c *Controller, namespace *corev1.Namespace) error {
	return c.deleteHost("kube-system", namespace.Name)
}

func (m *HostMapping) MonitorNodesGroup(c *Controller) error {
	kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting kube-system namespace: %s", err.Error())
	}

	return c.reconcileHost(&icingav1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "nodes",
			Namespace:       "kube-system",
			OwnerReferences: MakeOwnerRef(kubeSystem, "Namespace", "v1"),
		},
		Spec: icingav1.HostSpec{
			Name:         "nodes",
			CheckCommand: "dummy",
			Hostgroups:   []string{EMPTY},
			Vars:         map[string]string{VarCluster: c.Tag},
		},
	})
}

func (m *HostMapping) MonitorInfrastructureGroup(c *Controller) error {
	kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("error getting kube-system namespace: %s", err.Error())
	}

	return c.reconcileHost(&icingav1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:            "infrastructure",
			Namespace:       "kube-system",
			OwnerReferences: MakeOwnerRef(kubeSystem, "Namespace", "v1"),
		},
		Spec: icingav1.HostSpec{
			Name:         "infrastructure",
			CheckCommand: "dummy",
			Hostgroups:   []string{EMPTY},
			Vars:         map[string]string{VarCluster: c.Tag},
		},
	})
}

func (m *HostMapping) MonitorNode(c *Controller, node *corev1.Node) error {
	return c.reconcileCheck(&icingav1.Check{
		ObjectMeta: MakeObjectMeta(node, "Node", "v1", "", true),
		Spec: icingav1.CheckSpec{
			Host:         "nodes",
			CheckCommand: "check_kubernetes",
			Name:         node.Name,
			Vars:         c.MakeVars(node, "node", false)},
	})
}

func (m *HostMapping) UnmonitorNode(c *Controller, node *corev1.Node) error {
	return c.deleteCheck("kube-system", node.Name)
}

func (m *HostMapping) MonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error {
	return c.reconcileCheck(
		&icingav1.Check{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cs-" + cs.Name,
				Namespace: "kube-system",
			},
			Spec: icingav1.CheckSpec{
				Name:         "cs-" + cs.Name,
				Host:         "infrastructure",
				CheckCommand: "check_kubernetes",
				Vars:         c.MakeVars(cs, "componentstatus", false),
			},
		})
}

func (m *HostMapping) UnmonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error {
	return c.deleteCheck("kube-system", "cs-"+cs.Name)
}

func (m *HostMapping) MonitorWorkload(c *Controller, o metav1.Object, abbrev, typ, kind, apiVersion string) error {
	h := &icingav1.Check{
		ObjectMeta: MakeObjectMeta(o, kind, apiVersion, abbrev, false),
		Spec: icingav1.CheckSpec{
			Host:         o.GetNamespace(),
			Name:         fmt.Sprintf("%s-%s", abbrev, o.GetName()),
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

	return c.reconcileCheck(h)
}

func (m *HostMapping) UnmonitorWorkload(c *Controller, o metav1.Object, abbrev string) error {
	return c.deleteCheck(o.GetNamespace(), fmt.Sprintf("%s-%s", abbrev, o.GetName()))
}
