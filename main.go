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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/red-hat-storage/mcg-osd-deployer/controllers"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	obv1 "github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis"
	noobaav1alpha1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	openshiftv1 "github.com/openshift/api/network/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"

	//+kubebuilder:scaffold:imports

	consolev1 "github.com/openshift/api/console/v1"
	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

const (
	namespaceKey         = "NAMESPACE"
	addonNameKey         = "ADDON_NAME"
	sopEndpointKey       = "SOP_ENDPOINT"
	alertSMTPFromAddrKey = "ALERT_SMTP_FROM_ADDR"
	rhobsEndpoint        = "RHOBS_ENDPOINT"
	rhssoTokenEndpoint   = "RH_SSO_TOKEN_ENDPOINT"
	addonEnvironment     = "ADDON_ENVIRONMENT"
	healthProbePort      = ":8082"
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mcgv1alpha1.AddToScheme(scheme))
	utilruntime.Must(noobaa.AddToScheme(scheme))
	utilruntime.Must(opv1a1.AddToScheme(scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme))
	utilruntime.Must(promv1.AddToScheme(scheme))
	utilruntime.Must(promv1a1.AddToScheme(scheme))
	utilruntime.Must(noobaav1alpha1.SchemeBuilder.AddToScheme(scheme))
	utilruntime.Must(obv1.AddToScheme(scheme))
	utilruntime.Must(openshiftv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	utilruntime.Must(consolev1.AddToScheme(scheme))
	utilruntime.Must(consolev1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	envMap, err := setupEnvMap()
	if err != nil {
		setupLog.Error(err, "failed to get environment variables")
		os.Exit(1)
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "af4bf43b.openshift.io",
		Namespace:              envMap[namespaceKey],
		HealthProbeBindAddress: healthProbePort,
	})
	if err != nil {
		setupLog.Error(err, "failed to start manager")
		os.Exit(1)
	}
	addonName := envMap[addonNameKey]
	if err = (&controllers.ManagedMCGReconciler{
		Client:                       mgr.GetClient(),
		Log:                          ctrl.Log.WithName("controllers").WithName("ManagedMCG"),
		Scheme:                       mgr.GetScheme(),
		AddonParamSecretName:         fmt.Sprintf("addon-%v-parameters", addonName),
		AddonConfigMapName:           addonName,
		AddonConfigMapDeleteLabelKey: fmt.Sprintf("api.openshift.com/addon-%v-delete", addonName),
		PagerdutySecretName:          fmt.Sprintf("%v-pagerduty", addonName),
		DeadMansSnitchSecretName:     fmt.Sprintf("%v-deadmanssnitch", addonName),
		SMTPSecretName:               fmt.Sprintf("%v-smtp", addonName),
		SOPEndpoint:                  envMap[sopEndpointKey],
		ConsolePort:                  9002,
		AlertSMTPFrom:                envMap[alertSMTPFromAddrKey],
		CustomerNotificationHTMLPath: "templates/customernotification.html",
		RHOBSSecretName:              fmt.Sprintf("%v-prom-remote-write", addonName),
		RHOBSEndpoint:                envMap[rhobsEndpoint],
		RHSSOTokenEndpoint:           envMap[rhssoTokenEndpoint],
		AddonEnvironment:             envMap[addonEnvironment],
		AddonVariant:                 addonName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ManagedMCG")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := ensureManagedMCG(mgr.GetClient(), setupLog, envMap); err != nil {
		os.Exit(1)
	}
	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
	}
	if err := mgr.AddReadyzCheck("check", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
	}
	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupEnvMap() (map[string]string, error) {
	envMap := map[string]string{
		namespaceKey:         "",
		addonNameKey:         "",
		sopEndpointKey:       "",
		alertSMTPFromAddrKey: "",
		rhobsEndpoint:        "",
		rhssoTokenEndpoint:   "",
		addonEnvironment:     "",
	}
	for key := range envMap {
		value, found := os.LookupEnv(key)
		if !found {
			return nil, fmt.Errorf("%s environment variable not set", key)
		}
		envMap[key] = value
	}

	return envMap, nil
}

func ensureManagedMCG(c client.Client, log logr.Logger, envMap map[string]string) error {
	err := c.Create(context.Background(), &mcgv1alpha1.ManagedMCG{
		ObjectMeta: metav1.ObjectMeta{
			Name:       controllers.ManagedMCGName,
			Namespace:  envMap[namespaceKey],
			Finalizers: []string{controllers.ManagedMCGFinalizer},
		},
	})
	switch {
	case err == nil:
		log.Info("ManagedMCG resource created")

		return nil
	case errors.IsAlreadyExists(err):
		log.Info("ManagedMCG resource already exists")

		return nil
	default:
		return fmt.Errorf("failed to create ManagedMCG resource: %w", err)
	}
}
