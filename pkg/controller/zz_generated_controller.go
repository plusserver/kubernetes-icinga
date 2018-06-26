package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubernetesinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	corev1 "k8s.io/api/core/v1"

	corelisterv1 "k8s.io/client-go/listers/core/v1"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"

	extensionslisterv1beta1 "k8s.io/client-go/listers/extensions/v1beta1"

	appsv1beta2 "k8s.io/api/apps/v1beta2"

	appslisterv1beta2 "k8s.io/client-go/listers/apps/v1beta2"

	icingaclientset "github.com/Nexinto/kubernetes-icinga/pkg/client/clientset/versioned"

	"github.com/Nexinto/go-icinga2-client/icinga2"
	icingav1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	icingainformers "github.com/Nexinto/kubernetes-icinga/pkg/client/informers/externalversions"
	icingalisterv1 "github.com/Nexinto/kubernetes-icinga/pkg/client/listers/icinga.nexinto.com/v1"
)

type Controller struct {
	Kubernetes        kubernetes.Interface
	KubernetesFactory kubernetesinformers.SharedInformerFactory

	PodQueue  workqueue.RateLimitingInterface
	PodLister corelisterv1.PodLister
	PodSynced cache.InformerSynced

	NodeQueue  workqueue.RateLimitingInterface
	NodeLister corelisterv1.NodeLister
	NodeSynced cache.InformerSynced

	NamespaceQueue  workqueue.RateLimitingInterface
	NamespaceLister corelisterv1.NamespaceLister
	NamespaceSynced cache.InformerSynced

	DeploymentQueue  workqueue.RateLimitingInterface
	DeploymentLister extensionslisterv1beta1.DeploymentLister
	DeploymentSynced cache.InformerSynced

	DaemonSetQueue  workqueue.RateLimitingInterface
	DaemonSetLister extensionslisterv1beta1.DaemonSetLister
	DaemonSetSynced cache.InformerSynced

	ReplicaSetQueue  workqueue.RateLimitingInterface
	ReplicaSetLister extensionslisterv1beta1.ReplicaSetLister
	ReplicaSetSynced cache.InformerSynced

	StatefulSetQueue  workqueue.RateLimitingInterface
	StatefulSetLister appslisterv1beta2.StatefulSetLister
	StatefulSetSynced cache.InformerSynced

	IcingaClient  icingaclientset.Interface
	IcingaFactory icingainformers.SharedInformerFactory

	HostGroupQueue  workqueue.RateLimitingInterface
	HostGroupLister icingalisterv1.HostGroupLister
	HostGroupSynced cache.InformerSynced

	HostQueue  workqueue.RateLimitingInterface
	HostLister icingalisterv1.HostLister
	HostSynced cache.InformerSynced

	CheckQueue  workqueue.RateLimitingInterface
	CheckLister icingalisterv1.CheckLister
	CheckSynced cache.InformerSynced

	Icinga      icinga2.Client
	Tag         string
	DefaultVars map[string]string
	Mapping     Mapping
}

