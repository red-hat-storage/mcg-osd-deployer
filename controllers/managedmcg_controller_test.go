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
	"testing"

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	noobaa "github.com/noobaa/noobaa-operator/v5/pkg/apis"
	operatorv1 "github.com/openshift/api/operator/v1"
	opv1a1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	mcgv1alpha1 "github.com/red-hat-storage/mcg-osd-deployer/api/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func newSchemeFake() *runtime.Scheme {
	schemeFake := runtime.NewScheme()
	schemes := []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		noobaa.AddToScheme,
		opv1a1.AddToScheme,
		mcgv1alpha1.AddToScheme,
		operatorv1.Install,
	}
	for _, f := range schemes {
		if err := f(schemeFake); err != nil {
			panic(err)
		}
	}

	return schemeFake
}

func newODFCSVFake() opv1a1.ClusterServiceVersion {
	ODFCSVFake := opv1a1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "odf-operator",
			Namespace: "openshift-storage",
		},
	}
	ODFCSVFake.Spec.InstallStrategy.StrategySpec.DeploymentSpecs = []opv1a1.StrategyDeploymentSpec{
		{
			Spec: v1.DeploymentSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name: "manager",
							},
						},
					},
				},
			},
		},
	}

	return ODFCSVFake
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

func TestManagedMCGReconcilerReconcile(t *testing.T) {
	r := &ManagedMCGReconciler{}
	r.Log = ctrl.Log.WithName("controllers").WithName("ManagedMCGFake")
	r.Scheme = newSchemeFake()
	r.initializeReconciler(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "managedmcgfake",
			Namespace: "openshift-storage",
		},
	})
	ODFCSVFake := newODFCSVFake()
	managedMCGFake := newManagedMCGFake()
	fakeClient := fake.NewClientBuilder().WithScheme(r.Scheme).WithObjects(&ODFCSVFake, &managedMCGFake).Build()
	r.Client = fakeClient

	r.Log.Info("Reconciling ManagedMCG object")
	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      "managedmcgfake",
			Namespace: "openshift-storage",
		},
	})
	if err != nil {
		t.Errorf("ManagedMCGReconciler.Reconcile() error: %s", err)
	}
}
