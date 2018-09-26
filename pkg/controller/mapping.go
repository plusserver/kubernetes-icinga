package main

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Mapping interface {
	Name() string
	MonitorCluster(c *Controller) error
	MonitorNamespace(c *Controller, namespace *corev1.Namespace) error
	UnmonitorNamespace(c *Controller, namespace *corev1.Namespace) error
	MonitorNodesGroup(c *Controller) error
	MonitorInfrastructureGroup(c *Controller) error
	MonitorNode(c *Controller, node *corev1.Node) error
	UnmonitorNode(c *Controller, node *corev1.Node) error
	MonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error
	UnmonitorComponentStatus(c *Controller, cs *corev1.ComponentStatus) error
	MonitorWorkload(c *Controller, o metav1.Object, abbrev, typ, kind, apiVersion string) error
	UnmonitorWorkload(c *Controller, o metav1.Object, abbrev string) error
}
