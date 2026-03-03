package memory

import (
	"context"
	"fmt"
	"sync"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
)

// Store implements an in-memory storage backend for policy data.
// It uses sync.RWMutex to provide thread-safe access to the data.
type Store struct {
	mu     sync.RWMutex
	users  map[string]*v1alpha1.UserSpec
	models map[string]*v1alpha1.ModelSpec
}

// New creates a new in-memory store.
func New() *Store {
	return &Store{
		users:  make(map[string]*v1alpha1.UserSpec),
		models: make(map[string]*v1alpha1.ModelSpec),
	}
}

// User operations

func (s *Store) GetUser(ctx context.Context, id string) (*v1alpha1.UserSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user not found: %s", id)
	}

	// Return a deep copy to prevent external modifications
	return user.DeepCopy(), nil
}

func (s *Store) ListUsers(ctx context.Context) ([]v1alpha1.UserSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]v1alpha1.UserSpec, 0, len(s.users))
	for _, user := range s.users {
		var c v1alpha1.UserSpec
		user.DeepCopyInto(&c)
		users = append(users, c)
	}
	return users, nil
}

func (s *Store) SyncUser(ctx context.Context, user *v1alpha1.UserSpec) error {
	if user == nil {
		return fmt.Errorf("user cannot be nil")
	}
	if user.Id == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a deep copy to prevent external modifications
	s.users[user.Id] = user.DeepCopy()
	return nil
}

func (s *Store) DeleteUser(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.users, id)
	return nil
}

func (s *Store) UserExists(ctx context.Context, id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.users[id]
	return ok, nil
}

// Model operations

func (s *Store) GetModel(ctx context.Context, id string) (*v1alpha1.ModelSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	model, ok := s.models[id]
	if !ok {
		return nil, fmt.Errorf("model not found: %s", id)
	}

	// Return a deep copy to prevent external modifications
	return model.DeepCopy(), nil
}

func (s *Store) ListModels(ctx context.Context) ([]v1alpha1.ModelSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]v1alpha1.ModelSpec, 0, len(s.models))
	for _, model := range s.models {
		var c v1alpha1.ModelSpec
		model.DeepCopyInto(&c)
		models = append(models, c)
	}
	return models, nil
}

func (s *Store) ListModelsByType(ctx context.Context, modelType string) ([]v1alpha1.ModelSpec, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	models := make([]v1alpha1.ModelSpec, 0)
	for _, model := range s.models {
		if model.Type == modelType {
			var c v1alpha1.ModelSpec
			model.DeepCopyInto(&c)
			models = append(models, c)
		}
	}
	return models, nil
}

func (s *Store) SyncModel(ctx context.Context, model *v1alpha1.ModelSpec) error {
	if model == nil {
		return fmt.Errorf("model cannot be nil")
	}
	if model.Id == "" {
		return fmt.Errorf("model id cannot be empty")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Store a deep copy to prevent external modifications
	s.models[model.Id] = model.DeepCopy()
	return nil
}

func (s *Store) DeleteModel(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.models, id)
	return nil
}

func (s *Store) ModelExists(ctx context.Context, id string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.models[id]
	return ok, nil
}

// Close implements the Store interface. For in-memory store, this is a no-op.
func (s *Store) Close() error {
	return nil
}
