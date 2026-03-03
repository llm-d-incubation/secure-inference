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

const modelFinalizer = "model.accesscontrol.llm-d.io/finalizer"

// ModelReconciler reconciles a Model object.
type ModelReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Store  store.Store
	Synced *atomic.Bool
}

// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=models,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=models/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=accesscontrol.llm-d.io,resources=models/finalizers,verbs=update

func (r *ModelReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)

	model := &accesscontrolllmdiov1alpha1.Model{}
	err := r.Get(ctx, req.NamespacedName, model)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("model resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get model resource")
		return ctrl.Result{}, err
	}

	isMarkedForDeletion := model.GetDeletionTimestamp() != nil
	if isMarkedForDeletion {
		if controllerutil.ContainsFinalizer(model, modelFinalizer) {
			logger.Info("Finalizer is present. Handling cleanup.")

			if err = r.Store.DeleteModel(ctx, model.Spec.Id); err != nil {
				logger.Error(err, "Failed to delete model from store")
				return ctrl.Result{}, fmt.Errorf("failed to delete model: %w", err)
			}

			controllerutil.RemoveFinalizer(model, modelFinalizer)
			err = r.Update(ctx, model)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(model, modelFinalizer) {
		controllerutil.AddFinalizer(model, modelFinalizer)
		err = r.Update(ctx, model)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// Sync model to store
	if err = r.Store.SyncModel(ctx, &model.Spec); err != nil {
		logger.Error(err, "Failed to sync model to store")
		return ctrl.Result{}, fmt.Errorf("failed to sync model: %w", err)
	}
	r.Synced.Store(true)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ModelReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&accesscontrolllmdiov1alpha1.Model{}).
		Named("model").
		Complete(r)
}
