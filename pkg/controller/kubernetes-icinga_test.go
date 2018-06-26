package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

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

type KubernetesIcingaTestSuite struct {
	suite.Suite
	Controller   *Controller
	Mapping      Mapping
	GetContainer func(s *KubernetesIcingaTestSuite, name string) (icinga2.Object, error)
	GetCheckable func(s *KubernetesIcingaTestSuite, container, name string) (icinga2.Checkable, error)
}

func TestHostGroupMapping(t *testing.T) {
	suite.Run(t, &KubernetesIcingaTestSuite{
		Mapping: &HostGroupMapping{},
		GetContainer: func(s *KubernetesIcingaTestSuite, name string) (icinga2.Object, error) {
			return s.Controller.Icinga.GetHostGroup(name)
		},
		GetCheckable: func(s *KubernetesIcingaTestSuite, container, name string) (icinga2.Checkable, error) {
			return s.Controller.Icinga.GetHost(container + "." + name)
		},
	})
}

func TestHostMapping(t *testing.T) {
	suite.Run(t, &KubernetesIcingaTestSuite{
		Mapping: &HostMapping{},
		GetContainer: func(s *KubernetesIcingaTestSuite, name string) (icinga2.Object, error) {
			return s.Controller.Icinga.GetHost(name)
		},
		GetCheckable: func(s *KubernetesIcingaTestSuite, container, name string) (icinga2.Checkable, error) {
			return s.Controller.Icinga.GetService(container + "!" + name)
		},
	})
}

func (s *KubernetesIcingaTestSuite) SetupTest() {
	s.Controller = testEnvironment(s.Mapping)
}

// Create a test environment with some useful defaults.
func testEnvironment(mapping Mapping) *Controller {

	log.SetLevel(log.InfoLevel)

	c := &Controller{
		Kubernetes:   fake.NewSimpleClientset(),
		IcingaClient: icingafake.NewSimpleClientset(),
		Icinga:       icinga2.NewMockClient(),
		Tag:          "testing",
		Mapping:      mapping,
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

func (s *KubernetesIcingaTestSuite) TestDefaultCase() {
	a := assert.New(s.T())
	c := s.Controller

	c.Kubernetes.CoreV1().Nodes().Create(&corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node1"},
		Status: corev1.NodeStatus{
			Addresses: []corev1.NodeAddress{
				{Address: "10.100.11.1", Type: corev1.NodeInternalIP}}},
	})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	def, err := s.GetContainer(s, "testing.default")

	if !a.Nil(err) {
		return
	}
	if !a.NotNil(def) {
		return
	}

	a.Equal("testing", def.GetVars()[VarCluster])

	if _, err := s.GetContainer(s, "testing.infrastructure"); !a.Nil(err) {
		return
	}
	if _, err := s.GetContainer(s, "testing.nodes"); !a.Nil(err) {
		return
	}

	// Test the resulting icinga host
	node1, err := s.GetCheckable(s, "testing.nodes", "node1")
	if !a.Nil(err) {
		return
	}
	if !a.NotNil(node1) {
		return
	}
	a.Equal("check_kubernetes", node1.GetCheckCommand())
	a.Equal("testing", node1.GetVars()[VarCluster])
	a.Equal("node", node1.GetVars()[VarType])
	a.Equal("node1", node1.GetVars()[VarName])
	a.Equal("", node1.GetVars()[VarNamespace])
	a.Equal("kube-system/node1", node1.GetVars()[VarOwner])

	switch node1.(type) {
	case icinga2.Host:
		if !a.Equal(1, len(node1.(icinga2.Host).Groups)) {
			return
		}
		a.Equal("testing.nodes", node1.(icinga2.Host).Groups[0])
	}
}

func (s *KubernetesIcingaTestSuite) TestDoNotTouchSomeoneElsesHostgroup() {
	a := assert.New(s.T())
	c := s.Controller

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
	a.Equal("someone", hg.GetVars()[VarCluster])
}

func (s *KubernetesIcingaTestSuite) TestNamespace() {
	a := assert.New(s.T())
	c := s.Controller

	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "dev"}})

	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test"}})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	dev, err := s.GetContainer(s, "testing.dev")
	if !a.Nil(err) {
		return
	}
	a.Equal("testing", dev.GetVars()[VarCluster])
	a.Equal("namespace", dev.GetVars()[VarType])
	a.Equal("dev", dev.GetVars()[VarName])
	a.Equal("", dev.GetVars()[VarNamespace])
	a.Equal("kube-system/dev", dev.GetVars()[VarOwner])

	if _, err := s.GetContainer(s, "testing.test"); !a.Nil(err) {
		return
	}

	// Add prod, and we do not test any more
	c.Kubernetes.CoreV1().Namespaces().Create(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "prod"}})

	c.Kubernetes.CoreV1().Namespaces().Delete("test", &metav1.DeleteOptions{})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetContainer(s, "testing.test"); !a.NotNil(err) {
		return
	}
	if _, err := s.GetContainer(s, "testing.prod"); !a.Nil(err) {
		return
	}
}

