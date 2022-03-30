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
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"os"
	"strings"

	"github.com/go-logr/logr"
	noobaav1alpha1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	operatorv1 "github.com/openshift/api/operator/v1"
	ofv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	odfv1alpha1 "github.com/red-hat-data-services/odf-operator/api/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	ocsv1 "github.com/red-hat-storage/ocs-operator/api/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ManagedMCGFinalizer = "managedmcg.openshift.io"
	ManagedMCGName      = "managedmcg"

	deployerCSVPrefix           = "mcg-osd-deployer"
	noobaaFinalizer             = "noobaa.io/graceful_finalizer"
	noobaaName                  = "noobaa"
	odfConsoleName              = "odf-console"
	odfOperatorManagerConfigMap = "odf-operator-manager-config"
	odfOperatorName             = "odf-operator"
	operatorConsoleName         = "cluster"
	storageClusterName          = "mcg-storagecluster"
	storageSystemName           = "mcg-storagesystem"
)

type ImageMap struct {
	NooBaaCore string
	NooBaaDB   string
}

type ManagedMCGReconciler struct {
	AddonConfigMapDeleteLabelKey string
	AddonConfigMapName           string
	Client                       client.Client
	Log                          logr.Logger
	Scheme                       *runtime.Scheme

	ctx                         context.Context
	images                      ImageMap
	managedMCG                  *mcgv1alpha1.ManagedMCG
	namespace                   string
	noobaa                      *noobaav1alpha1.NooBaa
	odfOperatorManagerConfigMap *v1.ConfigMap
	operatorConsole             *operatorv1.Console
	reconcileStrategy           mcgv1alpha1.ReconcileStrategy
	storageCluster              *ocsv1.StorageCluster
	storageSystem               *odfv1alpha1.StorageSystem
}

func (r *ManagedMCGReconciler) initializeReconciler(req ctrl.Request) {
	r.ctx = context.Background()
	r.namespace = req.NamespacedName.Namespace

	r.managedMCG = &mcgv1alpha1.ManagedMCG{}
	r.managedMCG.Name = req.NamespacedName.Name
	r.managedMCG.Namespace = r.namespace

	r.odfOperatorManagerConfigMap = &v1.ConfigMap{}
	r.odfOperatorManagerConfigMap.Data = make(map[string]string)
	r.odfOperatorManagerConfigMap.Name = odfOperatorManagerConfigMap
	r.odfOperatorManagerConfigMap.Namespace = r.namespace

	r.noobaa = &noobaav1alpha1.NooBaa{}
	r.noobaa.Name = noobaaName
	r.noobaa.Namespace = r.namespace

	r.storageSystem = &odfv1alpha1.StorageSystem{}
	r.storageSystem.Name = storageSystemName
	r.storageSystem.Namespace = r.namespace

	r.operatorConsole = &operatorv1.Console{}
	r.operatorConsole.Name = operatorConsoleName

	r.storageCluster = &ocsv1.StorageCluster{}
	r.storageCluster.Name = storageClusterName
	r.storageCluster.Namespace = r.namespace
}

