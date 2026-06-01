package openapi

import (
	"context"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

// LLMRawChatProvider adapts the shared LLM client to the open API service.
type LLMRawChatProvider struct {
	client        *llm.Client
	imageResolver chatImageResolver
}

// NewLLMRawChatProvider 创建原始 Chat Completions 调用器。
func NewLLMRawChatProvider(client *llm.Client) *LLMRawChatProvider {
	return &LLMRawChatProvider{client: client}
}

// NewLLMRawChatProviderWithImageResolver 创建支持远程 image_url 下载的开放 API 调用器。
func NewLLMRawChatProviderWithImageResolver(client *llm.Client, resolver chatImageResolver) *LLMRawChatProvider {
	return &LLMRawChatProvider{client: client, imageResolver: resolver}
}

// CompleteChat 透传非流式 Chat Completions 请求。
func (p *LLMRawChatProvider) CompleteChat(ctx context.Context, route llm.RouteConfig, body map[string]interface{}) (RawChatCompletionResult, error) {
	if p == nil || p.client == nil {
		return RawChatCompletionResult{}, ErrModelNotAllowed
	}
	if llm.NormalizeAdapter(route.Protocol) != llm.AdapterOpenAIChatCompletions {
		input, err := buildGenerateInputFromChatCompletion(ctx, body, p.imageResolver)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		output, err := p.client.Generate(ctx, route, input)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		return chatResultFromGenerateOutput(output, route.UpstreamModel), nil
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
	if llm.NormalizeAdapter(route.Protocol) != llm.AdapterOpenAIChatCompletions {
		input, err := buildGenerateInputFromChatCompletion(ctx, body, p.imageResolver)
		if err != nil {
			return RawChatCompletionResult{}, err
		}
		output, err := p.client.GenerateStream(ctx, route, input, func(event llm.GenerateStreamEvent) error {
			if onEvent == nil {
				return nil
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
		return chatResultFromGenerateOutput(output, route.UpstreamModel), nil
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

func chatResultFromGenerateOutput(output *llm.GenerateOutput, model string) RawChatCompletionResult {
	if output == nil {
		return RawChatCompletionResult{Body: chatCompletionBody("", model, "", llm.Usage{})}
	}
	body := chatCompletionBody(output.ResponseID, model, output.Text, output.Usage)
	result := RawChatCompletionResult{
		Body:       body,
		Usage:      output.Usage,
		ResponseID: output.ResponseID,
	}
	if output.Reasoning != nil {
		result.ReasoningText = firstNonEmpty(output.Reasoning.Summary, output.Reasoning.Text)
	}
	return result
}

func chatCompletionBody(responseID string, model string, content string, usage llm.Usage) map[string]interface{} {
	body := map[string]interface{}{
		"id":      chatCompletionID(responseID),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   strings.TrimSpace(model),
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": content,
				},
				"finish_reason": "stop",
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
		if output.Result.Reasoning != nil {
			result.ReasoningText = firstNonEmpty(
				output.Result.Reasoning.Summary,
				output.Result.Reasoning.Text,
			)
		}
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
