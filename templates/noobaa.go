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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// NoobaaTemplate is the template that serves as the base for the storage clsuter deployed by the operator

var NoobaaTemplate = &v1alpha1.NooBaa{
	Spec: v1alpha1.NooBaaSpec{
		Endpoints: &v1alpha1.EndpointsSpec{
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
