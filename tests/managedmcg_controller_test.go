package tests

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	managedmcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"github.com/red-hat-storage/mcg-osd-deployer/controllers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	managedMCGName = "managedmcg"
	managedMCG     = managedmcgv1alpha1.ManagedMCG{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedMCGName,
			Namespace: Namespace,
		},
		Spec: managedmcgv1alpha1.ManagedMCGSpec{
			ReconcileStrategy: "ignore",
		},
	}
)

var _ = Describe("ManagedMCG validations", func() {
	When("Creating and deleting ManagedMCG", func() {
		It("should not return validation error", func() {
			By("using default values", func() {
				newManagedMCG := managedMCG.DeepCopy()
				err := k8sClient.Create(context.Background(), newManagedMCG, &client.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				err = k8sClient.Delete(context.Background(), newManagedMCG, &client.DeleteOptions{})
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
				Namespace: Namespace,
			},
		}
		addonSecretFake.Data = make(map[string][]byte, 1)
		addonSecretFake.Data["addonparam"] = []byte("foo")

		pagerDutySecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pagerduty-secret-fake",
				Namespace: Namespace,
			},
		}
		pagerDutySecretFake.Data = make(map[string][]byte, 1)
		pagerDutySecretFake.Data["PAGERDUTY_KEY"] = []byte("foo")

		deadMansSecretfake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deadmans-secret-fake",
				Namespace: Namespace,
			},
		}
		deadMansSecretfake.Data = make(map[string][]byte, 1)
		deadMansSecretfake.Data["SNITCH_URL"] = []byte("foo")

		smtpSecretFake := corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "smtp-secret-fake",
				Namespace: Namespace,
			},
		}
		smtpSecretFake.Data = make(map[string][]byte, 1)
		smtpSecretFake.Data["host"] = []byte("host")
		smtpSecretFake.Data["port"] = []byte("8080")
		smtpSecretFake.Data["username"] = []byte("username")
		smtpSecretFake.Data["password"] = []byte("password")

		ocscsv := opv1a1.ClusterServiceVersion{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ocs-operator",
				Namespace: Namespace,
			},
		}

		BeforeEach(func() {
			newManagedMCG := managedMCG.DeepCopy()
			err := k8sClient.Create(context.Background(), newManagedMCG, &client.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			newManagedMCG := managedMCG.DeepCopy()
			err := k8sClient.Delete(context.TODO(), newManagedMCG, &client.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			err = k8sClient.DeleteAllOf(context.TODO(), &corev1.Secret{}, &client.DeleteAllOfOptions{
				ListOptions: client.ListOptions{
					Namespace: Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("should be able to read ManagedMCG object", func() {
			By("providing valid ManagedMCG name", func() {

				r := &controllers.ManagedMCGReconciler{
					Scheme: k8sClient.Scheme(),
				}

				r.Log = ctrl.Log.WithName("controllers").WithName("ManagedMCGFake")

				r.Client = fake.NewClientBuilder().WithScheme(r.Scheme).WithObjects(&ocscsv, &smtpSecretFake, &addonSecretFake, &deadMansSecretfake, &pagerDutySecretFake).Build()

				req := ctrl.Request{
					NamespacedName: types.NamespacedName{
						Name:      "managedmcg",
						Namespace: Namespace,
					},
				}

				_, err := r.Reconcile(context.TODO(), req)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
