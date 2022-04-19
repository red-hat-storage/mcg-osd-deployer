// This program creates a web server to verify if the managedmcg resource is ready. It is used as a readiness probe by
// the ocs-osd-deployer operator.

package main

import (
	"fmt"
	"os"

	v1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/readinessProbe/readiness"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	log := ctrl.Log.WithName("readiness")

	var options client.Options
	options.Scheme = runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(options.Scheme))
	utilruntime.Must(v1.AddToScheme(options.Scheme))
	k8sClient, err := client.New(config.GetConfigOrDie(), options)
	if err != nil {
		log.Error(err, "error creating client")
		os.Exit(1)
	}

	namespace, found := os.LookupEnv(readiness.NamespaceEnvVarName)
	if !found {
		log.Error(fmt.Errorf("%q not set", readiness.NamespaceEnvVarName), "error in environment variables")
		os.Exit(2)
	}

	managedMCGResource := types.NamespacedName{
		Name:      "managedmcg",
		Namespace: namespace,
	}

	log.Info("starting HTTP server", "namespace", namespace)
	err = readiness.RunServer(k8sClient, managedMCGResource, log)
	if err != nil {
		log.Error(err, "server error")
	}
	log.Info("HTTP server terminated")
}
