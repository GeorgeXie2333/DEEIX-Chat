package openapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
)

// LLMRawChatProvider adapts the shared LLM client to the open API service.
type LLMRawChatProvider struct {
	client        llm.ChatClient
	imageResolver chatImageResolver
}

// NewLLMRawChatProvider 创建原始 Chat Completions 调用器。
func NewLLMRawChatProvider(client llm.ChatClient) *LLMRawChatProvider {
	return &LLMRawChatProvider{client: client}
}

// NewLLMRawChatProviderWithImageResolver 创建支持远程 image_url 下载的开放 API 调用器。
func NewLLMRawChatProviderWithImageResolver(client llm.ChatClient, resolver chatImageResolver) *LLMRawChatProvider {
	return &LLMRawChatProvider{client: client, imageResolver: resolver}
}

// CompleteChat 透传非流式 Chat Completions 请求。
func (p *LLMRawChatProvider) CompleteChat(ctx context.Context, route llm.RouteConfig, body map[string]interface{}) (RawChatCompletionResult, error) {
	if p == nil || p.client == nil {
		return RawChatCompletionResult{}, ErrModelNotAllowed
	}
	if !usesOpenAPIChatCompletionsPassthrough(route.Protocol) {
		input, err := buildGenerateInputFromChatCompletion(ctx, body, p.imageResolver)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		output, err := p.client.Generate(ctx, route, input)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		return chatResultFromGenerateOutput(output, route.UpstreamModel, requestUsesLegacyFunctionCalling(body)), nil
	}
	output, err := p.client.GenerateRawChatCompletion(ctx, route, body)
	if err != nil {
		return RawChatCompletionResult{}, err
	}
	return rawResultFromLLMOutput(output), nil
}

// StreamChat 透传流式 Chat Completions 请求。
func (p *LLMRawChatProvider) StreamChat(
	ctx context.Context,
	route llm.RouteConfig,
	body map[string]interface{},
	onEvent func(RawChatStreamEvent) error,
) (RawChatCompletionResult, error) {
	if p == nil || p.client == nil {
		return RawChatCompletionResult{}, ErrModelNotAllowed
	}
	if !usesOpenAPIChatCompletionsPassthrough(route.Protocol) {
		input, err := buildGenerateInputFromChatCompletion(ctx, body, p.imageResolver)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		toolEmitter := newStreamToolCallEmitter(route.UpstreamModel)
		output, err := p.client.GenerateStream(ctx, route, input, func(event llm.GenerateStreamEvent) error {
			if onEvent == nil {
				return nil
			}
			if event.ServerToolCall != nil && isClientFunctionToolCall(*event.ServerToolCall) {
				for _, chunk := range toolEmitter.chunks(event.ResponseID, *event.ServerToolCall) {
					if err := onEvent(RawChatStreamEvent{Body: chunk}); err != nil {
						return err
					}
				}
			}
			if event.Reasoning != nil {
				if err := onEvent(RawChatStreamEvent{Reasoning: event.Reasoning}); err != nil {
					return err
				}
			}
			if event.Delta != "" {
				if err := onEvent(RawChatStreamEvent{Body: chatStreamContentChunk(route.UpstreamModel, event.ResponseID, event.Delta)}); err != nil {
					return err
				}
			}
			if event.Usage != (llm.Usage{}) {
				if err := onEvent(RawChatStreamEvent{
					Body:  makeStreamUsageChunk(route.UpstreamModel, event.Usage),
					Usage: event.Usage,
				}); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		return chatResultFromGenerateOutput(output, route.UpstreamModel, requestUsesLegacyFunctionCalling(body)), nil
	}
	output, err := p.client.GenerateRawChatCompletionStream(ctx, route, body, func(event llm.RawChatCompletionStreamEvent) error {
		if onEvent == nil {
			return nil
		}
		return onEvent(RawChatStreamEvent{
			Body:      event.Body,
			Usage:     event.Usage,
			Reasoning: event.Reasoning,
		})
	})
	if err != nil {
		return RawChatCompletionResult{}, err
	}
	return rawResultFromLLMOutput(output), nil
}

func usesOpenAPIChatCompletionsPassthrough(protocol string) bool {
	switch llm.NormalizeAdapter(protocol) {
	case llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIResponses, llm.AdapterXAIResponses:
		return true
	default:
		return false
	}
}

func chatResultFromGenerateOutput(output *llm.GenerateOutput, model string, legacyFunctionCall bool) RawChatCompletionResult {
	if output == nil {
		return RawChatCompletionResult{Body: chatCompletionBody("", model, "", nil, llm.Usage{}, legacyFunctionCall)}
	}
	toolCalls := normalizeOpenAIOutputToolCalls(output.ToolCalls)
	body := chatCompletionBody(output.ResponseID, model, output.Text, toolCalls, output.Usage, legacyFunctionCall)
	result := RawChatCompletionResult{
		Body:       body,
		Usage:      output.Usage,
		ResponseID: output.ResponseID,
		ToolCalls:  toolCalls,
	}
	if output.Reasoning != nil {
		result.ReasoningText = firstNonEmpty(output.Reasoning.Summary, output.Reasoning.Text)
	}
	return result
}

func chatCompletionBody(responseID string, model string, content string, toolCalls []llm.ToolCall, usage llm.Usage, legacyFunctionCall bool) map[string]interface{} {
	message := map[string]interface{}{
		"role":    "assistant",
		"content": content,
	}
	finishReason := "stop"
	if len(toolCalls) > 0 {
		finishReason = "tool_calls"
		message["content"] = nil
		message["tool_calls"] = chatToolCallsPayload(toolCalls)
		if legacyFunctionCall && len(toolCalls) == 1 {
			message["function_call"] = legacyFunctionCallPayload(toolCalls[0])
		}
	}
	body := map[string]interface{}{
		"id":      chatCompletionID(responseID),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   strings.TrimSpace(model),
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"message":       message,
				"finish_reason": finishReason,
			},
		},
	}
	if usage != (llm.Usage{}) {
		body["usage"] = usageResponseMap(usage)
	}
	return body
}

func chatStreamContentChunk(model string, responseID string, delta string) map[string]interface{} {
	return map[string]interface{}{
		"id":      chatCompletionID(responseID),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   strings.TrimSpace(model),
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"content": delta,
				},
			},
		},
	}
}

