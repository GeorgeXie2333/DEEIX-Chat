package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestLLMRawChatProviderConvertsAnthropicMessagesToChatCompletion(t *testing.T) {
	var upstreamPath string
	var upstreamBody map[string]interface{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"msg_test",
			"type":"message",
			"role":"assistant",
			"content":[{"type":"text","text":"hello from anthropic"}],
			"usage":{"input_tokens":3,"output_tokens":5}
		}`))
	}))
	defer upstream.Close()

	provider := NewLLMRawChatProvider(llm.NewClientWithEnv("test", false))
	result, err := provider.CompleteChat(context.Background(), llm.RouteConfig{
		Protocol:      llm.AdapterAnthropicMessages,
		BaseURL:       upstream.URL,
		UpstreamModel: "claude-upstream",
	}, map[string]interface{}{
		"model":    "public-model",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hi"}},
	})
	if err != nil {
		t.Fatalf("CompleteChat returned error: %v", err)
	}
	if upstreamPath != "/v1/messages" {
		t.Fatalf("expected Anthropic Messages path, got %q", upstreamPath)
	}
	if upstreamBody["model"] != "claude-upstream" {
		t.Fatalf("expected upstream model rewrite, got %#v", upstreamBody["model"])
	}
	if result.Body["object"] != "chat.completion" {
		t.Fatalf("expected chat completion response, got %#v", result.Body)
	}
	message := result.Body["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	if message["content"] != "hello from anthropic" {
		t.Fatalf("unexpected assistant content: %#v", message["content"])
	}
	if result.Usage.InputTokens != 3 || result.Usage.OutputTokens != 5 {
		t.Fatalf("unexpected usage: %#v", result.Usage)
	}
}
