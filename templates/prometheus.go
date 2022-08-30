/*


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

package templates

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

	"github.com/red-hat-storage/mcg-osd-deployer/utils"
)

var (
	KubeRBACProxyPortNumber             = 9339
	PrometheusServingCertSecretName     = "prometheus-serving-cert-secret"
	PrometheusKubeRBACPoxyConfigMapName = "prometheus-kube-rbac-proxy-config"
)

// PrometheusTemplate is the template that serves as the base for the prometheus deployed by the operator.
var resourceSelector = metav1.LabelSelector{
	MatchLabels: map[string]string{
		"app": "managed-mcg",
	},
}

var metrics = []string{
	"namespace:noobaa_unhealthy_bucket_claims:max",
	"namespace:noobaa_buckets_claims:max",
	"namespace:noobaa_unhealthy_namespace_resources:max",
	"namespace:noobaa_namespace_resources:max",
	"namespace:noobaa_unhealthy_namespace_buckets:max",
	"namespace:noobaa_namespace_buckets:max",
	"namespace:noobaa_accounts:max",
	"namespace:noobaa_usage:max",
	"namespace:noobaa_system_health_status:max",
}

var alerts = []string{
	"BucketPolicyErrorState",
	"CacheBucketErrorState",
	"DataSourceErrorState",
}

var PrometheusTemplate = promv1.Prometheus{
	Spec: promv1.PrometheusSpec{
		CommonPrometheusFields: promv1.CommonPrometheusFields{
			ServiceMonitorSelector: &resourceSelector,
			PodMonitorSelector:     &resourceSelector,
			Volumes: []corev1.Volume{
				{
					Name: "serving-cert",
					VolumeSource: corev1.VolumeSource{
						Secret: &corev1.SecretVolumeSource{
							SecretName: PrometheusServingCertSecretName,
						},
					},
				},
				{
					Name: "kube-rbac-config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: PrometheusKubeRBACPoxyConfigMapName,
							},
						},
					},
				},
			},
			Resources:          utils.GetResourceRequirements("prometheus"),
			ServiceAccountName: "prometheus-k8s",
			ListenLocal:        true,
			Containers: []corev1.Container{{
				Name: "kube-rbac-proxy",
				Args: []string{
					fmt.Sprintf("--secure-listen-address=0.0.0.0:%d", KubeRBACProxyPortNumber),
					"--upstream=http://127.0.0.1:9090/",
					"--logtostderr=true",
					"--v=10",
					"--tls-cert-file=/etc/tls-secret/tls.crt",
					"--tls-private-key-file=/etc/tls-secret/tls.key",
					"--client-ca-file=/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt",
					"--config-file=/etc/kube-rbac-config/config-file.json",
				},
				Ports: []corev1.ContainerPort{{
					Name:          "https",
					ContainerPort: int32(KubeRBACProxyPortNumber),
				}},
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      "serving-cert",
						MountPath: "/etc/tls-secret",
					},
					{
						Name:      "kube-rbac-config",
						MountPath: "/etc/kube-rbac-config",
					},
				},
				Resources: utils.GetResourceRequirements("kube-rbac-proxy"),
			}},
			RemoteWrite: []promv1.RemoteWriteSpec{
				{
					OAuth2: &promv1.OAuth2{
						ClientSecret: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{},
						},
						ClientID: promv1.SecretOrConfigMap{
							Secret: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{},
							},
						},
						EndpointParams: map[string]string{},
					},
					WriteRelabelConfigs: []promv1.RelabelConfig{
						{
							SourceLabels: []promv1.LabelName{"__name__", "alertname"},
							Regex:        getRelabelRegex(alerts, metrics),
							Action:       "keep",
						},
					},
				},
			},
		},
		RuleSelector: &resourceSelector,
		Alerting: &promv1.AlertingSpec{
			Alertmanagers: []promv1.AlertmanagerEndpoints{
				{
					Namespace: "", 
					Name: "alertmanager-operated",
					Port: intstr.FromString("web"),
				},
			},
		},
	},
}

func getRelabelRegex(alerts []string, metrics []string) string {
	return fmt.Sprintf(
		"(ALERTS;(%s))|%s",
		strings.Join(alerts, "|"),
		strings.Join(
			utils.MapItems(metrics, func(str string) string { return fmt.Sprintf("(%s;)", str) }),
			"|",
		),
	)
}
