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
	"fmt"
	"io/ioutil"
	"strings"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/templates"
	"github.com/red-hat-storage/mcg-osd-deployer/utils"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	dmsRuleName                  = "dms-monitor-rule"
	alertmanagerConfigName       = "managed-mcg-alertmanager-config"
)

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
		utils.AddLabel(obj, monLabelKey, monLabelValue)
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
				receiver.EmailConfigs[0].Smarthost = fmt.Sprintf("%s:%s", r.smtpSecret.Data["host"], r.smtpSecret.Data["port"])
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

func (r *ManagedMCGReconciler) reconcilePrometheus() error {
	r.Log.Info("Reconciling Prometheus")
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.prometheus, func() error {
		if err := r.own(r.prometheus); err != nil {
			return err
		}
		desired := templates.PrometheusTemplate.DeepCopy()
		r.prometheus.ObjectMeta.Labels = map[string]string{monLabelKey: monLabelValue}
		r.prometheus.Spec = desired.Spec
		r.prometheus.Spec.Alerting.Alertmanagers[0].Namespace = r.namespace
		r.prometheus.Spec.AdditionalAlertRelabelConfigs = &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{
				Name: alertRelabelConfigSecretName,
			},
			Key: alertRelabelConfigSecretKey,
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("unable to update prometheus: %w", err)
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
