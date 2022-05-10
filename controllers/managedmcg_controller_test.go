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
	"context"
	"os"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis"
	operatorv1 "github.com/openshift/api/operator/v1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promv1a1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	CustomerNotificationHTMLPath = "/tmp/customernotification.html"
)

func newSchemeFake() *runtime.Scheme {
	schemeFake := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(schemeFake))
	utilruntime.Must(mcgv1alpha1.AddToScheme(schemeFake))
	utilruntime.Must(noobaa.AddToScheme(schemeFake))
	utilruntime.Must(opv1a1.AddToScheme(schemeFake))
	utilruntime.Must(operatorv1.Install(schemeFake))
	utilruntime.Must(promv1.AddToScheme(schemeFake))
	utilruntime.Must(promv1a1.AddToScheme(schemeFake))

	return schemeFake
}

func newManagedMCGFake() mcgv1alpha1.ManagedMCG {
	managedMCGFake := mcgv1alpha1.ManagedMCG{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "managedmcgfake",
			Namespace: "openshift-storage",
			UID:       "fake-uid",
		},
		Spec: mcgv1alpha1.ManagedMCGSpec{
			ReconcileStrategy: "ignore",
		},
	}

	return managedMCGFake
}

func newAddonSecretFake() corev1.Secret {
	secretFake := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "AddOnSecretfake",
			Namespace: "openshift-storage",
		},
	}
	secretFake.Data = make(map[string][]byte, 1)
	secretFake.Data["addonparam"] = []byte("foo")

	return secretFake
}

func newPagerDutySecretFake() corev1.Secret {
	secretFake := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "PagerDutySecretfake",
			Namespace: "openshift-storage",
		},
	}
	secretFake.Data = make(map[string][]byte, 1)
	secretFake.Data["PAGERDUTY_KEY"] = []byte("foo")

	return secretFake
}

func newDeadMansSecretFake() corev1.Secret {
	secretFake := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "DeadMansSecretfake",
			Namespace: "openshift-storage",
		},
	}
	secretFake.Data = make(map[string][]byte, 1)
	secretFake.Data["SNITCH_URL"] = []byte("foo")

	return secretFake
}

func newSMTPSecretFake() corev1.Secret {
	secretFake := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "SmtpSecretfake",
			Namespace: "openshift-storage",
		},
	}
	secretFake.Data = make(map[string][]byte, 1)
	secretFake.Data["host"] = []byte("host")
	secretFake.Data["port"] = []byte("8080")
	secretFake.Data["username"] = []byte("username")
	secretFake.Data["password"] = []byte("password")

	return secretFake
}

func newOcsCsvFake() opv1a1.ClusterServiceVersion {
	ocscsv := opv1a1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ocs-operator",
			Namespace: "openshift-storage",
		},
	}

	return ocscsv
}

// Remove extraneous artifacts after tests are executed.
func cleanup() {
	os.Remove(CustomerNotificationHTMLPath)
}

func TestManagedMCGReconcilerReconcile(t *testing.T) {
	r := &ManagedMCGReconciler{}
	r.Log = ctrl.Log.WithName("controllers").WithName("ManagedMCGFake")

	r.Scheme = newSchemeFake()
	ocscsvFake := newOcsCsvFake()
	managedMCGFake := newManagedMCGFake()
	addonsecretFake := newAddonSecretFake()
	smtpsecretfake := newSMTPSecretFake()
	pagerdutysecretFake := newPagerDutySecretFake()
	deadmansercretFake := newDeadMansSecretFake()

	r.AddonParamSecretName = "AddOnSecretfake"
	r.DeadMansSnitchSecretName = "DeadMansSecretfake"
	r.PagerdutySecretName = "PagerDutySecretfake"
	r.SMTPSecretName = "SmtpSecretfake"
	r.CustomerNotificationHTMLPath = CustomerNotificationHTMLPath

	data := []byte{}
	err := os.WriteFile(CustomerNotificationHTMLPath, data, 0o444)
	if err != nil {
		t.Errorf("Can not create file : %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(r.Scheme).WithObjects(&ocscsvFake,
		&smtpsecretfake, &addonsecretFake, &deadmansercretFake, &pagerdutysecretFake, &managedMCGFake).Build()

	r.Client = fakeClient
	r.Log.Info("Reconciling ManagedMCG object")
	_, err = r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "managedmcgfake",
			Namespace: "openshift-storage",
		},
	})
	if err != nil {
		t.Errorf("ManagedMCGReconciler.Reconcile() error: %v", err)
	}

	t.Setenv("NOOBAA_CORE_IMAGE", "coreimage")
	t.Setenv("NOOBAA_DB_IMAGE", "dbimage")
	err = r.lookupImages()
	if err != nil || r.images.NooBaaCore != "coreimage" || r.images.NooBaaDB != "dbimage" {
		t.Errorf("Failed to lookup images: %v", err)
	}

	r.managedMCG.UID = ""
	_, err = r.reconcilePhases()
	if err != nil {
		r.Log.Error(err, "error reconciling ManagedMCG")
	}

	if err := r.removeNoobaa(); err != nil {
		t.Errorf("Error while removing Nooba: %v", err)
	}

	if err := r.removeOLMComponents(); err != nil {
		t.Errorf("Error while removing OLM: %v", err)
	}

	cleanup()
}
