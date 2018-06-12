package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	log "github.com/sirupsen/logrus"

	"github.com/Nexinto/go-icinga2-client/icinga2"

	appsv1beta2 "k8s.io/api/apps/v1beta2"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	icingafake "github.com/Nexinto/kubernetes-icinga/pkg/client/clientset/versioned/fake"
)

// Create a test environment with some useful defaults.
func testEnvironment() *Controller {

	log.SetLevel(log.InfoLevel)

	c := &Controller{
		Kubernetes:   fake.NewSimpleClientset(),
		IcingaClient: icingafake.NewSimpleClientset(),
		Icinga:       icinga2.NewMockClient(),
		Tag:          "testing",
	}

	c.Kubernetes.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}})
	c.Kubernetes.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}})
	c.Kubernetes.CoreV1().Namespaces().Create(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-public"}})

	c.Initialize()
	go c.Start()

	stopCh := make(chan struct{})

	go c.Run(stopCh)

	log.Debug("waiting for cache sync")

	if !cache.WaitForCacheSync(stopCh, c.PodSynced, c.NodeSynced, c.NamespaceSynced, c.DeploymentSynced, c.DaemonSetSynced, c.ReplicaSetSynced, c.StatefulSetSynced, c.HostGroupSynced, c.HostSynced, c.CheckSynced) {
		panic("Timed out waiting for caches to sync")
	}

	go c.RefreshComponentStatutes()
	go c.EnsureDefaultHostgroups()

	return c
}

// simulate the behaviour of the controllers we depend on
func (c *Controller) simulate() error {

	// This isn't what it looks like.
	time.Sleep(2 * time.Second)

	return nil
}

func TestDefaultCase(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	c.Kubernetes.CoreV1().Nodes().Create(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Address: "10.100.11.1", Type: corev1.NodeInternalIP}}},
	})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	hg, err := c.Icinga.GetHostGroup("testing.default")
	if !a.Nil(err) {
		return
	}
	if !a.NotNil(hg) {
		return
	}

	a.Equal("testing", hg.Vars[VarCluster])

	if _, err := c.Icinga.GetHostGroup("testing.infrastructure"); !a.Nil(err) {
		return
	}
	if _, err := c.Icinga.GetHostGroup("testing.nodes"); !a.Nil(err) {
		return
	}

	// Test the intermediate custom resource
	n, err := c.IcingaClient.IcingaV1().Hosts("kube-system").Get("node1", metav1.GetOptions{})
	if !a.Nil(err) {
		return
	}
	a.Equal("node1", n.Name)
	a.Equal("nodes.node1", n.Spec.Name)
	a.Equal([]string{"nodes"}, n.Spec.Hostgroups)
	a.Equal("check_kubernetes", n.Spec.CheckCommand)
	a.Equal(1, len(n.OwnerReferences))

	// Test the resulting icinga host
	node1, err := c.Icinga.GetHost("testing.nodes.node1")
	if !a.Nil(err) {
		return
	}
	if !a.NotNil(node1) {
		return
	}
	a.Equal("check_kubernetes", node1.CheckCommand)
	a.Equal("testing", node1.Vars[VarCluster])
	a.Equal("node", node1.Vars[VarType])
	a.Equal("node1", node1.Vars[VarName])
	a.Equal("", node1.Vars[VarNamespace])
	a.Equal("kube-system/node1", node1.Vars[VarOwner])
	if !a.Equal(1, len(node1.Groups)) {
		return
	}
	a.Equal("testing.nodes", node1.Groups[0])
}

func TestDoNotTouchSomeoneElsesHostgroup(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	err := c.Icinga.CreateHostGroup(icinga2.HostGroup{Name: "testing.default", Vars: icinga2.Vars{VarCluster: "someone"}})
	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	hg, err := c.Icinga.GetHostGroup("testing.default")
	if !a.Nil(err) {
		return
	}
	a.Equal("someone", hg.Vars[VarCluster])
}

func TestNamespace(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev"}})

	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	dev, err := c.Icinga.GetHostGroup("testing.dev")
	if !a.Nil(err) {
		return
	}
	a.Equal("testing", dev.Vars[VarCluster])
	a.Equal("namespace", dev.Vars[VarType])
	a.Equal("dev", dev.Vars[VarName])
	a.Equal("", dev.Vars[VarNamespace])
	a.Equal("kube-system/dev", dev.Vars[VarOwner])

	if _, err := c.Icinga.GetHostGroup("testing.test"); !a.Nil(err) {
		return
	}

	// Add prod, and we do not test any more
	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "prod"}})

	c.Kubernetes.CoreV1().Namespaces().Delete("test", &metav1.DeleteOptions{})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHostGroup("testing.test"); !a.NotNil(err) {
		return
	}
	if _, err := c.Icinga.GetHostGroup("testing.prod"); !a.Nil(err) {
		return
	}
}

