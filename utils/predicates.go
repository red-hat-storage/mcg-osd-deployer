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

package utils

import (
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var ssPredicateFunc = func(obj runtime.Object) bool {
	instance, _ := obj.(*ocsv1.StorageCluster)
	name := instance.GetName()
	return name == "storage-cluster"
}

var StorageClusterPredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return ssPredicateFunc(e.Object)
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		return ssPredicateFunc(e.Object)
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return ssPredicateFunc(e.ObjectNew)
	},
	GenericFunc: func(e event.GenericEvent) bool {
		return ssPredicateFunc(e.Object)
	},
}
