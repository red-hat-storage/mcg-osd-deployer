/*
Copyright 2022.

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

package templates

import (
	//
	_ "github.com/go-openapi/spec"
	"github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NoobaaTemplate is the template that serves as the base for the storage clsuter deployed by the operator

var NoobaaTemplate = &noobaa.NooBaa{
	Spec: noobaa.NooBaaSpec{
		DefaultBackingStoreSpec: &v1alpha1.BackingStoreSpec{
			PVPool: &v1alpha1.PVPoolSpec{
				StorageClass: "gp2",
				NumVolumes:   1,
				VolumeResources: &corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("16Gi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceStorage: resource.MustParse("16Gi"),
					},
				},
			},
			Type: v1alpha1.StoreTypePVPool,
		},
		Endpoints: &noobaa.EndpointsSpec{
			MinCount: 1,
			MaxCount: 2,
			Resources: &v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:    resource.MustParse("1"),
					v1.ResourceMemory: resource.MustParse("2Gi"),
				},
			},
		},
	},
}
