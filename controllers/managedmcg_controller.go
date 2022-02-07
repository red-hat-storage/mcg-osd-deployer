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

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	odfv1alpha1 "github.com/red-hat-data-services/odf-operator/api/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"github.com/red-hat-storage/mcg-osd-deployer/utils"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ManagedMCGFinalizer = "managedmcg.ocs.openshift.io"

	StorageClusterName              = "ocs-storagecluster"
	odfOperatorManagerconfigMapName = "odf-operator-manager-config"
	noobaaName                      = "noobaa"
	storageSystemName               = "ocs-storagecluster-storagesystem"
)

// ManagedMCGReconciler reconciles a ManagedMCG object
type ManagedMCGReconciler struct {
	Client             client.Client
	UnrestrictedClient client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	ctx                context.Context
	managedMCG         *mcgv1alpha1.ManagedMCG
	namespace          string
	reconcileStrategy  mcgv1alpha1.ReconcileStrategy

	storageCluster              *ocsv1.StorageCluster
	odfOperatorManagerconfigMap *corev1.ConfigMap
	noobaa                      *noobaa.NooBaa
	storageSystem               *odfv1alpha1.StorageSystem
}

func (r *ManagedMCGReconciler) initReconciler(req ctrl.Request) {
	r.ctx = context.Background()
	r.namespace = req.NamespacedName.Namespace

	r.managedMCG = &mcgv1alpha1.ManagedMCG{}
	r.managedMCG.Name = req.NamespacedName.Name
	r.managedMCG.Namespace = r.namespace

	r.storageCluster = &ocsv1.StorageCluster{}
	r.storageCluster.Name = StorageClusterName
	r.storageCluster.Namespace = r.namespace

	r.odfOperatorManagerconfigMap = &corev1.ConfigMap{}
	r.odfOperatorManagerconfigMap.Name = odfOperatorManagerconfigMapName
	r.odfOperatorManagerconfigMap.Namespace = r.namespace

	r.noobaa = &noobaa.NooBaa{}
	r.noobaa.Name = noobaaName
	r.noobaa.Namespace = r.namespace

	r.storageSystem = &odfv1alpha1.StorageSystem{}
	r.storageSystem.Name = storageSystemName
	r.storageSystem.Namespace = r.namespace

}

//+kubebuilder:rbac:groups=mcg.openshift.io,resources={managedmcg,managedmcg/finalizers},verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mcg.openshift.io,resources=managedmcg/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mcg.openshift.io,resources={managedmcgs,managedmcgs/finalizers},verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mcg.openshift.io,resources=managedmcgs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",namespace=system,resources={secrets,configmaps},verbs=create;get;list;watch;update
// +kubebuilder:rbac:groups="coordination.k8s.io",namespace=system,resources=leases,verbs=create;get;list;watch;update
// +kubebuilder:rbac:groups=operators.coreos.com,namespace=system,resources=clusterserviceversions,verbs=get;list;watch;delete;update;patch

// +kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=noobaas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=odf.openshift.io,namespace=system,resources=storagesystems,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ocs.openshift.io,namespace=system,resources=storageclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="apps",namespace=system,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclass,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ManagedMCG object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.10.0/pkg/reconcile
func (r *ManagedMCGReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("req.Namespace", req.Namespace, "req.Name", req.Name)
	log.Info("Starting reconcile for ManagedMCG.*")

	// Initalize the reconciler properties from the request
	r.initReconciler(req)

	if err := r.get(r.managedMCG); err != nil {
		if errors.IsNotFound(err) {
			r.Log.V(-1).Info("ManagedMCG resource not found..")
		} else {
			return ctrl.Result{}, err
		}
	}
	// Run the reconcile phases
	result, err := r.reconcilePhases()
	if err != nil {
		r.Log.Error(err, "An error was encountered during reconcilePhases")
	}

	// Ensure status is updated once even on failed reconciles
	var statusErr error
	if r.managedMCG.UID != "" {
		statusErr = r.Client.Status().Update(r.ctx, r.managedMCG)
	}

	// Reconcile errors have priority to status update errors
	if err != nil {
		return ctrl.Result{}, err
	} else if statusErr != nil {
		return ctrl.Result{}, statusErr
	} else {
		return result, nil
	}
}

