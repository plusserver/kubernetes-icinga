package main

import (
	log "github.com/sirupsen/logrus"
	"reflect"

	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) reconcileHostGroup(hostgroup *icingav1.HostGroup) error {
	ohg, err := c.IcingaClient.IcingaV1().HostGroups(hostgroup.Namespace).Get(hostgroup.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(ohg.Spec, hostgroup.Spec) {
			hostgroup.Spec.DeepCopyInto(&ohg.Spec)
			log.Infof("updating hostgroup '%s/%s'", ohg.Namespace, ohg.Name)
			_, err := c.IcingaClient.IcingaV1().HostGroups(ohg.Namespace).Update(ohg)
			if err != nil {
				log.Errorf("error updating hostgroup '%s/%s': %s", ohg.Namespace, ohg.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating hostgroup '%s/%s'", hostgroup.Namespace, hostgroup.Name)
		_, err := c.IcingaClient.IcingaV1().HostGroups(hostgroup.Namespace).Create(hostgroup)
		if err != nil {
			log.Errorf("error creating hostgroup '%s/%s': %s", hostgroup.Namespace, hostgroup.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting hostgroup '%s/%s': %s", hostgroup.Namespace, hostgroup.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteHostGroup(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().HostGroups(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted hostgroup '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting hostgroup '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}

func (c *Controller) reconcileHost(host *icingav1.Host) error {
	oh, err := c.IcingaClient.IcingaV1().Hosts(host.Namespace).Get(host.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(oh.Spec, host.Spec) {
			host.Spec.DeepCopyInto(&oh.Spec)
			log.Infof("updating host '%s/%s'", oh.Namespace, oh.Name)
			_, err := c.IcingaClient.IcingaV1().Hosts(oh.Namespace).Update(oh)
			if err != nil {
				log.Errorf("error updating host '%s/%s': %s", oh.Namespace, oh.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating host '%s/%s'", host.Namespace, host.Name)
		_, err := c.IcingaClient.IcingaV1().Hosts(host.Namespace).Create(host)
		if err != nil {
			log.Errorf("error creating host '%s/%s': %s", host.Namespace, host.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting host '%s/%s': %s", host.Namespace, host.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteHost(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().Hosts(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted host '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting host '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}

func (c *Controller) reconcileCheck(check *icingav1.Check) error {
	oc, err := c.IcingaClient.IcingaV1().Checks(check.Namespace).Get(check.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(check.Spec, oc.Spec) {
			check.Spec.DeepCopyInto(&oc.Spec)
			log.Infof("updating check '%s/%s'", oc.Namespace, oc.Name)
			_, err := c.IcingaClient.IcingaV1().Checks(oc.Namespace).Update(oc)
			if err != nil {
				log.Errorf("error updating check '%s/%s': %s", oc.Namespace, oc.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating check '%s/%s'", check.Namespace, check.Name)
		_, err := c.IcingaClient.IcingaV1().Checks(check.Namespace).Create(check)
		if err != nil {
			log.Errorf("error creating check '%s/%s': %s", check.Namespace, check.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting check '%s/%s': %s", check.Namespace, check.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteCheck(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().Checks(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted check '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting check '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}