// Expects the clientsets to be set.
func (c *Controller) Initialize() {

	if c.Kubernetes == nil {
		panic("c.Kubernetes is nil")
	}
	c.KubernetesFactory = kubernetesinformers.NewSharedInformerFactory(c.Kubernetes, time.Second*300)

	PodInformer := c.KubernetesFactory.Core().V1().Pods()
	PodQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.PodQueue = PodQueue
	c.PodLister = PodInformer.Lister()
	c.PodSynced = PodInformer.Informer().HasSynced

	PodInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				PodQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				PodQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*corev1.Pod)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*corev1.Pod)
				if !ok {
					log.Errorf("tombstone contained object that is not a Pod %+v", obj)
					return
				}
			}

			err := c.PodDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	NodeInformer := c.KubernetesFactory.Core().V1().Nodes()
	NodeQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.NodeQueue = NodeQueue
	c.NodeLister = NodeInformer.Lister()
	c.NodeSynced = NodeInformer.Informer().HasSynced

	NodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				NodeQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				NodeQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*corev1.Node)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*corev1.Node)
				if !ok {
					log.Errorf("tombstone contained object that is not a Node %+v", obj)
					return
				}
			}

			err := c.NodeDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	NamespaceInformer := c.KubernetesFactory.Core().V1().Namespaces()
	NamespaceQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.NamespaceQueue = NamespaceQueue
	c.NamespaceLister = NamespaceInformer.Lister()
	c.NamespaceSynced = NamespaceInformer.Informer().HasSynced

	NamespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				NamespaceQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				NamespaceQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*corev1.Namespace)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*corev1.Namespace)
				if !ok {
					log.Errorf("tombstone contained object that is not a Namespace %+v", obj)
					return
				}
			}

			err := c.NamespaceDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	DeploymentInformer := c.KubernetesFactory.Extensions().V1beta1().Deployments()
	DeploymentQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.DeploymentQueue = DeploymentQueue
	c.DeploymentLister = DeploymentInformer.Lister()
	c.DeploymentSynced = DeploymentInformer.Informer().HasSynced

	DeploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				DeploymentQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				DeploymentQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*extensionsv1beta1.Deployment)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*extensionsv1beta1.Deployment)
				if !ok {
					log.Errorf("tombstone contained object that is not a Deployment %+v", obj)
					return
				}
			}

			err := c.DeploymentDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	DaemonSetInformer := c.KubernetesFactory.Extensions().V1beta1().DaemonSets()
	DaemonSetQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.DaemonSetQueue = DaemonSetQueue
	c.DaemonSetLister = DaemonSetInformer.Lister()
	c.DaemonSetSynced = DaemonSetInformer.Informer().HasSynced

	DaemonSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				DaemonSetQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				DaemonSetQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*extensionsv1beta1.DaemonSet)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*extensionsv1beta1.DaemonSet)
				if !ok {
					log.Errorf("tombstone contained object that is not a DaemonSet %+v", obj)
					return
				}
			}

			err := c.DaemonSetDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	ReplicaSetInformer := c.KubernetesFactory.Extensions().V1beta1().ReplicaSets()
	ReplicaSetQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.ReplicaSetQueue = ReplicaSetQueue
	c.ReplicaSetLister = ReplicaSetInformer.Lister()
	c.ReplicaSetSynced = ReplicaSetInformer.Informer().HasSynced

	ReplicaSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				ReplicaSetQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				ReplicaSetQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*extensionsv1beta1.ReplicaSet)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*extensionsv1beta1.ReplicaSet)
				if !ok {
					log.Errorf("tombstone contained object that is not a ReplicaSet %+v", obj)
					return
				}
			}

			err := c.ReplicaSetDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	StatefulSetInformer := c.KubernetesFactory.Apps().V1beta2().StatefulSets()
	StatefulSetQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.StatefulSetQueue = StatefulSetQueue
	c.StatefulSetLister = StatefulSetInformer.Lister()
	c.StatefulSetSynced = StatefulSetInformer.Informer().HasSynced

	StatefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				StatefulSetQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				StatefulSetQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*appsv1beta2.StatefulSet)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*appsv1beta2.StatefulSet)
				if !ok {
					log.Errorf("tombstone contained object that is not a StatefulSet %+v", obj)
					return
				}
			}

			err := c.StatefulSetDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	if c.IcingaClient == nil {
		panic("c.IcingaClient is nil")
	}
	c.IcingaFactory = icingainformers.NewSharedInformerFactory(c.IcingaClient, time.Second*300)

	HostGroupInformer := c.IcingaFactory.Icinga().V1().HostGroups()
	HostGroupQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.HostGroupQueue = HostGroupQueue
	c.HostGroupLister = HostGroupInformer.Lister()
	c.HostGroupSynced = HostGroupInformer.Informer().HasSynced

	HostGroupInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				HostGroupQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				HostGroupQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*icingav1.HostGroup)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*icingav1.HostGroup)
				if !ok {
					log.Errorf("tombstone contained object that is not a HostGroup %+v", obj)
					return
				}
			}

			err := c.HostGroupDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	HostInformer := c.IcingaFactory.Icinga().V1().Hosts()
	HostQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.HostQueue = HostQueue
	c.HostLister = HostInformer.Lister()
	c.HostSynced = HostInformer.Informer().HasSynced

	HostInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				HostQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				HostQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*icingav1.Host)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*icingav1.Host)
				if !ok {
					log.Errorf("tombstone contained object that is not a Host %+v", obj)
					return
				}
			}

			err := c.HostDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	CheckInformer := c.IcingaFactory.Icinga().V1().Checks()
	CheckQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	c.CheckQueue = CheckQueue
	c.CheckLister = CheckInformer.Lister()
	c.CheckSynced = CheckInformer.Informer().HasSynced

	CheckInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				CheckQueue.Add(key)
			}
		},

		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				CheckQueue.Add(key)
			}
		},

		DeleteFunc: func(obj interface{}) {
			o, ok := obj.(*icingav1.Check)

			if !ok {
				tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
				if !ok {
					log.Errorf("couldn't get object from tombstone %+v", obj)
					return
				}
				o, ok = tombstone.Obj.(*icingav1.Check)
				if !ok {
					log.Errorf("tombstone contained object that is not a Check %+v", obj)
					return
				}
			}

			err := c.CheckDeleted(o)

			if err != nil {
				log.Errorf("failed to process deletion: %s", err.Error())
			}
		},
	})

	return
}