// Standalone pods are monitored as icinga hosts.
func (s *KubernetesIcingaTestSuite) TestPod() {
	a := assert.New(s.T())
	c := s.Controller

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

	h, err := s.GetCheckable(s, "testing.default", "po-standalone")
	a.Nil(err)
	a.Equal("check_kubernetes", h.GetCheckCommand())
	a.Equal("testing", h.GetVars()[VarCluster])
	a.Equal("pod", h.GetVars()[VarType])
	a.Equal("default", h.GetVars()[VarNamespace])
	a.Equal("standalone", h.GetVars()[VarName])
	a.Equal("default/po-standalone", h.GetVars()[VarOwner])

	_, err = s.GetCheckable(s, "testing.default", "po-deployed")
	a.NotNil(err)

	// Add a second standalone pod
	c.Kubernetes.CoreV1().Pods("default").Create(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "standalone2"}})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	h, err = s.GetCheckable(s, "testing.default", "po-standalone2")
	a.Nil(err)
	a.Equal("check_kubernetes", h.GetCheckCommand())
	a.Equal("testing", h.GetVars()[VarCluster])
	a.Equal("pod", h.GetVars()[VarType])
	a.Equal("default", h.GetVars()[VarNamespace])
	a.Equal("standalone2", h.GetVars()[VarName])

	// Delete the first pod
	c.Kubernetes.CoreV1().Pods("default").Delete("standalone", &metav1.DeleteOptions{})

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "po-standalone"); a.NotNil(err) {
		return
	}
	if _, err := s.GetCheckable(s, "testing.default", "po-standalone2"); a.Nil(err) {
		return
	}
}

