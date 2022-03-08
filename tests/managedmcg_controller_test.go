package tests

import (
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	managedmcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	Namespace = "openshift-storage"
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
	BeforeEach(func() {
		namespace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: Namespace,
			},
		}
		err := k8sClient.Create(context.Background(), namespace, &client.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())
	})
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
