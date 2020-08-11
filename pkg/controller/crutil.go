package main

import (
	icingav1 "github.com/Soluto-Private/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"reflect"
)

func (c *Controller) reconcileHostGroup(hostgroup *icingav1.HostGroup) error {
	ohg, err := c.IcingaClient.IcingaV1().HostGroups(hostgroup.Namespace).Get(hostgroup.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(ohg.Spec, hostgroup.Spec) {
			hostgroup.Spec.DeepCopyInto(&ohg.Spec)
			log.Infof("updating hostgroup cr '%s/%s'", ohg.Namespace, ohg.Name)
			_, err := c.IcingaClient.IcingaV1().HostGroups(ohg.Namespace).Update(ohg)
			if err != nil {
				log.Errorf("error updating hostgroup cr '%s/%s': %s", ohg.Namespace, ohg.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating hostgroup cr '%s/%s'", hostgroup.Namespace, hostgroup.Name)
		_, err := c.IcingaClient.IcingaV1().HostGroups(hostgroup.Namespace).Create(hostgroup)
		if err != nil {
			log.Errorf("error creating hostgroup cr '%s/%s': %s", hostgroup.Namespace, hostgroup.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting hostgroup cr '%s/%s': %s", hostgroup.Namespace, hostgroup.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteHostGroup(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().HostGroups(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted hostgroup cr '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting hostgroup cr '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}

func (c *Controller) reconcileHost(host *icingav1.Host) error {
	oh, err := c.IcingaClient.IcingaV1().Hosts(host.Namespace).Get(host.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(oh.Spec, host.Spec) {
			host.Spec.DeepCopyInto(&oh.Spec)
			log.Infof("updating host cr '%s/%s'", oh.Namespace, oh.Name)
			_, err := c.IcingaClient.IcingaV1().Hosts(oh.Namespace).Update(oh)
			if err != nil {
				log.Errorf("error updating host cr '%s/%s': %s", oh.Namespace, oh.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating host cr '%s/%s'", host.Namespace, host.Name)
		_, err := c.IcingaClient.IcingaV1().Hosts(host.Namespace).Create(host)
		if err != nil {
			log.Errorf("error creating host cr '%s/%s': %s", host.Namespace, host.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting host cr '%s/%s': %s", host.Namespace, host.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteHost(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().Hosts(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted host cr '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting host cr '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}

func (c *Controller) reconcileCheck(check *icingav1.Check) error {
	oc, err := c.IcingaClient.IcingaV1().Checks(check.Namespace).Get(check.Name, metav1.GetOptions{})
	if err == nil {
		if !reflect.DeepEqual(check.Spec, oc.Spec) {
			check.Spec.DeepCopyInto(&oc.Spec)
			log.Infof("updating check cr '%s/%s'", oc.Namespace, oc.Name)
			_, err := c.IcingaClient.IcingaV1().Checks(oc.Namespace).Update(oc)
			if err != nil {
				log.Errorf("error updating check cr '%s/%s': %s", oc.Namespace, oc.Name, err.Error())
				return err
			}
		}
	} else if errors.IsNotFound(err) {
		log.Infof("creating check cr '%s/%s'", check.Namespace, check.Name)
		_, err := c.IcingaClient.IcingaV1().Checks(check.Namespace).Create(check)
		if err != nil {
			log.Errorf("error creating check cr '%s/%s': %s", check.Namespace, check.Name, err.Error())
			return err
		}
	} else {
		log.Errorf("error getting check cr '%s/%s': %s", check.Namespace, check.Name, err.Error())
		return err
	}

	return nil
}

func (c *Controller) deleteCheck(namespace, name string) error {
	err := c.IcingaClient.IcingaV1().Checks(namespace).Delete(name, &metav1.DeleteOptions{})
	if err == nil {
		log.Debugf("deleted check cr '%s/%s'", namespace, name)
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		log.Debugf("error deleting check cr '%s/%s': %s", namespace, name, err.Error())
		return err
	}
}
