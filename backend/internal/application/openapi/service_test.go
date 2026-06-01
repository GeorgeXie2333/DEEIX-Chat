package openapi

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestCreateAPIKeyStoresOnlyHashAndAuthenticatesPlaintext(t *testing.T) {
	repo := newKeyRepoStub()
	service := NewService(Dependencies{
		KeyRepo:           repo,
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	created, err := service.CreateAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	if !strings.HasPrefix(created.APIKey, APIKeyPlaintextPrefix) {
		t.Fatalf("expected generated key prefix %q, got %q", APIKeyPlaintextPrefix, created.APIKey)
	}
	stored := repo.byUser[42]
	if stored == nil {
		t.Fatalf("expected key to be stored")
	}
	if stored.KeyHash == "" || stored.KeyHash == created.APIKey {
		t.Fatalf("expected only a hash in storage, got %q", stored.KeyHash)
	}
	if stored.KeyPrefix == "" || !strings.HasPrefix(created.APIKey, stored.KeyPrefix) {
		t.Fatalf("expected stored display prefix to match plaintext, got prefix=%q key=%q", stored.KeyPrefix, created.APIKey)
	}

	view, err := service.GetAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetAPIKey returned error: %v", err)
	}
	if view.APIKey != "" {
		t.Fatalf("expected GET to never return plaintext, got %q", view.APIKey)
	}
	if !view.Exists || view.KeyPrefix != stored.KeyPrefix {
		t.Fatalf("unexpected key view: %#v", view)
	}

	authenticated, err := service.AuthenticateAPIKey(context.Background(), created.APIKey)
	if err != nil {
		t.Fatalf("AuthenticateAPIKey returned error: %v", err)
	}
	if authenticated.UserID != 42 || authenticated.ID != stored.ID {
		t.Fatalf("unexpected authenticated key: %#v", authenticated)
	}
}

func TestRegenerateAPIKeyReplacesPreviousHash(t *testing.T) {
	repo := newKeyRepoStub()
	service := NewService(Dependencies{
		KeyRepo:           repo,
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	first, err := service.CreateAPIKey(context.Background(), 7)
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	firstHash := repo.byUser[7].KeyHash

	second, err := service.RegenerateAPIKey(context.Background(), 7)
	if err != nil {
		t.Fatalf("RegenerateAPIKey returned error: %v", err)
	}
	if second.APIKey == "" || second.APIKey == first.APIKey {
		t.Fatalf("expected a new plaintext key, got first=%q second=%q", first.APIKey, second.APIKey)
	}
	if repo.byUser[7].KeyHash == firstHash {
		t.Fatalf("expected stored hash to be replaced")
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), first.APIKey); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("expected old key to stop authenticating, got %v", err)
	}
	if _, err := service.AuthenticateAPIKey(context.Background(), second.APIKey); err != nil {
		t.Fatalf("expected regenerated key to authenticate: %v", err)
	}
}

func TestListModelsFiltersAllowlistedActiveTextChatCompletionModels(t *testing.T) {
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		Settings:          settingsStub{"model_allowlist": "chat-openai\nimage-only\nchat-anthropic", "rate_limit_rpm": "60"},
		Channel:           channelStub{},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	result, err := service.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if result.Object != "list" {
		t.Fatalf("expected OpenAI list object, got %q", result.Object)
	}
	if len(result.Data) != 1 || result.Data[0].ID != "chat-openai" {
		t.Fatalf("expected only openai chat allowlisted model, got %#v", result.Data)
	}
}

func TestCompatibleCompletionInjectsThinkAndFallbackUsageBeforeBilling(t *testing.T) {
	key := &domainopenapi.UserAPIKey{ID: 9, UserID: 42, Status: domainopenapi.APIKeyStatusActive}
	provider := &chatProviderStub{
		complete: RawChatCompletionResult{
			Body: map[string]interface{}{
				"id":      "chatcmpl_test",
				"object":  "chat.completion",
				"model":   "upstream-model",
				"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"role": "assistant", "content": "最终答案"}}},
			},
			ReasoningText: "先分析问题",
		},
	}
	billing := &billingStub{}
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		Settings:          settingsStub{"model_allowlist": "chat-openai", "rate_limit_rpm": "60"},
		Channel:           channelStub{},
		Billing:           billing,
		ChatProvider:      provider,
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	prepared, err := service.PrepareChatCompletion(context.Background(), key, map[string]interface{}{
		"model":    "chat-openai",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "你好"}},
	}, "req_1", false)
	if err != nil {
		t.Fatalf("PrepareChatCompletion returned error: %v", err)
	}
	body, err := service.CompleteChatCompletion(context.Background(), prepared)
	if err != nil {
		t.Fatalf("CompleteChatCompletion returned error: %v", err)
	}

	if body["model"] != "chat-openai" {
		t.Fatalf("expected public model name in response, got %#v", body["model"])
	}
	message := body["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	if got := message["content"]; got != "<think>先分析问题</think>最终答案" {
		t.Fatalf("unexpected assistant content: %#v", got)
	}
	usage := body["usage"].(map[string]interface{})
	if usage["prompt_tokens"].(int64) == 0 || usage["completion_tokens"].(int64) == 0 {
		t.Fatalf("expected fallback usage tokens, got %#v", usage)
	}
	if billing.recorded == nil || billing.recorded.InputTokens == 0 || billing.recorded.OutputTokens == 0 {
		t.Fatalf("expected billing ledger to use fallback usage, got %#v", billing.recorded)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
}

type keyRepoStub struct {
	nextID uint
	byUser map[uint]*domainopenapi.UserAPIKey
	byHash map[string]*domainopenapi.UserAPIKey
}

