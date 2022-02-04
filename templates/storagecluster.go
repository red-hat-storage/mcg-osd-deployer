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
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StorageClusterTemplate is the template that serves as the base for the storage clsuter deployed by the operato

var StorageClusterTemplate = ocsv1.StorageCluster{
	Spec: ocsv1.StorageClusterSpec{
		// The label selector is used to select only the worker nodes for
		// both labeling and scheduling

		LabelSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "node-role.kubernetes.io/worker",
				Operator: metav1.LabelSelectorOpExists,
			}, {
				Key:      "node-role.kubernetes.io/infra",
				Operator: metav1.LabelSelectorOpDoesNotExist,
			}},
		},
		ManageNodes: false,
		MultiCloudGateway: &ocsv1.MultiCloudGatewaySpec{
			ReconcileStrategy: "ignore",
		},
	},
}
