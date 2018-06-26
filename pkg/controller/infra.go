package main

import (
	log "github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"time"
)

func (c *Controller) RefreshComponentStatutes() {
	for {
		// Have to do it manually as they cannot be watched.

		componentstatuses, err := c.Kubernetes.CoreV1().ComponentStatuses().List(metav1.ListOptions{})

		if err != nil {
			log.Errorf("error listing componentstatuses: %s", err.Error())
		} else {
			for _, cs := range componentstatuses.Items {
				err := c.Mapping.MonitorComponentStatus(c, &cs)
				if err != nil {
					log.Errorf("error creating check for componentstatus '%s': %s", cs.Name, err.Error())
				}
			}
		}

		time.Sleep(60 * time.Second)
	}
}

func (c *Controller) EnsureDefaultHostgroups() {
	for {
		if err := c.Mapping.MonitorCluster(c); err != nil {
			log.Errorf("error setting up monitoring for the cluster: %s", err.Error())
		}

		if err := c.Mapping.MonitorNodesGroup(c); err != nil {
			log.Error(err)
		}

		if err := c.Mapping.MonitorInfrastructureGroup(c); err != nil {
			log.Error(err)
		}

		time.Sleep(60 * time.Second)
	}
}
