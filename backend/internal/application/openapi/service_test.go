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
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

func TestCreateAPIKeyStoresOnlyHashAndAuthenticatesPlaintext(t *testing.T) {
	repo := newKeyRepoStub()
	service := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: true},
		UserStatus:        &userStatusStub{status: domainuser.StatusActive},
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
		UserStatus:        &userStatusStub{status: domainuser.StatusActive},
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

func TestCreateAPIKeyMapsAtomicCreateConflictToAlreadyExists(t *testing.T) {
	repo := newKeyRepoStub()
	repo.createErr = repository.ErrConflict
	service := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: true},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})

	if _, err := service.CreateAPIKey(context.Background(), 42); !errors.Is(err, ErrAPIKeyAlreadyExists) {
		t.Fatalf("expected atomic create conflict to report existing key, got %v", err)
	}
}

func TestRegenerateAPIKeyReturnsConflictWhenSnapshotChanges(t *testing.T) {
	repo := newKeyRepoStub()
	service := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: true},
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	if _, err := service.CreateAPIKey(context.Background(), 42); err != nil {
		t.Fatalf("create initial key: %v", err)
	}
	repo.replaceErr = repository.ErrConflict

	if view, err := service.RegenerateAPIKey(context.Background(), 42); !errors.Is(err, ErrAPIKeyConflict) || view != nil {
		t.Fatalf("expected stale regeneration snapshot to return no plaintext and ErrAPIKeyConflict, view=%#v err=%v", view, err)
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
		UserStatus:        &userStatusStub{status: domainuser.StatusActive},
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
		UserStatus:        &userStatusStub{status: domainuser.StatusActive},
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

	result, err := service.ListModels(context.Background(), 42)
	if err != nil {
		t.Fatalf("ListModels returned error: %v", err)
	}
	if result.Object != "list" {
		t.Fatalf("expected OpenAI list object, got %q", result.Object)
	}
	if !result.Success {
		t.Fatalf("expected new-api compatible success flag")
	}
	if got, want := sortedModelIDs(result.Data), []string{"claude-upstream", "gemini-upstream", "gpt-responses-upstream", "grok-upstream", "upstream-model"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected public models: got %#v want %#v", got, want)
	}
	for _, item := range result.Data {
		if item.Object != "model" {
			t.Fatalf("expected model object, got %#v", item)
		}
		if item.Created != newAPIModelCreatedUnix {
			t.Fatalf("expected new-api compatible created timestamp, got %#v", item)
		}
		if item.OwnedBy == "" {
			t.Fatalf("expected owned_by to be populated, got %#v", item)
		}
		if !reflect.DeepEqual(item.SupportedEndpointTypes, []string{"openai"}) {
			t.Fatalf("expected supported endpoint types, got %#v", item)
		}
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
		ModelOptionFilter: passthroughModelOptionFilter{},
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

func TestPrepareChatCompletionNormalizesMaxTokensByResolvedRoute(t *testing.T) {
	key := &domainopenapi.UserAPIKey{ID: 9, UserID: 42, Status: domainopenapi.APIKeyStatusActive}
	tests := []struct {
		name                string
		protocol            string
		compatible          string
		request             map[string]interface{}
		wantMaxTokens       interface{}
		wantMaxCompletion   interface{}
		wantMaxOutput       interface{}
		wantNoMaxTokens     bool
		wantNoMaxCompletion bool
		wantNoMaxOutput     bool
	}{
		{
			name:       "official OpenAI Chat Completions maps legacy max_tokens",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "openai",
			request: map[string]interface{}{
				"max_tokens": 128,
			},
			wantMaxCompletion: 128,
			wantNoMaxTokens:   true,
			wantNoMaxOutput:   true,
		},
		{
			name:       "custom Chat Completions keeps legacy max_tokens",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "custom",
			request: map[string]interface{}{
				"max_tokens": 256,
			},
			wantMaxTokens:       256,
			wantNoMaxCompletion: true,
			wantNoMaxOutput:     true,
		},
		{
			name:       "official OpenAI target field wins conflicts",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "openai",
			request: map[string]interface{}{
				"max_tokens":            128,
				"max_completion_tokens": 64,
			},
			wantMaxCompletion: 64,
			wantNoMaxTokens:   true,
			wantNoMaxOutput:   true,
		},
		{
			name:       "official OpenAI Responses route maps legacy max_tokens for Chat Completions passthrough",
			protocol:   llm.AdapterOpenAIResponses,
			compatible: "openai",
			request: map[string]interface{}{
				"max_tokens": 128,
			},
			wantMaxCompletion: 128,
			wantNoMaxTokens:   true,
			wantNoMaxOutput:   true,
		},
		{
			name:       "official OpenAI Responses route maps Responses max_output_tokens for Chat Completions passthrough",
			protocol:   llm.AdapterOpenAIResponses,
			compatible: "openai",
			request: map[string]interface{}{
				"max_output_tokens": 96,
			},
			wantMaxCompletion: 96,
			wantNoMaxTokens:   true,
			wantNoMaxOutput:   true,
		},
		{
			name:       "official OpenAI Responses route keeps Chat Completions target field in conflicts",
			protocol:   llm.AdapterOpenAIResponses,
			compatible: "openai",
			request: map[string]interface{}{
				"max_tokens":            128,
				"max_output_tokens":     96,
				"max_completion_tokens": 64,
			},
			wantMaxCompletion: 64,
			wantNoMaxTokens:   true,
			wantNoMaxOutput:   true,
		},
		{
			name:       "Gemini maps legacy max_tokens",
			protocol:   llm.AdapterGoogleGenerateContent,
			compatible: "google",
			request: map[string]interface{}{
				"max_tokens": 300,
			},
			wantMaxOutput:       300,
			wantNoMaxTokens:     true,
			wantNoMaxCompletion: true,
		},
		{
			name:       "Gemini maps OpenAI completion alias",
			protocol:   llm.AdapterGoogleGenerateContent,
			compatible: "google",
			request: map[string]interface{}{
				"max_completion_tokens": 200,
			},
			wantMaxOutput:       200,
			wantNoMaxTokens:     true,
			wantNoMaxCompletion: true,
		},
		{
			name:       "Gemini target field wins conflicts",
			protocol:   llm.AdapterGoogleGenerateContent,
			compatible: "google",
			request: map[string]interface{}{
				"max_tokens":            300,
				"max_completion_tokens": 200,
				"max_output_tokens":     100,
			},
			wantMaxOutput:       100,
			wantNoMaxTokens:     true,
			wantNoMaxCompletion: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &capturingChannelStub{protocol: tt.protocol, compatible: tt.compatible}
			service := NewService(Dependencies{
				KeyRepo:           newKeyRepoStub(),
				Settings:          settingsStub{"model_allowlist": "chat-openai", "rate_limit_rpm": "60"},
				Channel:           channel,
				ChatProvider:      &chatProviderStub{},
				ModelOptionFilter: passthroughModelOptionFilter{},
				DataEncryptionKey: "test-secret",
				Now:               fixedNow,
			})
			request := map[string]interface{}{
				"model":    "upstream-model",
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
			}
			for key, value := range tt.request {
				request[key] = value
			}

			prepared, err := service.PrepareChatCompletion(context.Background(), key, request, "req_1", false)
			if err != nil {
				t.Fatalf("PrepareChatCompletion returned error: %v", err)
			}
			assertRequestField(t, prepared.request, "max_tokens", tt.wantMaxTokens, tt.wantNoMaxTokens)
			assertRequestField(t, prepared.request, "max_completion_tokens", tt.wantMaxCompletion, tt.wantNoMaxCompletion)
			assertRequestField(t, prepared.request, "max_output_tokens", tt.wantMaxOutput, tt.wantNoMaxOutput)
		})
	}
}

func TestPrepareChatCompletionDropsOfficialOpenAIReasoningEffortWhenToolsPresent(t *testing.T) {
	key := &domainopenapi.UserAPIKey{ID: 9, UserID: 42, Status: domainopenapi.APIKeyStatusActive}
	tests := []struct {
		name                string
		protocol            string
		compatible          string
		request             map[string]interface{}
		wantReasoningEffort interface{}
		wantNoReasoning     bool
	}{
		{
			name:       "official OpenAI Chat Completions drops reasoning effort with tools",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "openai",
			request: map[string]interface{}{
				"reasoning_effort": "medium",
				"tools": []interface{}{map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": "lookup",
						"parameters": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				}},
			},
			wantNoReasoning: true,
		},
		{
			name:       "official OpenAI Responses Chat Completions passthrough drops reasoning effort with tools",
			protocol:   llm.AdapterOpenAIResponses,
			compatible: "openai",
			request: map[string]interface{}{
				"reasoning_effort": "high",
				"tools": []interface{}{map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": "lookup",
						"parameters": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				}},
			},
			wantNoReasoning: true,
		},
		{
			name:       "official OpenAI keeps reasoning effort without tools",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "openai",
			request: map[string]interface{}{
				"reasoning_effort": "low",
			},
			wantReasoningEffort: "low",
		},
		{
			name:       "custom Chat Completions keeps reasoning effort with tools",
			protocol:   llm.AdapterOpenAIChatCompletions,
			compatible: "custom",
			request: map[string]interface{}{
				"reasoning_effort": "medium",
				"tools": []interface{}{map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name": "lookup",
						"parameters": map[string]interface{}{
							"type":       "object",
							"properties": map[string]interface{}{},
						},
					},
				}},
			},
			wantReasoningEffort: "medium",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			channel := &capturingChannelStub{protocol: tt.protocol, compatible: tt.compatible}
			service := NewService(Dependencies{
				KeyRepo:           newKeyRepoStub(),
				Settings:          settingsStub{"model_allowlist": "chat-openai", "rate_limit_rpm": "60"},
				Channel:           channel,
				ChatProvider:      &chatProviderStub{},
				ModelOptionFilter: passthroughModelOptionFilter{},
				DataEncryptionKey: "test-secret",
				Now:               fixedNow,
			})
			request := map[string]interface{}{
				"model":    "upstream-model",
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
			}
			for key, value := range tt.request {
				request[key] = value
			}

			prepared, err := service.PrepareChatCompletion(context.Background(), key, request, "req_1", false)
			if err != nil {
				t.Fatalf("PrepareChatCompletion returned error: %v", err)
			}
			assertRequestField(t, prepared.request, "reasoning_effort", tt.wantReasoningEffort, tt.wantNoReasoning)
			if _, ok := prepared.request["tools"]; !ok && tt.request["tools"] != nil {
				t.Fatalf("expected tools to be preserved, got %#v", prepared.request)
			}
		})
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

