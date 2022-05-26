/*
Copyright 2022 Red Hat OpenShift Data Federation.

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

package console

import (
	"strings"

	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const MAIN_BASE_PATH = "/"
const COMPATIBILITY_BASE_PATH = "/compatibility/"

func GetDeployment(namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcg-ms-console",
			Namespace: namespace,
		},
	}
}

func GetService(port int, namespace string) *apiv1.Service {
	return &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mcg-ms-console-service",
			Namespace: namespace,
			Annotations: map[string]string{
				//TODO: Replace "addon-mcg-osd-parameters" with appr. secret
				"service.alpha.openshift.io/serving-cert-secret-name": "addon-mcg-osd-parameters",
			},
			Labels: map[string]string{
				"app": "mcg-ms-console",
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{Protocol: "TCP",
					TargetPort: intstr.IntOrString{IntVal: int32(port)},
					Port:       int32(port),
					Name:       "console-port",
				},
			},
			Selector: map[string]string{
				"app": "mcg-ms-console",
			},
			Type: "ClusterIP",
		},
	}
}

func GetConsolePluginCR(consolePort int, basePath string, serviceNamespace string) *consolev1alpha1.ConsolePlugin {
	return &consolev1alpha1.ConsolePlugin{
		ObjectMeta: metav1.ObjectMeta{
			Name: "mcg-ms-console",
		},
		Spec: consolev1alpha1.ConsolePluginSpec{
			DisplayName: "mcg Plugin",
			Service: consolev1alpha1.ConsolePluginService{
				Name:      "mcg-ms-console-service",
				Namespace: serviceNamespace,
				Port:      int32(consolePort),
				BasePath:  basePath,
			},
		},
	}
}

func GetBasePath(clusterVersion string) string {
	if strings.Contains(clusterVersion, "4.11") {
		return COMPATIBILITY_BASE_PATH
	}

	return MAIN_BASE_PATH
}
