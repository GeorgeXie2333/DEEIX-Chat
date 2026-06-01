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
	ReplaceForUser(ctx context.Context, item *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error)
	RevokeForUser(ctx context.Context, userID uint, now time.Time) (*domainopenapi.UserAPIKey, error)
	TouchLastUsedAt(ctx context.Context, id uint, at time.Time) error
}
