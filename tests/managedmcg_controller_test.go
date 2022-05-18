package tests

import (
	"context"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	managedmcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	noobaav1alpha1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	namespace                    = "redhat-data-federation"
	customerNotificationHTMLPath = "/tmp/customernotification.html"
)

var (
	managedMCGName = "managedmcg"
	managedMCG     = managedmcgv1alpha1.ManagedMCG{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedMCGName,
			Namespace: namespace,
			UID:       "123456",
		},
		Spec: managedmcgv1alpha1.ManagedMCGSpec{
			ReconcileStrategy: "ignore",
		},
	}
	namespaceFake = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	r = &controllers.ManagedMCGReconciler{
		Log: ctrl.Log.WithName("controllers").WithName("managedmcg"),
	}
)

var _ = Describe("ManagedMCG validations", func() {
	BeforeEach(func() {
		r.Scheme = k8sClient.Scheme()
	})

	When("Creating and deleting ManagedMCG", func() {
		It("should not return validation error", func() {
			By("using default values", func() {
				newNamespace := namespaceFake.DeepCopy()
				newManagedMCG := managedMCG.DeepCopy()
				r.Client = fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(newNamespace, newManagedMCG).Build()

				err := r.Client.Delete(context.Background(), newManagedMCG, &client.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())

				err = r.Client.Delete(context.Background(), newNamespace, &client.DeleteOptions{})
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

var _ = Describe("ManagedMCGReconciler Reconcile", func() {
	When("Creating ManagedMCG", func() {
		addonSecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "addon-secret-fake",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"addonparam": []byte("foo"),
			},
		}

		pagerDutySecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pagerduty-secret-fake",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"PAGERDUTY_KEY": []byte("foo"),
			},
		}

		deadMansSecretfake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deadmans-secret-fake",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"SNITCH_URL": []byte("foo"),
			},
		}

		smtpSecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "smtp-secret-fake",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"host":     []byte("host"),
				"port":     []byte("8080"),
				"username": []byte("username"),
				"password": []byte("password"),
			},
		}

		ocscsv := opv1a1.ClusterServiceVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ocs-operator",
				Namespace: namespace,
			},
		}

		noobacsv := noobaav1alpha1.NooBaa{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nooba-operator",
				Namespace: namespace,
			},
		}

		prometheusFake := promv1.Prometheus{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prometheus-fake",
				Namespace: namespace,
			},
		}

		alertmanagerFake := promv1.Alertmanager{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-fake",
				Namespace: namespace,
			},
		}

		alertmanagerConfigFake := promv1a1.AlertmanagerConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-fake",
				Namespace: namespace,
			},
		}

		alertRelabelConfigSecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "alertmanager-config-fake",
				Namespace: namespace,
			},
		}

		dmsRuleFake := promv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "dms-rule-fake",
				Namespace: namespace,
			},
		}

		newNamespace := namespaceFake.DeepCopy()
		newManagedMCG := managedMCG.DeepCopy()
		newOcscsv := ocscsv.DeepCopy()
		newAddonSecret := addonSecretFake.DeepCopy()
		newSMTPSecret := smtpSecretFake.DeepCopy()
		newDeadMansSecret := deadMansSecretfake.DeepCopy()
		newPagerDutySecret := pagerDutySecretFake.DeepCopy()
		newNoobacsv := noobacsv.DeepCopy()
		newPrometheus := prometheusFake.DeepCopy()
		newAlertManager := alertmanagerFake.DeepCopy()
		newAlertManagerConfig := alertmanagerConfigFake.DeepCopy()
		newAlertRelabelConfigSecret := alertRelabelConfigSecretFake.DeepCopy()
		newDMSRule := dmsRuleFake.DeepCopy()

		BeforeEach(func() {
			r.Scheme = k8sClient.Scheme()
			r.Client = fake.NewClientBuilder().WithScheme(k8sClient.Scheme()).WithObjects(newNamespace, newManagedMCG, newOcscsv,
				newAddonSecret, newSMTPSecret, newDeadMansSecret, newPagerDutySecret, newNoobacsv, newPrometheus,
				newAlertManager, newAlertManagerConfig, newAlertRelabelConfigSecret, newDMSRule).Build()
			r.AddonParamSecretName = newAddonSecret.Name
			r.PagerdutySecretName = newPagerDutySecret.Name
			r.DeadMansSnitchSecretName = newDeadMansSecret.Name
			r.SMTPSecretName = newSMTPSecret.Name

			r.CustomerNotificationHTMLPath = customerNotificationHTMLPath

			err := os.WriteFile(customerNotificationHTMLPath, []byte{}, 0o444)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err := r.Client.Delete(context.Background(), newOcscsv, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newAddonSecret, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newSMTPSecret, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newDeadMansSecret, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newPagerDutySecret, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newNoobacsv, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newManagedMCG, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = r.Client.Delete(context.Background(), newNamespace, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = os.Remove(customerNotificationHTMLPath)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be able to reconcile ManagedMCG object", func() {
			By("providing valid ManagedMCG params", func() {
				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "managedmcg",
						Namespace: namespace,
					},
				}

				_, err := r.Reconcile(context.Background(), req)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
