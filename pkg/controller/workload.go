package main

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
)

func (c *Controller) PodCreatedOrUpdated(pod *corev1.Pod) error {
	return c.processWorkload(pod, "po", "pod", "Pod")
}

func (c *Controller) PodDeleted(pod *corev1.Pod) error {
	log.Debugf("processing deleted pod '%s/%s'", pod.Namespace, pod.Name)
	return c.deleteHost(pod.Namespace, "po-"+pod.Name)
}

func (c *Controller) NodeCreatedOrUpdated(node *corev1.Node) error {
	log.Debugf("processing node '%s'", node.Name)
	return c.reconcileHost(&icingav1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Node",
				Name: node.GetName(),
				UID:  node.GetUID(),
			}},
		},
		Spec: icingav1.HostSpec{
			CheckCommand: "check_kubernetes",
			Name:         "nodes." + node.Name,
			Hostgroups:   []string{"nodes"},
			Vars:         c.MakeVars(node, "node", false)},
	})
}

func (c *Controller) NodeDeleted(node *corev1.Node) error {
	log.Debugf("processing deleted node '%s'", node.Name)
	return nil
}

func (c *Controller) NamespaceCreatedOrUpdated(namespace *corev1.Namespace) error {
	log.Debugf("processing namespace '%s'", namespace.Name)
	if c.monitored(namespace) {
		return c.reconcileHostGroup(
			&icingav1.HostGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespace.GetName(),
					// The hostgroup objects that represent Namespaces are all stored in kube-system.
					Namespace: "kube-system",
					OwnerReferences: []metav1.OwnerReference{{
						Kind: "Namespace",
						Name: namespace.GetName(),
						UID:  namespace.GetUID(),
					}},
				},
				Spec: icingav1.HostGroupSpec{
					Name: namespace.GetName(),
					Vars: c.MakeVars(namespace, "namespace", false),
				},
			},
		)
	} else {
		return c.deleteHostGroup("kube-system", namespace.Name)
	}
}

func (c *Controller) NamespaceDeleted(namespace *corev1.Namespace) error {
	log.Debugf("processing deleted namespace '%s'", namespace.Name)
	return c.deleteHostGroup("kube-system", namespace.Name)
}

func (c *Controller) DeploymentCreatedOrUpdated(deployment *extensionsv1beta1.Deployment) error {
	return c.processWorkload(deployment, "deploy", "deployment", "Deployment")
}

func (c *Controller) DeploymentDeleted(deployment *extensionsv1beta1.Deployment) error {
	log.Debugf("processing deleted deployment '%s/%s'", deployment.Namespace, deployment.Name)
	return c.deleteHost(deployment.Namespace, "deploy-"+deployment.Name)
}

func (c *Controller) DaemonSetCreatedOrUpdated(daemonset *extensionsv1beta1.DaemonSet) error {
	return c.processWorkload(daemonset, "ds", "daemonset", "DaemonSet")
}

func (c *Controller) DaemonSetDeleted(daemonset *extensionsv1beta1.DaemonSet) error {
	log.Debugf("processing deleted daemonset '%s/%s'", daemonset.Namespace, daemonset.Name)
	return c.deleteHost(daemonset.Namespace, "ds-"+daemonset.Name)
}

func (c *Controller) ReplicaSetCreatedOrUpdated(replicaset *extensionsv1beta1.ReplicaSet) error {
	return c.processWorkload(replicaset, "rs", "replicaset", "ReplicaSet")
}

func (c *Controller) ReplicaSetDeleted(replicaset *extensionsv1beta1.ReplicaSet) error {
	log.Debugf("processing deleted replicaset '%s/%s'", replicaset.Namespace, replicaset.Name)
	return c.deleteHost(replicaset.Namespace, "rs-"+replicaset.Name)
}

func (c *Controller) StatefulSetCreatedOrUpdated(statefulset *appsv1beta2.StatefulSet) error {
	return c.processWorkload(statefulset, "statefulset", "statefulset", "StatefulSet")
}

func (c *Controller) StatefulSetDeleted(statefulset *appsv1beta2.StatefulSet) error {
	log.Debugf("processing deleted statefulset '%s/%s'", statefulset.Namespace, statefulset.Name)
	return c.deleteHost(statefulset.Namespace, "statefulset-"+statefulset.Name)
}

func (c *Controller) processWorkload(o metav1.Object, abbrev, typ, kind string) error {
	log.Debugf("processing %s '%s/%s'", typ, o.GetNamespace(), o.GetName())
	if !c.monitored(o) {
		return c.deleteHost(o.GetNamespace(), fmt.Sprintf("%s-%s", abbrev, o.GetName()))
	} else {
		return c.reconcileHost(c.hostForWorkload(o, abbrev, typ, kind))
	}
}

// Create a generic Host object for workloads
func (c *Controller) hostForWorkload(o metav1.Object, abbrev, typ, kind string) *icingav1.Host {
	h := &icingav1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-%s", abbrev, o.GetName()),
			Namespace: o.GetNamespace(),
			OwnerReferences: []metav1.OwnerReference{{
				Kind: kind,
				Name: o.GetName(),
				UID:  o.GetUID(),
			}},
		},
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

	return h
}
