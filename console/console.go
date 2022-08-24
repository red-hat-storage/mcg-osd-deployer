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
	"github.com/red-hat-storage/mcg-osd-deployer/templates"

	consolev1alpha1 "github.com/openshift/api/console/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	mainBasePath = "/"
)

var tlsService = "prometheus"

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
				"service.alpha.openshift.io/serving-cert-secret-name": "mcg-ms-console-serving-cert",
			},
			Labels: map[string]string{
				"app": "mcg-ms-console",
			},
		},
		Spec: apiv1.ServiceSpec{
			Ports: []apiv1.ServicePort{
				{
					Protocol:   "TCP",
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
			DisplayName: "mcg-managed-service-console plugin",
			Service: consolev1alpha1.ConsolePluginService{
				Name:      "mcg-ms-console-service",
				Namespace: serviceNamespace,
				Port:      int32(consolePort),
				BasePath:  basePath,
			},
			Proxy: []consolev1alpha1.ConsolePluginProxy{
				{
					Type:      consolev1alpha1.ProxyTypeService,
					Alias:     tlsService + "-proxy",
					Authorize: true,
					Service: consolev1alpha1.ConsolePluginProxyServiceConfig{
						Name:      tlsService,
						Namespace: serviceNamespace,
						Port:      int32(templates.KubeRBACProxyPortNumber),
					},
				},
			},
		},
	}
}

func GetBasePath() string {
	return mainBasePath
}