// Standalone pods are monitored as icinga hosts.
func TestPod(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	// Create a standalone pod and a pod that looks like it was created by a Deployment.
	c.Kubernetes.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone"}})

	c.Kubernetes.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployed",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "Invisible Deployment",
			}},
		}})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	h, err := c.Icinga.GetHost("testing.default.po-standalone")
	a.Nil(err)
	a.Equal("check_kubernetes", h.CheckCommand)
	a.Equal("testing", h.Vars[VarCluster])
	a.Equal("pod", h.Vars[VarType])
	a.Equal("default", h.Vars[VarNamespace])
	a.Equal("standalone", h.Vars[VarName])
	a.Equal("default/po-standalone", h.Vars[VarOwner])

	_, err = c.Icinga.GetHost("testing.default.po-deployed")
	a.NotNil(err)

	// Add a second standalone pod
	c.Kubernetes.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone2"}})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	h, err = c.Icinga.GetHost("testing.default.po-standalone2")
	a.Nil(err)
	a.Equal("check_kubernetes", h.CheckCommand)
	a.Equal("testing", h.Vars[VarCluster])
	a.Equal("pod", h.Vars[VarType])
	a.Equal("default", h.Vars[VarNamespace])
	a.Equal("standalone2", h.Vars[VarName])

	// Delete the first pod
	c.Kubernetes.CoreV1().Pods("default").Delete("standalone", &metav1.DeleteOptions{})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.po-standalone"); a.NotNil(err) {
		return
	}
	if _, err := c.Icinga.GetHost("testing.default.po-standalone2"); a.Nil(err) {
		return
	}
}

func TestDeployment(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.ExtensionsV1beta1().Deployments("default").Create(&extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mydeploy",
			Annotations: map[string]string{
				AnnNotes:    "a nice deployment",
				AnnNotesURL: "http://site.com/docs",
			},
		},
	})

	if !a.Nil(err) {
		return
	}

	_, err = c.Kubernetes.ExtensionsV1beta1().Deployments("default").Create(&extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "controlled",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Something",
				Name: "an-obscure-controller",
			}},
		},
	})

	if !a.Nil(err) {
		return
	}

	_, err = c.Kubernetes.ExtensionsV1beta1().Deployments("default").Create(&extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "unmonitored",
			Annotations: map[string]string{
				AnnDisableMonitoring: "true",
			},
		},
	})

	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.deploy-controlled"); !a.NotNil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.deploy-unmonitored"); !a.NotNil(err) {
		return
	}

	host, err := c.Icinga.GetHost("testing.default.deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("a nice deployment", host.Notes)
	a.Equal("http://site.com/docs", host.NotesURL)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("mydeploy", host.Vars[VarName])
	a.Equal("deployment", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.Vars[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().Deployments("default").Delete("mydeploy", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.deploy-mydeploy"); !a.NotNil(err) {
		return
	}
}

func TestDaemonSet(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.ExtensionsV1beta1().DaemonSets("default").Create(&extensionsv1beta1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myds",
		},
	})

	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	host, err := c.Icinga.GetHost("testing.default.ds-myds")
	if !a.Nil(err) {
		return
	}
	//a.Equal("a nice deployment", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("myds", host.Vars[VarName])
	a.Equal("daemonset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/ds-myds", host.Vars[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().DaemonSets("default").Delete("myds", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.ds-myds"); !a.NotNil(err) {
		return
	}
}

func TestStatefulSet(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.AppsV1beta2().StatefulSets("default").Create(&appsv1beta2.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mystate",
		},
	})

	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	host, err := c.Icinga.GetHost("testing.default.statefulset-mystate")
	if !a.Nil(err) {
		return
	}
	//a.Equal("a nice deployment", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("mystate", host.Vars[VarName])
	a.Equal("statefulset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/statefulset-mystate", host.Vars[VarOwner])

	if err := c.Kubernetes.AppsV1beta2().StatefulSets("default").Delete("mystate", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.statefulset-mystate"); !a.NotNil(err) {
		return
	}
}

func TestReplicaSet(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.ExtensionsV1beta1().ReplicaSets("default").Create(&extensionsv1beta1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "myreplica",
			Annotations: map[string]string{
				AnnNotes: "a nice deployment",
			},
		},
	})

	if !a.Nil(err) {
		return
	}

	_, err = c.Kubernetes.ExtensionsV1beta1().ReplicaSets("default").Create(&extensionsv1beta1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "controlled",
			OwnerReferences: []metav1.OwnerReference{{
				Kind: "Deployment",
				Name: "Invisible Deployment",
			}},
		},
	})

	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	host, err := c.Icinga.GetHost("testing.default.rs-myreplica")
	if !a.Nil(err) {
		return
	}

	a.Equal("a nice deployment", host.Notes)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("myreplica", host.Vars[VarName])
	a.Equal("replicaset", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/rs-myreplica", host.Vars[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().ReplicaSets("default").Delete("myreplica", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.default.rs-myreplica"); !a.NotNil(err) {
		return
	}
}

