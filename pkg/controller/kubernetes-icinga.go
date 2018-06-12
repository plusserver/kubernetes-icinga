package main

import (
	"flag"
	log "github.com/sirupsen/logrus"
	"os"

	"github.com/Nexinto/go-icinga2-client/icinga2"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"

	icingaclientset "github.com/Nexinto/kubernetes-icinga/pkg/client/clientset/versioned"
)

func main() {

	flag.Parse()

	// If this is not set, glog tries to log into something below /tmp which doesn't exist.
	flag.Lookup("log_dir").Value.Set("/")

	if e := os.Getenv("LOG_LEVEL"); e != "" {
		if l, err := log.ParseLevel(e); err == nil {
			log.SetLevel(l)
		} else {
			log.SetLevel(log.WarnLevel)
			log.Warnf("unknown log level %s, setting to 'warn'", e)
		}
	}

	var kubeconfig string

	if e := os.Getenv("KUBECONFIG"); e != "" {
		kubeconfig = e
	}

	clientConfig, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	kubernetesclient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err.Error())
	}

	icingaclient, err := icingaclientset.NewForConfig(clientConfig)
	if err != nil {
		panic(err.Error())
	}

	var tag string

	if e := os.Getenv("TAG"); e != "" {
		tag = e
	} else {
		tag = "kubernetes"
	}

	icingaApi, err := icinga2.New(icinga2.WebClient{
		URL:         os.Getenv("ICINGA_URL"),
		Username:    os.Getenv("ICINGA_USER"),
		Password:    os.Getenv("ICINGA_PASSWORD"),
		Debug:       os.Getenv("ICINGA_DEBUG") == "true",
		InsecureTLS: true})

	if err != nil {
		panic(err.Error())
	}

	c := &Controller{
		Kubernetes:   kubernetesclient,
		IcingaClient: icingaclient,
		Icinga:       icingaApi,
		Tag:          tag,
	}

	c.Initialize()

	go c.RefreshComponentStatutes()
	go c.EnsureDefaultHostgroups()
	go c.IcingaHousekeeping()

	c.Start()
}
