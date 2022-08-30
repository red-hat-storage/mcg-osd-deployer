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
	"time"

	noobaav1alpha1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"github.com/go-logr/logr"
	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
	operatorv1 "github.com/openshift/api/operator/v1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/console"
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"github.com/red-hat-storage/mcg-osd-deployer/utils"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	ManagedMCGFinalizer = "managedmcg.openshift.io"
	ManagedMCGName      = "managedmcg"

	deployerCSVPrefix                 = "mcg-osd-deployer"
	noobaaFinalizer                   = "noobaa.io/graceful_finalizer"
	noobaaName                        = "noobaa"
	prometheusProxyNetworkPolicyName  = "prometheus-proxy-rule"
	prometheusServiceName             = "prometheus"
	ingressNetworkPolicyName          = "ingress-rule"
	ObjectBucketClaimFinalizer        = "objectbucket.io/finalizer"
	mcgmsConsoleName                  = "mcg-ms-console"
	operatorConsoleName               = "cluster"
	rhobsRemoteWriteConfigIDSecretKey = "prom-remote-write-config-id"
	rhobsRemoteWriteConfigSecretName  = "prom-remote-write-config-secret"
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
	AddonParamSecretName         string
	DeadMansSnitchSecretName     string
	CustomerNotificationHTMLPath string
	SMTPSecretName               string
	SOPEndpoint                  string
	AlertSMTPFrom                string
	ConsolePort                  int
	PagerdutySecretName          string
	RHOBSSecretName              string
	RHOBSEndpoint                string
	RHSSOTokenEndpoint           string
	AddonVariant                 string
	AddonEnvironment             string

	objectBucketClaim            *noobaav1alpha1.ObjectBucketClaim
	bucketClass                  *noobaav1alpha1.BucketClass
	ctx                          context.Context
	images                       ImageMap
	managedMCG                   *mcgv1alpha1.ManagedMCG
	namespace                    string
	noobaa                       *noobaav1alpha1.NooBaa
	reconcileStrategy            mcgv1alpha1.ReconcileStrategy
	prometheus                   *promv1.Prometheus
	console                      *consolev1alpha1.ConsolePlugin
	pagerdutySecret              *v1.Secret
	deadMansSnitchSecret         *v1.Secret
	smtpSecret                   *v1.Secret
	alertmanagerConfig           *promv1a1.AlertmanagerConfig
	alertRelabelConfigSecret     *v1.Secret
	addonParams                  map[string]string
	alertmanager                 *promv1.Alertmanager
	dmsRule                      *promv1.PrometheusRule
	noobaaRules                  *promv1.PrometheusRule
	prometheusProxyNetworkPolicy *netv1.NetworkPolicy
	kubeRBACConfigMap            *v1.ConfigMap
	prometheusService            *v1.Service
	rhobsRemoteWriteConfigSecret *v1.Secret

	namespaceStore          *noobaav1alpha1.NamespaceStore
	backingStore            *noobaav1alpha1.BackingStore
	noobaaObjectBucketClaim *noobaav1alpha1.ObjectBucketClaim
	noobaaBucketClass       *noobaav1alpha1.BucketClass

	ingressNetworkPolicy *netv1.NetworkPolicy
	operatorConsole      *operatorv1.Console
}

func (r *ManagedMCGReconciler) initializeReconciler(req ctrl.Request) {
	r.ctx = context.Background()
	r.namespace = req.NamespacedName.Namespace
	r.addonParams = make(map[string]string)

	r.managedMCG = &mcgv1alpha1.ManagedMCG{}
	r.managedMCG.Name = req.NamespacedName.Name
	r.managedMCG.Namespace = r.namespace

	r.noobaa = &noobaav1alpha1.NooBaa{}
	r.noobaa.Name = noobaaName
	r.noobaa.Namespace = r.namespace

	r.objectBucketClaim = &noobaav1alpha1.ObjectBucketClaim{}
	r.bucketClass = &noobaav1alpha1.BucketClass{}
	r.bucketClass.Namespace = r.namespace

	r.namespaceStore = &noobaav1alpha1.NamespaceStore{}
	r.namespaceStore.Namespace = r.namespace
	r.backingStore = &noobaav1alpha1.BackingStore{}
	r.backingStore.Namespace = r.namespace
	r.noobaaObjectBucketClaim = &noobaav1alpha1.ObjectBucketClaim{}
	r.noobaaObjectBucketClaim.Namespace = r.namespace
	r.noobaaBucketClass = &noobaav1alpha1.BucketClass{}
	r.noobaaBucketClass.Namespace = r.namespace

	r.console = &consolev1alpha1.ConsolePlugin{}
	r.console.Name = mcgmsConsoleName
	r.console.Namespace = r.namespace

	r.initializePrometheusReconciler()

	r.ingressNetworkPolicy = &netv1.NetworkPolicy{}
	r.ingressNetworkPolicy.Name = ingressNetworkPolicyName
	r.ingressNetworkPolicy.Namespace = r.namespace

	r.operatorConsole = &operatorv1.Console{}
	r.operatorConsole.Name = operatorConsoleName
}

