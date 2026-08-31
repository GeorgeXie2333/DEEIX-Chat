package admin

import (
	"context"
	"errors"
	"strings"
	"testing"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

func TestImportOpenWebUIUsersImportsPasswordHashWhenRequested(t *testing.T) {
	sourceHash := mustBcryptHash(t, "source-password")
	users := newOpenWebUIImportUserServiceFake()
	service := NewService(users, auditServiceFake{})
	service.SetOpenWebUIUserSource(openWebUIUserSourceFake{
		rows: []repository.OpenWebUIUserRow{
			{
				PublicID:          "openwebui-user-1",
				Username:          "openwebui-user-1",
				DisplayName:       "Ada Lovelace",
				Email:             "ada@example.com",
				PasswordHash:      sourceHash,
				PasswordAvailable: true,
			},
		},
	})

	result, err := service.ImportOpenWebUIUsers(
		context.Background(),
		"req_1",
		1,
		OpenWebUIImportInput{
			DSN:              "test.db",
			CreditMultiplier: 1,
			ImportPasswords:  true,
		},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("ImportOpenWebUIUsers() error = %v", err)
	}
	if result.PasswordsImported != 1 || result.PasswordsGenerated != 0 || result.PasswordsUnavailable != 0 || result.PasswordsInvalidHash != 0 {
		t.Fatalf("unexpected password stats: %+v", result)
	}
	if len(users.importedRecords) != 1 {
		t.Fatalf("expected 1 imported record, got %d", len(users.importedRecords))
	}
	credential := users.importedRecords[0].Credential
	if credential.PasswordHash != sourceHash {
		t.Fatalf("expected imported credential to reuse source hash, got %q", credential.PasswordHash)
	}
	if credential.PasswordOrigin != domainuser.PasswordOriginOpenWebUIImport {
		t.Fatalf("expected password origin %q, got %q", domainuser.PasswordOriginOpenWebUIImport, credential.PasswordOrigin)
	}
}

func TestImportOpenWebUIUsersGeneratesPasswordByDefault(t *testing.T) {
	sourceHash := mustBcryptHash(t, "source-password")
	users := newOpenWebUIImportUserServiceFake()
	service := NewService(users, auditServiceFake{})
	service.SetOpenWebUIUserSource(openWebUIUserSourceFake{
		rows: []repository.OpenWebUIUserRow{
			{
				PublicID:          "openwebui-user-1",
				Username:          "openwebui-user-1",
				DisplayName:       "Ada Lovelace",
				Email:             "ada@example.com",
				PasswordHash:      sourceHash,
				PasswordAvailable: true,
			},
		},
	})

	result, err := service.ImportOpenWebUIUsers(
		context.Background(),
		"req_1",
		1,
		OpenWebUIImportInput{DSN: "test.db", CreditMultiplier: 1},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("ImportOpenWebUIUsers() error = %v", err)
	}
	if result.PasswordsImported != 0 || result.PasswordsGenerated != 1 {
		t.Fatalf("unexpected password stats: %+v", result)
	}
	if len(users.importedRecords) != 1 {
		t.Fatalf("expected 1 imported record, got %d", len(users.importedRecords))
	}
	credential := users.importedRecords[0].Credential
	if credential.PasswordHash == sourceHash {
		t.Fatal("expected default import to generate a new password hash instead of copying source hash")
	}
	if credential.PasswordOrigin != domainuser.PasswordOriginAdminCreated {
		t.Fatalf("expected generated password origin %q, got %q", domainuser.PasswordOriginAdminCreated, credential.PasswordOrigin)
	}
}

func TestImportOpenWebUIUsersPersistsLowercaseEmail(t *testing.T) {
	users := newOpenWebUIImportUserServiceFake()
	service := NewService(users, auditServiceFake{})
	service.SetOpenWebUIUserSource(openWebUIUserSourceFake{
		rows: []repository.OpenWebUIUserRow{
			{
				PublicID:    "openwebui-user-1",
				Username:    "openwebui-user-1",
				DisplayName: "Ada Lovelace",
				Email:       "Ada@Example.COM",
			},
		},
	})

	if _, err := service.ImportOpenWebUIUsers(
		context.Background(),
		"req_1",
		1,
		OpenWebUIImportInput{DSN: "test.db", CreditMultiplier: 1},
		"127.0.0.1",
		"test",
	); err != nil {
		t.Fatalf("ImportOpenWebUIUsers() error = %v", err)
	}
	if len(users.importedRecords) != 1 {
		t.Fatalf("expected 1 imported record, got %d", len(users.importedRecords))
	}
	if got := users.importedRecords[0].User.Email; got != "ada@example.com" {
		t.Fatalf("imported email = %q, want lowercase email", got)
	}
}

func TestImportOpenWebUIUsersFallsBackForUnavailableAndInvalidPasswords(t *testing.T) {
	users := newOpenWebUIImportUserServiceFake()
	service := NewService(users, auditServiceFake{})
	service.SetOpenWebUIUserSource(openWebUIUserSourceFake{
		rows: []repository.OpenWebUIUserRow{
			{
				PublicID:          "openwebui-user-1",
				Username:          "openwebui-user-1",
				DisplayName:       "Ada Lovelace",
				Email:             "ada@example.com",
				PasswordHash:      "$2b$12$abcdefghijklmnopqrstuuJvOQ3PE5pNN5c4IK88wr4PJ4h5yLaH2",
				PasswordAvailable: false,
			},
			{
				PublicID:          "openwebui-user-2",
				Username:          "openwebui-user-2",
				DisplayName:       "Grace Hopper",
				Email:             "grace@example.com",
				PasswordHash:      "not-a-bcrypt-hash",
				PasswordAvailable: true,
			},
		},
	})

	result, err := service.ImportOpenWebUIUsers(
		context.Background(),
		"req_1",
		1,
		OpenWebUIImportInput{
			DSN:              "test.db",
			CreditMultiplier: 1,
			ImportPasswords:  true,
		},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("ImportOpenWebUIUsers() error = %v", err)
	}
	if result.PasswordsImported != 0 || result.PasswordsGenerated != 2 || result.PasswordsUnavailable != 1 || result.PasswordsInvalidHash != 1 {
		t.Fatalf("unexpected password stats: %+v", result)
	}
	if len(users.importedRecords) != 2 {
		t.Fatalf("expected 2 imported records, got %d", len(users.importedRecords))
	}
	for _, record := range users.importedRecords {
		if record.Credential.PasswordOrigin != domainuser.PasswordOriginAdminCreated {
			t.Fatalf("expected fallback generated password origin %q, got %q", domainuser.PasswordOriginAdminCreated, record.Credential.PasswordOrigin)
		}
	}
}

func TestImportOpenWebUIUsersDryRunCountsPasswordsWithoutWriting(t *testing.T) {
	sourceHash := mustBcryptHash(t, "source-password")
	users := newOpenWebUIImportUserServiceFake()
	users.existingByEmail["existing@example.com"] = domainuser.User{ID: 2, Email: "existing@example.com"}
	service := NewService(users, auditServiceFake{})
	service.SetOpenWebUIUserSource(openWebUIUserSourceFake{
		rows: []repository.OpenWebUIUserRow{
			{
				PublicID:          "openwebui-user-1",
				Username:          "openwebui-user-1",
				DisplayName:       "Ada Lovelace",
				Email:             "ada@example.com",
				PasswordHash:      sourceHash,
				PasswordAvailable: true,
			},
			{
				PublicID:          "openwebui-user-2",
				Username:          "openwebui-user-2",
				DisplayName:       "Existing",
				Email:             "existing@example.com",
				PasswordHash:      sourceHash,
				PasswordAvailable: true,
			},
		},
	})

	result, err := service.ImportOpenWebUIUsers(
		context.Background(),
		"req_1",
		1,
		OpenWebUIImportInput{
			DSN:              "test.db",
			CreditMultiplier: 1,
			DryRun:           true,
			ImportPasswords:  true,
		},
		"127.0.0.1",
		"test",
	)
	if err != nil {
		t.Fatalf("ImportOpenWebUIUsers() error = %v", err)
	}
	if result.Imported != 1 || result.SkippedExistingEmail != 1 || result.PasswordsImported != 1 {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if len(users.importedRecords) != 0 {
		t.Fatalf("expected dry-run not to write records, got %d", len(users.importedRecords))
	}
}

func TestValidOpenWebUIBcryptHashRejectsMalformedPayload(t *testing.T) {
	hash := "$2b$12$" + strings.Repeat("!", 53)

	if validOpenWebUIBcryptHash(hash) {
		t.Fatal("expected malformed bcrypt payload to be rejected")
	}
}

type openWebUIUserSourceFake struct {
	rows []repository.OpenWebUIUserRow
	err  error
}

func (s openWebUIUserSourceFake) LoadUsers(context.Context, string) ([]repository.OpenWebUIUserRow, error) {
	if s.err != nil {
		return nil, s.err
	}
	return append([]repository.OpenWebUIUserRow(nil), s.rows...), nil
}

type openWebUIImportUserServiceFake struct {
	*adminUserServiceFake
	existingByEmail map[string]domainuser.User
	usernames       []string
	importedRecords []repository.UserImportRecord
}

func newOpenWebUIImportUserServiceFake() *openWebUIImportUserServiceFake {
	base := newAdminUserServiceFake(map[uint]domainuser.User{
		1: {ID: 1, Role: domainuser.RoleAdmin, Status: domainuser.StatusActive},
	})
	return &openWebUIImportUserServiceFake{
		adminUserServiceFake: base,
		existingByEmail:      make(map[string]domainuser.User),
	}
}

func (s *openWebUIImportUserServiceFake) ListUsersByLowerEmails(_ context.Context, emails []string) (map[string]domainuser.User, error) {
	if s.existingByEmail == nil {
		return nil, errors.New("existingByEmail map is nil")
	}
	result := make(map[string]domainuser.User)
	for _, email := range emails {
		if item, ok := s.existingByEmail[email]; ok {
			result[email] = item
		}
	}
	return result, nil
}

func (s *openWebUIImportUserServiceFake) ListAllUsernames(context.Context) ([]string, error) {
	return append([]string(nil), s.usernames...), nil
}

func (s *openWebUIImportUserServiceFake) ImportUsersWithCredentialsAndBalances(_ context.Context, records []repository.UserImportRecord) ([]domainuser.User, error) {
	s.importedRecords = append(s.importedRecords, records...)
	users := make([]domainuser.User, 0, len(records))
	for i, record := range records {
		user := record.User
		user.ID = uint(100 + i)
		users = append(users, user)
	}
	return users, nil
}

func mustBcryptHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}
	return string(hash)
}
