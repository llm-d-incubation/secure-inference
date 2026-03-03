/*
Copyright 2025.

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
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	accesscontrolllmdiov1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
)

var _ = Describe("User Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		createUser := func(name string) types.NamespacedName {
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			resource := &accesscontrolllmdiov1alpha1.User{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: "default",
				},
				Spec: accesscontrolllmdiov1alpha1.UserSpec{
					Id: name,
					Attributes: map[string]string{
						"role": "test_role",
					},
				},
			}
			Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			return nn
		}

		newReconciler := func() *UserReconciler {
			store := newTestStore()
			var synced atomic.Bool
			return &UserReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
				Store:  store,
				Synced: &synced,
			}
		}

		It("should successfully reconcile the resource", func() {
			nn := createUser("user-reconcile")
			r := newReconciler()

			By("Reconciling the created resource")
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			// Verify the user was synced to the store
			exists, err := r.Store.UserExists(ctx, nn.Name)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("should add a finalizer on first reconcile", func() {
			nn := createUser("user-finalizer")
			r := newReconciler()

			By("Reconciling the created resource")
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())

			// Verify finalizer is set
			updatedUser := &accesscontrolllmdiov1alpha1.User{}
			err = k8sClient.Get(ctx, nn, updatedUser)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedUser.Finalizers).To(ContainElement(userFinalizer))
		})
	})
})
