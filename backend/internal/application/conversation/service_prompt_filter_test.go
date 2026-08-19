package conversation

import (
	"context"
	"errors"
	"testing"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"go.uber.org/zap"
)

func TestValidatePromptSensitiveWords(t *testing.T) {
	service := NewService(
		config.Config{PromptSensitiveWords: "Block\n敏感词"},
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		zap.NewNop(),
	)

	if err := service.ValidatePromptSensitiveWords("ordinary message"); err != nil {
		t.Fatalf("expected ordinary prompt to pass, got %v", err)
	}
	if err := service.ValidatePromptSensitiveWords("please ＢＬＯＣＫ this"); !errors.Is(err, ErrSensitivePromptBlocked) {
		t.Fatalf("expected sensitive prompt to be blocked, got %v", err)
	}
	if _, err := service.SendMessage(context.Background(), SendMessageInput{Content: "包含敏感词"}); !errors.Is(err, ErrSensitivePromptBlocked) {
		t.Fatalf("expected SendMessage to block before repository work, got %v", err)
	}
}

func TestValidatePromptSensitiveWordsEmptyDictionary(t *testing.T) {
	service := NewService(config.Config{}, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, zap.NewNop())
	if err := service.ValidatePromptSensitiveWords("anything"); err != nil {
		t.Fatalf("expected empty dictionary to pass, got %v", err)
	}
}
