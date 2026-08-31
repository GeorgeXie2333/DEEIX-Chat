package openapi

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

type chatClientStub struct {
	generate          func(context.Context, llm.RouteConfig, llm.GenerateInput) (*llm.GenerateOutput, error)
	generateStream    func(context.Context, llm.RouteConfig, llm.GenerateInput, func(llm.GenerateStreamEvent) error) (*llm.GenerateOutput, error)
	generateRaw       func(context.Context, llm.RouteConfig, map[string]interface{}) (*llm.RawChatCompletionOutput, error)
	generateRawStream func(context.Context, llm.RouteConfig, map[string]interface{}, func(llm.RawChatCompletionStreamEvent) error) (*llm.RawChatCompletionOutput, error)
}

func (s chatClientStub) Generate(ctx context.Context, route llm.RouteConfig, input llm.GenerateInput) (*llm.GenerateOutput, error) {
	return s.generate(ctx, route, input)
}

func (s chatClientStub) GenerateStream(ctx context.Context, route llm.RouteConfig, input llm.GenerateInput, onEvent func(llm.GenerateStreamEvent) error) (*llm.GenerateOutput, error) {
	return s.generateStream(ctx, route, input, onEvent)
}

func (s chatClientStub) GenerateRawChatCompletion(ctx context.Context, route llm.RouteConfig, body map[string]interface{}) (*llm.RawChatCompletionOutput, error) {
	if s.generateRaw != nil {
		return s.generateRaw(ctx, route, body)
	}
	return nil, nil
}

func (s chatClientStub) GenerateRawChatCompletionStream(ctx context.Context, route llm.RouteConfig, body map[string]interface{}, onEvent func(llm.RawChatCompletionStreamEvent) error) (*llm.RawChatCompletionOutput, error) {
	if s.generateRawStream != nil {
		return s.generateRawStream(ctx, route, body, onEvent)
	}
	return nil, nil
}

func TestLLMRawChatProviderInlinesProtectedImagesBeforePassthrough(t *testing.T) {
	rawCalled := false
	provider := NewLLMRawChatProviderWithImageResolver(chatClientStub{
		generateRaw: func(_ context.Context, _ llm.RouteConfig, body map[string]interface{}) (*llm.RawChatCompletionOutput, error) {
			rawCalled = true
			messages := body["messages"].([]interface{})
			parts := messages[0].(map[string]interface{})["content"].([]interface{})
			imageURL := parts[0].(map[string]interface{})["image_url"].(map[string]interface{})
			if imageURL["detail"] != "high" || imageURL["url"] != "data:image/png;base64,aW1hZ2U=" {
				t.Fatalf("unexpected inlined image payload: %#v", imageURL)
			}
			return &llm.RawChatCompletionOutput{}, nil
		},
	}, chatImageResolverFunc(func(_ context.Context, rawURL string) (llm.ContentPart, error) {
		if rawURL != "https://cdn.example.test/image.png" {
			t.Fatalf("unexpected image URL %q", rawURL)
		}
		return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: "image/png", Data: []byte("image")}, nil
	}))

	_, err := provider.CompleteChat(t.Context(), llm.RouteConfig{Protocol: llm.AdapterOpenAIChatCompletions}, map[string]interface{}{
		"messages": []interface{}{map[string]interface{}{
			"role": "user",
			"content": []interface{}{map[string]interface{}{
				"type":      "image_url",
				"image_url": map[string]interface{}{"url": "https://cdn.example.test/image.png", "detail": "high"},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("CompleteChat returned error: %v", err)
	}
	if !rawCalled {
		t.Fatal("expected passthrough client to be called")
	}
}

func TestLLMRawChatProviderRejectsUnsafePassthroughImageBeforeUpstream(t *testing.T) {
	rawCalled := false
	provider := NewLLMRawChatProviderWithImageResolver(chatClientStub{
		generateRaw: func(context.Context, llm.RouteConfig, map[string]interface{}) (*llm.RawChatCompletionOutput, error) {
			rawCalled = true
			return nil, nil
		},
	}, NewHTTPChatImageResolver(1024))

	_, err := provider.CompleteChat(t.Context(), llm.RouteConfig{Protocol: llm.AdapterOpenAIChatCompletions}, map[string]interface{}{
		"messages": []interface{}{userImageMessage("https://127.0.0.1/image.png")},
	})
	if !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("expected unsafe passthrough image to be rejected, got %v", err)
	}
	if rawCalled {
		t.Fatal("unsafe image reached passthrough upstream")
	}
}

func TestLLMRawChatProviderConvertsAnthropicMessagesToChatCompletion(t *testing.T) {
	provider := NewLLMRawChatProvider(chatClientStub{generate: func(_ context.Context, route llm.RouteConfig, input llm.GenerateInput) (*llm.GenerateOutput, error) {
		if route.Protocol != llm.AdapterAnthropicMessages || route.UpstreamModel != "claude-upstream" {
			t.Fatalf("unexpected route: %#v", route)
		}
		if len(input.Messages) != 1 || input.Messages[0].Content != "hi" {
			t.Fatalf("unexpected normalized messages: %#v", input.Messages)
		}
		return &llm.GenerateOutput{
			ResponseID: "msg_test",
			Text:       "hello from anthropic",
			Usage:      llm.Usage{InputTokens: 3, OutputTokens: 5},
		}, nil
	}})
	result, err := provider.CompleteChat(context.Background(), llm.RouteConfig{
		Protocol:      llm.AdapterAnthropicMessages,
		UpstreamModel: "claude-upstream",
	}, map[string]interface{}{
		"model":    "public-model",
		"messages": []interface{}{map[string]interface{}{"role": "user", "content": "hi"}},
	})
	if err != nil {
		t.Fatalf("CompleteChat returned error: %v", err)
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
		Usage:               llm.Usage{InputTokens: 4, OutputTokens: 1},
		ServerSideToolUsage: map[string]int64{"web_search": 2},
	}, "claude-upstream", false)
	if result.ServerSideToolUsage["web_search"] != 2 {
		t.Fatalf("expected server-side tool usage to be preserved, got %#v", result.ServerSideToolUsage)
	}

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
	toolCall := llm.ToolCall{ToolCallID: "toolu_1", ToolType: "function", ToolName: "get_weather", ArgumentsJSON: `{"city":"Paris"}`}
	provider := NewLLMRawChatProvider(chatClientStub{generateStream: func(_ context.Context, route llm.RouteConfig, input llm.GenerateInput, onEvent func(llm.GenerateStreamEvent) error) (*llm.GenerateOutput, error) {
		if route.Protocol != llm.AdapterAnthropicMessages || len(input.Tools) != 1 {
			t.Fatalf("unexpected stream input: route=%#v input=%#v", route, input)
		}
		if err := onEvent(llm.GenerateStreamEvent{ResponseID: "msg_tool", ServerToolCall: &toolCall}); err != nil {
			return nil, err
		}
		return &llm.GenerateOutput{ResponseID: "msg_tool", ToolCalls: []llm.ToolCall{toolCall}}, nil
	}})
	var chunks []map[string]interface{}
	result, err := provider.StreamChat(context.Background(), llm.RouteConfig{
		Protocol:      llm.AdapterAnthropicMessages,
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
