package openapi

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"sort"
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
		TwoFactor:         twoFactorStub{enabled: true},
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
	if stored.KeyPlaintextEncrypted == "" || strings.Contains(stored.KeyPlaintextEncrypted, created.APIKey) {
		t.Fatalf("expected encrypted plaintext in storage, got %q", stored.KeyPlaintextEncrypted)
	}
	if stored.KeyPrefix == "" || !strings.HasPrefix(created.APIKey, stored.KeyPrefix) {
		t.Fatalf("expected stored display prefix to match plaintext, got prefix=%q key=%q", stored.KeyPrefix, created.APIKey)
	}

	view, err := service.GetAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetAPIKey returned error: %v", err)
	}
	if view.APIKey != created.APIKey {
		t.Fatalf("expected GET to return decryptable plaintext, got %q want %q", view.APIKey, created.APIKey)
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
		TwoFactor:         twoFactorStub{enabled: true},
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

func TestCreateAndRegenerateAPIKeyRequireTwoFactor(t *testing.T) {
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		TwoFactor:         twoFactorStub{enabled: false},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	if _, err := service.CreateAPIKey(context.Background(), 42); !errors.Is(err, ErrTwoFactorRequired) {
		t.Fatalf("expected create to require two factor, got %v", err)
	}
	if _, err := service.RegenerateAPIKey(context.Background(), 42); !errors.Is(err, ErrTwoFactorRequired) {
		t.Fatalf("expected regenerate to require two factor, got %v", err)
	}
}

func TestGetAPIKeyHidesPlaintextWhenTwoFactorDisabled(t *testing.T) {
	repo := newKeyRepoStub()
	enabledService := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: true},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	created, err := enabledService.CreateAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}

	disabledService := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: false},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	view, err := disabledService.GetAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("GetAPIKey returned error: %v", err)
	}
	if view.APIKey != "" {
		t.Fatalf("expected API key plaintext to be hidden without two factor, got %q", view.APIKey)
	}
	if !view.TwoFactorRequired || view.Exportable {
		t.Fatalf("expected two-factor-required non-exportable view, got %#v", view)
	}
	if _, err := disabledService.AuthenticateAPIKey(context.Background(), created.APIKey); err != nil {
		t.Fatalf("/v1 authentication should not depend on current 2FA state: %v", err)
	}
}

func TestListModelsFiltersAllowlistedActiveTextChatCompletionModels(t *testing.T) {
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		Settings:          settingsStub{"model_allowlist": "chat-openai\nchat-responses\nchat-anthropic\nchat-google\nchat-xai\nimage-only", "rate_limit_rpm": "60"},
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
	if got, want := sortedModelIDs(result.Data), []string{"claude-upstream", "gemini-upstream", "gpt-responses-upstream", "grok-upstream", "upstream-model"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected public models: got %#v want %#v", got, want)
	}
}

func TestPrepareChatCompletionAllowsPublicChatProtocolsWithoutPreferredProtocol(t *testing.T) {
	key := &domainopenapi.UserAPIKey{ID: 9, UserID: 42, Status: domainopenapi.APIKeyStatusActive}
	channel := &capturingChannelStub{protocol: llm.AdapterAnthropicMessages}
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		Settings:          settingsStub{"model_allowlist": "chat-anthropic", "rate_limit_rpm": "60"},
		Channel:           channel,
		ChatProvider:      &chatProviderStub{},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	prepared, err := service.PrepareChatCompletion(context.Background(), key, map[string]interface{}{
		"model":    "claude-upstream",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
	}, "req_1", false)
	if err != nil {
		t.Fatalf("PrepareChatCompletion returned error: %v", err)
	}
	if channel.preferredProtocol != "" {
		t.Fatalf("expected route resolution without preferred protocol, got %q", channel.preferredProtocol)
	}
	if channel.upstreamModelName != "claude-upstream" {
		t.Fatalf("expected route resolution by upstream model id, got %q", channel.upstreamModelName)
	}
	if prepared.platformModelName != "chat-anthropic" || prepared.publicModelID != "claude-upstream" {
		t.Fatalf("unexpected prepared model ids: platform=%q public=%q", prepared.platformModelName, prepared.publicModelID)
	}
	if prepared.route.Protocol != llm.AdapterAnthropicMessages {
		t.Fatalf("expected anthropic route, got %q", prepared.route.Protocol)
	}
}

