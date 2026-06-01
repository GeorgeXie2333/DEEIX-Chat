package openapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/openapi"
	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

// Handler 处理开放 API 相关 HTTP 请求。
type Handler struct {
	service *appopenapi.Service
}

// NewHandler 创建处理器。
func NewHandler(service *appopenapi.Service) *Handler {
	return &Handler{service: service}
}

// GetAPIKey 查询当前用户 API Key 元数据。
func (h *Handler) GetAPIKey(c *gin.Context) {
	view, err := h.service.GetAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "get open api key failed")
		return
	}
	response.Success(c, view)
}

// CreateAPIKey 创建当前用户 API Key。
func (h *Handler) CreateAPIKey(c *gin.Context) {
	view, err := h.service.CreateAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if errors.Is(err, appopenapi.ErrAPIKeyAlreadyExists) {
		response.Error(c, http.StatusConflict, "api key already exists")
		return
	}
	if errors.Is(err, appopenapi.ErrTwoFactorRequired) {
		response.Error(c, http.StatusForbidden, "two factor authentication is required")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "create open api key failed")
		return
	}
	response.Success(c, view)
}

// RegenerateAPIKey 重新生成当前用户 API Key。
func (h *Handler) RegenerateAPIKey(c *gin.Context) {
	view, err := h.service.RegenerateAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if errors.Is(err, appopenapi.ErrTwoFactorRequired) {
		response.Error(c, http.StatusForbidden, "two factor authentication is required")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "regenerate open api key failed")
		return
	}
	response.Success(c, view)
}

// DeleteAPIKey 停用当前用户 API Key。
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	view, err := h.service.RevokeAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "delete open api key failed")
		return
	}
	response.Success(c, view)
}

// ListModels 处理 GET /v1/models。
func (h *Handler) ListModels(c *gin.Context) {
	key, ok := h.authenticateCompatible(c)
	if !ok {
		return
	}
	if err := h.service.EnforceRateLimit(c.Request.Context(), key); err != nil {
		writeOpenAIError(c, err)
		return
	}
	models, err := h.service.ListModels(c.Request.Context())
	if err != nil {
		writeOpenAIError(c, err)
		return
	}
	c.JSON(http.StatusOK, models)
}

// ChatCompletions 处理 POST /v1/chat/completions。
func (h *Handler) ChatCompletions(c *gin.Context) {
	key, ok := h.authenticateCompatible(c)
	if !ok {
		return
	}
	if err := h.service.EnforceRateLimit(c.Request.Context(), key); err != nil {
		writeOpenAIError(c, err)
		return
	}
	body, err := decodeRequestBody(c)
	if err != nil {
		writeOpenAIError(c, err)
		return
	}
	stream, err := requestStreamFlag(body)
	if err != nil {
		writeOpenAIError(c, err)
		return
	}
	prepared, err := h.service.PrepareChatCompletion(
		c.Request.Context(),
		key,
		body,
		middleware.MustRequestID(c),
		stream,
	)
	if err != nil {
		writeOpenAIError(c, err)
		return
	}
	if !stream {
		result, err := h.service.CompleteChatCompletion(c.Request.Context(), prepared)
		if err != nil {
			writeOpenAIError(c, err)
			return
		}
		c.JSON(http.StatusOK, result)
		return
	}

	prepareSSE(c)
	err = h.service.StreamChatCompletion(c.Request.Context(), prepared, func(event map[string]interface{}) error {
		return writeSSEData(c, event)
	})
	if err != nil {
		_ = writeSSEData(c, openAIErrorBody(err))
	}
	writeSSEDone(c)
}

func (h *Handler) authenticateCompatible(c *gin.Context) (*domainopenapi.UserAPIKey, bool) {
	token := bearerToken(c.GetHeader("Authorization"))
	key, err := h.service.AuthenticateAPIKey(c.Request.Context(), token)
	if err != nil {
		writeOpenAIError(c, err)
		return nil, false
	}
	return key, true
}

func bearerToken(header string) string {
	value := strings.TrimSpace(header)
	if value == "" {
		return ""
	}
	prefix := "Bearer "
	if len(value) >= len(prefix) && strings.EqualFold(value[:len(prefix)], prefix) {
		return strings.TrimSpace(value[len(prefix):])
	}
	return ""
}

func decodeRequestBody(c *gin.Context) (map[string]interface{}, error) {
	if c.Request.Body == nil {
		return nil, appopenapi.ErrInvalidRequest
	}
	decoder := json.NewDecoder(c.Request.Body)
	decoder.UseNumber()
	var body map[string]interface{}
	if err := decoder.Decode(&body); err != nil {
		return nil, appopenapi.ErrInvalidRequest
	}
	if body == nil {
		return nil, appopenapi.ErrInvalidRequest
	}
	return body, nil
}

