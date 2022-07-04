package utils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// ResourceRequirements DaemonResources map contains the default resource requirements for the various MCG daemons.
var ResourceRequirements = map[string]corev1.ResourceRequirements{
	"noobaa-core": {
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	},
	"noobaa-db": {
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("500m"),
			corev1.ResourceMemory: resource.MustParse("4Gi"),
		},
	},
	"noobaa-db-vol": {
		Requests: corev1.ResourceList{
			corev1.ResourceStorage: resource.MustParse("50Gi"),
		},
	},
	"noobaa-endpoint": {
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("2Gi"),
		},
	},
	"prometheus": {
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("500m"),
			"memory": resource.MustParse("250Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("500m"),
			"memory": resource.MustParse("250Mi"),
		},
	},
	"alertmanager": {
		Requests: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("200Mi"),
		},
		Limits: corev1.ResourceList{
			"cpu":    resource.MustParse("200m"),
			"memory": resource.MustParse("200Mi"),
		},
	},
	"kube-rbac-proxy": {
		Limits: corev1.ResourceList{
			"memory": resource.MustParse("30Mi"),
			"cpu":    resource.MustParse("50m"),
		},
		Requests: corev1.ResourceList{
			"memory": resource.MustParse("30Mi"),
			"cpu":    resource.MustParse("50m"),
		},
	},
}