func TestAuthenticateAPIKeyRejectsInactiveUser(t *testing.T) {
	repo := newKeyRepoStub()
	status := &userStatusStub{status: domainuser.StatusActive}
	service := NewService(Dependencies{
		KeyRepo:           repo,
		TwoFactor:         twoFactorStub{enabled: true},
		UserStatus:        status,
		DataEncryptionKey: "test-secret",
		Now:               fixedNow,
	})
	created, err := service.CreateAPIKey(context.Background(), 42)
	if err != nil {
		t.Fatalf("CreateAPIKey returned error: %v", err)
	}
	status.status = domainuser.StatusSuspended
	if _, err = service.AuthenticateAPIKey(context.Background(), created.APIKey); !errors.Is(err, ErrInvalidAPIKey) {
		t.Fatalf("expected suspended user key to be rejected, got %v", err)
	}
	status.status = domainuser.StatusActive
	status.err = errors.New("user repository unavailable")
	if _, err = service.AuthenticateAPIKey(context.Background(), created.APIKey); !errors.Is(err, status.err) {
		t.Fatalf("expected user repository failure to remain an internal error, got %v", err)
	}
}

func TestFilterPreparedChatRequestKeepsClientFunctionsAndAppliesProviderOptionPolicy(t *testing.T) {
	filter := modelOptionFilterStub{filtered: map[string]interface{}{
		"temperature": 0.2,
	}}
	request := map[string]interface{}{
		"model":               "upstream-model",
		"messages":            []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
		"temperature":         0.8,
		"web_search":          true,
		"tool_choice":         map[string]interface{}{"type": "function", "function": map[string]interface{}{"name": "lookup"}},
		"parallel_tool_calls": false,
		"tools": []interface{}{
			map[string]interface{}{
				"type":     "function",
				"function": map[string]interface{}{"name": "lookup", "parameters": map[string]interface{}{"type": "object"}},
			},
			map[string]interface{}{"type": "web_search_preview"},
		},
	}
	route := &appchannel.ResolvedRoute{Protocol: llm.AdapterOpenAIResponses, ModelCapabilitiesJSON: `{}`}
	prepared := filterPreparedChatRequest(request, route, filter)

	if prepared["web_search"] != nil {
		t.Fatalf("expected disallowed provider option to be removed, got %#v", prepared)
	}
	if prepared["temperature"] != 0.2 {
		t.Fatalf("expected filtered option value, got %#v", prepared["temperature"])
	}
	tools, ok := prepared["tools"].([]interface{})
	if !ok || len(tools) != 1 {
		t.Fatalf("expected only the client function tool to remain, got %#v", prepared["tools"])
	}
	if choice := asMap(prepared["tool_choice"]); getStringFromNestedMap(choice, "function", "name") != "lookup" {
		t.Fatalf("expected client function tool_choice to remain, got %#v", prepared["tool_choice"])
	}
	if parallel, ok := prepared["parallel_tool_calls"].(bool); !ok || parallel {
		t.Fatalf("expected parallel_tool_calls=false to remain, got %#v", prepared["parallel_tool_calls"])
	}
}

