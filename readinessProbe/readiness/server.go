package readiness

import (
	"context"
	"net/http"

	"github.com/go-logr/logr"
	v1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	listenAddr          string = ":8081"
	readinessPath       string = "/readyz/"
	NamespaceEnvVarName string = "NAMESPACE"
)

func isReady(client client.Client, managedMCGResource types.NamespacedName, log logr.Logger) (bool, error) {

	var managedMCG v1.ManagedMCG

	if err := client.Get(context.Background(), managedMCGResource, &managedMCG); err != nil {
		log.Error(err, "Error while ensuring managedMCG")
		return false, err
	}
	ready := managedMCG.Status.Components.StorageCluster.State == v1.ComponentReady //&&
	//managedMCG.Status.Components.Prometheus.State == v1.ComponentReady &&
	//managedMCG.Status.Components.Alertmanager.State == v1.ComponentReady

	return ready, nil
}

func RunServer(client client.Client, managedMCGResource types.NamespacedName, log logr.Logger) error {

	// Readiness probe is defined here.
	// From k8s documentation:
	// "Any code greater than or equal to 200 and less than 400 indicates success."
	// [indicates that the deployment is ready]
	// "Any other code indicates failure."
	// [indicates that the deployment is not ready]
	http.HandleFunc(readinessPath, func(httpw http.ResponseWriter, req *http.Request) {
		ready, err := isReady(client, managedMCGResource, log)

		if err != nil {
			log.Error(err, "error checking readiness\n")
			httpw.WriteHeader(http.StatusInternalServerError)
			return
		}

		if ready {
			httpw.WriteHeader(http.StatusOK)
		} else {
			httpw.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	return http.ListenAndServe(listenAddr, nil)
}
