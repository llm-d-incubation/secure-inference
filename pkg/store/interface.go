package store

import (
	"context"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
)

// ReadStore defines read-only access to policy data storage.
// Use this in components that only need to look up users and models.
type ReadStore interface {
	// User operations
	GetUser(ctx context.Context, id string) (*v1alpha1.UserSpec, error)
	ListUsers(ctx context.Context) ([]v1alpha1.UserSpec, error)
	UserExists(ctx context.Context, id string) (bool, error)

	// Model operations
	GetModel(ctx context.Context, id string) (*v1alpha1.ModelSpec, error)
	ListModels(ctx context.Context) ([]v1alpha1.ModelSpec, error)
	ListModelsByType(ctx context.Context, modelType string) ([]v1alpha1.ModelSpec, error)
	ModelExists(ctx context.Context, id string) (bool, error)
}

// Store defines the full interface for policy data storage.
// This abstraction allows swapping storage backends without changing consumers.
type Store interface {
	ReadStore

	SyncUser(ctx context.Context, user *v1alpha1.UserSpec) error
	DeleteUser(ctx context.Context, id string) error

	SyncModel(ctx context.Context, model *v1alpha1.ModelSpec) error
	DeleteModel(ctx context.Context, id string) error

	// Lifecycle
	Close() error
}