func (s *KubernetesIcingaTestSuite) TestDeployment() {
	a := assert.New(s.T())
	c := s.Controller

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

	if _, err := s.GetCheckable(s, "testing.default", "deploy-controlled"); !a.NotNil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "deploy-unmonitored"); !a.NotNil(err) {
		return
	}

	host, err := s.GetCheckable(s, "testing.default", "deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("a nice deployment", host.GetNotes())
	a.Equal("http://site.com/docs", host.GetNotesURL())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("mydeploy", host.GetVars()[VarName])
	a.Equal("deployment", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.GetVars()[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().Deployments("default").Delete("mydeploy", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "deploy-mydeploy"); !a.NotNil(err) {
		return
	}
}

func (s *KubernetesIcingaTestSuite) TestDaemonSet() {
	a := assert.New(s.T())
	c := s.Controller

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

	host, err := s.GetCheckable(s, "testing.default", "ds-myds")
	if !a.Nil(err) {
		return
	}
	//a.Equal("a nice deployment", host.GetNotes())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("myds", host.GetVars()[VarName])
	a.Equal("daemonset", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/ds-myds", host.GetVars()[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().DaemonSets("default").Delete("myds", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "ds-myds"); !a.NotNil(err) {
		return
	}
}

func (s *KubernetesIcingaTestSuite) TestStatefulSet() {
	a := assert.New(s.T())
	c := s.Controller

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

	host, err := s.GetCheckable(s, "testing.default", "statefulset-mystate")
	if !a.Nil(err) {
		return
	}
	//a.Equal("a nice deployment", host.GetNotes())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("mystate", host.GetVars()[VarName])
	a.Equal("statefulset", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/statefulset-mystate", host.GetVars()[VarOwner])

	if err := c.Kubernetes.AppsV1beta2().StatefulSets("default").Delete("mystate", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "statefulset-mystate"); !a.NotNil(err) {
		return
	}
}

func (s *KubernetesIcingaTestSuite) TestReplicaSet() {
	a := assert.New(s.T())
	c := s.Controller

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

	host, err := s.GetCheckable(s, "testing.default", "rs-myreplica")
	if !a.Nil(err) {
		return
	}

	a.Equal("a nice deployment", host.GetNotes())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("myreplica", host.GetVars()[VarName])
	a.Equal("replicaset", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/rs-myreplica", host.GetVars()[VarOwner])

	if err := c.Kubernetes.ExtensionsV1beta1().ReplicaSets("default").Delete("myreplica", &metav1.DeleteOptions{}); !a.Nil(err) {
		return
	}

	if err := c.simulate(); !a.Nil(err) {
		return
	}

	if _, err := s.GetCheckable(s, "testing.default", "rs-myreplica"); !a.NotNil(err) {
		return
	}
}

func (s *KubernetesIcingaTestSuite) TestChangeNotes() {
	a := assert.New(s.T())
	c := s.Controller

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

	host, err := s.GetCheckable(s, "testing.default", "deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("a nice deployment", host.GetNotes())
	a.Equal("http://site.com/docs", host.GetNotesURL())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("mydeploy", host.GetVars()[VarName])
	a.Equal("deployment", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.GetVars()[VarOwner])

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

	host, err = s.GetCheckable(s, "testing.default", "deploy-mydeploy")
	if !a.Nil(err) {
		return
	}
	a.Equal("an even nicer deployment", host.GetNotes())
	a.Equal("http://site.com/docsv2", host.GetNotesURL())
	a.NotNil(host.GetVars())
	a.Equal("check_kubernetes", host.GetCheckCommand())
	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("mydeploy", host.GetVars()[VarName])
	a.Equal("deployment", host.GetVars()[VarType])
	a.Equal("default", host.GetVars()[VarNamespace])
	a.Equal("default/deploy-mydeploy", host.GetVars()[VarOwner])
}

func (s *KubernetesIcingaTestSuite) TestUnmonitoredNamespace() {
	a := assert.New(s.T())
	c := s.Controller

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

	_, err = s.GetContainer(s, "testing.develop")
	a.Error(err)

	_, err = s.GetCheckable(s, "testing.develop", "po-standalone")
	a.Error(err)
}

func (s *KubernetesIcingaTestSuite) TestCustomCheck() {
	a := assert.New(s.T())
	c := s.Controller

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

	a.Equal("testing", hostgroup.GetVars()[VarCluster])
	a.Equal("something", hostgroup.GetVars()["myvar"])
	a.Equal("default/myhostgroup", hostgroup.GetVars()[VarOwner])
	a.Empty(hostgroup.GetVars()[VarType])
	a.Empty(hostgroup.GetVars()[VarNamespace])
	a.Empty(hostgroup.GetVars()[VarName])

	host, err := c.Icinga.GetHost("testing.myhost")
	if !a.Nil(err) {
		return
	}

	a.Equal("testing", host.GetVars()[VarCluster])
	a.Equal("nicevar", host.GetVars()["myanothervar"])
	a.Equal("default/myhost", host.GetVars()[VarOwner])
	a.Empty(host.GetVars()[VarType])
	a.Empty(host.GetVars()[VarNamespace])
	a.Empty(host.GetVars()[VarName])

	check, err := c.Icinga.GetService("testing.myhost!http-check")
	if !a.Nil(err) {
		return
	}

	a.Equal("testing", check.GetVars()[VarCluster])
	a.Equal("www.mysite.com", check.GetVars()["http_address"])
	a.Equal("/health", check.GetVars()["http_uri"])
	a.Equal("default/http-check", check.GetVars()[VarOwner])
	a.Empty(check.GetVars()[VarType])
	a.Empty(check.GetVars()[VarNamespace])
	a.Empty(check.GetVars()[VarName])

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

	if _, err := s.GetContainer(s, "testing.myhostgroup"); !a.Error(err) {
		return
	}

	if _, err := c.Icinga.GetHost("testing.myhost"); !a.Error(err) {
		return
	}

	if _, err := c.Icinga.GetService("testing.myhost!http-check"); !a.Error(err) {
		return
	}
}
