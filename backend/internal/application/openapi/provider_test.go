package openapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestChatResultFromGenerateOutputIncludesToolCalls(t *testing.T) {
	result := chatResultFromGenerateOutput(&llm.GenerateOutput{
		ResponseID: "msg_tool",
		ToolCalls: []llm.ToolCall{{
			ToolName:      "get_weather",
			ArgumentsJSON: `{"city":"Paris"}`,
			Status:        "requested",
		}},
		Usage: llm.Usage{InputTokens: 4, OutputTokens: 1},
	}, "claude-upstream", false)

	choices := result.Body["choices"].([]interface{})
	choice := choices[0].(map[string]interface{})
	if choice["finish_reason"] != "tool_calls" {
		t.Fatalf("expected tool_calls finish reason, got %#v", choice["finish_reason"])
	}
	message := choice["message"].(map[string]interface{})
	if message["content"] != nil {
		t.Fatalf("expected nil assistant content for tool call response, got %#v", message["content"])
	}
	toolCalls := message["tool_calls"].([]interface{})
	call := toolCalls[0].(map[string]interface{})
	if !strings.HasPrefix(call["id"].(string), "call_") || call["type"] != "function" {
		t.Fatalf("expected generated function tool call id, got %#v", call)
	}
	function := call["function"].(map[string]interface{})
	if function["name"] != "get_weather" || function["arguments"] != `{"city":"Paris"}` {
		t.Fatalf("unexpected tool call function payload: %#v", function)
	}
}

func TestChatResultFromGenerateOutputIncludesLegacyFunctionCallWhenRequested(t *testing.T) {
	result := chatResultFromGenerateOutput(&llm.GenerateOutput{
		ResponseID: "msg_tool",
		ToolCalls: []llm.ToolCall{{
			ToolCallID:    "call_legacy",
			ToolName:      "legacy_lookup",
			ArgumentsJSON: `{"id":"42"}`,
			Status:        "requested",
		}},
	}, "claude-upstream", true)

	message := result.Body["choices"].([]interface{})[0].(map[string]interface{})["message"].(map[string]interface{})
	functionCall := message["function_call"].(map[string]interface{})
	if functionCall["name"] != "legacy_lookup" || functionCall["arguments"] != `{"id":"42"}` {
		t.Fatalf("expected legacy function_call compatibility payload, got %#v", functionCall)
	}
}

func TestLLMRawChatProviderStreamsAnthropicToolUseAsChatToolCallDelta(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("event: message_start\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_start","message":{"id":"msg_tool","usage":{"input_tokens":3}}}` + "\n\n"))
		_, _ = w.Write([]byte("event: content_block_start\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}` + "\n\n"))
		_, _ = w.Write([]byte("event: content_block_delta\n"))
		_, _ = w.Write([]byte(`data: {"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":"{\"city\":\"Paris\"}"}}` + "\n\n"))
		_, _ = w.Write([]byte("event: message_delta\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_delta","usage":{"output_tokens":2}}` + "\n\n"))
		_, _ = w.Write([]byte("event: message_stop\n"))
		_, _ = w.Write([]byte(`data: {"type":"message_stop"}` + "\n\n"))
	}))
	defer upstream.Close()

	provider := NewLLMRawChatProvider(llm.NewClientWithEnv("test", false))
	var chunks []map[string]interface{}
	result, err := provider.StreamChat(context.Background(), llm.RouteConfig{
		Protocol:      llm.AdapterAnthropicMessages,
		BaseURL:       upstream.URL,
		UpstreamModel: "claude-upstream",
	}, map[string]interface{}{
		"model": "public-model",
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": "weather"},
		},
		"tools": []interface{}{
			map[string]interface{}{
				"type": "function",
				"function": map[string]interface{}{
					"name":       "get_weather",
					"parameters": map[string]interface{}{"type": "object"},
				},
			},
		},
	}, func(event RawChatStreamEvent) error {
		if len(event.Body) > 0 {
			chunks = append(chunks, event.Body)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("StreamChat returned error: %v", err)
	}
	if len(result.ToolCalls) != 1 || result.ToolCalls[0].ToolCallID != "toolu_1" {
		t.Fatalf("expected final tool call result, got %#v", result.ToolCalls)
	}
	if len(chunks) == 0 {
		t.Fatalf("expected streamed tool call chunks")
	}
	var delta map[string]interface{}
	for _, chunk := range chunks {
		choices, _ := chunk["choices"].([]interface{})
		if len(choices) == 0 {
			continue
		}
		candidate, _ := choices[0].(map[string]interface{})["delta"].(map[string]interface{})
		toolCallDeltas, _ := candidate["tool_calls"].([]interface{})
		if len(toolCallDeltas) > 0 {
			delta = candidate
			break
		}
	}
	if len(delta) == 0 {
		t.Fatalf("expected streamed tool call delta, got %#v", chunks)
	}
	toolCalls := delta["tool_calls"].([]interface{})
	function := toolCalls[0].(map[string]interface{})["function"].(map[string]interface{})
	if function["name"] != "get_weather" || function["arguments"] != `{"city":"Paris"}` {
		t.Fatalf("unexpected streamed tool call delta: %#v", delta)
	}
}