func requestStreamFlag(body map[string]interface{}) (bool, error) {
	raw, ok := body["stream"]
	if !ok || raw == nil {
		return false, nil
	}
	value, ok := raw.(bool)
	if !ok {
		return false, appopenapi.ErrInvalidRequest
	}
	return value, nil
}

type openAIErrorResponse struct {
	Error openAIError `json:"error"`
}

type openAIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

func writeOpenAIError(c *gin.Context, err error) {
	status := openAIStatus(err)
	c.JSON(status, openAIErrorBody(err))
}

func openAIErrorBody(err error) openAIErrorResponse {
	status := openAIStatus(err)
	errType := "api_error"
	code := "internal_error"
	message := "internal server error"
	switch {
	case errors.Is(err, appopenapi.ErrInvalidAPIKey):
		errType = "invalid_request_error"
		code = "invalid_api_key"
		message = "invalid api key"
	case errors.Is(err, appopenapi.ErrRateLimited):
		errType = "rate_limit_error"
		code = "rate_limit_exceeded"
		message = "rate limit exceeded"
	case errors.Is(err, appopenapi.ErrInvalidRequest):
		errType = "invalid_request_error"
		code = "invalid_request"
		message = "invalid request"
	case errors.Is(err, appopenapi.ErrModelNotAllowed):
		errType = "invalid_request_error"
		code = "model_not_found"
		message = "model is not available"
	case errors.Is(err, appbilling.ErrUsageBalanceInsufficient):
		errType = "insufficient_quota"
		code = "insufficient_quota"
		message = "usage balance is insufficient"
	case errors.Is(err, appbilling.ErrModelPricingRequired):
		errType = "invalid_request_error"
		code = "model_pricing_required"
		message = "model pricing is required"
	case errors.Is(err, appbilling.ErrPeriodCreditExceeded):
		errType = "insufficient_quota"
		code = "period_credit_exceeded"
		message = "period usage credit exceeded"
	case errors.Is(err, appbilling.ErrFreeModelRateLimitExceeded), errors.Is(err, appbilling.ErrFreeModelDailyLimitExceeded):
		errType = "rate_limit_error"
		code = "rate_limit_exceeded"
		message = "rate limit exceeded"
	default:
		var upstream *llm.UpstreamError
		if errors.As(err, &upstream) {
			errType = "api_error"
			code = "upstream_error"
			message = strings.TrimSpace(upstream.Message)
			if message == "" {
				message = "upstream request failed"
			}
		} else if status < http.StatusInternalServerError && strings.TrimSpace(err.Error()) != "" {
			message = strings.TrimSpace(err.Error())
		}
	}
	return openAIErrorResponse{Error: openAIError{Message: message, Type: errType, Code: code}}
}

func openAIStatus(err error) int {
	switch {
	case errors.Is(err, appopenapi.ErrInvalidAPIKey):
		return http.StatusUnauthorized
	case errors.Is(err, appopenapi.ErrRateLimited),
		errors.Is(err, appbilling.ErrFreeModelRateLimitExceeded),
		errors.Is(err, appbilling.ErrFreeModelDailyLimitExceeded):
		return http.StatusTooManyRequests
	case errors.Is(err, appopenapi.ErrInvalidRequest):
		return http.StatusBadRequest
	case errors.Is(err, appopenapi.ErrModelNotAllowed):
		return http.StatusNotFound
	case errors.Is(err, appbilling.ErrUsageBalanceInsufficient),
		errors.Is(err, appbilling.ErrPeriodCreditExceeded):
		return http.StatusPaymentRequired
	case errors.Is(err, appbilling.ErrModelPricingRequired):
		return http.StatusForbidden
	default:
		var upstream *llm.UpstreamError
		if errors.As(err, &upstream) {
			return http.StatusBadGateway
		}
		return http.StatusInternalServerError
	}
}

func prepareSSE(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
}

func writeSSEData(c *gin.Context, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte("data: ")); err != nil {
		return err
	}
	if _, err := c.Writer.Write(payload); err != nil {
		return err
	}
	if _, err := c.Writer.Write([]byte("\n\n")); err != nil {
		return err
	}
	flush(c)
	return nil
}

func writeSSEDone(c *gin.Context) {
	_, _ = c.Writer.Write([]byte("data: [DONE]\n\n"))
	flush(c)
}

func flush(c *gin.Context) {
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
