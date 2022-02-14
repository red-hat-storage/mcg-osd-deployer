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
	"reflect"
	"strings"

	odfv1alpha1 "github.com/red-hat-data-services/odf-operator/api/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClusterTemplate is the template that serves as the base for the storage clsuter deployed by the operator

var StorageSystemTemplate = &odfv1alpha1.StorageSystem{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "mcg-storagesystem",
		Namespace: "openshift-storage",
	},
	Spec: odfv1alpha1.StorageSystemSpec{
		Name:      "mcg-storagecluster",
		Namespace: "openshift-storage",
		Kind:      StorageClusterKind,
	},
}

var StorageClusterKind = odfv1alpha1.StorageKind(strings.ToLower(reflect.TypeOf(ocsv1.StorageCluster{}).Name()) +
	"." + ocsv1.GroupVersion.String())
