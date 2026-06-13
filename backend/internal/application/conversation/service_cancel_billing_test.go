package conversation

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	model "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/conversation"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
)

func TestNormalizeCanceledMessageGenerationError(t *testing.T) {
	err := normalizeCanceledMessageGenerationError(context.Canceled, true)

	if !errors.Is(err, ErrMessageGenerationCanceled) {
		t.Fatalf("error = %v, want message generation canceled", err)
	}
}

func TestShouldPersistCanceledGenerationAfterUpstreamDispatch(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:        &model.Message{},
		AssistantMessage:   &model.Message{},
		Error:              ErrMessageGenerationCanceled,
		UpstreamDispatched: true,
	}

	if !shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("expected dispatched canceled generation to be retained for billing")
	}
}

func TestShouldNotPersistCanceledGenerationBeforeUpstreamDispatch(t *testing.T) {
	input := persistInterruptedMessageGenerationInput{
		UserMessage:        &model.Message{},
		AssistantMessage:   &model.Message{},
		Error:              ErrMessageGenerationCanceled,
		UpstreamDispatched: false,
	}

	if shouldPersistInterruptedMessageGeneration(input) {
		t.Fatal("expected pre-dispatch cancellation to remain non-billable")
	}
}

func TestApplyInterruptedMessageGenerationStateKeepsCanceledStatus(t *testing.T) {
	userMessage := &model.Message{}
	assistantMessage := &model.Message{}
	input := persistInterruptedMessageGenerationInput{
		UserMessage:        userMessage,
		AssistantMessage:   assistantMessage,
		Error:              ErrMessageGenerationCanceled,
		UpstreamDispatched: true,
	}

	applyInterruptedMessageGenerationState(input, interruptedMessageGenerationMetrics{
		InputTokens:  12,
		OutputTokens: 3,
	})

	if assistantMessage.Status != "canceled" {
		t.Fatalf("assistant status = %q, want canceled", assistantMessage.Status)
	}
}

func TestBuildCanceledMediaGenerationResultRequiresDispatch(t *testing.T) {
	result := buildCanceledMediaGenerationResult(canceledMediaGenerationInput{
		Input: MediaImageInput{
			TaskType: MediaImageTaskGeneration,
			Prompt:   "draw a poster",
		},
		UserMessage:        &model.Message{},
		AssistantMessage:   &model.Message{},
		UpstreamDispatched: false,
	})

	if result != nil {
		t.Fatal("expected pre-dispatch media cancellation to remain non-billable")
	}
}

func TestBuildCanceledMediaGenerationResultKeepsVideoDuration(t *testing.T) {
	userMessage := &model.Message{}
	assistantMessage := &model.Message{}
	result := buildCanceledMediaGenerationResult(canceledMediaGenerationInput{
		Input: MediaImageInput{
			TaskType: MediaVideoTaskGeneration,
			Prompt:   "animate the skyline",
		},
		UserMessage:      userMessage,
		AssistantMessage: assistantMessage,
		Route: &channel.ResolvedRoute{
			PlatformModelName: "sora-2",
			UpstreamModel:     "sora-2",
		},
		EffectiveOptions:   map[string]interface{}{"seconds": "8"},
		UpstreamDispatched: true,
		StartedAt:          time.Now(),
	})

	if result == nil || !result.Billable {
		t.Fatal("expected dispatched media cancellation to be billable")
	}
	if result.DurationSeconds != 8 {
		t.Fatalf("duration seconds = %d, want 8", result.DurationSeconds)
	}
	if result.AssistantMessage.Status != "canceled" {
		t.Fatalf("assistant status = %q, want canceled", result.AssistantMessage.Status)
	}
	if result.UserMessage.InputTokens <= 0 {
		t.Fatal("expected canceled media request to retain prompt token usage")
	}
}

func TestResolveInterruptedMetricsMarksCalculatedTokenFallback(t *testing.T) {
	assistantText := "partial answer"
	metrics := resolveInterruptedMessageGenerationMetrics(persistInterruptedMessageGenerationInput{
		AssistantText:        assistantText,
		EstimatedInputTokens: 19,
		Error:                ErrMessageGenerationCanceled,
		StartedAt:            time.Now(),
	})

	if metrics.InputTokenSource != "calculated" {
		t.Fatalf("input token source = %q, want calculated", metrics.InputTokenSource)
	}
	if metrics.OutputTokenSource != "calculated" {
		t.Fatalf("output token source = %q, want calculated", metrics.OutputTokenSource)
	}
	if metrics.OutputTokens != estimateTokens(assistantText) {
		t.Fatalf("output tokens = %d, want local estimate %d", metrics.OutputTokens, estimateTokens(assistantText))
	}
	if metrics.OutputTokens == 4000 {
		t.Fatal("output tokens must not use the old fixed 4000-token charge")
	}
}

func TestCanceledGenerationUsageAccumulatorMergesUpstreamAndCalculatedUsage(t *testing.T) {
	var accumulator canceledGenerationUsageAccumulator
	accumulator.addAttempt(llm.Usage{
		InputTokens:  10,
		OutputTokens: 2,
	}, 99, 99)
	accumulator.addAttempt(llm.Usage{}, 7, 3)

	if accumulator.Usage.InputTokens != 17 {
		t.Fatalf("input tokens = %d, want 17", accumulator.Usage.InputTokens)
	}
	if accumulator.Usage.OutputTokens != 5 {
		t.Fatalf("output tokens = %d, want 5", accumulator.Usage.OutputTokens)
	}
	if accumulator.InputTokenSource != "mixed" {
		t.Fatalf("input token source = %q, want mixed", accumulator.InputTokenSource)
	}
	if accumulator.OutputTokenSource != "mixed" {
		t.Fatalf("output token source = %q, want mixed", accumulator.OutputTokenSource)
	}
}