func (c *Controller) Start() {
	stopCh := make(chan struct{})
	defer close(stopCh)
	go c.KubernetesFactory.Start(stopCh)
	go c.IcingaFactory.Start(stopCh)

	go c.Run(stopCh)

	sigterm := make(chan os.Signal, 1)
	signal.Notify(sigterm, syscall.SIGTERM)
	signal.Notify(sigterm, syscall.SIGINT)
	<-sigterm
}

func (c *Controller) Run(stopCh <-chan struct{}) {

	log.Infof("starting controller")

	defer runtime.HandleCrash()

	defer c.PodQueue.ShutDown()
	defer c.NodeQueue.ShutDown()
	defer c.NamespaceQueue.ShutDown()
	defer c.DeploymentQueue.ShutDown()
	defer c.DaemonSetQueue.ShutDown()
	defer c.ReplicaSetQueue.ShutDown()
	defer c.StatefulSetQueue.ShutDown()
	defer c.HostGroupQueue.ShutDown()
	defer c.HostQueue.ShutDown()
	defer c.CheckQueue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, c.PodSynced, c.NodeSynced, c.NamespaceSynced, c.DeploymentSynced, c.DaemonSetSynced, c.ReplicaSetSynced, c.StatefulSetSynced, c.HostGroupSynced, c.HostSynced, c.CheckSynced) {
		runtime.HandleError(fmt.Errorf("Timed out waiting for caches to sync"))
		return
	}

	log.Debugf("starting workers")

	go wait.Until(c.runPodWorker, time.Second, stopCh)

	go wait.Until(c.runNodeWorker, time.Second, stopCh)

	go wait.Until(c.runNamespaceWorker, time.Second, stopCh)

	go wait.Until(c.runDeploymentWorker, time.Second, stopCh)

	go wait.Until(c.runDaemonSetWorker, time.Second, stopCh)

	go wait.Until(c.runReplicaSetWorker, time.Second, stopCh)

	go wait.Until(c.runStatefulSetWorker, time.Second, stopCh)

	go wait.Until(c.runHostGroupWorker, time.Second, stopCh)

	go wait.Until(c.runHostWorker, time.Second, stopCh)

	go wait.Until(c.runCheckWorker, time.Second, stopCh)

	log.Debugf("started workers")
	<-stopCh
	log.Debugf("shutting down workers")
}

func (c *Controller) runPodWorker() {
	for c.processNextPod() {
	}
}

