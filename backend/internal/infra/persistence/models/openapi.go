package model

import "time"

// OpenAPIKey 记录用户开放 API Key 的哈希与状态。
type OpenAPIKey struct {
	BaseModel
	UserID                uint       `gorm:"not null;uniqueIndex:idx_openapi_keys_user_id;comment:用户ID"`
	KeyHash               string     `gorm:"size:128;not null;uniqueIndex:idx_openapi_keys_hash;comment:API Key 哈希"`
	KeyPrefix             string     `gorm:"size:24;not null;default:'';index:idx_openapi_keys_prefix;comment:明文前缀展示"`
	KeyPlaintextEncrypted string     `gorm:"type:text;not null;default:'';comment:API Key 明文密文"`
	Status                string     `gorm:"size:32;not null;default:'active';index:idx_openapi_keys_status;comment:状态(active/revoked)"`
	LastUsedAt            *time.Time `gorm:"comment:最后使用时间"`
}

// TableName 指定表名。
func (OpenAPIKey) TableName() string {
	return "openapi_keys"
}