// Please keep the RBAC specifications below sorted, in order to prevent merge conflicts originating from this part of
// the code in the future.

//+kubebuilder:rbac:groups="",namespace=system,resources=configmaps,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups="",namespace=system,resources=secrets,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups="",namespace=system,resources={services,endpoints},verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",namespace=system,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="apps",namespace=system,resources=deployments/finalizers,verbs=update
//+kubebuilder:rbac:groups="apps",namespace=system,resources=statefulsets,verbs=get;list;watch
//+kubebuilder:rbac:groups="coordination.k8s.io",namespace=system,resources=leases,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=podmonitors,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=prometheusrules,verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources=servicemonitors,verbs=get;list;watch;update;patch;create
//+kubebuilder:rbac:groups="monitoring.coreos.com",namespace=system,resources={alertmanagers,prometheuses,alertmanagerconfigs},verbs=get;list;watch;create;update
//+kubebuilder:rbac:groups="networking.k8s.io",namespace=system,resources=networkpolicies,verbs=create;get;list;watch;update
//+kubebuilder:rbac:groups=config.openshift.io,resources=clusterversions,verbs=get;list;watch;
//+kubebuilder:rbac:groups=console.openshift.io,resources=consoleplugins,verbs=*
//+kubebuilder:rbac:groups=mcg.openshift.io,resources=managedmcgs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mcg.openshift.io,resources={managedmcgs,managedmcgs/finalizers},verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=backingstores,verbs=get;list;watch;
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=backingstores,verbs=delete;
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=namespacestores,verbs=get;list;watch;delete;
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=bucketclasses,verbs=get;list;watch;create;delete;
//+kubebuilder:rbac:groups=noobaa.io,namespace=system,resources=noobaas,verbs=get;list;watch;create;update;delete
//+kubebuilder:rbac:groups=objectbucket.io,namespace=system,resources=objectbucketclaims,verbs=get;list;watch;delete;update;
//+kubebuilder:rbac:groups=objectbucket.io,resources=objectbucketclaims,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=objectbucket.io,namespace=system,resources=objectbucketclaims/finalizers,verbs=update
//+kubebuilder:rbac:groups=operators.coreos.com,namespace=system,resources=clusterserviceversions,verbs=get;list;watch;update;delete
//+kubebuilder:rbac:groups=operator.openshift.io,resources=consoles,verbs=get;list;watch;create;delete;update;

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
	switch {
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("error reconciling ManagedMCG: %w", err)
	case statusErr != nil:
		return ctrl.Result{}, fmt.Errorf("error updating ManagedMCG status: %w", statusErr)
	default:
		return result, nil
	}
}

func (r *ManagedMCGReconciler) reconcilePhases() (reconcile.Result, error) {
	r.Log.Info("reconciliation phases initiated")
	foundAddonDeletionKey := r.verifyAddonDeletionKey()
	r.updateComponentStatus()
	switch {
	case !r.managedMCG.DeletionTimestamp.IsZero():
		if r.managedMCG.Status.Components.Noobaa.State == mcgv1alpha1.ComponentNotFound {
			if err := r.removeManagedMCG(); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			if err := r.removeNoobaa(); err != nil {
				return ctrl.Result{}, err
			}
		}
	case r.managedMCG.UID != "":
		if !utils.Contains(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer) {
			if err := r.addManagedMCG(); err != nil {
				return ctrl.Result{}, err
			}
		}

		if r.managedMCG.Spec.ReconcileStrategy == mcgv1alpha1.ReconcileStrategyNone {
			r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyNone
		} else {
			r.reconcileStrategy = mcgv1alpha1.ReconcileStrategyStrict
		}

		if err := r.updateAddonParams(); err != nil {
			return ctrl.Result{}, err
		}

		if err := r.reconcileResources(); err != nil {
			return ctrl.Result{}, err
		}

		r.managedMCG.Status.ReconcileStrategy = r.reconcileStrategy

		if foundAddonDeletionKey && r.areComponentsReadyForUninstall() {
			r.Log.Info("commencing addon deletion Components in ready state", "addon deletion key", foundAddonDeletionKey)
			if err := r.delete(r.managedMCG); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to delete ManagedMCG: %w", err)
			}
		}
	case foundAddonDeletionKey:
		return ctrl.Result{}, r.removeOLMComponents()
	}

	return ctrl.Result{}, nil
}