func TestRecordBillingUsesAuthorizationSnapshotAndServerToolUsage(t *testing.T) {
	billing := &billingStub{}
	authorization := &domainbilling.UsageAuthorization{
		Mode:        "usage",
		Reservation: &domainbilling.UsageBalanceReservation{UserID: 42, RefNo: "req_1"},
	}
	service := NewService(Dependencies{Billing: billing, Now: fixedNow})
	prepared := &PreparedChatCompletion{
		key:               &domainopenapi.UserAPIKey{UserID: 42},
		authorization:     authorization,
		startedAt:         fixedNow(),
		platformModelName: "chat-openai",
		route:             &appchannel.ResolvedRoute{Protocol: llm.AdapterOpenAIResponses},
	}
	toolUsage := map[string]int64{"web_search": 2}
	if err := service.recordBilling(context.Background(), prepared, llm.Usage{InputTokens: 3, OutputTokens: 4}, toolUsage); err != nil {
		t.Fatalf("recordBilling returned error: %v", err)
	}
	if billing.pricingInput == nil || billing.pricingInput.Authorization != authorization {
		t.Fatalf("expected original authorization snapshot, got %#v", billing.pricingInput)
	}
	if billing.pricingInput.ServerSideToolUsage["web_search"] != 2 {
		t.Fatalf("expected server-side tool usage to reach billing, got %#v", billing.pricingInput.ServerSideToolUsage)
	}
	if !billing.pricingInput.BillingAt.Equal(fixedNow()) {
		t.Fatalf("expected request start time for billing, got %v", billing.pricingInput.BillingAt)
	}
}

