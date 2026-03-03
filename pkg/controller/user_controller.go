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
	"fmt"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	accesscontrolllmdiov1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
	"github.com/llm-d-incubation/secure-inference/pkg/store"
)

const userFinalizer = "user.accesscontrol.llm-d.io/finalizer"

// UserReconciler reconciles a User object.
type UserReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Store  store.Store
	Synced *atomic.Bool
}

// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=users,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=users/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=users/finalizers,verbs=update

func (r *UserReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	user := &accesscontrolllmdiov1alpha1.User{}
	err := r.Get(ctx, req.NamespacedName, user)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("user resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get user resource")
		return ctrl.Result{}, err
	}

	isMarkedForDeletion := user.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if controllerutil.ContainsFinalizer(user, userFinalizer) {
			logger.Info("Finalizer is present. Handling cleanup.")

			if err = r.Store.DeleteUser(ctx, user.Spec.Id); err != nil {
				logger.Error(err, "Failed to delete user from store")
				return ctrl.Result{}, fmt.Errorf("failed to delete user: %w", err)
			}

			controllerutil.RemoveFinalizer(user, userFinalizer)
			err = r.Update(ctx, user)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(user, userFinalizer) {
		controllerutil.AddFinalizer(user, userFinalizer)
		err = r.Update(ctx, user)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync user to store
	if err = r.Store.SyncUser(ctx, &user.Spec); err != nil {
		logger.Error(err, "Failed to sync user to store")
		return ctrl.Result{}, fmt.Errorf("failed to sync user: %w", err)
	}
	r.Synced.Store(true)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *UserReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accesscontrolllmdiov1alpha1.User{}).
		Named("user").
		Complete(r)
}
