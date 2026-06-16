package openwebui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Source struct{}

func NewSource() *Source {
	return &Source{}
}

type userRow struct {
	PublicID          string  `gorm:"column:public_id"`
	Username          string  `gorm:"column:username"`
	DisplayName       string  `gorm:"column:display_name"`
	Email             string  `gorm:"column:email"`
	Balance           float64 `gorm:"column:balance"`
	PasswordHash      string  `gorm:"column:password_hash"`
	PasswordAvailable bool    `gorm:"column:password_available"`
}

func (Source) LoadUsers(ctx context.Context, dsn string) ([]repository.OpenWebUIUserRow, error) {
	db, err := openDB(dsn)
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	rows := make([]userRow, 0)
	query := importQuery(db.Migrator().HasTable("credit"), db.Migrator().HasTable("auth"))
	if err = db.WithContext(ctx).Raw(query).Scan(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]repository.OpenWebUIUserRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, repository.OpenWebUIUserRow{
			PublicID:          row.PublicID,
			Username:          row.Username,
			DisplayName:       row.DisplayName,
			Email:             row.Email,
			Balance:           row.Balance,
			PasswordHash:      row.PasswordHash,
			PasswordAvailable: row.PasswordAvailable,
		})
	}
	return result, nil
}

func openDB(dsn string) (*gorm.DB, error) {
	trimmed := strings.TrimSpace(dsn)
	if trimmed == "" {
		return nil, repository.ErrInvalidInput
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "postgres://") || strings.HasPrefix(lower, "postgresql://") {
		if _, err := url.Parse(trimmed); err != nil {
			return nil, repository.ErrInvalidInput
		}
		return gorm.Open(gormpostgres.Open(trimmed), &gorm.Config{})
	}
	if strings.HasPrefix(lower, "sqlite://") {
		trimmed = trimmed[len("sqlite://"):]
		if strings.TrimSpace(trimmed) == "" {
			return nil, repository.ErrInvalidInput
		}
	}
	return gorm.Open(sqlite.Open(trimmed), &gorm.Config{})
}

func importQuery(hasCredit bool, hasAuth bool) string {
	creditSelect := `0 as "balance"`
	creditJoin := ""
	if hasCredit {
		creditSelect = `coalesce(c.credit, 0) as "balance"`
		creditJoin = `
         left join "credit" as c
              on u.id = c.user_id`
	}

	passwordSelect := `'' as "password_hash",
       false as "password_available"`
	authJoin := ""
	if hasAuth {
		passwordSelect = `coalesce(a.password, '') as "password_hash",
       case when a.id is not null and coalesce(a.active, false) and coalesce(a.password, '') <> '' then true else false end as "password_available"`
		authJoin = `
         left join "auth" as a
              on u.id = a.id`
	}

	return fmt.Sprintf(`select u.id as "public_id",
       u.id as "username",
       u.name as "display_name",
       u.email as "email",
       %s,
       %s
from "user" as u%s%s`, creditSelect, passwordSelect, creditJoin, authJoin)
}