func TestCompleteChatCompletionRetainsReservationForUnpricedFallbackInputs(t *testing.T) {
	tests := []struct {
		name               string
		request            map[string]interface{}
		wantReconciliation bool
	}{
		{
			name: "image input",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{
					"role": "user",
					"content": []interface{}{map[string]interface{}{
						"type":      "image_url",
						"image_url": "https://images.example.test/cat.png",
					}},
				}},
			},
			wantReconciliation: true,
		},
		{
			name: "tool definitions",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
				"tools": []interface{}{map[string]interface{}{
					"type": "function",
					"function": map[string]interface{}{
						"name":        "lookup",
						"description": "expensive schema omitted by text fallback",
					},
				}},
			},
			wantReconciliation: true,
		},
		{
			name: "plain text",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
			},
			wantReconciliation: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			billing := &billingStub{}
			service := NewService(Dependencies{
				Billing: billing,
				ChatProvider: &chatProviderStub{complete: RawChatCompletionResult{Body: map[string]interface{}{
					"choices": []interface{}{map[string]interface{}{
						"message": map[string]interface{}{"content": "answer"},
					}},
				}}},
				Now: fixedNow,
			})

			body, err := service.CompleteChatCompletion(context.Background(), preparedOpenAPIUsageTestRequest(test.request))
			if err != nil {
				t.Fatalf("CompleteChatCompletion returned error: %v", err)
			}
			if usage := asMap(body["usage"]); len(usage) == 0 {
				t.Fatalf("expected compatible fallback usage response, got %#v", body)
			}
			if billing.reconciled != test.wantReconciliation {
				t.Fatalf("reconciliation = %v, want %v", billing.reconciled, test.wantReconciliation)
			}
			if test.wantReconciliation {
				if billing.recorded != nil {
					t.Fatalf("expected no fallback ledger for unpriced input, got %#v", billing.recorded)
				}
				if billing.reconciliationFailureCode != "open_api_usage_missing_for_non_text_input" {
					t.Fatalf("unexpected reconciliation failure code %q", billing.reconciliationFailureCode)
				}
				return
			}
			if billing.recorded == nil || billing.recorded.InputTokens <= 0 {
				t.Fatalf("expected text-only fallback to settle usage, got %#v", billing.recorded)
			}
		})
	}
}

