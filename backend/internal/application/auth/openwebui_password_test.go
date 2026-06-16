package auth

import (
	"strings"
	"testing"

	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"golang.org/x/crypto/bcrypt"
)

func TestPasswordMatchesCredentialSupportsOpenWebUITruncatedLongPassword(t *testing.T) {
	longPassword := strings.Repeat("a", 73)
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.Repeat("a", 72)), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}

	credential := &domainuser.Credential{
		PasswordHash:    string(hash),
		PasswordEnabled: true,
		PasswordOrigin:  domainuser.PasswordOriginOpenWebUIImport,
	}

	if !passwordMatchesCredential(longPassword, credential) {
		t.Fatal("expected OpenWebUI imported credential to accept the long password after OpenWebUI-style truncation")
	}
}

func TestPasswordMatchesCredentialDoesNotTruncateNonOpenWebUIImportedPassword(t *testing.T) {
	longPassword := strings.Repeat("a", 73)
	hash, err := bcrypt.GenerateFromPassword([]byte(strings.Repeat("a", 72)), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword() error = %v", err)
	}

	credential := &domainuser.Credential{
		PasswordHash:    string(hash),
		PasswordEnabled: true,
		PasswordOrigin:  domainuser.PasswordOriginLocalRegister,
	}

	if passwordMatchesCredential(longPassword, credential) {
		t.Fatal("expected non-OpenWebUI credential to reject long password that only matches after truncation")
	}
}
