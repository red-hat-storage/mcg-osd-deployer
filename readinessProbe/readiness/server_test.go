package readiness

import (
	_ "github.com/onsi/ginkgo"
	_ "github.com/onsi/gomega"
)

/*var _ = Describe("ManagedMCG readiness probe behavior", func() {
	ctx := context.Background()

	managedMCG := &v1.ManagedMCG{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ManagedMCFName,
			Namespace: TestNamespace,
		},
	}

	setupReadinessConditions := func(
		storageClusterReady bool,
		prometheusReady bool,
		alertmanagerReady bool,
	) error {

		var noobaaeStatus, prometheusStatus, alertmanagerStatus v1.ComponentState

		if storageClusterReady == true {
			noobaaeStatus = v1.ComponentReady
		} else {
			noobaaeStatus = v1.ComponentPending
		}

		if prometheusReady == true {
			prometheusStatus = v1.ComponentReady
		} else {
			prometheusStatus = v1.ComponentPending
		}

		if alertmanagerReady == true {
			alertmanagerStatus = v1.ComponentReady
		} else {
			alertmanagerStatus = v1.ComponentPending
		}

		if err := k8sClient.Get(ctx, utils.GetResourceKey(managedMCG), managedMCG); err != nil {
			return err
		}

		managedMCG.Status = v1.ManagedMCGStatus{
			Components: v1.ComponentStatusMap{
				StorageCluster: v1.ComponentStatus{
					State: noobaaeStatus,
				},
				Prometheus: v1.ComponentStatus{
					State: prometheusStatus,
				},
				Alertmanager: v1.ComponentStatus{
					State: alertmanagerStatus,
				},
			},
		}

		return k8sClient.Status().Update(ctx, managedMCG)
	}

	Context("Readiness Probe", func() {
		When("the managedmcg resource lists its StorageCluster as not \"ready\"", func() {
			It("should cause the readiness probe to return StatusServiceUnavailable", func() {
				Expect(setupReadinessConditions(false, true, true)).Should(Succeed())

				status, err := utils.ProbeReadiness()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(http.StatusServiceUnavailable))
			})
		})

		When("managedmcg reports Prometheus as not \"ready\"", func() {
			It("should cause the readiness probe to return StatusServiceUnavailable", func() {
				Expect(setupReadinessConditions(true, false, true)).Should(Succeed())

				status, err := utils.ProbeReadiness()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(http.StatusServiceUnavailable))
			})
		})

		When("managedmcg reports Alertmanager as not \"ready\"", func() {
			It("should cause the readiness probe to return StatusServiceUnavailable", func() {
				Expect(setupReadinessConditions(true, true, false)).Should(Succeed())

				status, err := utils.ProbeReadiness()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(http.StatusServiceUnavailable))
			})
		})

		When("managedmcg reports all its components as \"ready\"", func() {
			It("should cause the readiness probe to return StatusOK", func() {
				Expect(setupReadinessConditions(true, true, true)).Should(Succeed())

				status, err := utils.ProbeReadiness()
				Expect(err).ToNot(HaveOccurred())
				Expect(status).To(Equal(http.StatusOK))
			})
		})

	})
})
*/