func TestBuildGenerateInputFromChatCompletionParsesMultimodalDataURL(t *testing.T) {
	input, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{
		"model":       "vision-chat",
		"temperature": jsonNumber("0.2"),
		"messages": []interface{}{map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "text", "text": "describe"},
				map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "data:image/png;base64,c291cmNl"}},
			},
		}},
	}, nil)
	if err != nil {
		t.Fatalf("buildGenerateInputFromChatCompletion returned error: %v", err)
	}
	if len(input.Messages) != 1 || input.Messages[0].Role != "user" {
		t.Fatalf("unexpected messages: %#v", input.Messages)
	}
	parts := input.Messages[0].Parts
	if len(parts) != 2 {
		t.Fatalf("expected text and image parts, got %#v", parts)
	}
	if parts[0].Kind != llm.ContentPartText || parts[0].Text != "describe" {
		t.Fatalf("unexpected text part: %#v", parts[0])
	}
	if parts[1].Kind != llm.ContentPartImage || parts[1].MimeType != "image/png" || string(parts[1].Data) != "source" {
		t.Fatalf("unexpected image part: %#v", parts[1])
	}
	if got := input.Options["temperature"]; got != jsonNumber("0.2") {
		t.Fatalf("expected options to preserve transparent fields, got %#v", input.Options)
	}
}

func TestBuildGenerateInputFromChatCompletionRejectsAssistantImage(t *testing.T) {
	_, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{
		"model": "vision-chat",
		"messages": []interface{}{map[string]interface{}{
			"role": "assistant",
			"content": []interface{}{
				map[string]interface{}{"type": "image_url", "image_url": "data:image/png;base64,c291cmNl"},
			},
		}},
	}, nil)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected invalid request for assistant image, got %v", err)
	}
}

func TestBuildGenerateInputFromChatCompletionDownloadsHTTPSImage(t *testing.T) {
	var requestedURL string
	resolver := chatImageResolverFunc(func(_ context.Context, rawURL string) (llm.ContentPart, error) {
		requestedURL = rawURL
		return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: "image/jpeg", Data: []byte("remote")}, nil
	})

	input, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{
		"model": "vision-chat",
		"messages": []interface{}{map[string]interface{}{
			"role": "user",
			"content": []interface{}{
				map[string]interface{}{"type": "image_url", "image_url": map[string]interface{}{"url": "https://cdn.example/image.jpg"}},
			},
		}},
	}, resolver)
	if err != nil {
		t.Fatalf("buildGenerateInputFromChatCompletion returned error: %v", err)
	}
	if requestedURL != "https://cdn.example/image.jpg" {
		t.Fatalf("expected resolver to receive remote URL, got %q", requestedURL)
	}
	if got := input.Messages[0].Parts[0]; got.Kind != llm.ContentPartImage || got.MimeType != "image/jpeg" || string(got.Data) != "remote" {
		t.Fatalf("unexpected remote image part: %#v", got)
	}
}

