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
	"os"
	"strings"

	"github.com/go-logr/logr"
	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	operatorv1 "github.com/openshift/api/operator/v1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	odfv1alpha1 "github.com/red-hat-data-services/odf-operator/api/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"github.com/red-hat-storage/mcg-osd-deployer/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
	ManagedMCGFinalizer = "managedmcg.openshift.io"

	odfOperatorManagerconfigMapName = "odf-operator-manager-config"
	noobaaName                      = "noobaa"
	storageSystemName               = "mcg-storagesystem"
	ManagedMCGName                  = "managedmcg"
	odfOperatorName                 = "odf-operator"
	operatorConsoleName             = "cluster"
	odfConsoleName                  = "odf-console"
)

// ImageMap holds mapping information between component image name and the image url
type ImageMap struct {
	NooBaaCore string
	NooBaaDB   string
}

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

	odfOperatorManagerconfigMap *corev1.ConfigMap
	noobaa                      *noobaa.NooBaa
	storageSystem               *odfv1alpha1.StorageSystem
	images                      ImageMap
	operatorConsole             *operatorv1.Console
}

func (r *ManagedMCGReconciler) initReconciler(req ctrl.Request) {
	r.ctx = context.Background()
	r.namespace = req.NamespacedName.Namespace

	r.managedMCG = &mcgv1alpha1.ManagedMCG{}
	r.managedMCG.Name = req.NamespacedName.Name
	r.managedMCG.Namespace = r.namespace

	r.odfOperatorManagerconfigMap = &corev1.ConfigMap{}
	r.odfOperatorManagerconfigMap.Name = odfOperatorManagerconfigMapName
	r.odfOperatorManagerconfigMap.Namespace = r.namespace

	r.noobaa = &noobaa.NooBaa{}
	r.noobaa.Name = noobaaName
	r.noobaa.Namespace = r.namespace

	r.storageSystem = &odfv1alpha1.StorageSystem{}
	r.storageSystem.Name = storageSystemName
	r.storageSystem.Namespace = r.namespace

	r.operatorConsole = &operatorv1.Console{}
	r.operatorConsole.Name = operatorConsoleName

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
// +kubebuilder:rbac:groups="storage.k8s.io",resources=storageclass,verbs=get;list;watch
//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=*
//+kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=*

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
			// noobaa needs to be deleted
			r.Log.Info("deleting noobaa")
			if err := r.delete(r.noobaa); err != nil {
				return ctrl.Result{}, fmt.Errorf("unable to delete nooba: %v", err)
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

		// Reconcile the different resources
		if err := r.reconcileODFOperatorMgrConfig(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileStorageSystem(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileConsoleCluster(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileNoobaa(); err != nil {
			return ctrl.Result{}, err
		}
		r.managedMCG.Status.ReconcileStrategy = r.reconcileStrategy

	}
	return ctrl.Result{}, nil
}

func (r *ManagedMCGReconciler) reconcileConsoleCluster() error {

	consoleList := operatorv1.ConsoleList{}
	if err := r.Client.List(r.ctx, &consoleList); err == nil {
		for _, cluster := range consoleList.Items {
			if utils.Contains(cluster.Spec.Plugins, odfConsoleName) {
				r.Log.Info("Cluster instnce already exists.")
				return nil
			}
		}
	}

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.operatorConsole, func() error {
		r.operatorConsole.Spec.Plugins = append(r.operatorConsole.Spec.Plugins, odfConsoleName)
		return nil
	})
	r.Log.Info("Updqted Console resources")
	return err
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
	desiredNooba := templates.NoobaTemplate.DeepCopy()
	err := r.setNooBaaDesiredState(desiredNooba)
	_, err = ctrl.CreateOrUpdate(r.ctx, r.Client, r.noobaa, func() error {
		if err := r.own(r.storageSystem); err != nil {
			return err
		}
		r.noobaa.Spec = desiredNooba.Spec
		return nil
	})
	return err
}

func (r *ManagedMCGReconciler) setNooBaaDesiredState(desiredNooba *noobaa.NooBaa) error {
	coreResources := utils.GetDaemonResources("noobaa-core")
	dbResources := utils.GetDaemonResources("noobaa-db")
	dBVolumeResources := utils.GetDaemonResources("noobaa-db-vol")
	endpointResources := utils.GetDaemonResources("noobaa-endpoint")

	desiredNooba.Labels = map[string]string{
		"app": "noobaa",
	}
	desiredNooba.Spec.CoreResources = &coreResources
	desiredNooba.Spec.DBResources = &dbResources

	desiredNooba.Spec.DBVolumeResources = &dBVolumeResources
	desiredNooba.Spec.Image = &r.images.NooBaaCore
	desiredNooba.Spec.DBImage = &r.images.NooBaaDB
	desiredNooba.Spec.DBType = noobaa.DBTypePostgres

	// Default endpoint spec.
	desiredNooba.Spec.Endpoints = &noobaa.EndpointsSpec{
		MinCount:               1,
		MaxCount:               2,
		AdditionalVirtualHosts: []string{},

		// TODO: After spec.resources["noobaa-endpoint"] is decleared obesolete this
		// definition should hold a constant value. and should not be read from
		// GetDaemonResources()
		Resources: &endpointResources,
	}

	return nil
}
func (r *ManagedMCGReconciler) reconcileStorageSystem() error {
	r.Log.Info("Reconciling StorageSystem.")

	ssList := odfv1alpha1.StorageSystemList{}
	if err := r.list(&ssList); err == nil {
		for _, storageSyste := range ssList.Items {
			if storageSyste.Name == storageSystemName {
				r.Log.Info("storageSystem instnce already exists.")
				return nil
			}
		}
	}
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.storageSystem, func() error {

		if r.reconcileStrategy == mcgv1alpha1.ReconcileStrategyStrict {
			var desired *odfv1alpha1.StorageSystem = nil
			var err error
			if desired, err = r.getDesiredConvergedStorageSystem(); err != nil {
				return err
			}
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
		r.odfOperatorManagerconfigMap.Data["NOOBAA_SUBSCRIPTION_STARTINGCSV"] = "mcg-operator.v4.9.2"

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedMCGReconciler) updateComponentStatus() {
	r.Log.Info("Updating component status")

	// Getting the status of the Noobaa component.
	noobaa := &r.managedMCG.Status.Components.Noobaa
	if err := r.get(r.noobaa); err == nil {
		if r.noobaa.Status.Phase == "Ready" {
			noobaa.State = mcgv1alpha1.ComponentReady
		} else {
			noobaa.State = mcgv1alpha1.ComponentPending
		}
	} else if errors.IsNotFound(err) {
		noobaa.State = mcgv1alpha1.ComponentNotFound
	} else {
		r.Log.V(-1).Info("error getting Noobaa, setting compoment status to Unknown")
		noobaa.State = mcgv1alpha1.ComponentUnknown
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ManagedMCGReconciler) SetupWithManager(mgr ctrl.Manager) error {

	if err := r.initializeImageVars(); err != nil {
		return err
	}

	managedMCGredicates := builder.WithPredicates(
		predicate.GenerationChangedPredicate{},
	)
	ctrlOptions := controller.Options{
		MaxConcurrentReconciles: 1,
	}
	enqueueManangedMCGRequest := handler.EnqueueRequestsFromMapFunc(
		func(client client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name:      ManagedMCGName,
					Namespace: client.GetNamespace(),
				},
			}}
		},
	)
	configMapPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				name := client.GetName()
				if name == odfOperatorManagerconfigMapName {
					return true
				}
				return false
			},
		),
	)

	odfPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				return strings.HasPrefix(client.GetName(), odfOperatorName)
			},
		),
	)

	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(ctrlOptions).
		For(&mcgv1alpha1.ManagedMCG{}, managedMCGredicates).

		// Watch owned resources
		Owns(&noobaa.NooBaa{}).
		Owns(&odfv1alpha1.StorageSystem{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&mcgv1alpha1.ManagedMCG{}).
		Owns(&opv1a1.ClusterServiceVersion{}).

		// Watch non-owned resources
		Watches(
			&source.Kind{Type: &corev1.ConfigMap{}},
			enqueueManangedMCGRequest,
			configMapPredicates,
		).
		Watches(
			&source.Kind{Type: &odfv1alpha1.StorageSystem{}},
			enqueueManangedMCGRequest,
		).
		Watches(
			&source.Kind{Type: &noobaa.NooBaa{}},
			enqueueManangedMCGRequest,
		).
		Watches(
			&source.Kind{Type: &opv1a1.ClusterServiceVersion{}},
			enqueueManangedMCGRequest,
			odfPredicates,
		).
		Complete(r)
}

func (r *ManagedMCGReconciler) initializeImageVars() error {
	r.images.NooBaaCore = os.Getenv("NOOBAA_CORE_IMAGE")
	r.images.NooBaaDB = os.Getenv("NOOBAA_DB_IMAGE")

	if r.images.NooBaaCore == "" {
		err := fmt.Errorf("NOOBAA_CORE_IMAGE environment variable not found")
		r.Log.Error(err, "Missing NOOBAA_CORE_IMAGE environment variable for ocs initialization.")
		return err
	} else if r.images.NooBaaDB == "" {
		err := fmt.Errorf("NOOBAA_DB_IMAGE environment variable not found")
		r.Log.Error(err, "Missing NOOBAA_DB_IMAGE environment variable for ocs initialization.")
		return err
	}
	return nil
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
