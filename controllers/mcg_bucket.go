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
	"strings"

	"github.com/kube-object-storage/lib-bucket-provisioner/pkg/apis/objectbucket.io/v1alpha1"
	noobaav1alpha1 "github.com/noobaa/noobaa-operator/v5/pkg/apis/noobaa/v1alpha1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	McgmsObcNamespace   = "mcgms-obc-namespace"
	McgmsCacheEnabled   = "mcgms-cache-enabled"
	DefaultBackingStore = "noobaa-default-backing-store"
)

func (r *ManagedMCGReconciler) watchBucketClass(object client.Object) {
	bucketName := object.GetName()
	annotations := object.GetAnnotations()
	if _, ok := annotations[McgmsObcNamespace]; ok {
		r.objectBucketClaim = &noobaav1alpha1.ObjectBucketClaim{}
		r.bucketClass = &noobaav1alpha1.BucketClass{}
		r.ctx = context.Background()
		if r.isOBCExists(object) {
			r.Log.Info("OBC already exists", "name", bucketName)

			return
		}

		obc := r.setOBCDesiredState(bucketName, object)
		_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, r.objectBucketClaim, func() error {
			r.Log.Info("Creating OBC ", "name", bucketName)
			r.objectBucketClaim.Spec = obc.Spec

			return nil
		})
		if err != nil {
			r.Log.Error(err, "Error while creating OBC, OBC not created")

			return
		}
		annotations := object.GetAnnotations()
		if isCacheEnabled, ok := annotations[McgmsCacheEnabled]; ok && isCacheEnabled == "true" {
			r.reconcileCacheBucketClass(object)
		}
	}
}

func (r *ManagedMCGReconciler) reconcileCacheBucketClass(object client.Object) {
	r.Log.Info("Create Cache bucketClass for cache enabled namespacestores", "name", object.GetName())
	bucketClass := r.setCacheBucketClassDesiredState(object)
	if bucketClass == nil {
		r.Log.Info("No Cache bucketClass returned for Cache enabled Bucket", "name", bucketClass.Name)

		return
	}
	cacheBucketClassAnnotations := make(map[string]string)
	cacheBucketClassAnnotations["cache-bucketclass"] = "true"
	bucketClass.Name = object.GetName() + "-cache"
	bucketClass.Namespace = r.namespace
	bucketClass.ObjectMeta.Annotations = cacheBucketClassAnnotations
	_, err := ctrl.CreateOrUpdate(r.ctx, r.Client, bucketClass, func() error {
		r.Log.Info("creating/updating bucketClass CR", "name", bucketClass.Name)

		return nil
	})
	if err != nil {
		r.Log.Error(err, "Error while creating bucketClass, bucketClass not created")
	}
}

func (r *ManagedMCGReconciler) setCacheBucketClassDesiredState(object client.Object) *noobaav1alpha1.BucketClass {
	basebucketClass, ok := object.(*noobaav1alpha1.BucketClass)
	if !ok {
		return nil
	}
	r.Log.Info("Configure spec for Cache bucketClass", "name", basebucketClass.GetName())
	defaultBackingStore := r.getDefaultBackingStore()
	if defaultBackingStore == "" {
		return nil
	}
	backingStoreName := defaultBackingStore
	backingStores := []noobaav1alpha1.BackingStoreName{backingStoreName}
	tier := noobaav1alpha1.Tier{
		BackingStores: backingStores,
	}
	bucketClass := &noobaav1alpha1.BucketClass{
		Spec: noobaav1alpha1.BucketClassSpec{
			NamespacePolicy: &noobaav1alpha1.NamespacePolicy{
				Type: noobaav1alpha1.NSBucketClassTypeCache,
				Cache: &noobaav1alpha1.CacheNamespacePolicy{
					HubResource: basebucketClass.Spec.NamespacePolicy.Single.Resource,
					Caching: &noobaav1alpha1.CacheSpec{
						TTL: 3600000,
					},
				},
			},
			PlacementPolicy: &noobaav1alpha1.PlacementPolicy{
				Tiers: []noobaav1alpha1.Tier{
					tier,
				},
			},
		},
	}

	return bucketClass
}

func (r *ManagedMCGReconciler) getDefaultBackingStore() string {
	backingStores := noobaav1alpha1.BackingStoreList{}
	if err := r.list(&backingStores); err != nil {
		r.Log.Error(err, "error getting BackingStore list")

		return ""
	}
	for _, backingStore := range backingStores.Items {
		if strings.HasPrefix(backingStore.Name, DefaultBackingStore) {
			return DefaultBackingStore
		}
	}

	return backingStores.Items[0].Name
}

func (r *ManagedMCGReconciler) isOBCExists(object client.Object) bool {
	objectBucketClaims := noobaav1alpha1.ObjectBucketClaimList{}
	if err := r.list(&objectBucketClaims); err != nil {
		r.Log.Error(err, "error getting BackingStore list")

		return false
	}
	for _, objectBucketClaim := range objectBucketClaims.Items {
		if objectBucketClaim.Name == object.GetName() {
			return true
		}
	}

	return false
}

func (r *ManagedMCGReconciler) setOBCDesiredState(bucketName string, object client.Object) v1alpha1.ObjectBucketClaim {
	AdditionalConfigMap := make(map[string]string)
	AdditionalConfigMap["bucketclass"] = bucketName
	obc := noobaav1alpha1.ObjectBucketClaim{
		Spec: noobaav1alpha1.ObjectBucketClaimSpec{
			BucketName:         bucketName,
			StorageClassName:   r.namespace + ".noobaa.io",
			GenerateBucketName: bucketName,
			AdditionalConfig:   AdditionalConfigMap,
		},
	}
	r.objectBucketClaim.Name = bucketName
	r.objectBucketClaim.Namespace = r.getOBCCreationNamespace(object)

	return obc
}

func (r *ManagedMCGReconciler) getOBCCreationNamespace(object client.Object) string {
	if obcNamespace, ok := object.GetAnnotations()[McgmsObcNamespace]; ok {
		return obcNamespace
	}

	return object.GetNamespace()
}