func (r *ManagedMCGReconciler) reconcilePhases() (reconcile.Result, error) {
	r.Log.Info("Reconciler phase started..&&")
	// Uninstallation depends on the status of the components.
	// We are checking the uninstallation condition before getting the component status
	// to mitigate scenarios where changes to the component status occurs while the uninstallation logic is running.
	//initiateUninstall := r.checkUninstallCondition()
	// Update the status of the components
	r.updateComponentStatus()

	if !r.managedMCG.DeletionTimestamp.IsZero() {
		r.Log.Info("removing managedMCG resource if DeletionTimestamp exceeded")
		if r.verifyComponentsDoNotExist() {
			r.Log.Info("removing finalizer from the ManagedMCG resource")
			r.managedMCG.SetFinalizers(utils.Remove(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer))
			if err := r.Client.Update(r.ctx, r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer from ManagedMCG: %v", err)
			}
			r.Log.Info("finallizer removed successfully")

		} else {
			// Storage cluster needs to be deleted before we delete the CSV so we can not leave it to the
			// k8s garbage collector to delete it
			r.Log.Info("deleting storagecluster")
			if err := r.delete(r.storageCluster); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to delete storageSystem: %v", err)
			}
		}

	} else if r.managedMCG.UID != "" {
		r.Log.Info("Reconciler phase started with valid UID")
		if !utils.Contains(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer) {
			r.Log.V(-1).Info("finalizer missing on the managedMCG resource, adding...")
			r.managedMCG.SetFinalizers(append(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer))
			if err := r.update(r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to update managedMCG with finalizer: %v", err)
			}
		}

		// Find the effective reconcile strategy
		r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyStrict
		if strings.EqualFold(string(r.managedMCG.Spec.ReconcileStrategy), string(mcgv1alpha1.ReconcileStrategyNone)) {
			r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyNone
		}

		/*if err := r.get(r.addonParamSecret); err != nil {
			return ctrl.Result{}, fmt.Errorf("Failed to get the addon param secret, Secret Name: %v", r.AddonParamSecretName)
		}*/

		// Reconcile the different resources
		if err := r.reconcileODFOperatorMgrConfig(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileStorageSystem(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileNoobaa(); err != nil {
			return ctrl.Result{}, err
		}
		/*
			if err := r.reconcileStorageCluster(); err != nil {
				return ctrl.Result{}, err
			}*/
		r.managedMCG.Status.ReconcileStrategy = r.reconcileStrategy

	} /*else if initiateUninstall {
		return ctrl.Result{}, r.removeOLMComponents()
	}*/

	return ctrl.Result{}, nil
}

func (r *ManagedMCGReconciler) reconcileNoobaa() error {
	r.Log.Info("Reconciling Noobaa")
	noobaList := noobaa.NooBaaList{}
	if err := r.list(&noobaList); err == nil {
		for _, noobaa := range noobaList.Items {
			if noobaa.Name == "nooba" {
				r.Log.Info("Noona instnce already exists.")
				return nil
			}
		}
	}
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.noobaa, func() error {
		desired := templates.NoobaTemplate.DeepCopy()
		r.noobaa.Spec = desired.Spec
		return nil
	})
	return err

}
func (r *ManagedMCGReconciler) reconcileStorageSystem() error {
	r.Log.Info("Reconciling StorageSystem.")

	/*ssList := odfv1alpha1.StorageSystemList{}
	if err := r.list(&ssList); err == nil {
		return nil
	} */
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.storageSystem, func() error {
		//TODO remove after 4.10
		// if err := r.own(r.storageCluster); err != nil {
		// 	return err
		// }

		// Handle only strict mode reconciliation
		if r.reconcileStrategy == mcgv1alpha1.ReconcileStrategyStrict {
			var desired *odfv1alpha1.StorageSystem = nil
			var err error

			if desired, err = r.getDesiredConvergedStorageSystem(); err != nil {
				return err
			}
			// Override storage cluster spec with desired spec from the template.
			// We do not replace meta or status on purpose
			r.storageSystem.Spec = desired.Spec
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *ManagedMCGReconciler) getDesiredConvergedStorageSystem() (*odfv1alpha1.StorageSystem, error) {

	ss := templates.StorageSystemTemplate.DeepCopy()
	return ss, nil
}

func (r *ManagedMCGReconciler) verifyComponentsDoNotExist() bool {
	subComponent := r.managedMCG.Status.Components

	if subComponent.Noobaa.State == mcgv1alpha1.ComponentNotFound {
		return true
	}
	return false
}

func (r *ManagedMCGReconciler) reconcileODFOperatorMgrConfig() error {
	r.Log.Info("Reconciling odf-operator-manager-config ConfigMap")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.odfOperatorManagerconfigMap, func() error {
		r.odfOperatorManagerconfigMap.Data["ODF_SUBSCRIPTION_NAME"] = "odf-operator-stable-4.9-redhat-operators-openshift-marketplace"
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedMCGReconciler) reconcileStorageCluster() error {
	r.Log.Info("Reconciling StorageCluster")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.storageCluster, func() error {
		/* 4.10 changes
		if err := r.own(r.storageCluster); err != nil {
			return err
		}*/

		// Handle only strict mode reconciliation
		if r.reconcileStrategy == mcgv1alpha1.ReconcileStrategyStrict {
			var desired *ocsv1.StorageCluster = nil
			var err error = nil
			if desired, err = r.getDesiredConvergedStorageCluster(); err != nil {
				return err
			}

			// Override storage cluster spec with desired spec from the template.
			// We do not replace meta or status on purpose
			r.storageCluster.Spec = desired.Spec
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (r *ManagedMCGReconciler) getDesiredConvergedStorageCluster() (*ocsv1.StorageCluster, error) {

	sc := templates.StorageClusterTemplate.DeepCopy()

	r.Log.Info("Enabling Multi Cloud Gateway")
	sc.Spec.MultiCloudGateway.ReconcileStrategy = "standalone"

	return sc, nil
}

func (r *ManagedMCGReconciler) updateComponentStatus() {
	// Getting the status of the StorageCluster component.
	noobaa := &r.managedMCG.Status.Components.Noobaa
	if err := r.get(r.noobaa); err == nil {
		if r.noobaa.Status.Conditions[0].Status == "Ready" {
			noobaa.State = mcgv1alpha1.ComponentReady
		} else {
			noobaa.State = mcgv1alpha1.ComponentPending
		}
	} else if errors.IsNotFound(err) {
		noobaa.State = mcgv1alpha1.ComponentNotFound
	} else {
		r.Log.V(-1).Info("error getting StorageCluster, setting compoment status to Unknown")
		noobaa.State = mcgv1alpha1.ComponentUnknown
	}

	// Getting the status of the Prometheus component.
	/*promStatus := &r.managedMCG.Status.Components.Prometheus
	if err := r.get(r.prometheus); err == nil {
		promStatefulSet := &appsv1.StatefulSet{}
		promStatefulSet.Namespace = r.namespace
		promStatefulSet.Name = fmt.Sprintf("prometheus-%s", prometheusName)
		if err := r.get(promStatefulSet); err == nil {
			desiredReplicas := int32(1)
			if r.prometheus.Spec.Replicas != nil {
				desiredReplicas = *r.prometheus.Spec.Replicas
			}
			if promStatefulSet.Status.ReadyReplicas != desiredReplicas {
				promStatus.State = v1.ComponentPending
			} else {
				promStatus.State = v1.ComponentReady
			}
		} else {
			promStatus.State = v1.ComponentPending
		}
	} else if errors.IsNotFound(err) {
		promStatus.State = v1.ComponentNotFound
	} else {
		r.Log.V(-1).Info("error getting Prometheus, setting compoment status to Unknown")
		promStatus.State = v1.ComponentUnknown
	}

	// Getting the status of the Alertmanager component.
	amStatus := &r.managedOCS.Status.Components.Alertmanager
	if err := r.get(r.alertmanager); err == nil {
		amStatefulSet := &appsv1.StatefulSet{}
		amStatefulSet.Namespace = r.namespace
		amStatefulSet.Name = fmt.Sprintf("alertmanager-%s", alertmanagerName)
		if err := r.get(amStatefulSet); err == nil {
			desiredReplicas := int32(1)
			if r.alertmanager.Spec.Replicas != nil {
				desiredReplicas = *r.alertmanager.Spec.Replicas
			}
			if amStatefulSet.Status.ReadyReplicas != desiredReplicas {
				amStatus.State = v1.ComponentPending
			} else {
				amStatus.State = v1.ComponentReady
			}
		} else {
			amStatus.State = v1.ComponentPending
		}
	} else if errors.IsNotFound(err) {
		amStatus.State = v1.ComponentNotFound
	} else {
		r.Log.V(-1).Info("error getting Alertmanager, setting compoment status to Unknown")
		amStatus.State = v1.ComponentUnknown
	}*/
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedMCGReconciler) SetupWithManager(mgr ctrl.Manager) error {

	ctrlOptions := controller.Options{
		MaxConcurrentReconciles: 1,
	}
	managedMCGredicates := builder.WithPredicates(
		predicate.GenerationChangedPredicate{},
	)

	r.Log.Info("Setting Reconciler...")

	/*ignoreCreatePredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			// Ignore create events as resource created by us
			return false
		},
	}*/

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(ctrlOptions).
		For(&mcgv1alpha1.ManagedMCG{}, managedMCGredicates).
		//Owns(&ocsv1.StorageCluster{}, builder.WithPredicates(utils.StorageClusterPredicate, ignoreCreatePredicate)).
		Watches(&source.Kind{Type: &noobaa.NooBaa{}}, &handler.EnqueueRequestForObject{}).
		Watches(&source.Kind{Type: &odfv1alpha1.StorageSystem{}}, &handler.EnqueueRequestForObject{}).
		//Watches(&source.Kind{Type: &odfv1alpha1.StorageSystem{}}, &handler.EnqueueRequestForObject{}).
		Complete(r)
}

func (r *ManagedMCGReconciler) get(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)

	return r.Client.Get(r.ctx, key, obj)
}

func (r *ManagedMCGReconciler) list(obj client.ObjectList) error {
	listOptions := client.InNamespace(r.namespace)
	return r.Client.List(r.ctx, obj, listOptions)
}

func (r *ManagedMCGReconciler) update(obj client.Object) error {
	return r.Client.Update(r.ctx, obj, nil)
}

func (r *ManagedMCGReconciler) delete(obj client.Object) error {
	if err := r.Client.Delete(r.ctx, obj); err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

func (r *ManagedMCGReconciler) own(resource metav1.Object) error {
	// Ensure ManagedMCG ownership on a resource
	if err := ctrl.SetControllerReference(r.managedMCG, resource, r.Scheme); err != nil {
		return err
	}
	return nil
}

func getCSVByPrefix(csvList opv1a1.ClusterServiceVersionList, name string) *opv1a1.ClusterServiceVersion {
	var csv *opv1a1.ClusterServiceVersion = nil
	for index := range csvList.Items {
		candidate := &csvList.Items[index]
		if strings.HasPrefix(candidate.Name, name) {
			csv = candidate
			break
		}
	}
	return csv
}

func (r *ManagedMCGReconciler) unrestrictedGet(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	return r.UnrestrictedClient.Get(r.ctx, key, obj)
}