func TestBuildGenerateInputFromChatCompletionParsesToolsAndToolMessages(t *testing.T) {
	input, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{
		"model":               "tool-chat",
		"tool_choice":         map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "get_weather"}},
		"parallel_tool_calls": false,
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":        "get_weather",
					"description": "Get weather",
					"parameters": map[string]interface{}{
						"type":       "object",
						"properties": map[string]interface{}{"city": map[string]interface{}{"type": "string"}},
						"required":   []interface{}{"city"},
					},
				},
			},
		},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "weather?"},
			map[string]interface{}{
				"role":    "assistant",
				"content": nil,
				"tool_calls": []interface{}{
					map[string]interface{}{
						"id":   "call_1",
						"type": "function",
						"function": map[string]interface{}{
							"name":      "get_weather",
							"arguments": `{"city":"Paris"}`,
						},
					},
				},
			},
			map[string]interface{}{
				"role":         "tool",
				"tool_call_id": "call_1",
				"content":      `{"temperature":20}`,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildGenerateInputFromChatCompletion returned error: %v", err)
	}
	if len(input.Tools) != 1 || input.Tools[0].Name != "get_weather" || input.Tools[0].Description != "Get weather" {
		t.Fatalf("expected function tool definition, got %#v", input.Tools)
	}
	var schema map[string]interface{}
	if err := json.Unmarshal(input.Tools[0].InputSchema, &schema); err != nil {
		t.Fatalf("tool schema should be JSON: %v", err)
	}
	if schema["type"] != "object" {
		t.Fatalf("expected object schema, got %#v", schema)
	}
	if _, ok := input.Options["tools"]; ok {
		t.Fatalf("expected public function tools to be removed from transparent options, got %#v", input.Options["tools"])
	}
	if got, ok := input.Options["parallel_tool_calls"].(bool); !ok || got {
		t.Fatalf("expected parallel_tool_calls=false to be preserved, got %#v", input.Options["parallel_tool_calls"])
	}
	assistant := input.Messages[1]
	if len(assistant.ToolCalls) != 1 || assistant.ToolCalls[0].ToolCallID != "call_1" || assistant.ToolCalls[0].ToolName != "get_weather" {
		t.Fatalf("expected assistant tool call, got %#v", assistant.ToolCalls)
	}
	toolResult := input.Messages[2]
	if len(toolResult.ToolResults) != 1 || toolResult.ToolResults[0].ToolCallID != "call_1" || toolResult.ToolResults[0].ToolName != "get_weather" {
		t.Fatalf("expected tool result linked to prior call, got %#v", toolResult.ToolResults)
	}
	if toolResult.ToolResults[0].OutputJSON != `{"temperature":20}` {
		t.Fatalf("unexpected tool result output: %#v", toolResult.ToolResults[0])
	}
}

