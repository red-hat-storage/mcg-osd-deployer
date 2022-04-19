package readiness

import (
	"context"
	"fmt"
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
		log.Error(err, "error while ensuring managedMCG")

		return false, fmt.Errorf("error while ensuring managedMCG: %w", err)
	}
	ready := managedMCG.Status.Components.Noobaa.State == v1.ComponentReady

	return ready, nil
}

func RunServer(client client.Client, managedMCGResource types.NamespacedName, log logr.Logger) error {
	// Refer https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes
	http.HandleFunc(readinessPath, func(httpw http.ResponseWriter, req *http.Request) {
		ready, err := isReady(client, managedMCGResource, log)
		if err != nil {
			log.Error(err, "error checking readiness")
			httpw.WriteHeader(http.StatusInternalServerError)

			return
		}
		if ready {
			httpw.WriteHeader(http.StatusOK)
		} else {
			httpw.WriteHeader(http.StatusServiceUnavailable)
		}
	})

	err := http.ListenAndServe(listenAddr, nil)

	return fmt.Errorf("error while running readiness server: %w", err)
}
