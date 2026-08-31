package openapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/openapi"
	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/response"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/transport/http/middleware"
	"github.com/gin-gonic/gin"
)

const defaultOpenAPIRequestMaxBytes int64 = 32 * 1024 * 1024

// Handler 处理开放 API 相关 HTTP 请求。
type Handler struct {
	service *appopenapi.Service
}

type apiKeyResponse struct {
	Exists            bool       `json:"exists"`
	APIKey            string     `json:"apiKey,omitempty"`
	KeyPrefix         string     `json:"keyPrefix"`
	Status            string     `json:"status"`
	LastUsedAt        *time.Time `json:"lastUsedAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt,omitempty"`
	UpdatedAt         time.Time  `json:"updatedAt,omitempty"`
	TwoFactorRequired bool       `json:"twoFactorRequired,omitempty"`
	Exportable        bool       `json:"exportable,omitempty"`
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
	response.Success(c, toAPIKeyResponse(view))
}

// CreateAPIKey 创建当前用户 API Key。
func (h *Handler) CreateAPIKey(c *gin.Context) {
	view, err := h.service.CreateAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if errors.Is(err, appopenapi.ErrAPIKeyAlreadyExists) {
		response.Error(c, http.StatusConflict, "api key already exists")
		return
	}
	if errors.Is(err, appopenapi.ErrAPIKeyConflict) {
		response.ErrorWithCode(c, http.StatusConflict, "openapi.api_key_conflict", "api key changed concurrently; refresh and retry")
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
	response.Success(c, toAPIKeyResponse(view))
}

// RegenerateAPIKey 重新生成当前用户 API Key。
func (h *Handler) RegenerateAPIKey(c *gin.Context) {
	view, err := h.service.RegenerateAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if errors.Is(err, appopenapi.ErrAPIKeyAlreadyExists) {
		response.Error(c, http.StatusConflict, "api key already exists")
		return
	}
	if errors.Is(err, appopenapi.ErrTwoFactorRequired) {
		response.Error(c, http.StatusForbidden, "two factor authentication is required")
		return
	}
	if errors.Is(err, appopenapi.ErrAPIKeyConflict) {
		response.ErrorWithCode(c, http.StatusConflict, "openapi.api_key_conflict", "api key changed concurrently; refresh and retry")
		return
	}
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "regenerate open api key failed")
		return
	}
	response.Success(c, toAPIKeyResponse(view))
}

// DeleteAPIKey 停用当前用户 API Key。
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	view, err := h.service.RevokeAPIKey(c.Request.Context(), middleware.MustUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "delete open api key failed")
		return
	}
	response.Success(c, toAPIKeyResponse(view))
}

// ListModels 处理 GET /v1/models。
// @Summary OpenAI-compatible Models
// @Description 面向用户 API Key 的模型列表，返回 new-api 风格的 success/data/object 结构；当前开放 API 模型均支持 OpenAI Chat Completions 兼容端点。
// @Tags openapi-compatible
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} ModelListResponseDoc
// @Failure 401 {object} openAIErrorResponse
// @Failure 429 {object} openAIErrorResponse
// @Failure 500 {object} openAIErrorResponse
// @Router /v1/models [get]
func (h *Handler) ListModels(c *gin.Context) {
	key, ok := h.authenticateCompatible(c)
	if !ok {
		return
	}
	if err := h.service.EnforceRateLimit(c.Request.Context(), key); err != nil {
		writeOpenAIError(c, err)
		return
	}
	models, err := h.service.ListModels(c.Request.Context(), key.UserID)
	if err != nil {
		writeOpenAIError(c, err)
		return
	}
	c.JSON(http.StatusOK, toModelListResponse(models))
}

func toAPIKeyResponse(view *appopenapi.APIKeyView) apiKeyResponse {
	if view == nil {
		return apiKeyResponse{}
	}
	return apiKeyResponse{
		Exists:            view.Exists,
		APIKey:            view.APIKey,
		KeyPrefix:         view.KeyPrefix,
		Status:            view.Status,
		LastUsedAt:        view.LastUsedAt,
		CreatedAt:         view.CreatedAt,
		UpdatedAt:         view.UpdatedAt,
		TwoFactorRequired: view.TwoFactorRequired,
		Exportable:        view.Exportable,
	}
}

func toModelListResponse(models appopenapi.OpenAIModelList) ModelListResponseDoc {
	items := make([]ModelItemDoc, 0, len(models.Data))
	for _, model := range models.Data {
		items = append(items, ModelItemDoc{
			ID:                     model.ID,
			Object:                 model.Object,
			Created:                model.Created,
			OwnedBy:                model.OwnedBy,
			SupportedEndpointTypes: model.SupportedEndpointTypes,
		})
	}
	return ModelListResponseDoc{Success: models.Success, Data: items, Object: models.Object}
}

// ChatCompletions 处理 POST /v1/chat/completions。
// @Summary OpenAI-compatible Chat Completions
// @Description 面向用户 API Key 的 OpenAI-compatible chat completions。支持客户端函数调用协议：tools/tool_choice/tool_calls、role=tool 结果消息，以及旧版 functions/function_call；平台只做协议转换，不代 API 用户执行函数。配置为 OpenAI Responses 或 xAI Responses 的开放 API 上游会使用 Chat Completions 兼容端点，不暴露 Responses 原生工具。
// @Tags openapi-compatible
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ChatCompletionRequestDoc true "OpenAI-compatible chat completion request"
// @Success 200 {object} ChatCompletionResponseDoc
// @Failure 400 {object} openAIErrorResponse
// @Failure 401 {object} openAIErrorResponse
// @Failure 429 {object} openAIErrorResponse
// @Failure 500 {object} openAIErrorResponse
// @Router /v1/chat/completions [post]
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
	if h == nil || h.service == nil {
		writeOpenAIError(c, appopenapi.ErrInvalidAPIKey)
		return nil, false
	}
	if err := h.service.EnforcePreAuthRateLimit(c.Request.Context(), c.ClientIP()); err != nil {
		writeOpenAIError(c, err)
		return nil, false
	}
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
	return decodeRequestBodyWithLimit(c, defaultOpenAPIRequestMaxBytes)
}

func decodeRequestBodyWithLimit(c *gin.Context, maxBytes int64) (map[string]interface{}, error) {
	if c.Request.Body == nil {
		return nil, appopenapi.ErrInvalidRequest
	}
	if maxBytes <= 0 {
		maxBytes = defaultOpenAPIRequestMaxBytes
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
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
