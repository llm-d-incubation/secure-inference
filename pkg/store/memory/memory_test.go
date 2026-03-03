package memory

import (
	"context"
	"testing"

	v1alpha1 "github.com/llm-d-incubation/secure-inference/api/v1alpha1"
)

func TestMemoryStore_User(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Test SyncUser
	user := &v1alpha1.UserSpec{
		Id: "alice",
		Attributes: map[string]string{
			"role": "admin",
		},
	}

	err := store.SyncUser(ctx, user)
	if err != nil {
		t.Fatalf("SyncUser failed: %v", err)
	}

	// Test GetUser
	retrieved, err := store.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Id != "alice" || retrieved.Attributes["role"] != "admin" {
		t.Errorf("Retrieved user mismatch: %+v", retrieved)
	}

	// Test UserExists
	exists, err := store.UserExists(ctx, "alice")
	if err != nil || !exists {
		t.Error("UserExists should return true for alice")
	}

	// Test ListUsers
	users, err := store.ListUsers(ctx)
	if err != nil || len(users) != 1 {
		t.Errorf("ListUsers failed: expected 1 user, got %d", len(users))
	}

	// Test DeleteUser
	err = store.DeleteUser(ctx, "alice")
	if err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}

	exists, err = store.UserExists(ctx, "alice")
	if err != nil {
		t.Fatalf("UserExists failed: %v", err)
	}
	if exists {
		t.Error("User should not exist after deletion")
	}
}

func TestMemoryStore_Model(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Test SyncModel
	model := &v1alpha1.ModelSpec{
		Id:   "llama-3",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"admin", "user"},
			},
		},
	}

	err := store.SyncModel(ctx, model)
	if err != nil {
		t.Fatalf("SyncModel failed: %v", err)
	}

	// Test GetModel
	retrieved, err := store.GetModel(ctx, "llama-3")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	if retrieved.Id != "llama-3" || retrieved.Type != v1alpha1.ModelTypeBase {
		t.Errorf("Retrieved model mismatch: %+v", retrieved)
	}

	// Test ModelExists
	exists, err := store.ModelExists(ctx, "llama-3")
	if err != nil || !exists {
		t.Error("ModelExists should return true for llama-3")
	}

	// Test ListModels
	models, err := store.ListModels(ctx)
	if err != nil || len(models) != 1 {
		t.Errorf("ListModels failed: expected 1 model, got %d", len(models))
	}

	// Test ListModelsByType
	baseModels, err := store.ListModelsByType(ctx, v1alpha1.ModelTypeBase)
	if err != nil || len(baseModels) != 1 {
		t.Errorf("ListModelsByType failed: expected 1 BaseModel, got %d", len(baseModels))
	}

	loras, err := store.ListModelsByType(ctx, v1alpha1.ModelTypeLora)
	if err != nil || len(loras) != 0 {
		t.Errorf("ListModelsByType failed: expected 0 lora, got %d", len(loras))
	}

	// Test DeleteModel
	err = store.DeleteModel(ctx, "llama-3")
	if err != nil {
		t.Fatalf("DeleteModel failed: %v", err)
	}

	exists, err = store.ModelExists(ctx, "llama-3")
	if err != nil {
		t.Fatalf("ModelExists failed: %v", err)
	}
	if exists {
		t.Error("Model should not exist after deletion")
	}
}

func TestMemoryStore_Concurrency(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Concurrent writes and reads
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			user := &v1alpha1.UserSpec{
				Id: "user-concurrent",
				Attributes: map[string]string{
					"role": "test",
				},
			}
			_ = store.SyncUser(ctx, user)
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			_, _ = store.GetUser(ctx, "user-concurrent")
			_, _ = store.ListUsers(ctx)
		}
		done <- true
	}()

	<-done
	<-done

	// Should not panic or race
}

func TestMemoryStore_DeepCopyIsolation(t *testing.T) {
	ctx := context.Background()
	store := New()

	// Test user deep copy isolation
	user := &v1alpha1.UserSpec{
		Id:         "alice",
		Attributes: map[string]string{"role": "admin"},
	}
	if err := store.SyncUser(ctx, user); err != nil {
		t.Fatalf("SyncUser failed: %v", err)
	}

	// Mutate the original — should not affect stored copy
	user.Attributes["role"] = "hacked"

	retrieved, err := store.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Attributes["role"] != "admin" {
		t.Errorf("Store was mutated via original pointer: got role=%s, want admin", retrieved.Attributes["role"])
	}

	// Mutate the retrieved copy — should not affect stored copy
	retrieved.Attributes["role"] = "mutated"

	retrieved2, err := store.GetUser(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved2.Attributes["role"] != "admin" {
		t.Errorf("Store was mutated via retrieved pointer: got role=%s, want admin", retrieved2.Attributes["role"])
	}

	// Test model deep copy isolation
	model := &v1alpha1.ModelSpec{
		Id:   "test-model",
		Type: v1alpha1.ModelTypeBase,
		AccessPolicy: v1alpha1.ModelAccessPolicy{
			UserAttributes: map[string][]string{
				"role": {"admin", "user"},
			},
		},
	}
	if err := store.SyncModel(ctx, model); err != nil {
		t.Fatalf("SyncModel failed: %v", err)
	}

	// Mutate original's nested map — should not affect stored copy
	model.AccessPolicy.UserAttributes["role"] = []string{"hacked"}

	retrievedModel, err := store.GetModel(ctx, "test-model")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	roles := retrievedModel.AccessPolicy.UserAttributes["role"]
	if len(roles) != 2 || roles[0] != "admin" || roles[1] != "user" {
		t.Errorf("Store model was mutated via original pointer: got roles=%v, want [admin user]", roles)
	}

	// Mutate retrieved model's nested map — should not affect stored copy
	retrievedModel.AccessPolicy.UserAttributes["role"] = []string{"mutated"}

	retrievedModel2, err := store.GetModel(ctx, "test-model")
	if err != nil {
		t.Fatalf("GetModel failed: %v", err)
	}
	roles2 := retrievedModel2.AccessPolicy.UserAttributes["role"]
	if len(roles2) != 2 || roles2[0] != "admin" || roles2[1] != "user" {
		t.Errorf("Store model was mutated via retrieved pointer: got roles=%v, want [admin user]", roles2)
	}
}