func newKeyRepoStub() *keyRepoStub {
	return &keyRepoStub{
		nextID: 1,
		byUser: make(map[uint]*domainopenapi.UserAPIKey),
		byHash: make(map[string]*domainopenapi.UserAPIKey),
	}
}

func (r *keyRepoStub) GetByUserID(_ context.Context, userID uint) (*domainopenapi.UserAPIKey, error) {
	item := r.byUser[userID]
	if item == nil {
		return nil, repository.ErrNotFound
	}
	cp := *item
	return &cp, nil
}

func (r *keyRepoStub) GetActiveByHash(_ context.Context, hash string) (*domainopenapi.UserAPIKey, error) {
	item := r.byHash[hash]
	if item == nil || item.Status != domainopenapi.APIKeyStatusActive {
		return nil, repository.ErrNotFound
	}
	cp := *item
	return &cp, nil
}

func (r *keyRepoStub) ReplaceForUser(_ context.Context, item *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	if current := r.byUser[item.UserID]; current != nil {
		delete(r.byHash, current.KeyHash)
	}
	cp := *item
	if cp.ID == 0 {
		cp.ID = r.nextID
		r.nextID++
	}
	r.byUser[cp.UserID] = &cp
	r.byHash[cp.KeyHash] = &cp
	out := cp
	return &out, nil
}

func (r *keyRepoStub) RevokeForUser(_ context.Context, userID uint, now time.Time) (*domainopenapi.UserAPIKey, error) {
	item := r.byUser[userID]
	if item == nil {
		return nil, repository.ErrNotFound
	}
	item.Status = domainopenapi.APIKeyStatusRevoked
	item.UpdatedAt = now
	out := *item
	return &out, nil
}

func (r *keyRepoStub) TouchLastUsedAt(_ context.Context, id uint, at time.Time) error {
	for _, item := range r.byUser {
		if item.ID == id {
			item.LastUsedAt = &at
			return nil
		}
	}
	return repository.ErrNotFound
}

type settingsStub map[string]string

func (s settingsStub) RuntimeValuesByNamespace(_ context.Context, namespace string) (map[string]string, error) {
	if namespace != "openapi" {
		return map[string]string{}, nil
	}
	return map[string]string(s), nil
}

type channelStub struct{}

func (channelStub) ListActiveModels(context.Context) ([]appchannel.ModelView, error) {
	return []appchannel.ModelView{
		{PlatformModelName: "chat-openai", KindsJSON: `["chat"]`, ProtocolsJSON: `["openai_chat_completions"]`},
		{PlatformModelName: "image-only", KindsJSON: `["image_gen"]`, ProtocolsJSON: `["openai_image_generations"]`},
		{PlatformModelName: "chat-anthropic", KindsJSON: `["chat"]`, ProtocolsJSON: `["anthropic_messages"]`},
		{PlatformModelName: "not-allowlisted", KindsJSON: `["chat"]`, ProtocolsJSON: `["openai_chat_completions"]`},
	}, nil
}

func (channelStub) ResolveRoute(_ context.Context, input appchannel.ResolveRouteInput) (*appchannel.ResolvedRoute, error) {
	if input.PlatformModelName != "chat-openai" {
		return nil, appchannel.ErrModelNotFound
	}
	return &appchannel.ResolvedRoute{
		PlatformModelName: input.PlatformModelName,
		Protocol:          llm.AdapterOpenAIChatCompletions,
		BaseURL:           "https://upstream.example",
		UpstreamModel:     "upstream-model",
		BindingCode:       "binding_1",
		UpstreamName:      "upstream",
	}, nil
}

func (channelStub) MarkRouteSuccess(context.Context, *appchannel.ResolvedRoute)        {}
func (channelStub) MarkRouteFailure(context.Context, *appchannel.ResolvedRoute, error) {}

type chatProviderStub struct {
	complete RawChatCompletionResult
}

func (p *chatProviderStub) CompleteChat(context.Context, llm.RouteConfig, map[string]interface{}) (RawChatCompletionResult, error) {
	return p.complete, nil
}

func (p *chatProviderStub) StreamChat(context.Context, llm.RouteConfig, map[string]interface{}, func(RawChatStreamEvent) error) (RawChatCompletionResult, error) {
	return RawChatCompletionResult{}, nil
}

type billingStub struct {
	recorded *domainbilling.UsageLedger
}

func (b *billingStub) EnsureModelUsable(context.Context, uint, string, time.Time) error {
	return nil
}

func (b *billingStub) ReserveUsageBalance(context.Context, uint, string, string) (*domainbilling.UsageBalanceReservation, error) {
	return &domainbilling.UsageBalanceReservation{UserID: 42, AmountNanousd: 1, RefNo: "req_1"}, nil
}

func (b *billingStub) ReleaseUsageBalanceReservation(context.Context, *domainbilling.UsageBalanceReservation, string) error {
	return nil
}

func (b *billingStub) BuildUsageLedger(_ context.Context, input appbilling.UsagePricingInput) (*domainbilling.UsageLedger, error) {
	return &domainbilling.UsageLedger{
		UserID:            input.UserID,
		PlatformModelName: input.PlatformModelName,
		InputTokens:       input.InputTokens,
		OutputTokens:      input.OutputTokens,
		ReasoningTokens:   input.ReasoningTokens,
	}, nil
}

func (b *billingStub) RecordUsageWithReservation(_ context.Context, ledger *domainbilling.UsageLedger, _ *domainbilling.UsageBalanceReservation) error {
	b.recorded = ledger
	return nil
}
