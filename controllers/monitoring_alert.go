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
	_ "embed"
	"fmt"
	"io/ioutil"
	"net"
	"strings"

	configv1 "github.com/openshift/api/config/v1"

	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"

	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"

	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"github.com/red-hat-storage/mcg-osd-deployer/utils"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	alertRelabelConfigSecretKey  = "alertrelabelconfig.yaml"
	prometheusName               = "managed-mcg-prometheus"
	monLabelKey                  = "app"
	monLabelValue                = "managed-mcg"
	alertRelabelConfigSecretName = "managed-mcg-alert-relabel-config-secret"
	alertmanagerName             = "managed-mcg-alertmanager"
	notificationEmailKeyPrefix   = "notification-email"
	noobaaRulesName              = "managed-mcg-noobaa-rules"
	dmsRuleName                  = "dms-monitor-rule"
	alertmanagerConfigName       = "managed-mcg-alertmanager-config"
)

//go:embed noobaa-rules.yaml
var noobaaRulesSpec []byte

func (r *ManagedMCGReconciler) initializePrometheusReconciler() {
	r.prometheus = &promv1.Prometheus{}
	r.prometheus.Name = prometheusName
	r.prometheus.Namespace = r.namespace

	r.alertmanager = &promv1.Alertmanager{}
	r.alertmanager.Name = alertmanagerName
	r.alertmanager.Namespace = r.namespace

	r.pagerdutySecret = &corev1.Secret{}
	r.pagerdutySecret.Name = r.PagerdutySecretName
	r.pagerdutySecret.Namespace = r.namespace

	r.dmsRule = &promv1.PrometheusRule{}
	r.dmsRule.Name = dmsRuleName
	r.dmsRule.Namespace = r.namespace

	r.noobaaRules = &promv1.PrometheusRule{}
	r.noobaaRules.Name = noobaaRulesName
	r.noobaaRules.Namespace = r.namespace

	r.deadMansSnitchSecret = &corev1.Secret{}
	r.deadMansSnitchSecret.Name = r.DeadMansSnitchSecretName
	r.deadMansSnitchSecret.Namespace = r.namespace

	r.alertmanagerConfig = &promv1a1.AlertmanagerConfig{}
	r.alertmanagerConfig.Name = alertmanagerConfigName
	r.alertmanagerConfig.Namespace = r.namespace

	r.alertRelabelConfigSecret = &corev1.Secret{}
	r.alertRelabelConfigSecret.Name = alertRelabelConfigSecretName
	r.alertRelabelConfigSecret.Namespace = r.namespace

	r.smtpSecret = &corev1.Secret{}
	r.smtpSecret.Name = r.SMTPSecretName
	r.smtpSecret.Namespace = r.namespace

	r.prometheusProxyNetworkPolicy = &netv1.NetworkPolicy{}
	r.prometheusProxyNetworkPolicy.Name = prometheusProxyNetworkPolicyName
	r.prometheusProxyNetworkPolicy.Namespace = r.namespace

	r.prometheusService = &corev1.Service{}
	r.prometheusService.Name = prometheusServiceName
	r.prometheusService.Namespace = r.namespace

	r.kubeRBACConfigMap = &corev1.ConfigMap{}
	r.kubeRBACConfigMap.Name = templates.PrometheusKubeRBACPoxyConfigMapName
	r.kubeRBACConfigMap.Namespace = r.namespace

	r.rhobsRemoteWriteConfigSecret = &corev1.Secret{}
	r.rhobsRemoteWriteConfigSecret.Name = r.RHOBSSecretName
	r.rhobsRemoteWriteConfigSecret.Namespace = r.namespace
}