func (r *ManagedMCGReconciler) reconcileResources() error {
	if err := r.reconcileNoobaaComponent(); err != nil {
		return err
	}
	if err := r.ensureConsolePlugin(); err != nil {
		return err
	}

	if err := r.reconcileConsoleCluster(); err != nil {
		return err
	}
	if err := r.reconcileOCSCSV(); err != nil {
		return err
	}
	if err := r.reconcileKubeRBACConfigMap(); err != nil {
		return err
	}
	if err := r.reconcilePrometheusService(); err != nil {
		return err
	}
	if err := r.reconcilePrometheusProxyNetworkPolicy(); err != nil {
		return err
	}
	if err := r.reconcileAlertMonitoring(); err != nil {
		return err
	}
	if err := r.reconcileIngressNetworkPolicy(); err != nil {
		return err
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileIngressNetworkPolicy() error {
	r.Log.Info("creating or updating IngressNetworkPolicy")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.ingressNetworkPolicy, func() error {
		if err := r.own(r.ingressNetworkPolicy); err != nil {
			return err
		}
		desired := templates.NetworkPolicyTemplate.DeepCopy()
		r.ingressNetworkPolicy.Spec = desired.Spec

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update ingress network policy: %w", err)
	}
	r.Log.Info("ingressNetworkPolicy applied successfully")

	return nil
}

func (r *ManagedMCGReconciler) removeManagedMCG() error {
	r.Log.Info("removing ManagedMCG finalizer")
	r.managedMCG.SetFinalizers(utils.Remove(r.managedMCG.Finalizers, ManagedMCGFinalizer))
	if err := r.update(r.managedMCG); err != nil {
		return fmt.Errorf("failed to remove ManagedMCG finalizer: %w", err)
	}
	r.Log.Info("ManagedMCG finalizer removed successfully")

	return nil
}

func (r *ManagedMCGReconciler) addManagedMCG() error {
	r.Log.Info("adding ManagedMCG finalizer")
	r.managedMCG.SetFinalizers(append(r.managedMCG.GetFinalizers(), ManagedMCGFinalizer))
	if err := r.update(r.managedMCG); err != nil {
		return fmt.Errorf("failed to add ManagedMCG finalizer: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) removeNoobaa() error {
	r.Log.Info("removing Noobaa")

	bucketClasses := &noobaav1alpha1.BucketClassList{}
	if err := r.list(bucketClasses); err != nil {
		return fmt.Errorf("failed to get the bucketClasses: %w", err)
	}

	for _, bucketClass := range bucketClasses.Items {
		r.noobaaBucketClass.Name = bucketClass.Name
		if err := r.delete(r.noobaaBucketClass); err != nil {
			return fmt.Errorf("failed to delete bucketClasses: %w", err)
		}
	}
	timeout := 10 * time.Second
	interval := 2 * time.Second
	pollBucketClasses := &noobaav1alpha1.BucketClassList{}
	err := utilwait.PollImmediate(interval, timeout, func() (done bool, err error) {
		if err := r.list(pollBucketClasses); err != nil {
			return false, err
		}
		if len(pollBucketClasses.Items) > 0 {
			return false, err
		}

		return true, nil
	})
	if err != nil {
		return fmt.Errorf("failed to get BucketClassList : %w", err)
	}

	r.noobaa.SetFinalizers(utils.Remove(r.noobaa.GetFinalizers(), noobaaFinalizer))
	if err := r.Client.Update(r.ctx, r.noobaa); err != nil {
		return fmt.Errorf("failed to remove Noobaa finalizer: %w", err)
	}
	if err := r.delete(r.noobaa); err != nil {
		return fmt.Errorf("failed to delete Noobaa CR: %w", err)
	}

	backingStores := &noobaav1alpha1.BackingStoreList{}
	if err := r.list(backingStores); err != nil {
		return fmt.Errorf("failed to get the backingStores: %w", err)
	}

	for _, backingStore := range backingStores.Items {
		r.backingStore.Name = backingStore.Name
		if err := r.delete(r.backingStore); err != nil {
			return fmt.Errorf("failed to delete backingStores: %w", err)
		}
	}

	namespaceStores := &noobaav1alpha1.NamespaceStoreList{}
	if err := r.list(namespaceStores); err != nil {
		return fmt.Errorf("failed to get the namespaceStores: %w", err)
	}

	for _, namespaceStore := range namespaceStores.Items {
		r.namespaceStore.Name = namespaceStore.Name
		if err := r.delete(r.namespaceStore); err != nil {
			return fmt.Errorf("failed to delete namespaceStores: %w", err)
		}
	}

	return nil
}

func (r *ManagedMCGReconciler) updateAddonParams() error {
	addonParamSecret := &v1.Secret{}
	addonParamSecret.Name = r.AddonParamSecretName
	addonParamSecret.Namespace = r.namespace
	if err := r.get(addonParamSecret); err != nil {
		return fmt.Errorf("failed to get the addon parameters secret %v", r.AddonParamSecretName)
	}
	for key, value := range addonParamSecret.Data {
		r.addonParams[key] = string(value)
	}

	return nil
}

func (r *ManagedMCGReconciler) removeOLMComponents() error {
	r.Log.Info("removing addon CSV")
	var err error
	if csv, err := r.getCSVByPrefix(deployerCSVPrefix); err == nil {
		if err := r.delete(csv); err != nil {
			return fmt.Errorf("failed to delete CSV: %w", err)
		}
	}

	return err
}

func (r *ManagedMCGReconciler) areComponentsReadyForUninstall() bool {
	subComponents := r.managedMCG.Status.Components

	return subComponents.Noobaa.State == mcgv1alpha1.ComponentReady &&
		subComponents.Prometheus.State == mcgv1alpha1.ComponentReady &&
		subComponents.Alertmanager.State == mcgv1alpha1.ComponentReady &&
		subComponents.Console.State == mcgv1alpha1.ComponentReady
}

// verifyAddonDeletionKey checks if the uninstallation condition is met
// by fetching the configmap and checking if the uninstallation key is set.
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

func (r *ManagedMCGReconciler) reconcileNoobaaComponent() error {
	r.Log.Info("reconciling Noobaa")
	desiredNoobaa := templates.NoobaaTemplate.DeepCopy()
	noobaaAnnotations := r.noobaa.GetAnnotations()
	defaultBackingStore := r.getDefaultBackingStore()
	r.setNoobaaDesiredState(desiredNoobaa)
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.noobaa, func() error {
		r.Log.Info("creating/updating Noobaa CR", "name", noobaaName)
		annotationDefaultBackingStore, ok := noobaaAnnotations["default-backing-store"]
		if (!ok || annotationDefaultBackingStore == "") && defaultBackingStore != "" {
			utils.AddAnnotation(r.noobaa, "default-backing-store", defaultBackingStore)
		}
		r.noobaa.Spec = desiredNoobaa.Spec

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile Noobaa: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileOCSCSV() error {
	var csv *opv1a1.ClusterServiceVersion
	var err error
	subComponents := r.managedMCG.Status.Components
	if subComponents.Noobaa.State != mcgv1alpha1.ComponentReady {
		r.Log.Info("OCS resource deletion is waiting for Noobaa")

		return nil
	}
	if csv, err = r.getCSVByPrefix("ocs-operator"); err != nil {
		return err
	}
	var isChanged bool
	var name string
	deployments := csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs
	zero := int32(0)
	for i := range deployments {
		name = deployments[i].Name
		switch name {
		case "ocs-operator", "ocs-metrics-exporter", "rook-ceph-operator":
			replicaCount := &deployments[i].Spec.Replicas
			if **replicaCount != zero {
				r.Log.Info("downscaling deployment replicas", "OCS Deployment", name)
				*replicaCount = &zero
				isChanged = true
			}
		default:
			r.Log.Info("could not find deployment", "Deployment", name)
		}
	}
	if isChanged {
		if err := r.update(csv); err != nil {
			return fmt.Errorf("failed to update OCS CSV: %w", err)
		}
	}

	return nil
}

func (r *ManagedMCGReconciler) setNoobaaDesiredState(desiredNoobaa *noobaav1alpha1.NooBaa) {
	coreResources := utils.GetResourceRequirements("noobaa-core")
	dbResources := utils.GetResourceRequirements("noobaa-db")
	dBVolumeResources := utils.GetResourceRequirements("noobaa-db-vol")
	endpointResources := utils.GetResourceRequirements("noobaa-endpoint")
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

func (r *ManagedMCGReconciler) updateComponentStatus() {
	r.UpdateNoobaaComponentStatus()

	r.UpdatePrometheusComponentStatus()

	r.UpdateAlertmanagerComponentStatus()

	r.UpdateConsoleComponentStatus()
}

func (r *ManagedMCGReconciler) UpdateNoobaaComponentStatus() {
	r.Log.Info("updating Noobaa component status")
	noobaaComponent := &r.managedMCG.Status.Components.Noobaa
	switch err := r.get(r.noobaa); {
	case err == nil:
		if r.noobaa.Status.Phase == "Ready" {
			noobaaComponent.State = mcgv1alpha1.ComponentReady
		} else {
			noobaaComponent.State = mcgv1alpha1.ComponentPending
		}
	case errors.IsNotFound(err):
		noobaaComponent.State = mcgv1alpha1.ComponentNotFound
	default:
		r.Log.Info("Could not fetch Noobaa CR")
		noobaaComponent.State = mcgv1alpha1.ComponentUnknown
	}
}

// Getting the status of the Prometheus component.
//nolint:dupl
func (r *ManagedMCGReconciler) UpdatePrometheusComponentStatus() {
	promStatus := &r.managedMCG.Status.Components.Prometheus
	switch err := r.get(r.prometheus); {
	case err == nil:
		promStatefulSet := &appsv1.StatefulSet{}
		promStatefulSet.Namespace = r.namespace
		promStatefulSet.Name = fmt.Sprintf("prometheus-%s", prometheusName)
		if err := r.get(promStatefulSet); err == nil {
			desiredReplicas := int32(1)
			if r.prometheus.Spec.Replicas != nil {
				desiredReplicas = *r.prometheus.Spec.Replicas
			}
			promStatus.State = r.CheckReplicaStatus(promStatefulSet.Status.ReadyReplicas, desiredReplicas)
		} else {
			promStatus.State = mcgv1alpha1.ComponentPending
		}
	case errors.IsNotFound(err):
		promStatus.State = mcgv1alpha1.ComponentNotFound
	default:
		r.Log.Info("error getting Prometheus, setting component status to Unknown")
		promStatus.State = mcgv1alpha1.ComponentUnknown
	}
}

// Getting the status of the Alertmanager component.
//nolint:dupl
func (r *ManagedMCGReconciler) UpdateAlertmanagerComponentStatus() {
	amStatus := &r.managedMCG.Status.Components.Alertmanager
	switch err := r.get(r.alertmanager); {
	case err == nil:
		amStatefulSet := &appsv1.StatefulSet{}
		amStatefulSet.Namespace = r.namespace
		amStatefulSet.Name = fmt.Sprintf("alertmanager-%s", alertmanagerName)
		if err := r.get(amStatefulSet); err == nil {
			desiredReplicas := int32(1)
			if r.alertmanager.Spec.Replicas != nil {
				desiredReplicas = *r.alertmanager.Spec.Replicas
			}
			amStatus.State = r.CheckReplicaStatus(amStatefulSet.Status.ReadyReplicas, desiredReplicas)
		} else {
			amStatus.State = mcgv1alpha1.ComponentPending
		}
	case errors.IsNotFound(err):
		amStatus.State = mcgv1alpha1.ComponentNotFound
	default:
		r.Log.Info("error getting Alertmanager, setting component status to Unknown")
		amStatus.State = mcgv1alpha1.ComponentUnknown
	}
}

// Getting the status of the Console component.
func (r *ManagedMCGReconciler) UpdateConsoleComponentStatus() {
	r.Log.Info("updating the Console component status")
	consoleStatus := &r.managedMCG.Status.Components.Console
	switch err := r.get(r.console); {
	case err == nil:
		consoleDeployment := &appsv1.Deployment{}
		consoleDeployment.Namespace = r.namespace
		consoleDeployment.Name = mcgmsConsoleName
		if err := r.get(consoleDeployment); err == nil {
			desiredReplicas := int32(1)
			consoleStatus.State = r.CheckReplicaStatus(consoleDeployment.Status.ReadyReplicas, desiredReplicas)
		} else {
			consoleStatus.State = mcgv1alpha1.ComponentPending
		}
	case errors.IsNotFound(err):
		consoleStatus.State = mcgv1alpha1.ComponentNotFound
	default:
		r.Log.Info("error getting Console, setting component status to unknown")
		consoleStatus.State = mcgv1alpha1.ComponentUnknown
	}
}

func (r *ManagedMCGReconciler) CheckReplicaStatus(readyReplicas, desiredReplicas int32) mcgv1alpha1.ComponentState {
	if readyReplicas < desiredReplicas {
		return mcgv1alpha1.ComponentPending
	}

	return mcgv1alpha1.ComponentReady
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
				if name == r.AddonConfigMapName {
					if _, ok := client.GetLabels()[r.AddonConfigMapDeleteLabelKey]; ok {
						return true
					}
				} else if name == alertmanagerConfigName {
					return true
				}

				return false
			},
		),
	)

	prometheusRulesPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				labels := client.GetLabels()

				return labels == nil || labels[monLabelKey] != monLabelValue
			},
		),
	)

	secretPredicates := builder.WithPredicates(
		predicate.NewPredicateFuncs(
			func(client client.Object) bool {
				name := client.GetName()

				return name == r.AddonParamSecretName ||
					name == r.PagerdutySecretName ||
					name == r.DeadMansSnitchSecretName ||
					name == r.SMTPSecretName ||
					name == r.RHOBSSecretName
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

	bucketclassPredicates := builder.WithPredicates(predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			r.bucketClassAdded(e.Object)

			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			if e.Object.GetName() == "noobaa-default-bucket-class" {
				return true
			}
			r.bucketClassDeleted(e.Object)

			return true
		},
	})

	enqueueBucketClassRequest := handler.EnqueueRequestsFromMapFunc(
		func(object client.Object) []reconcile.Request {
			return []reconcile.Request{{
				NamespacedName: types.NamespacedName{
					Name:      ManagedMCGName,
					Namespace: object.GetNamespace(),
				},
			}}
		},
	)

	err := ctrl.NewControllerManagedBy(mgr).
		For(&mcgv1alpha1.ManagedMCG{}, managedMCGPredicates).
		// Watch owned resources
		Owns(&v1.ConfigMap{}).
		Owns(&v1.Service{}).
		Owns(&netv1.NetworkPolicy{}).
		// Watch non-owned resources
		Watches(
			&source.Kind{Type: &v1.ConfigMap{}},
			enqueueManagedMCGRequest,
			configMapPredicates,
		).
		Watches(
			&source.Kind{Type: &noobaav1alpha1.NooBaa{}},
			enqueueManagedMCGRequest,
			noobaaPredicates,
		).
		Watches(
			&source.Kind{Type: &v1.Secret{}},
			enqueueManagedMCGRequest,
			secretPredicates,
		).
		Watches(
			&source.Kind{Type: &promv1.PrometheusRule{}},
			enqueueManagedMCGRequest,
			prometheusRulesPredicates,
		).
		Watches(
			&source.Kind{Type: &noobaav1alpha1.BucketClass{}},
			enqueueBucketClassRequest,
			bucketclassPredicates,
		).
		Complete(r)
	if err != nil {
		return fmt.Errorf("error setting up ManagedMCG controller: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) lookupImages() error {
	noobaaCoreImage, found := os.LookupEnv("NOOBAA_CORE_IMAGE")
	if !found {
		return fmt.Errorf("NOOBAA_CORE_IMAGE environment variable not set")
	}
	r.images.NooBaaCore = noobaaCoreImage
	noobaaDBImage, found := os.LookupEnv("NOOBAA_DB_IMAGE")
	if !found {
		return fmt.Errorf("NOOBAA_DB_IMAGE environment variable not set")
	}
	r.images.NooBaaDB = noobaaDBImage

	return nil
}

func (r *ManagedMCGReconciler) get(obj client.Object) error {
	key := client.ObjectKeyFromObject(obj)
	err := r.Client.Get(r.ctx, key, obj)
	if err != nil {
		return fmt.Errorf("error getting %s: %w", key, err)
	}

	return nil
}

func (r *ManagedMCGReconciler) list(obj client.ObjectList) error {
	listOptions := client.InNamespace(r.namespace)
	err := r.Client.List(r.ctx, obj, listOptions)
	if err != nil {
		return fmt.Errorf("error listing %s: %w", obj.GetObjectKind().GroupVersionKind(), err)
	}

	return nil
}

func (r *ManagedMCGReconciler) update(obj client.Object) error {
	err := r.Client.Update(r.ctx, obj)
	if err != nil {
		return fmt.Errorf("error updating %s: %w", obj, err)
	}

	return nil
}

func (r *ManagedMCGReconciler) delete(obj client.Object) error {
	if err := r.Client.Delete(r.ctx, obj); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete %s: %w", obj.GetName(), err)
	}

	return nil
}

func (r *ManagedMCGReconciler) own(resource metav1.Object) error {
	// Ensure ManagedMCG ownership on a resource
	if err := ctrl.SetControllerReference(r.managedMCG, resource, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference on %s: %w", resource.GetName(), err)
	}

	return nil
}

func (r *ManagedMCGReconciler) getCSVByPrefix(name string) (*opv1a1.ClusterServiceVersion, error) {
	csvList := opv1a1.ClusterServiceVersionList{}
	if err := r.list(&csvList); err != nil {
		return nil, fmt.Errorf("unable to list csv resources: %w", err)
	}
	var csv *opv1a1.ClusterServiceVersion
	for i := range csvList.Items {
		if strings.HasPrefix(csvList.Items[i].Name, name) {
			csv = &csvList.Items[i]

			break
		}
	}
	if csv == nil {
		return nil, fmt.Errorf("unable to get csv resources for %s ", name)
	}

	return csv, nil
}

func (r *ManagedMCGReconciler) reconcileConsoleCluster() error {
	consoleList := operatorv1.ConsoleList{}
	if err := r.Client.List(r.ctx, &consoleList); err != nil {
		return fmt.Errorf("failed to reconcile console cluster, %w", err)
	}
	for _, cluster := range consoleList.Items {
		if utils.Contains(cluster.Spec.Plugins, mcgmsConsoleName) {
			r.Log.Info("Cluster instnce already exists.")

			return nil
		}
	}
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.operatorConsole, func() error {
		r.operatorConsole.Spec.Plugins = append(r.operatorConsole.Spec.Plugins, mcgmsConsoleName)
		r.Log.Info("Updated Console resources")

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile ConsoleCluster: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) ensureConsolePlugin() error {
	// The base path to where the plugin's assets are stored. ex: plugin-manifest.json
	basePath := console.GetBasePath()
	// Get mcg console Deployment
	mcgConsoleDeployment := console.GetDeployment(r.namespace)
	err := r.Client.Get(r.ctx, types.NamespacedName{
		Name:      mcgConsoleDeployment.Name,
		Namespace: mcgConsoleDeployment.Namespace,
	}, mcgConsoleDeployment)
	if err != nil {
		return fmt.Errorf("failed to get the deployment, %w", err)
	}

	// Create/Update mcg console Service
	mcgConsoleService := console.GetService(r.ConsolePort, r.namespace)
	r.Log.Info("creating or updating mcgConsoleService")
	_, err = controllerutil.CreateOrUpdate(r.ctx, r.Client, mcgConsoleService, func() error {
		err = controllerutil.SetControllerReference(mcgConsoleDeployment, mcgConsoleService, r.Scheme)
		if err != nil {
			return fmt.Errorf("failed to set controller owner reference, %w", err)
		}

		return nil
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create/update console service, %w", err)
	}

	// Create/Update mcg console ConsolePlugin
	mcgConsolePlugin := console.GetConsolePluginCR(r.ConsolePort, basePath, r.namespace)
	r.Log.Info("creating or updating mcgConsolePlugin")
	_, err = controllerutil.CreateOrUpdate(r.ctx, r.Client, mcgConsolePlugin, func() error {
		return nil
	})
	if err != nil && !errors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to get console plugin CR, %w", err)
	}

	return nil
}
