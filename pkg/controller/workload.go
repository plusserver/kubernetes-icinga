package main

import (
	log "github.com/sirupsen/logrus"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) PodCreatedOrUpdated(pod *corev1.Pod) error {
	return c.processWorkload(pod, "po", "pod", "Pod", "v1")
}

func (c *Controller) PodDeleted(pod *corev1.Pod) error {
	log.Debugf("processing deleted pod '%s/%s'", pod.Namespace, pod.Name)
	return c.Mapping.UnmonitorWorkload(c, pod, "po")
}

func (c *Controller) NodeCreatedOrUpdated(node *corev1.Node) error {
	log.Debugf("processing node '%s'", node.Name)
	if node.GetDeletionTimestamp() != nil {
		return c.Mapping.UnmonitorNode(c, node)
	} else {
		return c.Mapping.MonitorNode(c, node)
	}
}

func (c *Controller) NodeDeleted(node *corev1.Node) error {
	log.Debugf("processing deleted node '%s'", node.Name)
	return c.Mapping.UnmonitorNode(c, node)
}

func (c *Controller) NamespaceCreatedOrUpdated(namespace *corev1.Namespace) error {
	log.Debugf("processing namespace '%s'", namespace.Name)
	if !c.monitored(namespace) {
		return c.Mapping.UnmonitorNamespace(c, namespace)
	} else if namespace.GetDeletionTimestamp() != nil {
		return c.Mapping.UnmonitorNamespace(c, namespace)
	} else {
		return c.Mapping.MonitorNamespace(c, namespace)
	}
}

func (c *Controller) NamespaceDeleted(namespace *corev1.Namespace) error {
	log.Debugf("processing deleted namespace '%s'", namespace.Name)
	return c.Mapping.UnmonitorNamespace(c, namespace)
}

func (c *Controller) DeploymentCreatedOrUpdated(deployment *appsv1.Deployment) error {
	return c.processWorkload(deployment, "deploy", "deployment", "Deployment", "v1")
}

func (c *Controller) DeploymentDeleted(deployment *appsv1.Deployment) error {
	log.Debugf("processing deleted deployment '%s/%s'", deployment.Namespace, deployment.Name)
	return c.Mapping.UnmonitorWorkload(c, deployment, "deploy")
}

func (c *Controller) DaemonSetCreatedOrUpdated(daemonset *appsv1.DaemonSet) error {
	return c.processWorkload(daemonset, "ds", "daemonset", "DaemonSet", "v1")
}

func (c *Controller) DaemonSetDeleted(daemonset *appsv1.DaemonSet) error {
	log.Debugf("processing deleted daemonset '%s/%s'", daemonset.Namespace, daemonset.Name)
	return c.Mapping.UnmonitorWorkload(c, daemonset, "ds")
}

func (c *Controller) ReplicaSetCreatedOrUpdated(replicaset *appsv1.ReplicaSet) error {
	return c.processWorkload(replicaset, "rs", "replicaset", "ReplicaSet", "v1")
}

func (c *Controller) ReplicaSetDeleted(replicaset *appsv1.ReplicaSet) error {
	log.Debugf("processing deleted replicaset '%s/%s'", replicaset.Namespace, replicaset.Name)
	return c.Mapping.UnmonitorWorkload(c, replicaset, "rs")
}

func (c *Controller) StatefulSetCreatedOrUpdated(statefulset *appsv1.StatefulSet) error {
	return c.processWorkload(statefulset, "statefulset", "statefulset", "StatefulSet", "v1")
}

func (c *Controller) StatefulSetDeleted(statefulset *appsv1.StatefulSet) error {
	log.Debugf("processing deleted statefulset '%s/%s'", statefulset.Namespace, statefulset.Name)
	return c.Mapping.UnmonitorWorkload(c, statefulset, "statefulset")
}

func (c *Controller) processWorkload(o metav1.Object, abbrev, typ, kind, apiVersion string) error {
	log.Debugf("processing %s '%s/%s'", typ, o.GetNamespace(), o.GetName())
	if !c.monitored(o) {
		return c.Mapping.UnmonitorWorkload(c, o, abbrev)
	} else if o.GetDeletionTimestamp() != nil {
		return c.Mapping.UnmonitorWorkload(c, o, abbrev)
	} else {
		return c.Mapping.MonitorWorkload(c, o, abbrev, typ, kind, apiVersion)
	}
}
