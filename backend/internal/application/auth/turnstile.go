package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/config"
	"go.uber.org/zap"
)

const (
	turnstileSiteverifyEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	turnstileTokenMaxLength     = 2048
)

type turnstileSiteverifyResponse struct {
	Success    bool     `json:"success"`
	ErrorCodes []string `json:"error-codes"`
}

func (s *Service) verifyRegistrationTurnstile(ctx context.Context, cfg config.Config, tokenValue string, remoteIP string) error {
	if !cfg.TurnstileRegistrationEnabled {
		return nil
	}
	siteKey := strings.TrimSpace(cfg.TurnstileSiteKey)
	secretKey := strings.TrimSpace(cfg.TurnstileSecretKey)
	if siteKey == "" || secretKey == "" {
		return fmt.Errorf("turnstile is not configured")
	}

	token := strings.TrimSpace(tokenValue)
	if token == "" {
		return fmt.Errorf("turnstile verification is required")
	}
	if len(token) > turnstileTokenMaxLength {
		return fmt.Errorf("turnstile token is too long")
	}

	form := url.Values{}
	form.Set("secret", secretKey)
	form.Set("response", token)
	if ip := strings.TrimSpace(remoteIP); ip != "" {
		form.Set("remoteip", ip)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, turnstileSiteverifyEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")

	client := s.turnstileHTTPClient
	if client == nil {
		s.warn("turnstile_siteverify_client_missing")
		return fmt.Errorf("turnstile verification failed")
	}
	response, err := client.Do(request)
	if err != nil {
		s.warn("turnstile_siteverify_request_failed", zap.Error(err))
		return fmt.Errorf("turnstile verification failed")
	}
	defer response.Body.Close()

	var result turnstileSiteverifyResponse
	if err = json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&result); err != nil {
		s.warn("turnstile_siteverify_decode_failed", zap.Int("status", response.StatusCode), zap.Error(err))
		return fmt.Errorf("turnstile verification failed")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !result.Success {
		s.warn("turnstile_siteverify_rejected", zap.Int("status", response.StatusCode), zap.Strings("error_codes", result.ErrorCodes))
		return fmt.Errorf("turnstile verification failed")
	}
	return nil
}