func chatStreamToolCallChunk(model string, responseID string, index int, item llm.ToolCall, argumentsDelta string, includeIdentity bool) map[string]interface{} {
	function := map[string]interface{}{}
	if includeIdentity {
		function["name"] = strings.TrimSpace(item.ToolName)
		if signature := strings.TrimSpace(item.ThoughtSignature); signature != "" {
			function["thought_signature"] = signature
			function["thoughtSignature"] = signature
		}
	}
	if argumentsDelta != "" {
		function["arguments"] = argumentsDelta
	}
	toolCall := map[string]interface{}{
		"index": index,
	}
	if includeIdentity {
		toolCall["id"] = openAPIToolCallID(item)
		toolCall["type"] = "function"
		if signature := strings.TrimSpace(item.ThoughtSignature); signature != "" {
			toolCall["thought_signature"] = signature
			toolCall["thoughtSignature"] = signature
		}
	}
	if len(function) > 0 {
		toolCall["function"] = function
	}
	return map[string]interface{}{
		"id":      chatCompletionID(responseID),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   strings.TrimSpace(model),
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"tool_calls": []interface{}{toolCall},
				},
			},
		},
	}
}

func chatStreamToolFinishChunk(model string, responseID string) map[string]interface{} {
	return map[string]interface{}{
		"id":      chatCompletionID(responseID),
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   strings.TrimSpace(model),
		"choices": []interface{}{
			map[string]interface{}{
				"index":         0,
				"delta":         map[string]interface{}{},
				"finish_reason": "tool_calls",
			},
		},
	}
}

func chatCompletionID(responseID string) string {
	value := strings.TrimSpace(responseID)
	if value == "" {
		return "chatcmpl-openapi"
	}
	if strings.HasPrefix(value, "chatcmpl") {
		return value
	}
	return "chatcmpl-" + value
}

func rawResultFromLLMOutput(output *llm.RawChatCompletionOutput) RawChatCompletionResult {
	if output == nil {
		return RawChatCompletionResult{}
	}
	result := RawChatCompletionResult{Body: output.Body}
	if output.Result != nil {
		result.Usage = output.Result.Usage
		result.ResponseID = output.Result.ResponseID
		result.ToolCalls = normalizeOpenAIOutputToolCalls(output.Result.ToolCalls)
		if output.Result.Reasoning != nil {
			result.ReasoningText = firstNonEmpty(
				output.Result.Reasoning.Summary,
				output.Result.Reasoning.Text,
			)
		}
	}
	return result
}