func TestStreamChatCompletionRetainsReservationForUnpricedFallbackInputs(t *testing.T) {
	tests := []struct {
		name               string
		request            map[string]interface{}
		wantReconciliation bool
	}{
		{
			name: "image input",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{
					"role": "user",
					"content": []interface{}{map[string]interface{}{
						"type":      "input_image",
						"image_url": "https://images.example.test/cat.png",
					}},
				}},
			},
			wantReconciliation: true,
		},
		{
			name: "legacy functions",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
				"functions": []interface{}{map[string]interface{}{
					"name":        "lookup",
					"description": "expensive schema omitted by text fallback",
				}},
			},
			wantReconciliation: true,
		},
		{
			name: "plain text",
			request: map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
			},
			wantReconciliation: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			billing := &billingStub{}
			service := NewService(Dependencies{
				Billing: billing,
				ChatProvider: &chatProviderStub{streamEvents: []RawChatStreamEvent{{
					Body: map[string]interface{}{
						"choices": []interface{}{map[string]interface{}{
							"delta": map[string]interface{}{"content": "answer"},
						}},
					},
				}}},
				Now: fixedNow,
			})
			emitted := make([]map[string]interface{}, 0)
			err := service.StreamChatCompletion(context.Background(), preparedOpenAPIUsageTestRequest(test.request), func(chunk map[string]interface{}) error {
				emitted = append(emitted, chunk)
				return nil
			})
			if err != nil {
				t.Fatalf("StreamChatCompletion returned error: %v", err)
			}
			if len(emitted) < 2 {
				t.Fatalf("expected content and compatible usage chunks, got %#v", emitted)
			}
			if billing.reconciled != test.wantReconciliation {
				t.Fatalf("reconciliation = %v, want %v", billing.reconciled, test.wantReconciliation)
			}
			if test.wantReconciliation {
				if billing.recorded != nil {
					t.Fatalf("expected no fallback ledger for unpriced input, got %#v", billing.recorded)
				}
				return
			}
			if billing.recorded == nil || billing.recorded.InputTokens <= 0 {
				t.Fatalf("expected text-only fallback to settle usage, got %#v", billing.recorded)
			}
		})
	}
}

func preparedOpenAPIUsageTestRequest(request map[string]interface{}) *PreparedChatCompletion {
	return &PreparedChatCompletion{
		key:               &domainopenapi.UserAPIKey{UserID: 42, Status: domainopenapi.APIKeyStatusActive},
		request:           request,
		authorization:     &domainbilling.UsageAuthorization{Mode: "usage", Reservation: &domainbilling.UsageBalanceReservation{UserID: 42, RefNo: "req_1"}},
		startedAt:         fixedNow(),
		platformModelName: "chat-openai",
		publicModelID:     "upstream-model",
		route:             &appchannel.ResolvedRoute{Protocol: llm.AdapterOpenAIChatCompletions},
	}
}

