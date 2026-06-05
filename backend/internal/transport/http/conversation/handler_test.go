package conversation

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appconversation "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/conversation"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	conversationmodel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type httpFreeModelRateLimiterStub struct {
	minuteExceeded bool
	dailyExceeded  bool
}

func (s httpFreeModelRateLimiterStub) AllowFreeModelUsage(context.Context, uint, int, int, time.Time) (bool, bool, bool, error) {
	return false, s.minuteExceeded, s.dailyExceeded, nil
}

type httpBillingRepositoryStub struct {
	repository.BillingRepository
	mode               string
	pricing            *domainbilling.ModelPricing
	freeModelRateLimit domainbilling.FreeModelRateLimit
}

type sensitivePromptConversationRepoStub struct {
	repository.ConversationRepository
}

func (r sensitivePromptConversationRepoStub) GetConversationByPublicID(ctx context.Context, publicID string, userID uint) (*conversationmodel.Conversation, error) {
	return &conversationmodel.Conversation{
		ID:       1,
		UserID:   userID,
		PublicID: publicID,
		Model:    "free-chat",
	}, nil
}

func (r *httpBillingRepositoryStub) GetBillingMode(context.Context) (string, error) {
	return r.mode, nil
}

func (r *httpBillingRepositoryStub) GetModelPricing(context.Context, string) (*domainbilling.ModelPricing, error) {
	if r.pricing == nil {
		return nil, repository.ErrNotFound
	}
	return r.pricing, nil
}

func (r *httpBillingRepositoryStub) GetFreeModelRateLimit(context.Context) (domainbilling.FreeModelRateLimit, error) {
	return r.freeModelRateLimit, nil
}

func newFreeModelLimitedHandler(minuteExceeded bool, dailyExceeded bool) *Handler {
	billingRepo := &httpBillingRepositoryStub{
		mode: "usage",
		pricing: &domainbilling.ModelPricing{
			PlatformModelName: "free-chat",
			IsFree:            true,
		},
		freeModelRateLimit: domainbilling.FreeModelRateLimit{
			RequestsPerMinute: 1,
			DailyRequests:     5,
		},
	}
	billingSvc := appbilling.NewService(billingRepo)
	billingSvc.SetFreeModelRateLimiter(httpFreeModelRateLimiterStub{
		minuteExceeded: minuteExceeded,
		dailyExceeded:  dailyExceeded,
	})
	conversationSvc := appconversation.NewService(
		config.Config{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)
	conversationSvc.SetBillingService(billingSvc)
	return NewHandler(conversationSvc, config.NewRuntime(config.Config{}))
}

func newSensitivePromptHandler() *Handler {
	cfg := config.Config{PromptSensitiveWords: "blocked"}
	conversationSvc := appconversation.NewService(
		cfg,
		sensitivePromptConversationRepoStub{},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)
	return NewHandler(conversationSvc, config.NewRuntime(cfg))
}

func newSensitivePromptContext(path string, body string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Params = gin.Params{{Key: "id", Value: "conv_test"}}
	ctx.Set(middleware.ContextKeyUserID, uint(7))
	return ctx, recorder
}

func newBillingAccessTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/test", nil)
	ctx.Set(middleware.ContextKeyUserID, uint(7))
	return ctx, recorder
}

func TestSafeFileContentTypeDowngradesActiveContent(t *testing.T) {
	tests := []struct {
		contentType string
		want        string
	}{
		{contentType: "text/html; charset=utf-8", want: "text/plain; charset=utf-8"},
		{contentType: "application/javascript", want: "text/plain; charset=utf-8"},
		{contentType: "image/svg+xml", want: "text/plain; charset=utf-8"},
		{contentType: "application/pdf", want: "application/pdf"},
	}
	for _, tt := range tests {
		if got := safeFileContentType(tt.contentType); got != tt.want {
			t.Fatalf("safeFileContentType(%q) = %q, want %q", tt.contentType, got, tt.want)
		}
	}
}

func TestBuildContentDispositionDefaultsToAttachment(t *testing.T) {
	got := buildContentDisposition("report.html", false)
	want := `attachment; filename="report.html"; filename*=UTF-8''report.html`
	if got != want {
		t.Fatalf("unexpected disposition: got %q want %q", got, want)
	}
}

