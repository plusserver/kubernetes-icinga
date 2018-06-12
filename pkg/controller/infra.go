package main

import (
	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"time"

	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
)

func (c *Controller) RefreshComponentStatutes() {
	for {
		// Have to do it manually as they cannot be watched.

		componentstatuses, err := c.Kubernetes.CoreV1().ComponentStatuses().List(metav1.ListOptions{})

		if err != nil {
			log.Errorf("error listing componentstatuses: %s", err.Error())
		} else {
			for _, cs := range componentstatuses.Items {
				err := c.reconcileHost(
					&icingav1.Host{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "cs-" + cs.Name,
							Namespace: "kube-system",
							OwnerReferences: []metav1.OwnerReference{{
								Kind:       "ComponentStatus",
								Name:       cs.GetName(),
								UID:        cs.GetUID(),
								APIVersion: "v1",
							}},
						},
						Spec: icingav1.HostSpec{
							Name:         "infrastructure.cs-" + cs.Name,
							Hostgroups:   []string{"infrastructure"},
							CheckCommand: "check_kubernetes",
							Vars: map[string]string{
								VarName:    cs.Name,
								VarType:    "componentstatus",
								VarCluster: c.Tag},
						},
					})
				if err != nil {
					log.Errorf("error listing componentstatuses: %s", err.Error())
				}
			}
		}

		time.Sleep(60 * time.Second)
	}
}

func (c *Controller) EnsureDefaultHostgroups() {
	for {
		kubeSystem, err := c.Kubernetes.CoreV1().Namespaces().Get("kube-system", metav1.GetOptions{})
		if err != nil {
			log.Errorf("error getting kube-system namespace: %s", err.Error())
			continue
		}

		for _, s := range []string{"infrastructure", "nodes"} {
			newHg := &icingav1.HostGroup{
				ObjectMeta: metav1.ObjectMeta{
					Name:      s,
					Namespace: "kube-system",
					OwnerReferences: []metav1.OwnerReference{{
						Kind:       "Namespace",
						Name:       "kube-system",
						UID:        kubeSystem.GetUID(),
						APIVersion: "v1",
					}},
				},
				Spec: icingav1.HostGroupSpec{
					Name: s,
					Vars: map[string]string{VarCluster: c.Tag},
				},
			}

			hg, err := c.IcingaClient.IcingaV1().HostGroups("kube-system").Get(s, metav1.GetOptions{})

			if err == nil {
				if hg.Spec.Name != newHg.Spec.Name {
					log.Infof("updating default hostgroup 'kube-system/%s'", s)
					_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Update(newHg)
					if err != nil {
						log.Errorf("error updating hostgroup 'kube-system/%s': %s", s, err.Error())
					}
				}
			} else if errors.IsNotFound(err) {
				log.Infof("creating default hostgroup 'kube-system/%s'", s)
				_, err = c.IcingaClient.IcingaV1().HostGroups("kube-system").Create(newHg)
				if err != nil {
					log.Errorf("error creating hostgroup 'kube-system/%s': %s", s, err.Error())
				}
			} else {
				log.Error(err)
			}
		}

		time.Sleep(60 * time.Second)
	}
}