func normalizeOpenAIOutputToolCalls(toolCalls []llm.ToolCall) []llm.ToolCall {
	if len(toolCalls) == 0 {
		return nil
	}
	result := make([]llm.ToolCall, 0, len(toolCalls))
	for index, item := range toolCalls {
		name := strings.TrimSpace(item.ToolName)
		if name == "" {
			continue
		}
		item.ToolName = name
		if strings.TrimSpace(item.ToolType) == "" {
			item.ToolType = "function"
		}
		if strings.TrimSpace(item.ToolCallID) == "" {
			item.ToolCallID = fmt.Sprintf("call_%d", index)
		}
		if strings.TrimSpace(item.ArgumentsJSON) == "" {
			item.ArgumentsJSON = "{}"
		}
		result = append(result, item)
	}
	return result
}

func chatToolCallsPayload(toolCalls []llm.ToolCall) []interface{} {
	items := make([]interface{}, 0, len(toolCalls))
	for _, item := range toolCalls {
		function := map[string]interface{}{
			"name":      strings.TrimSpace(item.ToolName),
			"arguments": firstNonEmpty(strings.TrimSpace(item.ArgumentsJSON), "{}"),
		}
		payload := map[string]interface{}{
			"id":   openAPIToolCallID(item),
			"type": "function",
		}
		if signature := strings.TrimSpace(item.ThoughtSignature); signature != "" {
			payload["thought_signature"] = signature
			payload["thoughtSignature"] = signature
			function["thought_signature"] = signature
			function["thoughtSignature"] = signature
		}
		payload["function"] = function
		items = append(items, payload)
	}
	return items
}

const openAPIToolCallThoughtSignatureMarker = "__gts_"

func openAPIToolCallID(item llm.ToolCall) string {
	id := strings.TrimSpace(item.ToolCallID)
	signature := strings.TrimSpace(item.ThoughtSignature)
	if id == "" || signature == "" || openAPIToolCallThoughtSignatureFromID(id) != "" {
		return id
	}
	encoded := base64.RawURLEncoding.EncodeToString([]byte(signature))
	return id + openAPIToolCallThoughtSignatureMarker + encoded
}

func openAPIToolCallThoughtSignatureFromID(id string) string {
	value := strings.TrimSpace(id)
	index := strings.LastIndex(value, openAPIToolCallThoughtSignatureMarker)
	if index < 0 {
		return ""
	}
	encoded := value[index+len(openAPIToolCallThoughtSignatureMarker):]
	decoded, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(decoded))
}

func legacyFunctionCallPayload(item llm.ToolCall) map[string]interface{} {
	return map[string]interface{}{
		"name":      strings.TrimSpace(item.ToolName),
		"arguments": firstNonEmpty(strings.TrimSpace(item.ArgumentsJSON), "{}"),
	}
}

func requestUsesLegacyFunctionCalling(body map[string]interface{}) bool {
	if body == nil {
		return false
	}
	if _, ok := body["functions"]; ok {
		return true
	}
	if _, ok := body["function_call"]; ok {
		return true
	}
	return false
}

func isClientFunctionToolCall(item llm.ToolCall) bool {
	toolType := strings.TrimSpace(item.ToolType)
	return toolType == "" || toolType == "function"
}

type streamToolCallEmitter struct {
	model string
	slots map[string]streamToolCallSlot
	next  int
}

type streamToolCallSlot struct {
	index     int
	toolID    string
	arguments string
	emitted   bool
}

func newStreamToolCallEmitter(model string) *streamToolCallEmitter {
	return &streamToolCallEmitter{
		model: strings.TrimSpace(model),
		slots: make(map[string]streamToolCallSlot),
	}
}

func (e *streamToolCallEmitter) chunks(responseID string, item llm.ToolCall) []map[string]interface{} {
	if e == nil || !isClientFunctionToolCall(item) || strings.TrimSpace(item.ToolName) == "" {
		return nil
	}
	item = normalizeOpenAIOutputToolCalls([]llm.ToolCall{item})[0]
	key := strings.TrimSpace(item.ToolCallID)
	if key == "" {
		key = strings.TrimSpace(item.ToolName)
	}
	slot, ok := e.slots[key]
	if !ok {
		slot = streamToolCallSlot{
			index:  e.next,
			toolID: strings.TrimSpace(item.ToolCallID),
		}
		e.next++
	}
	args := strings.TrimSpace(item.ArgumentsJSON)
	delta := args
	if slot.arguments != "" && strings.HasPrefix(args, slot.arguments) {
		delta = strings.TrimPrefix(args, slot.arguments)
	}
	includeIdentity := !slot.emitted
	slot.arguments = args
	slot.emitted = true
	e.slots[key] = slot
	if delta == "" && !includeIdentity {
		return nil
	}
	return []map[string]interface{}{
		chatStreamToolCallChunk(e.model, responseID, slot.index, item, delta, includeIdentity),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
