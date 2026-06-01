package openapi

import (
	"context"
	"errors"
	"time"

	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/persistence/models"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repo 封装 openapi_keys 数据访问。
type Repo struct {
	db *gorm.DB
}

// NewRepo 创建仓储。
func NewRepo(db *gorm.DB) *Repo {
	return &Repo{db: db}
}

func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return repository.ErrNotFound
	}
	return err
}

// GetByUserID 查询用户唯一开放 API Key。
func (r *Repo) GetByUserID(ctx context.Context, userID uint) (*domainopenapi.UserAPIKey, error) {
	var item model.OpenAPIKey
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomain(item), nil
}

// GetActiveByHash 按哈希查询有效 API Key。
func (r *Repo) GetActiveByHash(ctx context.Context, hash string) (*domainopenapi.UserAPIKey, error) {
	var item model.OpenAPIKey
	if err := r.db.WithContext(ctx).
		Where("key_hash = ? AND status = ?", hash, domainopenapi.APIKeyStatusActive).
		First(&item).Error; err != nil {
		return nil, translateError(err)
	}
	return toDomain(item), nil
}

// ReplaceForUser 创建或替换用户唯一 API Key。
func (r *Repo) ReplaceForUser(ctx context.Context, item *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	if item == nil {
		return nil, repository.ErrInvalidInput
	}
	dbItem := toModel(item)
	if err := r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"key_hash",
				"key_prefix",
				"key_plaintext_encrypted",
				"status",
				"last_used_at",
				"updated_at",
			}),
		}).
		Create(&dbItem).Error; err != nil {
		return nil, translateError(err)
	}
	return r.GetByUserID(ctx, item.UserID)
}

// RevokeForUser 停用用户 API Key。
func (r *Repo) RevokeForUser(ctx context.Context, userID uint, now time.Time) (*domainopenapi.UserAPIKey, error) {
	if userID == 0 {
		return nil, repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&model.OpenAPIKey{}).
		Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"status":     domainopenapi.APIKeyStatusRevoked,
			"updated_at": now,
		})
	if err := result.Error; err != nil {
		return nil, translateError(err)
	}
	if result.RowsAffected == 0 {
		return nil, repository.ErrNotFound
	}
	return r.GetByUserID(ctx, userID)
}

// TouchLastUsedAt 更新最后使用时间。
func (r *Repo) TouchLastUsedAt(ctx context.Context, id uint, at time.Time) error {
	if id == 0 {
		return repository.ErrInvalidInput
	}
	result := r.db.WithContext(ctx).Model(&model.OpenAPIKey{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"last_used_at": at,
			"updated_at":   at,
		})
	if err := result.Error; err != nil {
		return translateError(err)
	}
	if result.RowsAffected == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func toDomain(item model.OpenAPIKey) *domainopenapi.UserAPIKey {
	return &domainopenapi.UserAPIKey{
		ID:                    item.ID,
		UserID:                item.UserID,
		KeyHash:               item.KeyHash,
		KeyPrefix:             item.KeyPrefix,
		KeyPlaintextEncrypted: item.KeyPlaintextEncrypted,
		Status:                item.Status,
		LastUsedAt:            item.LastUsedAt,
		CreatedAt:             item.CreatedAt,
		UpdatedAt:             item.UpdatedAt,
	}
}

func toModel(item *domainopenapi.UserAPIKey) model.OpenAPIKey {
	if item == nil {
		return model.OpenAPIKey{}
	}
	return model.OpenAPIKey{
		BaseModel: model.BaseModel{
			ID:        item.ID,
			CreatedAt: item.CreatedAt,
			UpdatedAt: item.UpdatedAt,
		},
		UserID:                item.UserID,
		KeyHash:               item.KeyHash,
		KeyPrefix:             item.KeyPrefix,
		KeyPlaintextEncrypted: item.KeyPlaintextEncrypted,
		Status:                item.Status,
		LastUsedAt:            item.LastUsedAt,
	}
}