func TestCompleteChatCompletionReconcilesOnlyAcceptedUpstreamErrors(t *testing.T) {
	tests := []struct {
		name               string
		providerErr        error
		wantReconciliation bool
		wantRelease        bool
	}{
		{name: "accepted", providerErr: llm.MarkRequestAccepted(errors.New("response lost")), wantReconciliation: true},
		{name: "not dispatched", providerErr: errors.New("dial failed"), wantRelease: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			billing := &billingStub{}
			service := NewService(Dependencies{
				Billing:      billing,
				ChatProvider: &chatProviderStub{completeErr: test.providerErr},
				Now:          fixedNow,
			})
			_, err := service.CompleteChatCompletion(t.Context(), preparedOpenAPIUsageTestRequest(map[string]interface{}{
				"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
			}))
			if !errors.Is(err, test.providerErr) {
				t.Fatalf("CompleteChatCompletion error = %v, want %v", err, test.providerErr)
			}
			if billing.reconciled != test.wantReconciliation || billing.released != test.wantRelease {
				t.Fatalf("billing state: reconciled=%v released=%v", billing.reconciled, billing.released)
			}
			if test.wantReconciliation && billing.reconciliationFailureCode != "open_api_upstream_failed_after_acceptance" {
				t.Fatalf("unexpected reconciliation code %q", billing.reconciliationFailureCode)
			}
		})
	}
}

func TestStreamChatCompletionReconcilesAcceptedErrorBeforeFirstEvent(t *testing.T) {
	billing := &billingStub{}
	providerErr := llm.MarkRequestAccepted(errors.New("stream response lost"))
	service := NewService(Dependencies{
		Billing:      billing,
		ChatProvider: &chatProviderStub{streamErr: providerErr},
		Now:          fixedNow,
	})
	err := service.StreamChatCompletion(t.Context(), preparedOpenAPIUsageTestRequest(map[string]interface{}{
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hello"}},
	}), func(map[string]interface{}) error { return nil })
	if !errors.Is(err, providerErr) {
		t.Fatalf("StreamChatCompletion error = %v, want %v", err, providerErr)
	}
	if !billing.reconciled || billing.released {
		t.Fatalf("expected accepted stream error to reconcile without release: reconciled=%v released=%v", billing.reconciled, billing.released)
	}
	if billing.reconciliationFailureCode != "open_api_stream_failed_after_acceptance" {
		t.Fatalf("unexpected reconciliation code %q", billing.reconciliationFailureCode)
	}
}

func TestStreamChatCompletionMarksReconciliationAfterDownstreamWriteFailure(t *testing.T) {
	billing := &billingStub{}
	service := NewService(Dependencies{
		Billing: billing,
		ChatProvider: &chatProviderStub{streamEvents: []RawChatStreamEvent{{
			Body: map[string]interface{}{
				"choices": []interface{}{map[string]interface{}{
					"delta": map[string]interface{}{"content": "partial"},
				}},
			},
		}},
		},
		Now: fixedNow,
	})
	prepared := &PreparedChatCompletion{
		key:               &domainopenapi.UserAPIKey{UserID: 42},
		authorization:     &domainbilling.UsageAuthorization{Mode: "usage", Reservation: &domainbilling.UsageBalanceReservation{UserID: 42, RefNo: "req_1"}},
		platformModelName: "chat-openai",
		publicModelID:     "upstream-model",
		route:             &appchannel.ResolvedRoute{},
	}
	writeErr := errors.New("client disconnected")
	err := service.StreamChatCompletion(context.Background(), prepared, func(map[string]interface{}) error { return writeErr })
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected downstream write error, got %v", err)
	}
	if billing.released {
		t.Fatal("expected reservation not to be released after upstream output")
	}
	if !billing.reconciled {
		t.Fatal("expected reservation to be marked for reconciliation")
	}
}

