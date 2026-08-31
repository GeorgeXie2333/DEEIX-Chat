package openapi

import (
	"context"
	"errors"
	"testing"
	"time"

	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateAndCASReplaceReturnExactlyWrittenAPIKey(t *testing.T) {
	db := openOpenAPIKeySQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	createdAt := time.Date(2026, 8, 31, 9, 30, 0, 0, time.UTC)

	created, err := repo.CreateForUser(ctx, &domainopenapi.UserAPIKey{
		UserID:                42,
		KeyHash:               "hash-initial",
		KeyPrefix:             "dxsk_initial",
		KeyPlaintextEncrypted: "cipher-initial",
		Status:                domainopenapi.APIKeyStatusActive,
		CreatedAt:             createdAt,
		UpdatedAt:             createdAt,
	})
	if err != nil {
		t.Fatalf("CreateForUser returned error: %v", err)
	}
	if created.ID == 0 || created.UserID != 42 || created.KeyHash != "hash-initial" || created.KeyPrefix != "dxsk_initial" || created.KeyPlaintextEncrypted != "cipher-initial" || created.Status != domainopenapi.APIKeyStatusActive {
		t.Fatalf("CreateForUser did not return the inserted row: %#v", created)
	}
	if !created.CreatedAt.Equal(createdAt) {
		t.Fatalf("CreateForUser created_at = %v, want %v", created.CreatedAt, createdAt)
	}

	updatedAt := createdAt.Add(time.Minute)
	replaced, err := repo.ReplaceForUserIfCurrent(ctx, &domainopenapi.UserAPIKey{
		UserID:                42,
		KeyHash:               "hash-replacement",
		KeyPrefix:             "dxsk_replacement",
		KeyPlaintextEncrypted: "cipher-replacement",
		Status:                domainopenapi.APIKeyStatusActive,
		UpdatedAt:             updatedAt,
	}, created)
	if err != nil {
		t.Fatalf("ReplaceForUserIfCurrent returned error: %v", err)
	}
	if replaced.ID != created.ID || replaced.UserID != 42 || replaced.KeyHash != "hash-replacement" || replaced.KeyPrefix != "dxsk_replacement" || replaced.KeyPlaintextEncrypted != "cipher-replacement" || replaced.Status != domainopenapi.APIKeyStatusActive || replaced.LastUsedAt != nil {
		t.Fatalf("ReplaceForUserIfCurrent did not return the updated row: %#v", replaced)
	}
	if !replaced.CreatedAt.Equal(created.CreatedAt) || !replaced.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("ReplaceForUserIfCurrent timestamps = created %v updated %v, want created %v updated %v", replaced.CreatedAt, replaced.UpdatedAt, created.CreatedAt, updatedAt)
	}

	var persisted model.OpenAPIKey
	if err := db.First(&persisted, "user_id = ?", 42).Error; err != nil {
		t.Fatalf("load persisted API key: %v", err)
	}
	if persisted.ID != replaced.ID || persisted.KeyHash != replaced.KeyHash || persisted.KeyPrefix != replaced.KeyPrefix || persisted.KeyPlaintextEncrypted != replaced.KeyPlaintextEncrypted {
		t.Fatalf("persisted row and RETURNING result diverged: persisted=%#v returned=%#v", persisted, replaced)
	}

	if _, err := repo.ReplaceForUserIfCurrent(ctx, &domainopenapi.UserAPIKey{
		UserID:                42,
		KeyHash:               "hash-stale",
		KeyPrefix:             "dxsk_stale",
		KeyPlaintextEncrypted: "cipher-stale",
		Status:                domainopenapi.APIKeyStatusActive,
		UpdatedAt:             updatedAt.Add(time.Minute),
	}, created); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected stale CAS replacement to conflict, got %v", err)
	}

	var final model.OpenAPIKey
	if err := db.First(&final, "user_id = ?", 42).Error; err != nil {
		t.Fatalf("load final API key: %v", err)
	}
	if final.KeyHash != replaced.KeyHash {
		t.Fatalf("stale CAS replacement overwrote key: got %q want %q", final.KeyHash, replaced.KeyHash)
	}
}

func TestCreateForUserMapsUniqueUserKeyToConflict(t *testing.T) {
	db := openOpenAPIKeySQLiteTestDB(t)
	repo := NewRepo(db)
	ctx := context.Background()
	base := domainopenapi.UserAPIKey{
		UserID:                42,
		KeyPrefix:             "dxsk_key",
		KeyPlaintextEncrypted: "cipher",
		Status:                domainopenapi.APIKeyStatusActive,
	}
	first := base
	first.KeyHash = "hash-one"
	if _, err := repo.CreateForUser(ctx, &first); err != nil {
		t.Fatalf("first CreateForUser returned error: %v", err)
	}
	second := base
	second.KeyHash = "hash-two"
	if _, err := repo.CreateForUser(ctx, &second); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("second CreateForUser error = %v, want ErrConflict", err)
	}
}

func openOpenAPIKeySQLiteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.OpenAPIKey{}); err != nil {
		t.Fatalf("migrate openapi keys: %v", err)
	}
	return db
}