func TestChangeNotes(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.ExtensionsV1beta1().Deployments("default").Create(&extensionsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mydeploy",
			Annotations: map[string]string{
				AnnNotes:    "a nice deployment",
				AnnNotesURL: "http://site.com/docs",
			},
		},
	})

	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	host, err := c.Icinga.GetHost("testing.default.deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("a nice deployment", host.Notes)
	a.Equal("http://site.com/docs", host.NotesURL)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("mydeploy", host.Vars[VarName])
	a.Equal("deployment", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.Vars[VarOwner])

	d, err := c.DeploymentLister.Deployments("default").Get("mydeploy")
	if !a.Nil(err) {
		return
	}

	d = d.DeepCopy()

	d.Annotations[AnnNotes] = "an even nicer deployment"
	d.Annotations[AnnNotesURL] = "http://site.com/docsv2"
	_, err = c.Kubernetes.ExtensionsV1beta1().Deployments("default").Update(d)
	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	host, err = c.Icinga.GetHost("testing.default.deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("an even nicer deployment", host.Notes)
	a.Equal("http://site.com/docsv2", host.NotesURL)
	a.NotNil(host.Vars)
	a.Equal("check_kubernetes", host.CheckCommand)
	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("mydeploy", host.Vars[VarName])
	a.Equal("deployment", host.Vars[VarType])
	a.Equal("default", host.Vars[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.Vars[VarOwner])
}

func TestUnmonitoredNamespace(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.Kubernetes.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "develop",
			Annotations: map[string]string{AnnDisableMonitoring: "true"},
		}})
	if !a.Nil(err) {
		return
	}

	_, err = c.Kubernetes.CoreV1().Pods("develop").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone"}})
	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	_, err = c.Icinga.GetHostGroup("testing.develop")
	a.Error(err)

	_, err = c.Icinga.GetHost("testing.develop.po-standalone")
	a.Error(err)
}

func TestCustomCheck(t *testing.T) {
	a := assert.New(t)
	c := testEnvironment()

	_, err := c.IcingaClient.IcingaV1().HostGroups("default").Create(&icingav1.HostGroup{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhostgroup",
			Namespace: "default",
		},
		Spec: icingav1.HostGroupSpec{
			Name: "myhostgroup",
			Vars: map[string]string{"myvar": "something"},
		},
	})
	if !a.Nil(err) {
		return
	}

	_, err = c.IcingaClient.IcingaV1().Hosts("default").Create(&icingav1.Host{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "myhost",
			Namespace: "default",
		},
		Spec: icingav1.HostSpec{
			Name:       "myhost",
			Hostgroups: []string{"myhostgroup"},
			Vars:       map[string]string{"myanothervar": "nicevar"},
		},
	})
	if !a.Nil(err) {
		return
	}

	_, err = c.IcingaClient.IcingaV1().Checks("default").Create(&icingav1.Check{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "http-check",
			Namespace: "default",
		},
		Spec: icingav1.CheckSpec{
			Name:         "http-check",
			Host:         "myhost",
			CheckCommand: "check_http",
			Vars:         map[string]string{"http_address": "www.mysite.com", "http_uri": "/health"},
		},
	})
	if !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	hostgroup, err := c.Icinga.GetHostGroup("testing.myhostgroup")
	if !a.Nil(err) {
		return
	}

	a.Equal("testing", hostgroup.Vars[VarCluster])
	a.Equal("something", hostgroup.Vars["myvar"])
	a.Equal("default/myhostgroup", hostgroup.Vars[VarOwner])
	a.Empty(hostgroup.Vars[VarType])
	a.Empty(hostgroup.Vars[VarNamespace])
	a.Empty(hostgroup.Vars[VarName])

	host, err := c.Icinga.GetHost("testing.myhost")
	if !a.Nil(err) {
		return
	}

	a.Equal("testing", host.Vars[VarCluster])
	a.Equal("nicevar", host.Vars["myanothervar"])
	a.Equal("default/myhost", host.Vars[VarOwner])
	a.Empty(host.Vars[VarType])
	a.Empty(host.Vars[VarNamespace])
	a.Empty(host.Vars[VarName])

	check, err := c.Icinga.GetService("testing.myhost!http-check")
	if !a.Nil(err) {
		return
	}

	a.Equal("testing", check.Vars[VarCluster])
	a.Equal("www.mysite.com", check.Vars["http_address"])
	a.Equal("/health", check.Vars["http_uri"])
	a.Equal("default/http-check", check.Vars[VarOwner])
	a.Empty(check.Vars[VarType])
	a.Empty(check.Vars[VarNamespace])
	a.Empty(check.Vars[VarName])

	// Delete everything

	if err := c.IcingaClient.IcingaV1().Checks("default").Delete("http-check", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.IcingaClient.IcingaV1().Hosts("default").Delete("myhost", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.IcingaClient.IcingaV1().HostGroups("default").Delete("myhostgroup", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	go c.IcingaHousekeeping() // we do not want to wait...

	time.Sleep(2 * time.Second)

	if _, err := c.Icinga.GetHostGroup("testing.myhostgroup"); !a.Error(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.myhost"); !a.Error(err) {
		return
	}

	if _, err := c.Icinga.GetService("testing.myhost!http-check"); !a.Error(err) {
		return
	}
}
