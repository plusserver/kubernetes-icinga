# kubernetes-icinga

kubernetes-icinga uses Icinga2 to monitor Kubernetes workload and infrastructure.

## Overview

kubernetes-icinga watches infrastructure and workload deployed in a Kubernetes cluster and creates
resources in Icinga to monitor those. Each namespace is represented by a Icinga Hostgroup with two
additional Hostgroups created for Nodes and Infrastructure. Each infrastructure element and workload
is represented by an Icinga host with a check that monitors the state of the object through the
Kubernetes API.

For workload, state is monitored at the highest level. For example, with a Deployment, the
state of the Deployment is monitored, not the ReplicaSet or the individual Pods. A warning would
be triggered if some, but not all, Replicas are available; the service would become critical if
no Replicas are available. Basically, if a Workload resource is controlled by something else, it is
not monitored. So if you deploy a single Pod directly, it will be monitored.

Additional service checks can deployed using custom resources.

## Getting started

kubernetes-icinga requires a running Icinga2 instance and access to its API. `check_kubernetes`
(https://github.com/Nexinto/check_kubernetes) needs to be configured in Icinga2.

To run kubernetes-icinga in the cluster that is to be monitored:

* Create a configmap `kubernetes-icinga` in kube-system with the non-secret parameters.
  Use `deploy/configmap.yaml` as template. Important parameters are ICINGA_URL (point this to your Icinga2
  API URL) and TAG (set this to a value unique to your cluster, for example your cluster name).
  
  Create the configmap (`kubectl apply -f deploy/configmap.yaml`)

* Create a secret `kubernetes-icinga` with your Icinga API Username and password:

  ```bash
  kubectl create secret generic kubernetes-icinga \
  --from-literal=ICINGA_USER=... \
  --from-literal=ICINGA_PASSWORD=... \
  -n kube-system
  ```

* Add the custom resource definitions (`kubectl apply -f deploy/crd.yaml`)
  
* Review and create the RBAC configuration (`kubectl apply -f deploy/rbac.yaml`)

* Deploy the application (`kubectl apply -f deploy/deployment.yaml`)

If everything works, a number of hostgroups and hosts should now be created in your Icinga instance.

## Disabling monitoring

Resources can be excluded from monitoring by setting the annotion `icinga.nexinto.com/nomonitoring` on
the object to some string. Set this on a Namespace and all objects in that namespace aren't monitored.

## Adding Notes and Notes URL

Set the annotations `icinga.nexinto.com/notes` and `icinga.nexinto.com/notesurl` on any Kubernetes object
to create Notes and Notes URL fields in the corresponding Icinga Host.

## Custom resources

Icinga Hostgroups, Hosts and Checks ("Services") are represented by custom resources. See
`deploy/hostgroup.yaml`, `deploy/host.yaml` and `deploy/check.yaml` for examples. kubernetes-icinga
creates those resources for its checks, but you can add your own. For example, you can add
a HTTP check for a deployment "mysomething" in the default namespace by creating a Check and
setting the Hostname to `default/deploy-mysomething`. Note that all Icinga resources are prefixed
by the TAG parameter so multiple Kubernetes clusters can be monitored using a single Icinga instance
without naming conflicts.

## Configuring kubernetes-icinga

All configuration parameters are passed to the controller as environment variables:

| Variable | Description | Default |
|:-----|:------------|:--------|
|KUBECONFIG|your kubeconfig location (out of cluster only)||
|LOG_LEVEL|log level (debug, info, ...)|info|
|ICINGA_URL|URL of your Icinga API||
|ICINGA_USER|Icinga API user||
|ICINGA_PASSWORD|Icinga API user password||
|TAG|Unique name for your Cluster. Prefixes all Icinga resources created|kubernetes|
|ICINGA_DEBUG|Set to something to dump Icinga API requests/responses|""|
|DEFAULT_VARS|A YAML map with Icinga Vars to add|""|
