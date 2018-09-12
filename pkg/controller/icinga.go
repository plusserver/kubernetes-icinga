package main

import (
	"fmt"
	"reflect"

	log "github.com/sirupsen/logrus"

	"github.com/Nexinto/go-icinga2-client/icinga2"

	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
)

func (c *Controller) HostGroupCreatedOrUpdated(hostgroup *icingav1.HostGroup) error {
	owner := fmt.Sprintf("%s/%s", hostgroup.Namespace, hostgroup.Name)
	log.Debugf("processing hostgroup '%s'", owner)
	newHg := icinga2.HostGroup{
		Name: c.Tag + empty(hostgroup.Spec.Name),
		Vars: Vars(mergeVars(c.DefaultVars, hostgroup.Spec.Vars, map[string]string{VarCluster: c.Tag, VarOwner: owner})),
	}
	hg, err := c.Icinga.GetHostGroup(newHg.Name)
	if err == nil {
		if hg.Vars[VarCluster] != c.Tag {
			return fmt.Errorf("cannot update hostgroup '%s': it is not managed by us ('%s')", newHg.Name, hg.Vars[VarCluster])
		}

		if hg.Name != newHg.Name || varsDiffer(hg.Vars, newHg.Vars) {
			log.Infof("updating icinga hostgroup '%s'", newHg.Name)
			err = c.Icinga.UpdateHostGroup(newHg)
			if err != nil {
				log.Errorf("error updating icinga hostgroup '%s': %s", newHg.Name, err.Error())
				MakeEvent(c.Kubernetes, hostgroup, err.Error(), "HostGroup", true)
			} else {
				MakeEvent(c.Kubernetes, hostgroup, "hostgroup updated", "Check", false)
			}
			return err
		}
	} else {
		log.Infof("creating icinga hostgroup '%s'", newHg.Name)
		err = c.Icinga.CreateHostGroup(newHg)
		if err != nil {
			log.Errorf("error creating icinga hostgroup '%s': %s", newHg.Name, err.Error())
			MakeEvent(c.Kubernetes, hostgroup, err.Error(), "HostGroup", true)
		} else {
			MakeEvent(c.Kubernetes, hostgroup, "hostgroup created", "Check", false)
		}
		return err
	}
	return nil
}

func (c *Controller) HostGroupDeleted(hostgroup *icingav1.HostGroup) error {
	log.Debugf("processing deleted hostgroup '%s/%s'", hostgroup.Namespace, hostgroup.Name)

	hgName := c.Tag + empty(hostgroup.Spec.Name)

	ohg, err := c.Icinga.GetHostGroup(hgName)
	if err != nil {
		return nil
	}

	if ohg.Vars[VarCluster] != c.Tag {
		log.Debugf("cannot delete hostgroup '%s': it is not managed by us ('%s')", ohg.Name, ohg.Vars[VarCluster])
	}

	log.Infof("deleting icinga hostgroup '%s'", hgName)
	err = c.Icinga.DeleteHostGroup(hgName)
	if err != nil {
		log.Errorf("error deleting icinga hostgroup '%s'", hgName, err.Error())
		return err
	}

	return nil
}

func (c *Controller) HostCreatedOrUpdated(host *icingav1.Host) error {
	owner := fmt.Sprintf("%s/%s", host.Namespace, host.Name)
	log.Debugf("processing host '%s'", owner)

	hostgroups := make([]string, len(host.Spec.Hostgroups))
	for i, hg := range host.Spec.Hostgroups {
		hostgroups[i] = c.Tag + empty(hg)
	}

	ih := icinga2.Host{
		Name:         c.Tag + empty(host.Spec.Name),
		Groups:       hostgroups,
		CheckCommand: host.Spec.CheckCommand,
		Vars:         Vars(mergeVars(c.DefaultVars, host.Spec.Vars, map[string]string{VarCluster: c.Tag, VarOwner: owner})),
	}

	if host.Spec.Notes != "" {
		ih.Notes = host.Spec.Notes
	}

	if host.Spec.NotesURL != "" {
		ih.NotesURL = host.Spec.NotesURL
	}

	oh, err := c.Icinga.GetHost(ih.Name)
	if err == nil {
		if oh.Vars[VarCluster] != c.Tag {
			return fmt.Errorf("cannot update host '%s': it is not managed by us ('%s')", ih.Name, oh.Vars[VarCluster])
		}

		if oh.Name != ih.Name ||
			(oh.CheckCommand != ih.CheckCommand && ih.CheckCommand != "") ||
			varsDiffer(oh.Vars, ih.Vars) ||
			!reflect.DeepEqual(oh.Groups, ih.Groups) ||
			oh.Notes != ih.Notes ||
			oh.NotesURL != ih.NotesURL {
			log.Infof("updating icinga host '%s'", ih.Name)
			err = c.Icinga.UpdateHost(ih)
			if err != nil {
				log.Errorf("error updating icinga host '%s': %s", ih.Name, err.Error())
				MakeEvent(c.Kubernetes, host, err.Error(), "Host", true)
			} else {
				MakeEvent(c.Kubernetes, host, "host updated", "Check", false)
			}
			return err
		}
	} else {
		log.Infof("creating icinga host '%s'", ih.Name)
		err = c.Icinga.CreateHost(ih)
		if err != nil {
			log.Errorf("error creating icinga host '%s': %s", ih.Name, err.Error())
			MakeEvent(c.Kubernetes, host, err.Error(), "Host", true)
		} else {
			MakeEvent(c.Kubernetes, host, "host created", "Check", false)
		}
		return err
	}
	return nil
}