func (r *ManagedMCGReconciler) reconcileAlertMonitoring() error {
	if err := r.reconcileAlertRelabelConfigSecret(); err != nil {
		return err
	}
	if err := r.reconcilePrometheus(); err != nil {
		return err
	}
	if err := r.reconcileAlertmanager(); err != nil {
		return err
	}
	if err := r.reconcileAlertmanagerConfig(); err != nil {
		return err
	}
	if err := r.reconcileMonitoringResources(); err != nil {
		return err
	}
	if err := r.reconcileDMSPrometheusRule(); err != nil {
		return err
	}
	if err := r.reconcileNoobaaRules(); err != nil {
		return err
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileDMSPrometheusRule() error {
	r.Log.Info("Reconciling DMS Prometheus Rule")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.dmsRule, func() error {
		if err := r.own(r.dmsRule); err != nil {
			return err
		}
		desired := templates.DMSPrometheusRuleTemplate.DeepCopy()
		for _, group := range desired.Spec.Groups {
			if group.Name == "snitch-alert" {
				for _, rule := range group.Rules {
					if rule.Alert == "DeadMansSnitch" {
						rule.Labels["namespace"] = r.namespace
					}
				}
			}
		}
		r.dmsRule.Spec = desired.Spec

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile DMS Prometheus Rule: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileNoobaaRules() error {
	r.Log.Info("Reconciling custom Noobaa Prometheus Rules")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.noobaaRules, func() error {
		if err := r.own(r.noobaaRules); err != nil {
			return err
		}
		if err := k8syaml.Unmarshal(noobaaRulesSpec, &r.noobaaRules.Spec); err != nil {
			return fmt.Errorf("failed to unmarshal noobaa-rules.yaml: %w", err)
		}

		// addLabel is a helper function to add a particular label to all rules in the PrometheusRuleSpec.
		addLabel := func(label, value string) {
			r.Log.Info("Adding label", "label", label, "value", value)
			group := &r.noobaaRules.Spec.Groups
			for i := range *group {
				rules := &(*group)[i].Rules
				for j := range *rules {
					if (*rules)[j].Labels == nil {
						(*rules)[j].Labels = make(map[string]string)
					}
					(*rules)[j].Labels[label] = value
				}
			}
		}

		// append variant and env labels to noobaa rules
		addLabel("variant", r.AddonVariant)
		addLabel("environment", r.AddonEnvironment)

		// append cluster id to the noobaa rules
		clusterVersionList := configv1.ClusterVersionList{}
		if err := r.Client.List(r.ctx, &clusterVersionList); err != nil {
			return fmt.Errorf("failed to get the clusterversion, %w", err)
		}
		for _, cv := range clusterVersionList.Items {
			if cv.Name == "version" {
				addLabel("cluster_id", string(cv.Spec.ClusterID))

				break
			}
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to reconcile custom Noobaa Prometheus Rules: %w", err)
	}

	return nil
}

// reconcileMonitoringResources labels all monitoring resources (ServiceMonitors, PodMonitors, and PrometheusRules)
// found in the target namespace with a label that matches the label selector the defined on the Prometheus resource
// we are reconciling in reconcilePrometheus. Doing so instructs the Prometheus instance to notice and react to these
// labeled monitoring resources.
func (r *ManagedMCGReconciler) reconcileMonitoringResources() error {
	r.Log.Info("reconciling monitoring resources")

	podMonitorList := promv1.PodMonitorList{}
	if err := r.list(&podMonitorList); err != nil {
		return fmt.Errorf("could not list pod monitors: %w", err)
	}
	for i := range podMonitorList.Items {
		obj := podMonitorList.Items[i]
		utils.AddLabel(obj, monLabelKey, monLabelValue)
		if err := r.update(obj); err != nil {
			return err
		}
	}

	serviceMonitorList := promv1.ServiceMonitorList{}
	if err := r.list(&serviceMonitorList); err != nil {
		return fmt.Errorf("could not list service monitors: %w", err)
	}
	for i := range serviceMonitorList.Items {
		obj := serviceMonitorList.Items[i]
		utils.AddLabel(obj, monLabelKey, monLabelValue)
		if err := r.update(obj); err != nil {
			return err
		}
	}

	promRuleList := promv1.PrometheusRuleList{}
	if err := r.list(&promRuleList); err != nil {
		return fmt.Errorf("could not list prometheus rules: %w", err)
	}
	for i := range promRuleList.Items {
		obj := promRuleList.Items[i]
		// do not include the original noobaa rules (deployed by mcg-operator)
		// in the rule files set that our external prometheus instance will load
		if obj.GetName() == "noobaa-prometheus-rules" {
			r.Log.Info("found noobaa prometheus rules, checking for inclusion")
			if v, ok := obj.GetLabels()[monLabelKey]; ok && v == monLabelValue {
				utils.RemoveLabel(obj, monLabelKey)
				r.Log.Info(fmt.Sprintf("removed label %s=%s from noobaa prometheus rules", monLabelKey, monLabelValue))
			}
		} else {
			utils.AddLabel(obj, monLabelKey, monLabelValue)
		}
		if err := r.update(obj); err != nil {
			return err
		}
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileAlertmanagerConfig() error {
	r.Log.Info("Reconciling AlertmanagerConfig secret")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.alertmanagerConfig, func() error {
		err := r.checkPrerequisites()
		if err != nil {
			return err
		}

		dmsURL := string(r.deadMansSnitchSecret.Data["SNITCH_URL"])
		if dmsURL == "" {
			return fmt.Errorf("DeadMansSnitch secret does not contain a SNITCH_URL entry")
		}

		var alertingAddressList []string
		for i := 0; ; i++ {
			alertingAddressKey := fmt.Sprintf("%s-%v", notificationEmailKeyPrefix, i)
			alertingAddress, found := r.addonParams[alertingAddressKey]
			if !found {
				break
			}
			alertingAddressList = append(alertingAddressList, alertingAddress)
		}

		err = r.get(r.smtpSecret)
		if r.smtpSecret.UID == "" && err != nil {
			return fmt.Errorf("unable to get SMTP secret : %w", err)
		}
		params := []string{
			"host",
			"port",
			"username",
			"password",
		}
		for _, param := range params {
			if string(r.smtpSecret.Data[param]) == "" {
				return fmt.Errorf("SMTP secret does not contain a %s entry", param)
			}
		}
		smtpHTML, err := ioutil.ReadFile(r.CustomerNotificationHTMLPath)
		if err != nil {
			return fmt.Errorf("unable to read customernotification.html file: %w", err)
		}
		r.configReceiver(dmsURL, alertingAddressList, smtpHTML)

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update alertmanager config: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) checkPrerequisites() error {
	if err := r.own(r.alertmanagerConfig); err != nil {
		return fmt.Errorf("failed to own AlertmanagerConfig secret: %w", err)
	}

	if err := r.get(r.pagerdutySecret); err != nil {
		return fmt.Errorf("unable to get pagerduty secret: %w", err)
	}

	if string(r.pagerdutySecret.Data["PAGERDUTY_KEY"]) == "" {
		return fmt.Errorf("pagerduty secret does not contain a PAGERDUTY_KEY entry")
	}

	err := r.get(r.deadMansSnitchSecret)
	if r.deadMansSnitchSecret.UID == "" && err != nil {
		return fmt.Errorf("unable to get DeadMansSnitch secret: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) configReceiver(dmsURL string, alertingAddressList []string, smtpHTML []byte) {
	desired := templates.AlertmanagerConfigTemplate.DeepCopy()
	for i := range desired.Spec.Receivers {
		receiver := &desired.Spec.Receivers[i]
		switch receiver.Name {
		case "pagerduty":
			receiver.PagerDutyConfigs[0].ServiceKey.Key = "PAGERDUTY_KEY"
			receiver.PagerDutyConfigs[0].ServiceKey.LocalObjectReference.Name = r.PagerdutySecretName
			receiver.PagerDutyConfigs[0].Details[0].Key = "SOP"
			receiver.PagerDutyConfigs[0].Details[0].Value = r.SOPEndpoint
		case "DeadMansSnitch":
			receiver.WebhookConfigs[0].URL = &dmsURL
		case "SendGrid":
			if len(alertingAddressList) > 0 {
				receiver.EmailConfigs[0].Smarthost = net.JoinHostPort(string(r.smtpSecret.Data["host"]),
					string(r.smtpSecret.Data["port"]))
				receiver.EmailConfigs[0].AuthUsername = string(r.smtpSecret.Data["username"])
				receiver.EmailConfigs[0].AuthPassword.LocalObjectReference.Name = r.SMTPSecretName
				receiver.EmailConfigs[0].AuthPassword.Key = "password"
				receiver.EmailConfigs[0].From = r.AlertSMTPFrom
				receiver.EmailConfigs[0].To = strings.Join(alertingAddressList, ", ")
				receiver.EmailConfigs[0].HTML = string(smtpHTML)
			} else {
				r.Log.Info("customer email for alert notification is not provided")
			}
		}
	}
	r.alertmanagerConfig.Spec = desired.Spec
	utils.AddLabel(r.alertmanagerConfig, monLabelKey, monLabelValue)
}

func (r *ManagedMCGReconciler) reconcileAlertmanager() error {
	r.Log.Info("Reconciling Alertmanager")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.alertmanager, func() error {
		if err := r.own(r.alertmanager); err != nil {
			return err
		}

		desired := templates.AlertmanagerTemplate.DeepCopy()
		desired.Spec.AlertmanagerConfigSelector = &metav1.LabelSelector{
			MatchLabels: map[string]string{
				monLabelKey: monLabelValue,
			},
		}
		r.alertmanager.Spec = desired.Spec
		utils.AddLabel(r.alertmanager, monLabelKey, monLabelValue)

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update alertmanager: %w", err)
	}

	return nil
}

// AlertRelabelConfigSecret will have configuration for relabeling the alerts that are firing.
// It will add namespace label to firing alerts before they are sent to the alertmanager.
func (r *ManagedMCGReconciler) reconcileAlertRelabelConfigSecret() error {
	r.Log.Info("Reconciling alertRelabelConfigSecret")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.alertRelabelConfigSecret, func() error {
		if err := r.own(r.alertRelabelConfigSecret); err != nil {
			return err
		}
		alertRelabelConfig := []struct {
			TargetLabel string `yaml:"target_label,omitempty"`
			Replacement string `yaml:"replacement,omitempty"`
		}{{
			TargetLabel: "namespace",
			Replacement: r.namespace,
		}}
		config, err := yaml.Marshal(alertRelabelConfig)
		if err != nil {
			return fmt.Errorf("unable to encode alert relabel conifg: %w", err)
		}
		r.alertRelabelConfigSecret.Data = map[string][]byte{
			alertRelabelConfigSecretKey: config,
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create/update AlertRelabelConfigSecret: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcileKubeRBACConfigMap() error {
	r.Log.Info("Reconciling kubeRBACConfigMap")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.kubeRBACConfigMap, func() error {
		if err := r.own(r.kubeRBACConfigMap); err != nil {
			return err
		}

		r.kubeRBACConfigMap.Data = templates.KubeRBACProxyConfigMap.DeepCopy().Data

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create kubeRBACConfig config map: %w", err)
	}

	return nil
}

// reconcilePrometheusService function wait for prometheus Service
// to start and sets appropriate annotation for 'service-ca' controller.
func (r *ManagedMCGReconciler) reconcilePrometheusService() error {
	r.Log.Info("Reconciling PrometheusService")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.prometheusService, func() error {
		if err := r.own(r.prometheusService); err != nil {
			return err
		}

		r.prometheusService.Spec.Ports = []corev1.ServicePort{
			{
				Name:       "https",
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(templates.KubeRBACProxyPortNumber),
				TargetPort: intstr.FromString("https"),
			},
		}
		r.prometheusService.Spec.Selector = map[string]string{
			"app.kubernetes.io/name": r.prometheusService.Name,
		}
		utils.AddAnnotation(
			r.prometheusService,
			"service.beta.openshift.io/serving-cert-secret-name",
			templates.PrometheusServingCertSecretName,
		)
		utils.AddAnnotation(
			r.prometheusService,
			"service.alpha.openshift.io/serving-cert-secret-name",
			templates.PrometheusServingCertSecretName,
		)
		// This label is required to enable us to use metrics federation
		// mechanism provided by Managed-tenants
		utils.AddLabel(r.prometheusService, monLabelKey, monLabelValue)

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to create/update PrometheusService: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcilePrometheus() error {
	r.Log.Info("Reconciling Prometheus")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.prometheus, func() error {
		if err := r.own(r.prometheus); err != nil {
			return err
		}
		desired := templates.PrometheusTemplate.DeepCopy()
		utils.AddLabel(r.prometheus, monLabelKey, monLabelValue)
		deployerCSV, err := r.getCSVByPrefix(deployerCSVPrefix)
		if err != nil {
			return fmt.Errorf("unable to set image for kube-rbac-proxy container: %w", err)
		}
		deployerCSVDeployments := deployerCSV.Spec.InstallStrategy.StrategySpec.DeploymentSpecs
		var deployerCSVDeployment *opv1a1.StrategyDeploymentSpec
		for key := range deployerCSVDeployments {
			deployment := &deployerCSVDeployments[key]
			if deployment.Name == "mcg-osd-deployer-controller-manager" {
				deployerCSVDeployment = deployment
			}
		}
		deployerCSVContainers := deployerCSVDeployment.Spec.Template.Spec.Containers
		var kubeRbacImage string
		for key := range deployerCSVContainers {
			container := deployerCSVContainers[key]
			if container.Name == "kube-rbac-proxy" {
				kubeRbacImage = container.Image
			}
		}
		prometheusContainers := desired.Spec.Containers
		for key := range prometheusContainers {
			container := &prometheusContainers[key]
			if container.Name == "kube-rbac-proxy" {
				container.Image = kubeRbacImage
			}
		}
		r.prometheus.Spec = desired.Spec
		r.prometheus.Spec.Alerting.Alertmanagers[0].Namespace = r.namespace
		r.prometheus.Spec.AdditionalAlertRelabelConfigs = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: alertRelabelConfigSecretName,
			},
			Key: alertRelabelConfigSecretKey,
		}

		if err := r.configureRHOBSRemoteWrite(); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to create/update Prometheus: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) reconcilePrometheusProxyNetworkPolicy() error {
	r.Log.Info("reconciling PrometheusProxyNetworkPolicy resources")

	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.prometheusProxyNetworkPolicy, func() error {
		if err := r.own(r.prometheusProxyNetworkPolicy); err != nil {
			return err
		}
		desired := templates.PrometheusProxyNetworkPolicyTemplate.DeepCopy()
		r.prometheusProxyNetworkPolicy.Spec = desired.Spec

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update prometheus proxy NetworkPolicy: %w", err)
	}

	return nil
}

func (r *ManagedMCGReconciler) configureRHOBSRemoteWrite() error {
	if err := r.get(r.rhobsRemoteWriteConfigSecret); err != nil && !errors.IsNotFound(err) {
		return err
	}
	if r.rhobsRemoteWriteConfigSecret.UID == "" {
		r.prometheus.Spec.RemoteWrite = nil
		r.Log.Info("RHOBS remote write config secret not found, disabling remote write")

		return nil
	}
	rhobsSecretData := r.rhobsRemoteWriteConfigSecret.Data
	if _, found := rhobsSecretData[rhobsRemoteWriteConfigIDSecretKey]; !found {
		return fmt.Errorf("rhobs secret does not contain a value for key %v", rhobsRemoteWriteConfigIDSecretKey)
	}
	if _, found := rhobsSecretData[rhobsRemoteWriteConfigSecretName]; !found {
		return fmt.Errorf("rhobs secret does not contain a value for key %v", rhobsRemoteWriteConfigSecretName)
	}
	rhobsAudience, found := rhobsSecretData["rhobs-audience"]
	if !found {
		return fmt.Errorf("rhobs secret does not contain a value for key rhobs-audience")
	}
	remoteWriteSpec := &r.prometheus.Spec.RemoteWrite[0]
	remoteWriteSpec.URL = r.RHOBSEndpoint
	remoteWriteSpec.OAuth2.ClientID.Secret.LocalObjectReference.Name = r.RHOBSSecretName
	remoteWriteSpec.OAuth2.ClientID.Secret.Key = rhobsRemoteWriteConfigIDSecretKey
	remoteWriteSpec.OAuth2.ClientSecret.LocalObjectReference.Name = r.RHOBSSecretName
	remoteWriteSpec.OAuth2.ClientSecret.Key = rhobsRemoteWriteConfigSecretName
	remoteWriteSpec.OAuth2.TokenURL = r.RHSSOTokenEndpoint
	remoteWriteSpec.OAuth2.EndpointParams["audience"] = string(rhobsAudience)

	return nil
}
