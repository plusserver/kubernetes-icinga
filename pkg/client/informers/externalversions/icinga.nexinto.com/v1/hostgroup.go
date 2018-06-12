/*
Copyright 2018 Nexinto

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	time "time"

	icinga_nexinto_com_v1 "github.com/Nexinto/kubernetes-icinga/pkg/apis/icinga.nexinto.com/v1"
	versioned "github.com/Nexinto/kubernetes-icinga/pkg/client/clientset/versioned"
	internalinterfaces "github.com/Nexinto/kubernetes-icinga/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/Nexinto/kubernetes-icinga/pkg/client/listers/icinga.nexinto.com/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// HostGroupInformer provides access to a shared informer and lister for
// HostGroups.
type HostGroupInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.HostGroupLister
}

type hostGroupInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewHostGroupInformer constructs a new informer for HostGroup type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewHostGroupInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredHostGroupInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredHostGroupInformer constructs a new informer for HostGroup type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredHostGroupInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IcingaV1().HostGroups(namespace).List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.IcingaV1().HostGroups(namespace).Watch(options)
			},
		},
		&icinga_nexinto_com_v1.HostGroup{},
		resyncPeriod,
		indexers,
	)
}

func (f *hostGroupInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredHostGroupInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *hostGroupInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&icinga_nexinto_com_v1.HostGroup{}, f.defaultInformer)
}

func (f *hostGroupInformer) Lister() v1.HostGroupLister {
	return v1.NewHostGroupLister(f.Informer().GetIndexer())
}