func (c *Controller) processNextPod() bool {
	obj, shutdown := c.PodQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.PodQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.PodQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processPod(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.PodQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processPod(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.PodLister.Pods(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.PodCreatedOrUpdated(o)

}

func (c *Controller) runNodeWorker() {
	for c.processNextNode() {
	}
}

func (c *Controller) processNextNode() bool {
	obj, shutdown := c.NodeQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.NodeQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.NodeQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processNode(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.NodeQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processNode(key string) error {

	name := key

	o, err := c.NodeLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.NodeCreatedOrUpdated(o)

}

func (c *Controller) runNamespaceWorker() {
	for c.processNextNamespace() {
	}
}

func (c *Controller) processNextNamespace() bool {
	obj, shutdown := c.NamespaceQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.NamespaceQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.NamespaceQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processNamespace(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.NamespaceQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processNamespace(key string) error {

	name := key

	o, err := c.NamespaceLister.Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.NamespaceCreatedOrUpdated(o)

}

func (c *Controller) runDeploymentWorker() {
	for c.processNextDeployment() {
	}
}

func (c *Controller) processNextDeployment() bool {
	obj, shutdown := c.DeploymentQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.DeploymentQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.DeploymentQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processDeployment(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.DeploymentQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processDeployment(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.DeploymentLister.Deployments(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.DeploymentCreatedOrUpdated(o)

}

func (c *Controller) runDaemonSetWorker() {
	for c.processNextDaemonSet() {
	}
}

func (c *Controller) processNextDaemonSet() bool {
	obj, shutdown := c.DaemonSetQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.DaemonSetQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.DaemonSetQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processDaemonSet(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.DaemonSetQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processDaemonSet(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.DaemonSetLister.DaemonSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.DaemonSetCreatedOrUpdated(o)

}

func (c *Controller) runReplicaSetWorker() {
	for c.processNextReplicaSet() {
	}
}

func (c *Controller) processNextReplicaSet() bool {
	obj, shutdown := c.ReplicaSetQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.ReplicaSetQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.ReplicaSetQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processReplicaSet(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.ReplicaSetQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processReplicaSet(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.ReplicaSetLister.ReplicaSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.ReplicaSetCreatedOrUpdated(o)

}

func (c *Controller) runStatefulSetWorker() {
	for c.processNextStatefulSet() {
	}
}

func (c *Controller) processNextStatefulSet() bool {
	obj, shutdown := c.StatefulSetQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.StatefulSetQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.StatefulSetQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processStatefulSet(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.StatefulSetQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processStatefulSet(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.StatefulSetLister.StatefulSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.StatefulSetCreatedOrUpdated(o)

}

func (c *Controller) runHostGroupWorker() {
	for c.processNextHostGroup() {
	}
}

func (c *Controller) processNextHostGroup() bool {
	obj, shutdown := c.HostGroupQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.HostGroupQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.HostGroupQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processHostGroup(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.HostGroupQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processHostGroup(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.HostGroupLister.HostGroups(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.HostGroupCreatedOrUpdated(o)

}

func (c *Controller) runHostWorker() {
	for c.processNextHost() {
	}
}

func (c *Controller) processNextHost() bool {
	obj, shutdown := c.HostQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.HostQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.HostQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processHost(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.HostQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processHost(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.HostLister.Hosts(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.HostCreatedOrUpdated(o)

}

func (c *Controller) runCheckWorker() {
	for c.processNextCheck() {
	}
}

func (c *Controller) processNextCheck() bool {
	obj, shutdown := c.CheckQueue.Get()
	if shutdown {
		return false
	}

	err := func(obj interface{}) error {
		defer c.CheckQueue.Done(obj)
		var key string
		var ok bool

		if key, ok = obj.(string); !ok {
			c.CheckQueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}

		if err := c.processCheck(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}

		c.CheckQueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

func (c *Controller) processCheck(key string) error {

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return fmt.Errorf("could not parse name %s: %s", key, err.Error())
	}

	o, err := c.CheckLister.Checks(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("tried to get %s, but it was not found", key)
		} else {
			return fmt.Errorf("error getting %s from cache: %s", key, err.Error())
		}
	}

	return c.CheckCreatedOrUpdated(o)

}