func TestEnforcePreAuthRateLimitCountsRequestsBeforeKeyLookup(t *testing.T) {
	limiter := &rateLimiterStub{allowed: false}
	service := NewService(Dependencies{RateLimiter: limiter})
	if err := service.EnforcePreAuthRateLimit(context.Background(), "203.0.113.10"); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("expected pre-auth rate limit error, got %v", err)
	}
	if limiter.calls != 1 || !strings.Contains(limiter.key, "203.0.113.10") {
		t.Fatalf("expected IP bucket to be consumed, got calls=%d key=%q", limiter.calls, limiter.key)
	}
}

func TestParseDataURLImagePartRejectsDecodedPayloadOverLimit(t *testing.T) {
	_, err := parseDataURLImagePartWithLimit("data:image/png;base64,c291cmNl", 5)
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected oversized data URL to be rejected, got %v", err)
	}
	part, err := parseDataURLImagePartWithLimit("data:image/png;base64,c291cmNl", 6)
	if err != nil || string(part.Data) != "source" {
		t.Fatalf("expected exact decoded-size boundary to remain valid, part=%#v err=%v", part, err)
	}
}

func fixedNow() time.Time {
	return time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
}

type keyRepoStub struct {
	nextID     uint
	byUser     map[uint]*domainopenapi.UserAPIKey
	byHash     map[string]*domainopenapi.UserAPIKey
	createErr  error
	replaceErr error
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

func (r *keyRepoStub) CreateForUser(_ context.Context, item *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	if r.createErr != nil {
		return nil, r.createErr
	}
	if item == nil || item.UserID == 0 {
		return nil, repository.ErrInvalidInput
	}
	if r.byUser[item.UserID] != nil {
		return nil, repository.ErrConflict
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

func (r *keyRepoStub) ReplaceForUserIfCurrent(_ context.Context, item *domainopenapi.UserAPIKey, expected *domainopenapi.UserAPIKey) (*domainopenapi.UserAPIKey, error) {
	if r.replaceErr != nil {
		return nil, r.replaceErr
	}
	if item == nil || expected == nil || item.UserID == 0 || item.UserID != expected.UserID {
		return nil, repository.ErrInvalidInput
	}
	current := r.byUser[item.UserID]
	if current == nil || current.KeyHash != expected.KeyHash || current.Status != expected.Status {
		return nil, repository.ErrConflict
	}
	delete(r.byHash, current.KeyHash)
	cp := *item
	cp.ID = current.ID
	cp.CreatedAt = current.CreatedAt
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

type userStatusStub struct {
	status string
	err    error
}

func (s *userStatusStub) GetByID(_ context.Context, userID uint) (*domainuser.User, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &domainuser.User{ID: userID, Status: s.status}, nil
}

type passthroughModelOptionFilter struct{}

func (passthroughModelOptionFilter) FilterModelOptionsForRoute(options map[string]interface{}, _ string, _ string) map[string]interface{} {
	return cloneMap(options)
}

type modelOptionFilterStub struct {
	filtered map[string]interface{}
}

func (s modelOptionFilterStub) FilterModelOptionsForRoute(map[string]interface{}, string, string) map[string]interface{} {
	return cloneMap(s.filtered)
}

type rateLimiterStub struct {
	allowed bool
	err     error
	calls   int
	key     string
}

func (s *rateLimiterStub) AllowSlidingWindow(_ context.Context, key string, _ int, _ time.Duration, _ time.Duration) (bool, error) {
	s.calls++
	s.key = key
	return s.allowed, s.err
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

func (channelStub) ListActiveModels(context.Context, uint) ([]appchannel.ModelView, error) {
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
	compatible        string
	preferredProtocol string
	upstreamModelName string
}

func (c *capturingChannelStub) ListActiveModels(context.Context, uint) ([]appchannel.ModelView, error) {
	return nil, nil
}

func (c *capturingChannelStub) ListActiveModelRoutes(context.Context, string) ([]appchannel.ActiveModelRouteView, error) {
	return nil, nil
}

func (c *capturingChannelStub) ResolveRoute(_ context.Context, input appchannel.ResolveRouteInput) (*appchannel.ResolvedRoute, error) {
	c.preferredProtocol = input.PreferredProtocol
	c.upstreamModelName = input.UpstreamModelName
	return &appchannel.ResolvedRoute{
		PlatformModelName:  input.PlatformModelName,
		Protocol:           c.protocol,
		UpstreamCompatible: c.compatible,
		BaseURL:            "https://upstream.example",
		UpstreamModel:      strings.TrimSpace(input.UpstreamModelName),
		BindingCode:        "binding_1",
		UpstreamName:       "upstream",
	}, nil
}

func (*capturingChannelStub) MarkRouteSuccess(context.Context, *appchannel.ResolvedRoute)        {}
func (*capturingChannelStub) MarkRouteFailure(context.Context, *appchannel.ResolvedRoute, error) {}

type chatProviderStub struct {
	complete     RawChatCompletionResult
	completeErr  error
	streamEvents []RawChatStreamEvent
	streamResult RawChatCompletionResult
	streamErr    error
}

func (p *chatProviderStub) CompleteChat(context.Context, llm.RouteConfig, map[string]interface{}) (RawChatCompletionResult, error) {
	return p.complete, p.completeErr
}

func (p *chatProviderStub) StreamChat(_ context.Context, _ llm.RouteConfig, _ map[string]interface{}, onEvent func(RawChatStreamEvent) error) (RawChatCompletionResult, error) {
	for _, event := range p.streamEvents {
		if err := onEvent(event); err != nil {
			return RawChatCompletionResult{}, err
		}
	}
	return p.streamResult, p.streamErr
}

type billingStub struct {
	recorded                  *domainbilling.UsageLedger
	pricingInput              *appbilling.UsagePricingInput
	released                  bool
	reconciled                bool
	reconciliationFailureCode string
}

func (b *billingStub) AuthorizeUsage(context.Context, uint, string, string) (*domainbilling.UsageAuthorization, error) {
	return &domainbilling.UsageAuthorization{
		Mode:        "usage",
		Reservation: &domainbilling.UsageBalanceReservation{UserID: 42, BalanceNanousd: 1, RefNo: "req_1"},
	}, nil
}

func (b *billingStub) ReleaseUsageAuthorization(context.Context, *domainbilling.UsageAuthorization) error {
	b.released = true
	return nil
}

func (b *billingStub) RenewUsageAuthorization(context.Context, *domainbilling.UsageAuthorization) error {
	return nil
}

func (b *billingStub) MarkUsageAuthorizationForReconciliation(_ context.Context, _ *domainbilling.UsageAuthorization, failureCode string) error {
	b.reconciled = true
	b.reconciliationFailureCode = failureCode
	return nil
}

func (b *billingStub) BuildUsageLedger(_ context.Context, input appbilling.UsagePricingInput) (*domainbilling.UsageLedger, error) {
	b.pricingInput = &input
	return &domainbilling.UsageLedger{
		UserID:            input.UserID,
		PlatformModelName: input.PlatformModelName,
		InputTokens:       input.InputTokens,
		OutputTokens:      input.OutputTokens,
		ReasoningTokens:   input.ReasoningTokens,
	}, nil
}

func (b *billingStub) RecordUsageWithAuthorization(_ context.Context, ledger *domainbilling.UsageLedger, _ *domainbilling.UsageAuthorization) error {
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

func assertRequestField(t *testing.T, payload map[string]interface{}, key string, want interface{}, wantMissing bool) {
	t.Helper()
	got, ok := payload[key]
	if wantMissing {
		if ok {
			t.Fatalf("expected request field %q to be omitted, got %#v in %#v", key, got, payload)
		}
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected request field %q=%#v, got %#v in %#v", key, want, got, payload)
	}
}

func getStringFromNestedMap(payload map[string]interface{}, parent string, child string) string {
	nested, _ := payload[parent].(map[string]interface{})
	value, _ := nested[child].(string)
	return value
}
