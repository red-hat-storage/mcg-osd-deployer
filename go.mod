module github.com/red-hat-storage/mcg-osd-deployer

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/go-openapi/spec v0.19.8
	github.com/noobaa/noobaa-operator/v5 v5.0.0-20210912161037-7eb9969404e4
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.18.1
	github.com/operator-framework/api v0.10.0
	github.com/red-hat-data-services/odf-operator v0.0.0-20220125090341-1db73867dd7d
	github.com/red-hat-storage/ocs-operator v0.4.9
	go.uber.org/zap v1.20.0 // indirect
	k8s.io/api v0.22.2
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v12.0.0+incompatible
	sigs.k8s.io/controller-runtime v0.9.5

)

replace (
	github.com/go-logr/logr => github.com/go-logr/logr v0.4.0
	github.com/hashicorp/vault => github.com/hashicorp/vault v1.9.3
	github.com/irifrance/gini => github.com/go-air/gini v1.0.4
	github.com/kubernetes-incubator/external-storage => github.com/libopenstorage/external-storage v0.20.4-openstorage-rc3 // required by rook v1.7
	github.com/mikefarah/yaml/v2 => github.com/mikefarah/yq/v4 v4.18.1
	github.com/openshift/api => github.com/openshift/api v0.0.0-20210430163505-eeaa94b80043
	github.com/portworx/sched-ops => github.com/portworx/sched-ops v0.20.4-openstorage-rc3 // required by rook v1.7
	k8s.io/api => k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.20.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.21.3
	k8s.io/apiserver => k8s.io/apiserver v0.20.2
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.20.2
	k8s.io/client-go => k8s.io/client-go v0.21.3
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.20.2
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.20.2
	k8s.io/code-generator => k8s.io/code-generator v0.20.2
	k8s.io/component-base => k8s.io/component-base v0.20.2
	k8s.io/component-helpers => k8s.io/component-helpers v0.20.2
	k8s.io/controller-manager => k8s.io/controller-manager v0.20.2
	k8s.io/cri-api => k8s.io/cri-api v0.20.2
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.20.2
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.20.2
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.20.2
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.20.2
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.20.2
	k8s.io/kubectl => k8s.io/kubectl v0.20.2
	k8s.io/kubelet => k8s.io/kubelet v0.20.2
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.20.2
	k8s.io/metrics => k8s.io/metrics v0.20.2
	k8s.io/mount-utils => k8s.io/mount-utils v0.20.2
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.20.2
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.9.5
)

exclude github.com/kubernetes-incubator/external-storage v0.20.4-openstorage-rc2
