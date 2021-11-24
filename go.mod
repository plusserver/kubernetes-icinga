module github.com/Nexinto/kubernetes-icinga
go 1.13

require (
	github.com/Nexinto/go-icinga2-client v0.0.0-20180829072643-d4f6001a2110
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/imdario/mergo v0.3.8 // indirect
	github.com/jmcvetta/randutil v0.0.0-20150817122601-2bb1b664bcff // indirect
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.6.1
	gopkg.in/jmcvetta/napping.v3 v3.2.0 // indirect
	gopkg.in/yaml.v2 v2.2.8
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v0.20.5
)