func TestSendMessageBillingAccessFreeModelLimitReturns429(t *testing.T) {
	handler := newFreeModelLimitedHandler(true, false)
	ctx, recorder := newBillingAccessTestContext()

	err := handler.ensureBillingModelAccess(ctx, &conversationmodel.Conversation{ID: 1}, &SendMessageRequest{Model: "free-chat"})
	if !errors.Is(err, appbilling.ErrFreeModelRateLimitExceeded) {
		t.Fatalf("expected free model minute limit error, got %v", err)
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); strings.Contains(contentType, "application/x-ndjson") {
		t.Fatalf("did not expect stream content type before billing access passes, got %q", contentType)
	}

	var body struct {
		ErrorCode string `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.ErrorCode != "rate_limit.exceeded" || body.ErrorMsg != "free model rate limit exceeded" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestMediaImageBillingAccessFreeModelLimitReturns429(t *testing.T) {
	handler := newFreeModelLimitedHandler(false, true)
	ctx, recorder := newBillingAccessTestContext()

	err := handler.ensureMediaImageBillingModelAccess(ctx, &conversationmodel.Conversation{ID: 1}, &MediaImageRequest{Model: "free-chat"})
	if !errors.Is(err, appbilling.ErrFreeModelDailyLimitExceeded) {
		t.Fatalf("expected free model daily limit error, got %v", err)
	}
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", recorder.Code)
	}

	var body struct {
		ErrorCode string `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.ErrorCode != "rate_limit.exceeded" || body.ErrorMsg != "free model daily limit exceeded" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestBillingStreamErrorPayloadFreeModelLimitUsesRateLimitCode(t *testing.T) {
	mapped := mapBillingStreamError(appbilling.ErrFreeModelRateLimitExceeded)
	if mapped.Status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 stream status, got %d", mapped.Status)
	}
	if mapped.Code != "rate_limit.exceeded" || mapped.Message != "free model rate limit exceeded" {
		t.Fatalf("unexpected stream mapping: %#v", mapped)
	}
}

func TestSendMessageSensitivePromptBlockedBeforeGeneration(t *testing.T) {
	handler := newSensitivePromptHandler()
	ctx, recorder := newSensitivePromptContext(
		"/conversations/conv_test/messages",
		`{"contentType":"text","content":"please use ＢＬＯＣＫＥＤ term"}`,
	)

	handler.SendMessage(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); strings.Contains(contentType, "application/x-ndjson") {
		t.Fatalf("did not expect stream content type for preflight error, got %q", contentType)
	}
	var body struct {
		ErrorCode string `json:"errorCode"`
		ErrorMsg  string `json:"errorMsg"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.ErrorCode != "message.sensitive_prompt_blocked" || body.ErrorMsg != "message violates safety policy" {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

func TestStreamMessageSensitivePromptBlockedBeforeNDJSON(t *testing.T) {
	handler := newSensitivePromptHandler()
	ctx, recorder := newSensitivePromptContext(
		"/conversations/conv_test/messages/stream",
		`{"contentType":"text","content":"blocked content"}`,
	)

	handler.StreamMessage(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); strings.Contains(contentType, "application/x-ndjson") {
		t.Fatalf("did not expect stream content type for preflight error, got %q", contentType)
	}
}

func TestMediaImageSensitivePromptBlockedBeforeNDJSON(t *testing.T) {
	handler := newSensitivePromptHandler()
	ctx, recorder := newSensitivePromptContext(
		"/conversations/conv_test/images/generations/stream",
		`{"prompt":"blocked image prompt"}`,
	)

	handler.StreamImageGeneration(ctx)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
	if contentType := recorder.Header().Get("Content-Type"); strings.Contains(contentType, "application/x-ndjson") {
		t.Fatalf("did not expect stream content type for preflight error, got %q", contentType)
	}
}

func TestStreamErrorPayloadIncludesUpstreamDebug(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 401,
		Message:    "google authentication failed",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method:  "POST",
				Path:    "/v1beta/models/nano-banana-pro:streamGenerateContent",
				Headers: map[string]string{"x-goog-api-key": "[redacted]"},
				Body:    `{"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 401,
				Headers:    map[string]string{"Provider": "ExampleEdge"},
				Body:       `{"error":{"message":"unauthorized"}}`,
			},
		},
	})

	payload := streamErrorPayload(err)
	debug, ok := payload["debug"].(*llm.UpstreamDebugSnapshot)
	if !ok || debug == nil {
		t.Fatalf("expected upstream debug payload, got %#v", payload["debug"])
	}
	if debug.Request.Path != "/v1beta/models/nano-banana-pro:streamGenerateContent" {
		t.Fatalf("unexpected request debug: %#v", debug.Request)
	}
	if debug.Response.StatusCode != 401 {
		t.Fatalf("unexpected response debug: %#v", debug.Response)
	}
	if debug.Request.Headers != nil || debug.Response.Headers != nil {
		t.Fatalf("expected public error stream to omit upstream headers, got request=%#v response=%#v", debug.Request.Headers, debug.Response.Headers)
	}
}

func TestMapStreamErrorDoesNotExposeUpstreamUnauthorizedAsPlatformUnauthorized(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 401,
		Message:    "upstream authentication failed",
	})

	mapped := mapStreamError(err)
	if mapped.Status != 502 {
		t.Fatalf("expected upstream 401 to be mapped to gateway failure, got status=%d", mapped.Status)
	}
	if mapped.Code == "auth.unauthorized" || mapped.Code == "auth.invalid_token" || mapped.Code == "auth.session_invalid" {
		t.Fatalf("expected upstream 401 to avoid platform auth codes, got %#v", mapped)
	}
}

func TestStreamErrorPayloadClassifiesImageStreamConfigurationFailure(t *testing.T) {
	err := errors.Join(appconversation.ErrUpstreamRequestFailed, &llm.UpstreamError{
		StatusCode: 500,
		Message:    "invalid character 'e' looking for beginning of value",
		Debug: &llm.UpstreamDebugSnapshot{
			Request: llm.UpstreamDebugRequest{
				Method: "POST",
				Path:   "/v1/images/generations",
				Body:   `{"model":"gpt-image-2","prompt":"a cat","stream":true}`,
			},
			Response: llm.UpstreamDebugResponse{
				StatusCode: 500,
				Body:       `{"error":{"message":"invalid character 'e' looking for beginning of value"}}`,
			},
		},
	})

	payload := streamErrorPayload(err)
	if got := payload["errorCode"]; got != appconversation.MessageErrorCodeMediaImageStreamUnsupported {
		t.Fatalf("errorCode = %#v, want %q", got, appconversation.MessageErrorCodeMediaImageStreamUnsupported)
	}
}
