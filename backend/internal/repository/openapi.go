package repository

import (
	"context"
	"time"

	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
)

// OpenAPIKeyRepository 定义开放 API Key 的持久化能力。
type OpenAPIKeyRepository interface {
	GetByUserID(ctx context.Context, userID uint) (*domainopenapi.UserAPIKey, error)
	GetActiveByHash(ctx context.Context, hash string) (*domainopenapi.UserAPIKey, error)
	// CreateForUser atomically creates the user's only API key. It returns
	// ErrConflict when a key already exists for that user.
	CreateForUser(ctx context.Context, item *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error)
	// ReplaceForUserIfCurrent atomically replaces a key only if the persisted
	// key still has the expected hash and status. It returns ErrConflict when a
	// concurrent create, regeneration, or revocation changed it first.
	ReplaceForUserIfCurrent(ctx context.Context, item *domainopenapi.UserAPIKey, expected *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error)
	RevokeForUser(ctx context.Context, userID uint, now time.Time) (*domainopenapi.UserAPIKey, error)
	TouchLastUsedAt(ctx context.Context, id uint, at time.Time) error
}
