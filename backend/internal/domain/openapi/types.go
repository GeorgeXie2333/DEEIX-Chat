package openapi

import "time"

const (
	// APIKeyStatusActive 表示 API Key 可用于 /v1 兼容接口鉴权。
	APIKeyStatusActive = "active"
	// APIKeyStatusRevoked 表示 API Key 已停用。
	APIKeyStatusRevoked = "revoked"
)

// UserAPIKey 表示用户开放 API Key 的安全存储记录。
type UserAPIKey struct {
	ID                    uint
	UserID                uint
	KeyHash               string
	KeyPrefix             string
	KeyPlaintextEncrypted string
	Status                string
	LastUsedAt            *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}
