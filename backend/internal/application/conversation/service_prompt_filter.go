package conversation

import (
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/promptfilter"
	"go.uber.org/zap"
)

// ValidatePromptSensitiveWords blocks user-authored prompts that match the
// administrator-managed sensitive-word dictionary.
func (s *Service) ValidatePromptSensitiveWords(prompt string) error {
	if s == nil || s.cfg == nil {
		return nil
	}
	terms, err := promptfilter.ParseDictionary(s.cfg.Snapshot().PromptSensitiveWords)
	if err != nil {
		if s.logger != nil {
			s.logger.Warn("invalid_prompt_sensitive_words_setting", zap.Error(err))
		}
		return nil
	}
	if promptfilter.Contains(prompt, terms) {
		return ErrSensitivePromptBlocked
	}
	return nil
}
