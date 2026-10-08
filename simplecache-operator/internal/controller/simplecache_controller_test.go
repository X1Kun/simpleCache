/*
Copyright 2026.

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

package controller

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	cachev1 "x1kun.com/simplecache-operator/api/v1"
)

var _ = Describe("SimpleCache Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		simplecache := &cachev1.SimpleCache{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind SimpleCache")
			err := k8sClient.Get(ctx, typeNamespacedName, simplecache)
			if err != nil && errors.IsNotFound(err) {
				resource := &cachev1.SimpleCache{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: cachev1.SimpleCacheSpec{Size: 3, Image: "simplecache:envtest"},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
			Expect(k8sClient.Get(ctx, typeNamespacedName, simplecache)).To(Succeed())
		})

		AfterEach(func() {
			// Envtest does not run garbage collection controllers; remove children explicitly.
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName, Namespace: "default"},
			}))).To(Succeed())
			Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: resourceName + "-svc", Namespace: "default"},
			}))).To(Succeed())
			resource := &cachev1.SimpleCache{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance SimpleCache")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &SimpleCacheReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			var service corev1.Service
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-svc", Namespace: "default"}, &service)).To(Succeed())
			Expect(service.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
			Expect(service.Spec.Selector).To(Equal(map[string]string{"app": resourceName}))
			Expect(metav1.IsControlledBy(&service, simplecache)).To(BeTrue())

			var sts appsv1.StatefulSet
			Expect(k8sClient.Get(ctx, typeNamespacedName, &sts)).To(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(3)))
			Expect(sts.Spec.ServiceName).To(Equal(resourceName + "-svc"))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("simplecache:envtest"))
			Expect(metav1.IsControlledBy(&sts, simplecache)).To(BeTrue())

			By("reconciling the same desired state without rewriting resources")
			version := sts.ResourceVersion
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, &sts)).To(Succeed())
			Expect(sts.ResourceVersion).To(Equal(version))

			By("updating the baseline static peer list and image when replicas change")
			Expect(k8sClient.Get(ctx, typeNamespacedName, simplecache)).To(Succeed())
			simplecache.Spec.Size = 4
			simplecache.Spec.Image = "simplecache:envtest-v2"
			Expect(k8sClient.Update(ctx, simplecache)).To(Succeed())
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sClient.Get(ctx, typeNamespacedName, &sts)).To(Succeed())
			Expect(*sts.Spec.Replicas).To(Equal(int32(4)))
			Expect(sts.Spec.Template.Spec.Containers[0].Image).To(Equal("simplecache:envtest-v2"))
			var peers string
			for _, env := range sts.Spec.Template.Spec.Containers[0].Env {
				if env.Name == "PEERS" {
					peers = env.Value
				}
			}
			Expect(strings.Split(peers, ",")).To(HaveLen(4))
			Expect(peers).To(ContainSubstring(resourceName + "-3."))
		})

		It("should ignore a missing primary resource", func() {
			reconciler := &SimpleCacheReconciler{Client: k8sClient, Scheme: k8sClient.Scheme()}
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "missing-cache", Namespace: "default"}})
			Expect(err).NotTo(HaveOccurred())
			var sts appsv1.StatefulSet
			Expect(errors.IsNotFound(k8sClient.Get(ctx, types.NamespacedName{Name: "missing-cache", Namespace: "default"}, &sts))).To(BeTrue())
		})
	})
})
