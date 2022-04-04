package utils

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	// DaemonResources map contains the default resource requirements for the
	// various MCG daemons
	ResourceRequirements = map[string]corev1.ResourceRequirements{
		"noobaa-core": {
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("1Gi"),
			},
		},
		"noobaa-db": {
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		},
		"noobaa-db-vol": {
			Requests: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("5Gi"),
			},
		},
		"noobaa-endpoint": {
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("250m"),
				corev1.ResourceMemory: resource.MustParse("200Mi"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("500Mi"),
			},
		},
		"prometheus": {
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("250m"),
				"memory": resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse("500m"),
				"memory": resource.MustParse("250Mi"),
			},
		},
		"alertmanager": {
			Requests: corev1.ResourceList{
				"cpu":    resource.MustParse("100m"),
				"memory": resource.MustParse("100Mi"),
			},
			Limits: corev1.ResourceList{
				"cpu":    resource.MustParse("200m"),
				"memory": resource.MustParse("200Mi"),
			},
		},
	}
)
