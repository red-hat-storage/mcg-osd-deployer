/*


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
	_ "github.com/go-openapi/spec"
	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClusterTemplate is the template that serves as the base for the storage clsuter deployed by the operator

var image string = "registry.redhat.io/odf4/mcg-core-rhel8@sha256:5507f2c1074bfb023415f0fef16ec42fbe6e90c540fc45f1111c8c929e477910"
var imageDb string = "registry.redhat.io/rhel8/postgresql-12@sha256:623bdaa1c6ae047db7f62d82526220fac099837afd8770ccc6acfac4c7cff100"

var NoobaTemplate = &noobaa.NooBaa{
	Spec: noobaa.NooBaaSpec{
		CoreResources: &v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("1"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("1"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
		DBResources: &v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse("500m"),
				v1.ResourceMemory: resource.MustParse("4Gi"),
			},
		},
		DBVolumeResources: &v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceStorage: resource.MustParse("50Gi"),
			},
		},
		Image:   &(image),
		DBImage: &(imageDb),
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
