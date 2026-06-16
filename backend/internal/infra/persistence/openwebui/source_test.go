package openwebui

import (
	"context"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestLoadUsersReadsPasswordHashAndCredit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "openwebui.db")
	db := openSQLiteTestDB(t, dbPath)
	execSQL(t, db, `create table "user" (id text primary key, name text, email text)`)
	execSQL(t, db, `create table "auth" (id text primary key, email text, password text, active boolean)`)
	execSQL(t, db, `create table "credit" (user_id text primary key, credit real)`)
	execSQL(t, db, `insert into "user" (id, name, email) values ('u-1', 'Ada', 'ada@example.com')`)
	execSQL(t, db, `insert into "auth" (id, email, password, active) values ('u-1', 'ada@example.com', '$2b$12$abcdefghijklmnopqrstuuJvOQ3PE5pNN5c4IK88wr4PJ4h5yLaH2', true)`)
	execSQL(t, db, `insert into "credit" (user_id, credit) values ('u-1', 12.5)`)

	rows, err := NewSource().LoadUsers(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("LoadUsers() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].PasswordHash != "$2b$12$abcdefghijklmnopqrstuuJvOQ3PE5pNN5c4IK88wr4PJ4h5yLaH2" {
		t.Fatalf("expected password hash to be loaded, got %q", rows[0].PasswordHash)
	}
	if !rows[0].PasswordAvailable {
		t.Fatal("expected active auth row with non-empty hash to be password-available")
	}
	if rows[0].Balance != 12.5 {
		t.Fatalf("expected credit balance 12.5, got %v", rows[0].Balance)
	}
}

func TestLoadUsersWithoutOptionalTablesMarksPasswordUnavailableAndZeroBalance(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "openwebui.db")
	db := openSQLiteTestDB(t, dbPath)
	execSQL(t, db, `create table "user" (id text primary key, name text, email text)`)
	execSQL(t, db, `insert into "user" (id, name, email) values ('u-1', 'Ada', 'ada@example.com')`)

	rows, err := NewSource().LoadUsers(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("LoadUsers() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].PasswordHash != "" {
		t.Fatalf("expected missing auth table to produce empty password hash, got %q", rows[0].PasswordHash)
	}
	if rows[0].PasswordAvailable {
		t.Fatal("expected missing auth table to mark password unavailable")
	}
	if rows[0].Balance != 0 {
		t.Fatalf("expected missing credit table to produce zero balance, got %v", rows[0].Balance)
	}
}

func TestLoadUsersInactiveAuthMarksPasswordUnavailable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "openwebui.db")
	db := openSQLiteTestDB(t, dbPath)
	execSQL(t, db, `create table "user" (id text primary key, name text, email text)`)
	execSQL(t, db, `create table "auth" (id text primary key, email text, password text, active boolean)`)
	execSQL(t, db, `insert into "user" (id, name, email) values ('u-1', 'Ada', 'ada@example.com')`)
	execSQL(t, db, `insert into "auth" (id, email, password, active) values ('u-1', 'ada@example.com', '$2b$12$abcdefghijklmnopqrstuuJvOQ3PE5pNN5c4IK88wr4PJ4h5yLaH2', false)`)

	rows, err := NewSource().LoadUsers(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("LoadUsers() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].PasswordAvailable {
		t.Fatal("expected inactive auth row to mark password unavailable")
	}
}

func openSQLiteTestDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite test db: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func execSQL(t *testing.T, db *gorm.DB, query string) {
	t.Helper()
	if err := db.Exec(query).Error; err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
