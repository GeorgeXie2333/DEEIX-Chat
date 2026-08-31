package openapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/openapi"
	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

type preAuthRejectingLimiter struct{}

func (preAuthRejectingLimiter) AllowSlidingWindow(context.Context, string, int, time.Duration, time.Duration) (bool, error) {
	return false, nil
}

type countingOpenAPIKeyRepository struct {
	lookups int
}

func (*countingOpenAPIKeyRepository) GetByUserID(context.Context, uint) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (r *countingOpenAPIKeyRepository) GetActiveByHash(context.Context, string) (*domainopenapi.UserAPIKey, error) {
	r.lookups++
	return nil, repository.ErrNotFound
}

func (*countingOpenAPIKeyRepository) CreateForUser(context.Context, *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (*countingOpenAPIKeyRepository) ReplaceForUserIfCurrent(context.Context, *domainopenapi.UserAPIKey, *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (*countingOpenAPIKeyRepository) RevokeForUser(context.Context, uint, time.Time) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (*countingOpenAPIKeyRepository) TouchLastUsedAt(context.Context, uint, time.Time) error {
	return nil
}

func TestDecodeRequestBodyRejectsOversizedPayload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(`{"messages":[{"role":"user","content":"too large"}]}`))

	_, err := decodeRequestBodyWithLimit(ctx, 16)
	if !errors.Is(err, appopenapi.ErrInvalidRequest) {
		t.Fatalf("expected oversized request to be invalid, got %v", err)
	}
}

func TestAuthenticateCompatibleAppliesIPLimitBeforeAPIKeyLookup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &countingOpenAPIKeyRepository{}
	service := appopenapi.NewService(appopenapi.Dependencies{
		KeyRepo:     repo,
		RateLimiter: preAuthRejectingLimiter{},
	})
	handler := NewHandler(service)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest("GET", "/v1/models", nil)
	ctx.Request.Header.Set("Authorization", "Bearer invalid")

	if _, ok := handler.authenticateCompatible(ctx); ok {
		t.Fatal("expected pre-auth limiter to reject the request")
	}
	if repo.lookups != 0 {
		t.Fatalf("expected no API key lookup after IP limit rejection, got %d", repo.lookups)
	}
	if recorder.Code != 429 {
		t.Fatalf("expected OpenAI-compatible 429 response, got %d", recorder.Code)
	}
}

func TestAPIKeyMutationConflictsReturnHTTP409(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name         string
		existing     bool
		expectedCode string
		call         func(*Handler, *gin.Context)
	}{
		{
			name:         "create racing with another create",
			expectedCode: "api_key.already_exists",
			call: func(handler *Handler, ctx *gin.Context) {
				handler.CreateAPIKey(ctx)
			},
		},
		{
			name:         "regenerate absent key racing with create",
			expectedCode: "api_key.already_exists",
			call: func(handler *Handler, ctx *gin.Context) {
				handler.RegenerateAPIKey(ctx)
			},
		},
		{
			name:         "regenerate stale snapshot",
			existing:     true,
			expectedCode: "openapi.api_key_conflict",
			call: func(handler *Handler, ctx *gin.Context) {
				handler.RegenerateAPIKey(ctx)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &apiKeyConflictRepository{existing: test.existing}
			service := appopenapi.NewService(appopenapi.Dependencies{
				KeyRepo:   repo,
				TwoFactor: twoFactorEnabledForHandlerTest{},
			})
			handler := NewHandler(service)
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/user/openapi-key", nil)
			ctx.Set(middleware.ContextKeyUserID, uint(42))

			test.call(handler, ctx)
			if recorder.Code != http.StatusConflict {
				t.Fatalf("expected HTTP 409 for concurrent API key mutation, got %d body=%s", recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"errorCode":"`+test.expectedCode+`"`) {
				t.Fatalf("expected stable API key conflict code %q, body=%s", test.expectedCode, recorder.Body.String())
			}
		})
	}
}

type twoFactorEnabledForHandlerTest struct{}

func (twoFactorEnabledForHandlerTest) IsTwoFactorEnabled(context.Context, uint) (bool, error) {
	return true, nil
}

type apiKeyConflictRepository struct {
	existing bool
}

func (r *apiKeyConflictRepository) GetByUserID(context.Context, uint) (*domainopenapi.UserAPIKey, error) {
	if !r.existing {
		return nil, repository.ErrNotFound
	}
	return &domainopenapi.UserAPIKey{
		ID:      1,
		UserID:  42,
		KeyHash: "hash-current",
		Status:  domainopenapi.APIKeyStatusActive,
	}, nil
}

func (*apiKeyConflictRepository) GetActiveByHash(context.Context, string) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (*apiKeyConflictRepository) CreateForUser(context.Context, *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrConflict
}

func (*apiKeyConflictRepository) ReplaceForUserIfCurrent(context.Context, *domainopenapi.UserAPIKey, *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrConflict
}

func (*apiKeyConflictRepository) RevokeForUser(context.Context, uint, time.Time) (*domainopenapi.UserAPIKey, error) {
	return nil, repository.ErrNotFound
}

func (*apiKeyConflictRepository) TouchLastUsedAt(context.Context, uint, time.Time) error {
	return nil
}