//+kubebuilder:rbac:groups=mcg.openshift.io,resources={managedmcgs,managedmcgs/finalizers},verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mcg.openshift.io,resources=managedmcgs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups="coordination.k8s.io",namespace=system,resources=leases,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups=operators.coreos.com,namespace=system,resources=clusterserviceversions,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=ocs.openshift.io,namespace=system,resources=storageclusters,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=noobaas,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=odf.openshift.io,namespace=system,resources=storagesystems,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=get;list;watch;create;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *ManagedMCGReconciler) Reconcile(_ context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("req.Namespace", req.Namespace, "req.Name", req.Name)
	log.Info("starting reconciliation for ManagedMCG")
	r.initializeReconciler(req)
	if err := r.get(r.managedMCG); err != nil {
		if errors.IsNotFound(err) {
			r.Log.Info("ManagedMCG resource not found")
		} else {
			return ctrl.Result{}, err
		}
	}
	result, err := r.reconcilePhases()
	if err != nil {
		r.Log.Error(err, "error reconciling ManagedMCG")
	}
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
	r.Log.Info("reconciliation phases initiated")
	foundAddonDeletionKey := r.verifyAddonDeletionKey() // TODO
	r.updateNoobaaComponentStatus()
	if !r.managedMCG.DeletionTimestamp.IsZero() {
		if r.managedMCG.Status.Components.Noobaa.State == mcgv1alpha1.ComponentNotFound {
			r.Log.Info("removing ManagedMCG finalizer")
			r.managedMCG.SetFinalizers(Remove(r.managedMCG.Finalizers, ManagedMCGFinalizer))
			if err := r.update(r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove ManagedMCG finalizer: %v", err)
			}
			r.Log.Info("ManagedMCG finalizer removed successfully")
		} else {
			r.Log.Info("removing Noobaa")
			r.noobaa.SetFinalizers(Remove(r.noobaa.GetFinalizers(), noobaaFinalizer))
			if err := r.Client.Update(r.ctx, r.noobaa); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to remove Noobaa finalizer: %v", err)
			}
			if err := r.delete(r.noobaa); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete Noobaa CR: %v", err)
			}
			r.Log.Info("removing managed StorageSystem CR")
			if err := r.delete(r.storageSystem); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete StorageSystem CR: %v", err)
			}
		}
	} else if r.managedMCG.UID != "" {
		if !Contains(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer) {
			r.Log.Info("adding ManagedMCG finalizer")
			r.managedMCG.SetFinalizers(append(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer))
			if err := r.update(r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to add ManagedMCG finalizer: %v", err)
			}
		}
		r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyStrict
		if r.managedMCG.Spec.ReconcileStrategy == mcgv1alpha1.ReconcileStrategyNone {
			r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyNone
		}
		//TODO remove ODF hacks after migrating to 4.10
		if err := r.reconcileODFOperatorManagerConfigMap(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileConsole(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileNoobaaComponent(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileODFCSV(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileStorageCluster(); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.reconcileStorageSystem(); err != nil {
			return ctrl.Result{}, err
		}
		r.managedMCG.Status.ReconcileStrategy = r.reconcileStrategy
		isNoobaaReady := r.managedMCG.Status.Components.Noobaa.State == mcgv1alpha1.ComponentReady
		if foundAddonDeletionKey && isNoobaaReady {
			r.Log.Info("commencing addon deletion", "addon deletion key", foundAddonDeletionKey, "Noobaa CR ready state", isNoobaaReady)
			if err := r.delete(r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete ManagedMCG: %v", err)
			}
		}
	} else if foundAddonDeletionKey {
		return ctrl.Result{}, r.removeOLMComponents()
	}
	return ctrl.Result{}, nil
}

func (r *ManagedMCGReconciler) removeOLMComponents() error {
	r.Log.Info("removing addon CSV")
	var err error
	if csv, err := r.getCSVByPrefix(deployerCSVPrefix); err == nil {
		if err := r.delete(csv); err != nil {
			return fmt.Errorf("failed to delete CSV: %v", err)
		}
	}
	return err
}

// verifyAddonDeletionKey checks if the uninstallation condition is met
// by fetching the configmap and checking if the uninstallation key is set
func (r *ManagedMCGReconciler) verifyAddonDeletionKey() bool {
	configmap := &v1.ConfigMap{}
	configmap.Name = r.AddonConfigMapName
	configmap.Namespace = r.namespace
	err := r.get(configmap)
	if err != nil {
		if !errors.IsNotFound(err) {
			r.Log.Error(err, "Unable to get addon delete configmap")
		}
		return false
	}
	_, ok := configmap.Labels[r.AddonConfigMapDeleteLabelKey]
	return ok
}

func (r *ManagedMCGReconciler) reconcileStorageCluster() error {
	r.Log.Info("reconciling StorageCluster")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.storageCluster, func() error {
		storageCluster := templates.StorageClusterTemplate.DeepCopy()
		r.storageCluster.Spec = storageCluster.Spec
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedMCGReconciler) reconcileConsole() error {
	r.Log.Info("reconciling Console")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.operatorConsole, func() error {
		r.Log.Info("creating/updating ODF Console plugin", "plugin", odfConsoleName)
		r.operatorConsole.Spec.Plugins = append(r.operatorConsole.Spec.Plugins, odfConsoleName)
		return nil
	})
	return err
}

func (r *ManagedMCGReconciler) reconcileODFCSV() error {
	r.Log.Info("reconciling ODF CSV")
	var csv *ofv1alpha1.ClusterServiceVersion
	var err error
	if csv, err = r.getCSVByPrefix(odfOperatorName); err != nil {
		return err
	}
	var isChanged bool
	deployments := csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs
	for i := range deployments {
		containers := deployments[i].Spec.Template.Spec.Containers
		for j := range containers {
			container := &containers[j]
			name := container.Name
			switch name {
			case "manager":
				resources := GetDaemonResources("odf-operator")
				if !equality.Semantic.DeepEqual(container.Resources, resources) {
					container.Resources = resources
					isChanged = true
				}
			}
		}
	}
	if isChanged {
		if err := r.update(csv); err != nil {
			return fmt.Errorf("failed to update ODF CSV with resource requirements: %v", err)
		}
	}
	return nil
}

func (r *ManagedMCGReconciler) reconcileNoobaaComponent() error {
	r.Log.Info("reconciling Noobaa")
	desiredNoobaa := templates.NoobaaTemplate.DeepCopy()
	r.setNoobaaDesiredState(desiredNoobaa)
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.noobaa, func() error {
		r.Log.Info("creating/updating Noobaa CR", "name", noobaaName)
		r.noobaa.Spec = desiredNoobaa.Spec
		return nil
	})
	return err
}

func (r *ManagedMCGReconciler) setNoobaaDesiredState(desiredNoobaa *noobaav1alpha1.NooBaa) {
	coreResources := GetDaemonResources("noobaa-core")
	dbResources := GetDaemonResources("noobaa-db")
	dBVolumeResources := GetDaemonResources("noobaa-db-vol")
	endpointResources := GetDaemonResources("noobaa-endpoint")
	desiredNoobaa.Labels = map[string]string{
		"app": "noobaa",
	}
	desiredNoobaa.Spec.CoreResources = &coreResources
	desiredNoobaa.Spec.DBResources = &dbResources
	desiredNoobaa.Spec.DBVolumeResources = &dBVolumeResources
	desiredNoobaa.Spec.Image = &r.images.NooBaaCore
	desiredNoobaa.Spec.DBImage = &r.images.NooBaaDB
	desiredNoobaa.Spec.DBType = noobaav1alpha1.DBTypePostgres
	desiredNoobaa.Spec.Endpoints = &noobaav1alpha1.EndpointsSpec{
		MinCount:               1,
		MaxCount:               2,
		AdditionalVirtualHosts: []string{},
		Resources:              &endpointResources,
	}
}

func (r *ManagedMCGReconciler) reconcileStorageSystem() error {
	r.Log.Info("reconciling StorageSystem")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.storageSystem, func() error {
		if r.reconcileStrategy == mcgv1alpha1.ReconcileStrategyStrict {
			desiredStorageSystem := templates.StorageSystemTemplate.DeepCopy()
			desiredStorageSystem.Spec.Name = storageClusterName
			desiredStorageSystem.Spec.Namespace = r.namespace
			r.storageSystem.Spec = desiredStorageSystem.Spec
		}
		return nil
	})
	if err != nil {
		return err
	}
	err = r.own(r.storageSystem)
	return err
}

func (r *ManagedMCGReconciler) reconcileODFOperatorManagerConfigMap() error {
	r.Log.Info("reconciling odf-operator-manager-config ConfigMap")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.odfOperatorManagerConfigMap, func() error {
		r.odfOperatorManagerConfigMap.Data["ODF_SUBSCRIPTION_NAME"] = "odf-operator-stable-4.9-redhat-operators-openshift-marketplace"
		r.odfOperatorManagerConfigMap.Data["NOOBAA_SUBSCRIPTION_STARTINGCSV"] = "mcg-operator.v4.9.2"
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (r *ManagedMCGReconciler) updateNoobaaComponentStatus() {
	r.Log.Info("updating Noobaa component status")
	noobaaComponent := &r.managedMCG.Status.Components.Noobaa
	if err := r.get(r.noobaa); err == nil {
		if r.noobaa.Status.Phase == "Ready" {
			noobaaComponent.State = mcgv1alpha1.ComponentReady
		} else {
			noobaaComponent.State = mcgv1alpha1.ComponentPending
		}
	} else if errors.IsNotFound(err) {
		noobaaComponent.State = mcgv1alpha1.ComponentNotFound
	} else {
		r.Log.Info("Could not fetch Noobaa CR")
		noobaaComponent.State = mcgv1alpha1.ComponentUnknown
	}
}

func (r *ManagedMCGReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := r.lookupImages(); err != nil {
		return err
	}
	managedMCGPredicates := builder.WithPredicates(
		predicate.GenerationChangedPredicate{},
	)
	enqueueManagedMCGRequest := handler.EnqueueRequestsFromMapFunc(
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
				if name == odfOperatorManagerConfigMap {
					return true
				} else if name == r.AddonConfigMapName {
					if _, ok := client.GetLabels()[r.AddonConfigMapDeleteLabelKey]; ok {
						return true
					}
				}
				return false
			},
		),
	)
	odfOperatorPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				return strings.HasPrefix(client.GetName(), odfOperatorName)
			},
		),
	)
	noobaaPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				return strings.HasPrefix(client.GetName(), noobaaName)
			},
		),
	)
	storageClusterPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				return strings.HasPrefix(client.GetName(), storageClusterName)
			},
		),
	)
	storageSystemPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				return strings.HasPrefix(client.GetName(), storageSystemName)
			},
		),
	)
	return ctrl.NewControllerManagedBy(mgr).
		For(&mcgv1alpha1.ManagedMCG{}, managedMCGPredicates).
		// Watch non-owned resources
		Watches(
			&source.Kind{Type: &v1.ConfigMap{}},
			enqueueManagedMCGRequest,
			configMapPredicates,
		).
		Watches(
			&source.Kind{Type: &odfv1alpha1.StorageSystem{}},
			enqueueManagedMCGRequest,
			storageSystemPredicates,
		).
		Watches(
			&source.Kind{Type: &noobaav1alpha1.NooBaa{}},
			enqueueManagedMCGRequest,
			noobaaPredicates,
		).
		Watches(
			&source.Kind{Type: &ofv1alpha1.ClusterServiceVersion{}},
			enqueueManagedMCGRequest,
			odfOperatorPredicates,
		).
		Watches(
			&source.Kind{Type: &ocsv1.StorageCluster{}},
			enqueueManagedMCGRequest,
			storageClusterPredicates,
		).
		Complete(r)
}

func (r *ManagedMCGReconciler) lookupImages() error {
	noobaaCoreImage, found := os.LookupEnv("NOOBAA_CORE_IMAGE")
	if !found {
		return fmt.Errorf("NOOBAA_CORE_IMAGE environment variable not set")
	} else {
		r.images.NooBaaCore = noobaaCoreImage
	}
	noobaaDBImage, found := os.LookupEnv("NOOBAA_DB_IMAGE")
	if !found {
		return fmt.Errorf("NOOBAA_DB_IMAGE environment variable not set")
	} else {
		r.images.NooBaaDB = noobaaDBImage
	}
	return nil
}