func (c *Controller) HostDeleted(host *icingav1.Host) error {
	log.Debugf("processing deleted host '%s/%s'", host.Namespace, host.Name)

	hName := c.Tag + empty(host.Spec.Name)

	oh, err := c.Icinga.GetHost(hName)
	if err != nil {
		return nil
	}

	if oh.Vars[VarCluster] != c.Tag {
		log.Debugf("cannot delete host '%s': it is not managed by us ('%s')", oh.Name, oh.Vars[VarCluster])
	}

	log.Infof("deleting icinga host '%s'", hName)
	err = c.Icinga.DeleteHost(hName)
	if err != nil {
		log.Errorf("error deleting icinga host '%s'", hName, err.Error())
		return err
	}

	return nil
}

func (c *Controller) CheckCreatedOrUpdated(check *icingav1.Check) error {
	owner := fmt.Sprintf("%s/%s", check.Namespace, check.Name)
	log.Debugf("processing check '%s'", owner)
	name := c.Tag + "." + check.Spec.Host + "!" + check.Spec.Name

	nc := icinga2.Service{
		Name:         check.Spec.Name,
		HostName:     c.Tag + "." + check.Spec.Host,
		CheckCommand: check.Spec.CheckCommand,
		Notes:        check.Spec.Notes,
		NotesURL:     check.Spec.NotesURL,
		Vars:         Vars(mergeVars(c.DefaultVars, check.Spec.Vars, map[string]string{VarCluster: c.Tag, VarOwner: owner})),
	}

	oc, err := c.Icinga.GetService(name)
	if err == nil {
		if oc.CheckCommand != nc.CheckCommand ||
			oc.Notes != nc.Notes ||
			oc.NotesURL != nc.NotesURL ||
			varsDiffer(oc.Vars, nc.Vars) {
			log.Infof("updating icinga service '%s'", nc.Name)
			err = c.Icinga.UpdateService(nc)
			if err != nil {
				log.Errorf("error updating icinga service '%s': %s", nc.Name, err.Error())
				MakeEvent(c.Kubernetes, check, err.Error(), "Check", true)
			} else {
				MakeEvent(c.Kubernetes, check, "service updated", "Check", false)
			}
			return err
		}
	} else {
		log.Infof("creating icinga check '%s'", nc.Name)
		err = c.Icinga.CreateService(nc)
		if err != nil {
			MakeEvent(c.Kubernetes, check, err.Error(), "Check", true)
		} else {
			MakeEvent(c.Kubernetes, check, "service created", "Check", false)
		}
		return err
	}

	return nil
}

func (c *Controller) CheckDeleted(check *icingav1.Check) error {
	log.Debugf("processing deleted check '%s/%s'", check.Namespace, check.Name)
	name := c.Tag + "." + check.Spec.Host + "!" + check.Spec.Name

	oc, err := c.Icinga.GetService(name)
	if err != nil {
		return nil
	}

	if oc.Vars[VarCluster] != c.Tag {
		log.Debugf("cannot delete service '%s': it is not managed by us ('%s')", oc.Name, oc.Vars[VarCluster])
	}

	log.Infof("deleting icinga service '%s'", name)
	err = c.Icinga.DeleteService(name)
	if err != nil {
		log.Errorf("error deleting icinga host '%s'", name, err.Error())
		return err
	}

	return nil
}

func empty(a string) string {
	if a == EMPTY {
		return ""
	} else {
		return "." + a
	}
}
