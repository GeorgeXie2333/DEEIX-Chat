package openapi

import (
	"context"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

// LLMRawChatProvider adapts the shared LLM client to the open API service.
type LLMRawChatProvider struct {
	client *llm.Client
}

// NewLLMRawChatProvider 创建原始 Chat Completions 调用器。
func NewLLMRawChatProvider(client *llm.Client) *LLMRawChatProvider {
	return &LLMRawChatProvider{client: client}
}

// CompleteChat 透传非流式 Chat Completions 请求。
func (p *LLMRawChatProvider) CompleteChat(ctx context.Context, route llm.RouteConfig, body map[string]interface{}) (RawChatCompletionResult, error) {
	if p == nil || p.client == nil {
		return RawChatCompletionResult{}, ErrModelNotAllowed
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