func TestBuildGenerateInputFromChatCompletionNormalizesLegacyFunctions(t *testing.T) {
	input, err := buildGenerateInputFromChatCompletion(context.Background(), map[string]interface{}{
		"model": "legacy-tool-chat",
		"functions": []interface{}{
			map[string]interface{}{
				"name":        "legacy_lookup",
				"description": "Legacy lookup",
				"parameters":  map[string]interface{}{"type": "object"},
			},
		},
		"function_call": map[string]interface{}{"name": "legacy_lookup"},
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "lookup"},
			map[string]interface{}{
				"role":    "assistant",
				"content": nil,
				"function_call": map[string]interface{}{
					"name":      "legacy_lookup",
					"arguments": `{"id":"42"}`,
				},
			},
			map[string]interface{}{
				"role":    "function",
				"name":    "legacy_lookup",
				"content": `{"value":"ok"}`,
			},
		},
	}, nil)
	if err != nil {
		t.Fatalf("buildGenerateInputFromChatCompletion returned error: %v", err)
	}
	if len(input.Tools) != 1 || input.Tools[0].Name != "legacy_lookup" {
		t.Fatalf("expected legacy function to become tool definition, got %#v", input.Tools)
	}
	if _, ok := input.Options["functions"]; ok {
		t.Fatalf("expected legacy functions to be removed from transparent options")
	}
	toolChoice := asMap(input.Options["tool_choice"])
	if toolChoice["type"] != "function" || getStringFromNestedMap(toolChoice, "function", "name") != "legacy_lookup" {
		t.Fatalf("expected legacy function_call to become tool_choice, got %#v", input.Options["tool_choice"])
	}
	if len(input.Messages[1].ToolCalls) != 1 {
		t.Fatalf("expected legacy assistant function_call to become tool call, got %#v", input.Messages[1])
	}
	call := input.Messages[1].ToolCalls[0]
	if call.ToolCallID == "" || call.ToolName != "legacy_lookup" || call.ArgumentsJSON != `{"id":"42"}` {
		t.Fatalf("unexpected legacy tool call: %#v", call)
	}
	if len(input.Messages[2].ToolResults) != 1 {
		t.Fatalf("expected legacy function result to become tool result, got %#v", input.Messages[2])
	}
	result := input.Messages[2].ToolResults[0]
	if result.ToolCallID != call.ToolCallID || result.ToolName != "legacy_lookup" || result.OutputJSON != `{"value":"ok"}` {
		t.Fatalf("unexpected legacy tool result: call=%#v result=%#v", call, result)
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
		"model":    "upstream-model",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "你好"}},
	}, "req_1", false)
	if err != nil {
		t.Fatalf("PrepareChatCompletion returned error: %v", err)
	}
	body, err := service.CompleteChatCompletion(context.Background(), prepared)
	if err != nil {
		t.Fatalf("CompleteChatCompletion returned error: %v", err)
	}

	if body["model"] != "upstream-model" {
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

func TestStreamChatCompletionEmitsThinkContentAndUsage(t *testing.T) {
	key := &domainopenapi.UserAPIKey{ID: 9, UserID: 42, Status: domainopenapi.APIKeyStatusActive}
	provider := &chatProviderStub{
		streamEvents: []RawChatStreamEvent{
			{Reasoning: &llm.ReasoningDelta{Text: "先想"}},
			{Body: makeStreamContentChunk("upstream-model", "答案")},
		},
		streamResult: RawChatCompletionResult{Usage: llm.Usage{InputTokens: 2, OutputTokens: 3}},
	}
	service := NewService(Dependencies{
		KeyRepo:           newKeyRepoStub(),
		Settings:          settingsStub{"model_allowlist": "chat-openai", "rate_limit_rpm": "60"},
		Channel:           channelStub{},
		Billing:           &billingStub{},
		ChatProvider:      provider,
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	prepared, err := service.PrepareChatCompletion(context.Background(), key, map[string]interface{}{
		"model":    "upstream-model",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "你好"}},
	}, "req_1", true)
	if err != nil {
		t.Fatalf("PrepareChatCompletion returned error: %v", err)
	}

	var emitted []map[string]interface{}
	if err := service.StreamChatCompletion(context.Background(), prepared, func(event map[string]interface{}) error {
		emitted = append(emitted, event)
		return nil
	}); err != nil {
		t.Fatalf("StreamChatCompletion returned error: %v", err)
	}

	if len(emitted) != 4 {
		t.Fatalf("expected think open, think close, content, usage chunks; got %#v", emitted)
	}
	if got := streamContentDelta(emitted[0]); got != "<think>先想" {
		t.Fatalf("unexpected think open chunk: %q", got)
	}
	if got := streamContentDelta(emitted[1]); got != "</think>" {
		t.Fatalf("unexpected think close chunk: %q", got)
	}
	if got := streamContentDelta(emitted[2]); got != "答案" {
		t.Fatalf("unexpected content chunk: %q", got)
	}
	usage := emitted[3]["usage"].(map[string]interface{})
	if usage["prompt_tokens"].(int64) != 2 || usage["completion_tokens"].(int64) != 3 {
		t.Fatalf("unexpected usage chunk: %#v", usage)
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

type twoFactorStub struct {
	enabled bool
}

func (s twoFactorStub) IsTwoFactorEnabled(context.Context, uint) (bool, error) {
	return s.enabled, nil
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
		{PlatformModelName: "chat-responses", KindsJSON: `["chat"]`, ProtocolsJSON: `["openai_responses"]`},
		{PlatformModelName: "chat-google", KindsJSON: `["chat"]`, ProtocolsJSON: `["google_generate_content"]`},
		{PlatformModelName: "chat-xai", KindsJSON: `["chat"]`, ProtocolsJSON: `["xai_responses"]`},
		{PlatformModelName: "image-only", KindsJSON: `["image_gen"]`, ProtocolsJSON: `["openai_image_generations"]`},
		{PlatformModelName: "chat-anthropic", KindsJSON: `["chat"]`, ProtocolsJSON: `["anthropic_messages"]`},
		{PlatformModelName: "not-allowlisted", KindsJSON: `["chat"]`, ProtocolsJSON: `["openai_chat_completions"]`},
	}, nil
}

func (channelStub) ListActiveModelRoutes(_ context.Context, platformModelName string) ([]appchannel.ActiveModelRouteView, error) {
	routes := map[string][]appchannel.ActiveModelRouteView{
		"chat-openai": {
			{PlatformModelName: "chat-openai", UpstreamModelName: "upstream-model", Protocol: llm.AdapterOpenAIChatCompletions, KindsJSON: `["chat"]`},
		},
		"chat-responses": {
			{PlatformModelName: "chat-responses", UpstreamModelName: "gpt-responses-upstream", Protocol: llm.AdapterOpenAIResponses, KindsJSON: `["chat"]`},
		},
		"chat-google": {
			{PlatformModelName: "chat-google", UpstreamModelName: "gemini-upstream", Protocol: llm.AdapterGoogleGenerateContent, KindsJSON: `["chat"]`},
		},
		"chat-xai": {
			{PlatformModelName: "chat-xai", UpstreamModelName: "grok-upstream", Protocol: llm.AdapterXAIResponses, KindsJSON: `["chat"]`},
		},
		"chat-anthropic": {
			{PlatformModelName: "chat-anthropic", UpstreamModelName: "claude-upstream", Protocol: llm.AdapterAnthropicMessages, KindsJSON: `["chat"]`},
		},
		"image-only": {
			{PlatformModelName: "image-only", UpstreamModelName: "image-upstream", Protocol: llm.AdapterOpenAIImageGenerations, KindsJSON: `["image_gen"]`},
		},
	}
	return routes[strings.TrimSpace(platformModelName)], nil
}

func (channelStub) ResolveRoute(_ context.Context, input appchannel.ResolveRouteInput) (*appchannel.ResolvedRoute, error) {
	if input.PlatformModelName != "chat-openai" {
		return nil, appchannel.ErrModelNotFound
	}
	if strings.TrimSpace(input.UpstreamModelName) != "upstream-model" {
		return nil, appchannel.ErrRouteNotFound
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

type capturingChannelStub struct {
	protocol          string
	preferredProtocol string
	upstreamModelName string
}

func (c *capturingChannelStub) ListActiveModels(context.Context) ([]appchannel.ModelView, error) {
	return nil, nil
}

func (c *capturingChannelStub) ListActiveModelRoutes(context.Context, string) ([]appchannel.ActiveModelRouteView, error) {
	return nil, nil
}

func (c *capturingChannelStub) ResolveRoute(_ context.Context, input appchannel.ResolveRouteInput) (*appchannel.ResolvedRoute, error) {
	c.preferredProtocol = input.PreferredProtocol
	c.upstreamModelName = input.UpstreamModelName
	return &appchannel.ResolvedRoute{
		PlatformModelName: input.PlatformModelName,
		Protocol:          c.protocol,
		BaseURL:           "https://upstream.example",
		UpstreamModel:     strings.TrimSpace(input.UpstreamModelName),
		BindingCode:       "binding_1",
		UpstreamName:      "upstream",
	}, nil
}

func (*capturingChannelStub) MarkRouteSuccess(context.Context, *appchannel.ResolvedRoute)        {}
func (*capturingChannelStub) MarkRouteFailure(context.Context, *appchannel.ResolvedRoute, error) {}

type chatProviderStub struct {
	complete     RawChatCompletionResult
	streamEvents []RawChatStreamEvent
	streamResult RawChatCompletionResult
}

func (p *chatProviderStub) CompleteChat(context.Context, llm.RouteConfig, map[string]interface{}) (RawChatCompletionResult, error) {
	return p.complete, nil
}

func (p *chatProviderStub) StreamChat(_ context.Context, _ llm.RouteConfig, _ map[string]interface{}, onEvent func(RawChatStreamEvent) error) (RawChatCompletionResult, error) {
	for _, event := range p.streamEvents {
		if err := onEvent(event); err != nil {
			return RawChatCompletionResult{}, err
		}
	}
	return p.streamResult, nil
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

func sortedModelIDs(items []OpenAIModel) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		result = append(result, item.ID)
	}
	sort.Strings(result)
	return result
}

type jsonNumber string

func (n jsonNumber) String() string {
	return string(n)
}

func getStringFromNestedMap(payload map[string]interface{}, parent string, child string) string {
	nested, _ := payload[parent].(map[string]interface{})
	value, _ := nested[child].(string)
	return value
}
