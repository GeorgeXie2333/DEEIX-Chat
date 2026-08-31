package openapi

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	appbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/billing"
	appchannel "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/application/channel"
	domainbilling "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/billing"
	domainopenapi "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/openapi"
	domainuser "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/user"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/pkg/secretbox"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/ports/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/repository"
)

const (
	// APIKeyPlaintextPrefix 是用户看到的开放 API Key 明文前缀。
	APIKeyPlaintextPrefix = "dxsk_"

	openAPISettingsNamespace                 = "openapi"
	openAPISettingModelAllowlist             = "model_allowlist"
	openAPISettingRateLimitRPM               = "rate_limit_rpm"
	openAPIUsageAuthorizationRenewalInterval = 30 * time.Minute
	openAPIPreAuthRateLimitRPM               = 300
)

var (
	// ErrInvalidAPIKey 表示 API Key 缺失、无效或已停用。
	ErrInvalidAPIKey = errors.New("invalid api key")
	// ErrAPIKeyAlreadyExists 表示用户已有可用 API Key。
	ErrAPIKeyAlreadyExists = errors.New("api key already exists")
	// ErrAPIKeyConflict 表示并发操作已更改用户 API Key，调用方应刷新后重试。
	ErrAPIKeyConflict = errors.New("api key changed concurrently")
	// ErrTwoFactorRequired 表示用户必须先开启 2FA 才能创建、重新生成或导出 API Key。
	ErrTwoFactorRequired = errors.New("two factor authentication is required")
	// ErrModelNotAllowed 表示模型未在开放 API 白名单中。
	ErrModelNotAllowed = errors.New("model is not available for open api")
	// ErrInvalidRequest 表示请求体不符合 Chat Completions 基本要求。
	ErrInvalidRequest = errors.New("invalid openapi request")
	// ErrRateLimited 表示开放 API RPM 限流命中。
	ErrRateLimited = errors.New("open api rate limit exceeded")
)

// Dependencies 汇总开放 API 服务依赖。
type Dependencies struct {
	KeyRepo           repository.OpenAPIKeyRepository
	Settings          settingsReader
	Channel           channelService
	Billing           billingService
	ChatProvider      rawChatProvider
	RateLimiter       rateLimiter
	TwoFactor         twoFactorChecker
	UserStatus        userStatusChecker
	ModelOptionFilter modelOptionFilter
	DataEncryptionKey string
	Now               func() time.Time
}

type settingsReader interface {
	RuntimeValuesByNamespace(ctx context.Context, namespace string) (map[string]string, error)
}

type channelService interface {
	ListActiveModels(ctx context.Context, userID uint) ([]appchannel.ModelView, error)
	ListActiveModelRoutes(ctx context.Context, platformModelName string) ([]appchannel.ActiveModelRouteView, error)
	ResolveRoute(ctx context.Context, input appchannel.ResolveRouteInput) (*appchannel.ResolvedRoute, error)
	MarkRouteSuccess(ctx context.Context, route *appchannel.ResolvedRoute)
	MarkRouteFailure(ctx context.Context, route *appchannel.ResolvedRoute, cause error)
}

type billingService interface {
	AuthorizeUsage(ctx context.Context, userID uint, platformModelName string, refNo string) (*domainbilling.UsageAuthorization, error)
	ReleaseUsageAuthorization(ctx context.Context, authorization *domainbilling.UsageAuthorization) error
	RenewUsageAuthorization(ctx context.Context, authorization *domainbilling.UsageAuthorization) error
	MarkUsageAuthorizationForReconciliation(ctx context.Context, authorization *domainbilling.UsageAuthorization, failureCode string) error
	BuildUsageLedger(ctx context.Context, input appbilling.UsagePricingInput) (*domainbilling.UsageLedger, error)
	RecordUsageWithAuthorization(ctx context.Context, usage *domainbilling.UsageLedger, authorization *domainbilling.UsageAuthorization) error
}

type rawChatProvider interface {
	CompleteChat(ctx context.Context, route llm.RouteConfig, body map[string]interface{}) (RawChatCompletionResult, error)
	StreamChat(ctx context.Context, route llm.RouteConfig, body map[string]interface{}, onEvent func(RawChatStreamEvent) error) (RawChatCompletionResult, error)
}

type rateLimiter interface {
	AllowSlidingWindow(ctx context.Context, key string, limit int, window time.Duration, ttl time.Duration) (bool, error)
}

type twoFactorChecker interface {
	IsTwoFactorEnabled(ctx context.Context, userID uint) (bool, error)
}

type userStatusChecker interface {
	GetByID(ctx context.Context, userID uint) (*domainuser.User, error)
}

type modelOptionFilter interface {
	FilterModelOptionsForRoute(options map[string]interface{}, protocol string, modelCapabilitiesJSON string) map[string]interface{}
}

// Service 封装 API Key、白名单、限流、兼容推理和计费流程。
type Service struct {
	keyRepo           repository.OpenAPIKeyRepository
	settings          settingsReader
	channel           channelService
	billing           billingService
	chatProvider      rawChatProvider
	rateLimiter       rateLimiter
	twoFactor         twoFactorChecker
	userStatus        userStatusChecker
	modelOptionFilter modelOptionFilter
	dataEncryptionKey string
	now               func() time.Time
}

// NewService 创建开放 API 服务。
func NewService(deps Dependencies) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	return &Service{
		keyRepo:           deps.KeyRepo,
		settings:          deps.Settings,
		channel:           deps.Channel,
		billing:           deps.Billing,
		chatProvider:      deps.ChatProvider,
		rateLimiter:       deps.RateLimiter,
		twoFactor:         deps.TwoFactor,
		userStatus:        deps.UserStatus,
		modelOptionFilter: deps.ModelOptionFilter,
		dataEncryptionKey: strings.TrimSpace(deps.DataEncryptionKey),
		now:               now,
	}
}

// SetChatProvider 注入原始 Chat Completions 调用器。
func (s *Service) SetChatProvider(provider rawChatProvider) {
	s.chatProvider = provider
}

// APIKeyView 是前端展示用的 API Key 元数据。
type APIKeyView struct {
	Exists            bool
	APIKey            string
	KeyPrefix         string
	Status            string
	LastUsedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	TwoFactorRequired bool
	Exportable        bool
}

const newAPIModelCreatedUnix int64 = 1626777600

// OpenAIModelList 是 /v1/models 的兼容响应。
type OpenAIModelList struct {
	Success bool
	Data    []OpenAIModel
	Object  string
}

// OpenAIModel 是 /v1/models 的模型项。
type OpenAIModel struct {
	ID                     string
	Object                 string
	Created                int64
	OwnedBy                string
	SupportedEndpointTypes []string
}

// RawChatCompletionResult 表示原始 Chat Completions 调用结果。
type RawChatCompletionResult struct {
	Body                map[string]interface{}
	Usage               llm.Usage
	ReasoningText       string
	ResponseID          string
	ToolCalls           []llm.ToolCall
	ServerSideToolUsage map[string]int64
}

// RawChatStreamEvent 表示原始 Chat Completions 流式片段。
type RawChatStreamEvent struct {
	Body      map[string]interface{}
	Usage     llm.Usage
	Reasoning *llm.ReasoningDelta
}

// PreparedChatCompletion 记录一次通过鉴权、白名单、路由和预扣校验的调用。
type PreparedChatCompletion struct {
	key               *domainopenapi.UserAPIKey
	request           map[string]interface{}
	requestID         string
	stream            bool
	route             *appchannel.ResolvedRoute
	routeConfig       llm.RouteConfig
	authorization     *domainbilling.UsageAuthorization
	startedAt         time.Time
	platformModelName string
	publicModelID     string
}

// GetAPIKey 返回用户 API Key 元数据；仅在用户已开启 2FA 时返回完整明文。
func (s *Service) GetAPIKey(ctx context.Context, userID uint) (*APIKeyView, error) {
	if s == nil || s.keyRepo == nil || userID == 0 {
		return &APIKeyView{Exists: false}, nil
	}
	twoFactorEnabled, err := s.isTwoFactorEnabled(ctx, userID)
	if err != nil {
		return nil, err
	}
	item, err := s.keyRepo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return &APIKeyView{Exists: false, TwoFactorRequired: !twoFactorEnabled}, nil
	}
	if err != nil {
		return nil, err
	}
	if item.Status != domainopenapi.APIKeyStatusActive || !twoFactorEnabled {
		return toAPIKeyView(item, "", !twoFactorEnabled, false), nil
	}
	plaintext, err := s.decryptAPIKeyPlaintext(item.KeyPlaintextEncrypted)
	if err != nil {
		return nil, err
	}
	return toAPIKeyView(item, plaintext, false, plaintext != ""), nil
}

// CreateAPIKey 创建用户唯一 API Key。
func (s *Service) CreateAPIKey(ctx context.Context, userID uint) (*APIKeyView, error) {
	if s == nil || s.keyRepo == nil || userID == 0 {
		return nil, ErrInvalidRequest
	}
	if err := s.requireTwoFactor(ctx, userID); err != nil {
		return nil, err
	}
	current, err := s.keyRepo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return s.createAPIKey(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrAPIKeyConflict
	}
	if current.Status == domainopenapi.APIKeyStatusActive {
		return nil, ErrAPIKeyAlreadyExists
	}
	return s.replaceAPIKey(ctx, userID, current)
}

// RegenerateAPIKey 重新生成用户唯一 API Key。
func (s *Service) RegenerateAPIKey(ctx context.Context, userID uint) (*APIKeyView, error) {
	if s == nil || s.keyRepo == nil || userID == 0 {
		return nil, ErrInvalidRequest
	}
	if err := s.requireTwoFactor(ctx, userID); err != nil {
		return nil, err
	}
	current, err := s.keyRepo.GetByUserID(ctx, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return s.createAPIKey(ctx, userID)
	}
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, ErrAPIKeyConflict
	}
	return s.replaceAPIKey(ctx, userID, current)
}

// RevokeAPIKey 停用用户 API Key。
func (s *Service) RevokeAPIKey(ctx context.Context, userID uint) (*APIKeyView, error) {
	if s == nil || s.keyRepo == nil || userID == 0 {
		return nil, ErrInvalidRequest
	}
	item, err := s.keyRepo.RevokeForUser(ctx, userID, s.now())
	if errors.Is(err, repository.ErrNotFound) {
		return &APIKeyView{Exists: false}, nil
	}
	if err != nil {
		return nil, err
	}
	return toAPIKeyView(item, "", false, false), nil
}

// AuthenticateAPIKey 校验开放 API Key，并更新最后使用时间。
func (s *Service) AuthenticateAPIKey(ctx context.Context, plaintext string) (*domainopenapi.UserAPIKey, error) {
	if s == nil || s.keyRepo == nil {
		return nil, ErrInvalidAPIKey
	}
	key := strings.TrimSpace(plaintext)
	if key == "" {
		return nil, ErrInvalidAPIKey
	}
	item, err := s.keyRepo.GetActiveByHash(ctx, s.hashAPIKey(key))
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, err
	}
	if item.Status != domainopenapi.APIKeyStatusActive {
		return nil, ErrInvalidAPIKey
	}
	if s.userStatus == nil {
		return nil, ErrInvalidAPIKey
	}
	userItem, err := s.userStatus.GetByID(ctx, item.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, ErrInvalidAPIKey
	}
	if err != nil {
		return nil, err
	}
	if userItem == nil || userItem.Status != domainuser.StatusActive {
		return nil, ErrInvalidAPIKey
	}
	now := s.now()
	_ = s.keyRepo.TouchLastUsedAt(ctx, item.ID, now)
	item.LastUsedAt = &now
	return item, nil
}

// EnforcePreAuthRateLimit protects API-key lookup itself from invalid-token
// floods. It intentionally runs before AuthenticateAPIKey and is independent
// of the per-key RPM setting.
func (s *Service) EnforcePreAuthRateLimit(ctx context.Context, clientIP string) error {
	if s == nil || s.rateLimiter == nil {
		return nil
	}
	ip := strings.TrimSpace(clientIP)
	if ip == "" {
		ip = "unknown"
	}
	allowed, err := s.rateLimiter.AllowSlidingWindow(
		ctx,
		"ratelimit:openapi:preauth:ip:"+ip,
		openAPIPreAuthRateLimitRPM,
		time.Minute,
		2*time.Minute,
	)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrRateLimited
	}
	return nil
}

// EnforceRateLimit 对开放 API 按 key 维度执行 RPM 限流。
func (s *Service) EnforceRateLimit(ctx context.Context, key *domainopenapi.UserAPIKey) error {
	if s == nil || s.rateLimiter == nil || key == nil || key.ID == 0 {
		return nil
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return err
	}
	if settings.RPM <= 0 {
		return nil
	}
	allowed, err := s.rateLimiter.AllowSlidingWindow(
		ctx,
		fmt.Sprintf("ratelimit:openapi:key:%d", key.ID),
		settings.RPM,
		time.Minute,
		2*time.Minute,
	)
	if err != nil || allowed {
		return err
	}
	return ErrRateLimited
}

// ListModels 返回管理员开放白名单内、当前 API Key 用户可访问的 active 文本模型。
func (s *Service) ListModels(ctx context.Context, userID uint) (OpenAIModelList, error) {
	result := OpenAIModelList{Success: true, Object: "list", Data: []OpenAIModel{}}
	if s == nil || s.channel == nil {
		return result, nil
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return result, err
	}
	allowlist := modelAllowlistSet(settings.ModelAllowlist)
	if len(allowlist) == 0 {
		return result, nil
	}
	models, err := s.channel.ListActiveModels(ctx, userID)
	if err != nil {
		return result, err
	}
	activeModels := make(map[string]appchannel.ModelView, len(models))
	for _, item := range models {
		name := strings.TrimSpace(item.PlatformModelName)
		if name == "" {
			continue
		}
		activeModels[name] = item
	}
	seen := make(map[string]struct{})
	for _, platformModelName := range settings.ModelAllowlist {
		platformModelName = strings.TrimSpace(platformModelName)
		if _, ok := allowlist[platformModelName]; !ok {
			continue
		}
		if _, ok := activeModels[platformModelName]; !ok {
			continue
		}
		routes, err := s.channel.ListActiveModelRoutes(ctx, platformModelName)
		if err != nil {
			return result, err
		}
		for _, route := range routes {
			publicID := strings.TrimSpace(route.UpstreamModelName)
			if publicID == "" {
				continue
			}
			if _, ok := seen[publicID]; ok {
				continue
			}
			if !isPublicChatProtocol(route.Protocol) {
				continue
			}
			if !appchannel.IsRouteAllowedForTask(appchannel.TaskTypeChat, route.KindsJSON, llm.NormalizeAdapter(route.Protocol)) {
				continue
			}
			seen[publicID] = struct{}{}
			result.Data = append(result.Data, OpenAIModel{
				ID:                     publicID,
				Object:                 "model",
				Created:                newAPIModelCreatedUnix,
				OwnedBy:                "deeix",
				SupportedEndpointTypes: []string{"openai"},
			})
		}
	}
	return result, nil
}

// PrepareChatCompletion 执行白名单、路由、可用性和预扣校验。
func (s *Service) PrepareChatCompletion(
	ctx context.Context,
	key *domainopenapi.UserAPIKey,
	request map[string]interface{},
	requestID string,
	stream bool,
) (*PreparedChatCompletion, error) {
	if s == nil || key == nil || key.Status != domainopenapi.APIKeyStatusActive {
		return nil, ErrInvalidAPIKey
	}
	if s.channel == nil || s.chatProvider == nil {
		return nil, ErrModelNotAllowed
	}
	publicModelID := strings.TrimSpace(stringValue(request["model"]))
	if publicModelID == "" {
		return nil, fmt.Errorf("%w: model is required", ErrInvalidRequest)
	}
	if _, ok := request["messages"]; !ok {
		return nil, fmt.Errorf("%w: messages is required", ErrInvalidRequest)
	}
	settings, err := s.loadSettings(ctx)
	if err != nil {
		return nil, err
	}
	route, err := s.resolvePublicModelRoute(ctx, settings.ModelAllowlist, publicModelID, key.UserID, requestID)
	if err != nil {
		return nil, ErrModelNotAllowed
	}
	if !isPublicChatProtocol(route.Protocol) {
		return nil, ErrModelNotAllowed
	}
	if s.modelOptionFilter == nil {
		return nil, ErrModelNotAllowed
	}

	platformModelName := strings.TrimSpace(route.PlatformModelName)
	if platformModelName == "" {
		return nil, ErrModelNotAllowed
	}
	startedAt := s.now()
	var authorization *domainbilling.UsageAuthorization
	if s.billing != nil {
		authorization, err = s.billing.AuthorizeUsage(ctx, key.UserID, platformModelName, strings.TrimSpace(requestID))
		if err != nil {
			return nil, err
		}
	}

	preparedRequest := cloneMap(request)
	normalizePreparedChatMaxTokens(preparedRequest, route)
	normalizePreparedChatReasoningEffort(preparedRequest, route)
	preparedRequest = filterPreparedChatRequest(preparedRequest, route, s.modelOptionFilter)

	return &PreparedChatCompletion{
		key:               key,
		request:           preparedRequest,
		requestID:         strings.TrimSpace(requestID),
		stream:            stream,
		route:             route,
		routeConfig:       routeConfigFromResolvedRoute(route),
		authorization:     authorization,
		startedAt:         startedAt,
		platformModelName: platformModelName,
		publicModelID:     publicModelID,
	}, nil
}

func normalizePreparedChatMaxTokens(request map[string]interface{}, route *appchannel.ResolvedRoute) {
	if len(request) == 0 || route == nil {
		return
	}
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterOpenAIChatCompletions:
		if strings.EqualFold(strings.TrimSpace(route.UpstreamCompatible), "openai") {
			normalizeMaxTokenTarget(request, "max_completion_tokens", "max_tokens")
		}
	case llm.AdapterOpenAIResponses:
		if strings.EqualFold(strings.TrimSpace(route.UpstreamCompatible), "openai") {
			normalizeMaxTokenTarget(request, "max_completion_tokens", "max_output_tokens", "max_tokens")
		}
	case llm.AdapterGoogleGenerateContent:
		normalizeMaxTokenTarget(request, "max_output_tokens", "max_completion_tokens", "max_tokens")
	}
}

func filterPreparedChatRequest(request map[string]interface{}, route *appchannel.ResolvedRoute, filter modelOptionFilter) map[string]interface{} {
	prepared := make(map[string]interface{})
	for _, key := range []string{"model", "messages", "stream", "functions", "function_call"} {
		if value, ok := request[key]; ok {
			prepared[key] = value
		}
	}

	options := cloneMap(request)
	for _, key := range []string{"model", "messages", "stream", "functions", "function_call"} {
		delete(options, key)
	}
	filtered := filter.FilterModelOptionsForRoute(options, route.Protocol, route.ModelCapabilitiesJSON)
	for key, value := range filtered {
		if key != "tools" {
			prepared[key] = value
		}
	}

	clientTools := clientFunctionToolPayloads(request["tools"])
	tools := append([]interface{}{}, clientTools...)
	for _, tool := range providerNativeToolPayloads(filtered["tools"]) {
		tools = append(tools, tool)
	}
	if len(tools) > 0 {
		prepared["tools"] = tools
	}
	if choice, ok := clientFunctionToolChoice(request["tool_choice"], clientTools); ok {
		prepared["tool_choice"] = choice
	}
	if len(clientTools) > 0 {
		if parallel, ok := request["parallel_tool_calls"].(bool); ok {
			prepared["parallel_tool_calls"] = parallel
		}
	}
	return prepared
}

func clientFunctionToolPayloads(raw interface{}) []interface{} {
	items := providerNativeToolPayloads(raw)
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		if !strings.EqualFold(strings.TrimSpace(stringValue(item["type"])), "function") {
			continue
		}
		if function, ok := item["function"].(map[string]interface{}); !ok || len(function) == 0 {
			continue
		}
		result = append(result, item)
	}
	return result
}

func providerNativeToolPayloads(raw interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0)
	switch items := raw.(type) {
	case []interface{}:
		for _, rawItem := range items {
			if item, ok := rawItem.(map[string]interface{}); ok {
				result = append(result, item)
			}
		}
	case []map[string]interface{}:
		result = append(result, items...)
	}
	return result
}

func clientFunctionToolChoice(raw interface{}, tools []interface{}) (interface{}, bool) {
	if len(tools) == 0 || raw == nil {
		return nil, false
	}
	if value, ok := raw.(string); ok {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "auto", "none", "required", "any":
			return raw, true
		default:
			return nil, false
		}
	}
	choice, ok := raw.(map[string]interface{})
	if !ok || !strings.EqualFold(strings.TrimSpace(stringValue(choice["type"])), "function") {
		return nil, false
	}
	function, _ := choice["function"].(map[string]interface{})
	name := strings.TrimSpace(stringValue(function["name"]))
	if name == "" {
		name = strings.TrimSpace(stringValue(choice["name"]))
	}
	if name == "" {
		return nil, false
	}
	for _, rawTool := range tools {
		tool, _ := rawTool.(map[string]interface{})
		toolFunction, _ := tool["function"].(map[string]interface{})
		if strings.TrimSpace(stringValue(toolFunction["name"])) == name {
			return raw, true
		}
	}
	return nil, false
}

func normalizePreparedChatReasoningEffort(request map[string]interface{}, route *appchannel.ResolvedRoute) {
	if len(request) == 0 || route == nil || !isOfficialOpenAIUpstream(route) {
		return
	}
	switch llm.NormalizeAdapter(route.Protocol) {
	case llm.AdapterOpenAIChatCompletions, llm.AdapterOpenAIResponses:
		if requestHasFunctionTools(request) {
			delete(request, "reasoning_effort")
		}
	}
}

func isOfficialOpenAIUpstream(route *appchannel.ResolvedRoute) bool {
	return route != nil && strings.EqualFold(strings.TrimSpace(route.UpstreamCompatible), "openai")
}

func requestHasFunctionTools(request map[string]interface{}) bool {
	return nonEmptyArrayValue(request["tools"]) || nonEmptyArrayValue(request["functions"])
}

func nonEmptyArrayValue(value interface{}) bool {
	switch typed := value.(type) {
	case []interface{}:
		return len(typed) > 0
	case []map[string]interface{}:
		return len(typed) > 0
	default:
		return false
	}
}

func normalizeMaxTokenTarget(payload map[string]interface{}, target string, aliases ...string) {
	if len(payload) == 0 || strings.TrimSpace(target) == "" {
		return
	}
	if _, ok := payload[target]; !ok {
		for _, alias := range aliases {
			if value, exists := payload[alias]; exists {
				payload[target] = value
				break
			}
		}
	}
	for _, alias := range aliases {
		delete(payload, alias)
	}
}

// CompleteChatCompletion 执行非流式 Chat Completions，并返回兼容 JSON。
func (s *Service) CompleteChatCompletion(ctx context.Context, prepared *PreparedChatCompletion) (map[string]interface{}, error) {
	if prepared == nil {
		return nil, ErrInvalidRequest
	}
	stopAuthorizationRenewal := s.startUsageAuthorizationRenewal(prepared)
	defer stopAuthorizationRenewal()
	result, err := s.chatProvider.CompleteChat(ctx, prepared.routeConfig, prepared.request)
	if err != nil {
		s.markRouteFailure(ctx, prepared, err)
		if llm.RequestWasAccepted(err) {
			return nil, s.markAuthorizationForReconciliation(prepared, "open_api_upstream_failed_after_acceptance", err)
		}
		s.releaseReservation(ctx, prepared, "open api upstream call failed")
		return nil, err
	}
	s.markRouteSuccess(ctx, prepared)

	body := cloneMap(result.Body)
	answerText := completionTextFromBody(body)
	body["model"] = prepared.publicModelID
	if result.ReasoningText != "" {
		addReasoningContentToCompletionBody(body, result.ReasoningText)
	}
	usage := result.Usage
	hasProviderUsage := usage != (llm.Usage{})
	if !hasProviderUsage {
		usage = fallbackUsage(prepared.request, answerText, result.ReasoningText)
	}
	ensureUsageInBody(body, usage)
	if !hasProviderUsage && requiresAuthoritativeProviderUsage(prepared.request) {
		if err := s.retainAuthorizationForReconciliation(prepared, "open_api_usage_missing_for_non_text_input"); err != nil {
			return nil, err
		}
		return body, nil
	}
	if err := s.recordBilling(ctx, prepared, usage, result.ServerSideToolUsage); err != nil {
		return nil, err
	}
	return body, nil
}

// StreamChatCompletion 执行流式 Chat Completions。emit 只接收 JSON data 片段，不含 [DONE]。
func (s *Service) StreamChatCompletion(ctx context.Context, prepared *PreparedChatCompletion, emit func(map[string]interface{}) error) error {
	if prepared == nil {
		return ErrInvalidRequest
	}
	stopAuthorizationRenewal := s.startUsageAuthorizationRenewal(prepared)
	defer stopAuthorizationRenewal()
	var outputText strings.Builder
	var reasoningText strings.Builder
	var usage llm.Usage
	usageSent := false
	observedUpstream := false
	var downstreamErr error
	emitChunk := func(chunk map[string]interface{}) error {
		err := emit(chunk)
		if err != nil {
			downstreamErr = err
		}
		return err
	}

	emitReasoning := func(content string) error {
		chunk := makeStreamReasoningChunk(prepared.publicModelID, content)
		return emitChunk(chunk)
	}

	result, err := s.chatProvider.StreamChat(ctx, prepared.routeConfig, prepared.request, func(event RawChatStreamEvent) error {
		observedUpstream = true
		if event.Usage != (llm.Usage{}) {
			usage = event.Usage
			usageSent = true
		}
		if event.Reasoning != nil && strings.TrimSpace(event.Reasoning.Text) != "" {
			text := event.Reasoning.Text
			reasoningText.WriteString(text)
			if len(event.Body) == 0 || !streamChunkHasReasoning(event.Body) {
				if err := emitReasoning(text); err != nil {
					return err
				}
			}
		}
		if len(event.Body) == 0 {
			return nil
		}
		chunk := cloneMap(event.Body)
		chunk["model"] = prepared.publicModelID
		if streamChunkHasContent(chunk) {
			outputText.WriteString(streamContentDelta(chunk))
		}
		return emitChunk(chunk)
	})
	if err != nil {
		if downstreamErr == nil {
			s.markRouteFailure(ctx, prepared, err)
		}
		if observedUpstream {
			return s.markAuthorizationForReconciliation(prepared, "open_api_stream_interrupted_after_output", err)
		}
		if llm.RequestWasAccepted(err) {
			return s.markAuthorizationForReconciliation(prepared, "open_api_stream_failed_after_acceptance", err)
		}
		s.releaseReservation(ctx, prepared, "open api upstream stream failed")
		return err
	}
	s.markRouteSuccess(ctx, prepared)

	hasProviderUsage := usage != (llm.Usage{})
	if !hasProviderUsage {
		usage = result.Usage
		hasProviderUsage = usage != (llm.Usage{})
	}
	if !hasProviderUsage {
		usage = fallbackUsage(prepared.request, outputText.String(), reasoningText.String())
	}
	if !hasProviderUsage && requiresAuthoritativeProviderUsage(prepared.request) {
		if err := s.retainAuthorizationForReconciliation(prepared, "open_api_usage_missing_for_non_text_input"); err != nil {
			return err
		}
	} else {
		if err := s.recordBilling(ctx, prepared, usage, result.ServerSideToolUsage); err != nil {
			return err
		}
	}
	if llm.NormalizeAdapter(prepared.routeConfig.Protocol) != llm.AdapterOpenAIChatCompletions && len(result.ToolCalls) > 0 {
		if err := emitChunk(chatStreamToolFinishChunk(prepared.publicModelID, result.ResponseID)); err != nil {
			return err
		}
	}
	if !usageSent {
		return emitChunk(makeStreamUsageChunk(prepared.publicModelID, usage))
	}
	return nil
}

func (s *Service) resolvePublicModelRoute(
	ctx context.Context,
	allowlist []string,
	publicModelID string,
	userID uint,
	requestID string,
) (*appchannel.ResolvedRoute, error) {
	publicModelID = strings.TrimSpace(publicModelID)
	if publicModelID == "" {
		return nil, ErrModelNotAllowed
	}
	for _, platformModelName := range allowlist {
		platformModelName = strings.TrimSpace(platformModelName)
		if platformModelName == "" {
			continue
		}
		route, err := s.channel.ResolveRoute(ctx, appchannel.ResolveRouteInput{
			PlatformModelName: platformModelName,
			UpstreamModelName: publicModelID,
			TaskType:          appchannel.TaskTypeChat,
			UserID:            userID,
			RequestID:         strings.TrimSpace(requestID),
		})
		if err != nil {
			continue
		}
		if !isPublicChatProtocol(route.Protocol) {
			continue
		}
		if strings.TrimSpace(route.UpstreamModel) != publicModelID {
			continue
		}
		return route, nil
	}
	return nil, ErrModelNotAllowed
}

func (s *Service) createAPIKey(ctx context.Context, userID uint) (*APIKeyView, error) {
	plaintext, item, err := s.newAPIKeyRecord(userID)
	if err != nil {
		return nil, err
	}
	stored, err := s.keyRepo.CreateForUser(ctx, item)
	if errors.Is(err, repository.ErrConflict) {
		return nil, ErrAPIKeyAlreadyExists
	}
	if err != nil {
		return nil, err
	}
	return apiKeyViewForStoredPlaintext(stored, item, plaintext)
}

func (s *Service) replaceAPIKey(ctx context.Context, userID uint, current *domainopenapi.UserAPIKey) (*APIKeyView, error) {
	plaintext, item, err := s.newAPIKeyRecord(userID)
	if err != nil {
		return nil, err
	}
	stored, err := s.keyRepo.ReplaceForUserIfCurrent(ctx, item, current)
	if errors.Is(err, repository.ErrConflict) {
		return nil, ErrAPIKeyConflict
	}
	if err != nil {
		return nil, err
	}
	return apiKeyViewForStoredPlaintext(stored, item, plaintext)
}

func (s *Service) newAPIKeyRecord(userID uint) (string, *domainopenapi.UserAPIKey, error) {
	plaintext, err := generateAPIKey()
	if err != nil {
		return "", nil, err
	}
	encrypted, err := s.encryptAPIKeyPlaintext(plaintext)
	if err != nil {
		return "", nil, err
	}
	now := s.now()
	item := &domainopenapi.UserAPIKey{
		UserID:                userID,
		KeyHash:               s.hashAPIKey(plaintext),
		KeyPrefix:             displayKeyPrefix(plaintext),
		KeyPlaintextEncrypted: encrypted,
		Status:                domainopenapi.APIKeyStatusActive,
		CreatedAt:             now,
		UpdatedAt:             now,
	}
	return plaintext, item, nil
}

func apiKeyViewForStoredPlaintext(stored *domainopenapi.UserAPIKey, expected *domainopenapi.UserAPIKey, plaintext string) (*APIKeyView, error) {
	if stored == nil || expected == nil || stored.UserID != expected.UserID || stored.Status != domainopenapi.APIKeyStatusActive || stored.KeyHash != expected.KeyHash {
		return nil, ErrAPIKeyConflict
	}
	return toAPIKeyView(stored, plaintext, false, true), nil
}

func (s *Service) requireTwoFactor(ctx context.Context, userID uint) error {
	enabled, err := s.isTwoFactorEnabled(ctx, userID)
	if err != nil {
		return err
	}
	if !enabled {
		return ErrTwoFactorRequired
	}
	return nil
}

func (s *Service) isTwoFactorEnabled(ctx context.Context, userID uint) (bool, error) {
	if s == nil || s.twoFactor == nil || userID == 0 {
		return false, nil
	}
	return s.twoFactor.IsTwoFactorEnabled(ctx, userID)
}

func (s *Service) encryptAPIKeyPlaintext(plaintext string) (string, error) {
	secret := strings.TrimSpace(s.dataEncryptionKey)
	if secret == "" {
		secret = "deeix-openapi-key"
	}
	return secretbox.EncryptString(secret, plaintext)
}

func (s *Service) decryptAPIKeyPlaintext(encrypted string) (string, error) {
	secret := strings.TrimSpace(s.dataEncryptionKey)
	if secret == "" {
		secret = "deeix-openapi-key"
	}
	return secretbox.DecryptString(secret, encrypted)
}

func (s *Service) hashAPIKey(plaintext string) string {
	secret := strings.TrimSpace(s.dataEncryptionKey)
	if secret == "" {
		secret = "deeix-openapi-key"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(strings.TrimSpace(plaintext)))
	return hex.EncodeToString(mac.Sum(nil))
}

func generateAPIKey() (string, error) {
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", err
	}
	return APIKeyPlaintextPrefix + base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func displayKeyPrefix(plaintext string) string {
	value := strings.TrimSpace(plaintext)
	if len(value) <= 16 {
		return value
	}
	return value[:16]
}

func toAPIKeyView(item *domainopenapi.UserAPIKey, plaintext string, twoFactorRequired bool, exportable bool) *APIKeyView {
	if item == nil {
		return &APIKeyView{Exists: false}
	}
	return &APIKeyView{
		Exists:            true,
		APIKey:            plaintext,
		KeyPrefix:         item.KeyPrefix,
		Status:            item.Status,
		LastUsedAt:        item.LastUsedAt,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
		TwoFactorRequired: twoFactorRequired,
		Exportable:        exportable,
	}
}

type openAPISettings struct {
	ModelAllowlist []string
	RPM            int
}

func (s *Service) loadSettings(ctx context.Context) (openAPISettings, error) {
	result := openAPISettings{RPM: 60}
	if s == nil || s.settings == nil {
		return result, nil
	}
	values, err := s.settings.RuntimeValuesByNamespace(ctx, openAPISettingsNamespace)
	if err != nil {
		return result, err
	}
	result.ModelAllowlist = domainbilling.ParseModelNameList(values[openAPISettingModelAllowlist])
	if raw := strings.TrimSpace(values[openAPISettingRateLimitRPM]); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed >= 0 {
			result.RPM = parsed
		}
	}
	return result, nil
}

func modelAllowlistSet(items []string) map[string]struct{} {
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item)
		if name != "" {
			result[name] = struct{}{}
		}
	}
	return result
}

func isPublicChatProtocol(protocol string) bool {
	switch llm.NormalizeAdapter(protocol) {
	case llm.AdapterOpenAIResponses,
		llm.AdapterOpenAIChatCompletions,
		llm.AdapterAnthropicMessages,
		llm.AdapterGoogleGenerateContent,
		llm.AdapterXAIResponses:
		return true
	default:
		return false
	}
}

func routeConfigFromResolvedRoute(route *appchannel.ResolvedRoute) llm.RouteConfig {
	if route == nil {
		return llm.RouteConfig{}
	}
	return llm.RouteConfig{
		Protocol:            route.Protocol,
		BaseURL:             route.BaseURL,
		APIKey:              route.APIKey,
		HeadersJSON:         route.HeadersJSON,
		ConnectTimeoutMS:    route.ConnectTimeoutMS,
		ReadTimeoutMS:       route.ReadTimeoutMS,
		StreamIdleTimeoutMS: route.StreamIdleTimeoutMS,
		Endpoint:            llm.DefaultEndpointForAdapter(route.Protocol),
		UpstreamModel:       route.UpstreamModel,
	}
}

func (s *Service) markRouteSuccess(ctx context.Context, prepared *PreparedChatCompletion) {
	if s != nil && s.channel != nil && prepared != nil {
		s.channel.MarkRouteSuccess(ctx, prepared.route)
	}
}

func (s *Service) markRouteFailure(ctx context.Context, prepared *PreparedChatCompletion, cause error) {
	if s != nil && s.channel != nil && prepared != nil {
		s.channel.MarkRouteFailure(ctx, prepared.route, cause)
	}
}

func (s *Service) releaseReservation(_ context.Context, prepared *PreparedChatCompletion, _ string) {
	if s == nil || s.billing == nil || prepared == nil || prepared.authorization == nil {
		return
	}
	releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.billing.ReleaseUsageAuthorization(releaseCtx, prepared.authorization)
}

func (s *Service) startUsageAuthorizationRenewal(prepared *PreparedChatCompletion) func() {
	if s == nil || s.billing == nil || prepared == nil || prepared.authorization == nil || prepared.authorization.Reservation == nil {
		return func() {}
	}
	stop := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(openAPIUsageAuthorizationRenewalInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				renewCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = s.billing.RenewUsageAuthorization(renewCtx, prepared.authorization)
				cancel()
			case <-stop:
				return
			}
		}
	}()
	return func() { once.Do(func() { close(stop) }) }
}

func (s *Service) markAuthorizationForReconciliation(prepared *PreparedChatCompletion, failureCode string, cause error) error {
	if err := s.retainAuthorizationForReconciliation(prepared, failureCode); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func (s *Service) retainAuthorizationForReconciliation(prepared *PreparedChatCompletion, failureCode string) error {
	if s == nil || s.billing == nil || prepared == nil || prepared.authorization == nil || prepared.authorization.Reservation == nil {
		return nil
	}
	reconcileCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.billing.MarkUsageAuthorizationForReconciliation(reconcileCtx, prepared.authorization, failureCode); err != nil {
		return fmt.Errorf("mark open api usage reconciliation: %w", err)
	}
	return nil
}

func (s *Service) recordBilling(ctx context.Context, prepared *PreparedChatCompletion, usage llm.Usage, serverSideToolUsage map[string]int64) error {
	if s == nil || s.billing == nil || prepared == nil {
		return nil
	}
	ledger, err := s.billing.BuildUsageLedger(ctx, appbilling.UsagePricingInput{
		Authorization:       prepared.authorization,
		UserID:              prepared.key.UserID,
		ConversationID:      0,
		PlatformModelName:   prepared.platformModelName,
		RoutedBindingCode:   prepared.route.BindingCode,
		ProviderProtocol:    prepared.route.Protocol,
		UpstreamName:        prepared.route.UpstreamName,
		UpstreamModelName:   prepared.route.UpstreamModel,
		InputTokens:         usage.InputTokens,
		CacheReadTokens:     usage.CacheReadTokens,
		CacheWriteTokens:    usage.CacheWriteTokens,
		CacheWrite5mTokens:  usage.CacheWrite5mTokens,
		CacheWrite1hTokens:  usage.CacheWrite1hTokens,
		OutputTokens:        usage.OutputTokens,
		ReasoningTokens:     usage.ReasoningTokens,
		CallCount:           1,
		LatencyMS:           int64(s.now().Sub(prepared.startedAt) / time.Millisecond),
		UsageSpeed:          usage.Speed,
		UsageServiceTier:    usage.ServiceTier,
		ServerSideToolUsage: serverSideToolUsage,
		BillingAt:           prepared.startedAt,
	})
	if err != nil {
		return s.markAuthorizationForReconciliation(prepared, "open_api_build_usage_failed", err)
	}
	if ledger == nil {
		return nil
	}
	if err = s.billing.RecordUsageWithAuthorization(ctx, ledger, prepared.authorization); err != nil {
		return s.markAuthorizationForReconciliation(prepared, "open_api_record_usage_failed", err)
	}
	return nil
}

func cloneMap(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(raw))
	for key, value := range raw {
		result[key] = value
	}
	return result
}

func addReasoningContentToCompletionBody(body map[string]interface{}, reasoning string) {
	content := strings.TrimSpace(reasoning)
	if content == "" {
		return
	}
	choices, _ := body["choices"].([]interface{})
	for _, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]interface{})
		if !ok {
			continue
		}
		message, ok := choice["message"].(map[string]interface{})
		if !ok {
			continue
		}
		if len(asSlice(message["tool_calls"])) > 0 {
			continue
		}
		if strings.TrimSpace(stringValue(message["reasoning_content"])) == "" {
			message["reasoning_content"] = content
		}
	}
}

func completionTextFromBody(body map[string]interface{}) string {
	var sb strings.Builder
	choices, _ := body["choices"].([]interface{})
	for _, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]interface{})
		if !ok {
			continue
		}
		message, _ := choice["message"].(map[string]interface{})
		sb.WriteString(contentText(message["content"]))
	}
	return sb.String()
}

func ensureUsageInBody(body map[string]interface{}, usage llm.Usage) {
	if body == nil || len(asMap(body["usage"])) > 0 {
		return
	}
	body["usage"] = usageResponseMap(usage)
}

func usageResponseMap(usage llm.Usage) map[string]interface{} {
	promptTokens := usage.InputTokens + usage.CacheReadTokens + usage.CacheWriteTokens
	completionTokens := usage.OutputTokens + usage.ReasoningTokens
	if promptTokens < 0 {
		promptTokens = 0
	}
	if completionTokens < 0 {
		completionTokens = 0
	}
	result := map[string]interface{}{
		"prompt_tokens":     promptTokens,
		"completion_tokens": completionTokens,
		"total_tokens":      promptTokens + completionTokens,
	}
	if usage.CacheReadTokens > 0 || usage.CacheWriteTokens > 0 {
		result["prompt_tokens_details"] = map[string]interface{}{
			"cached_tokens":      usage.CacheReadTokens,
			"cache_write_tokens": usage.CacheWriteTokens,
		}
	}
	if usage.ReasoningTokens > 0 {
		result["completion_tokens_details"] = map[string]interface{}{
			"reasoning_tokens": usage.ReasoningTokens,
		}
	}
	return result
}

func fallbackUsage(request map[string]interface{}, outputText string, reasoningText string) llm.Usage {
	input := estimateRequestTokens(request)
	output := estimateTokens(outputText)
	reasoning := estimateTokens(reasoningText)
	if input <= 0 {
		input = 1
	}
	if output <= 0 && reasoning <= 0 {
		output = 1
	}
	return llm.Usage{
		InputTokens:     input,
		OutputTokens:    output,
		ReasoningTokens: reasoning,
	}
}

func estimateRequestTokens(request map[string]interface{}) int64 {
	return estimateAnyTokens(request["messages"])
}

// requiresAuthoritativeProviderUsage identifies request inputs whose upstream
// token accounting is not represented by the text-only fallback estimator.
// When a compatible provider omits usage, settling from that estimate would
// underbill the request, so the pre-authorization is retained for
// reconciliation instead.
func requiresAuthoritativeProviderUsage(request map[string]interface{}) bool {
	for _, field := range []string{"tools", "functions", "tool_choice", "function_call"} {
		if hasNonEmptyOpenAPIInput(request[field]) {
			return true
		}
	}
	messages, ok := request["messages"].([]interface{})
	if !ok {
		return false
	}
	for _, rawMessage := range messages {
		message, ok := rawMessage.(map[string]interface{})
		if !ok {
			// A successfully processed request could not normally reach this
			// branch, but retaining the authorization is safer than estimating
			// an unrecognized shape.
			return true
		}
		if hasNonEmptyOpenAPIInput(message["tool_calls"]) || hasNonEmptyOpenAPIInput(message["function_call"]) {
			return true
		}
		role := strings.ToLower(strings.TrimSpace(stringValue(message["role"])))
		if role == "tool" || role == "function" {
			return true
		}
		parts, ok := message["content"].([]interface{})
		if !ok {
			continue
		}
		for _, rawPart := range parts {
			part, ok := rawPart.(map[string]interface{})
			if !ok {
				return true
			}
			switch strings.ToLower(strings.TrimSpace(stringValue(part["type"]))) {
			case "text", "input_text":
				continue
			default:
				// This includes image_url and input_image, as well as any
				// future multimodal part the local fallback cannot price.
				return true
			}
		}
	}
	return false
}

func hasNonEmptyOpenAPIInput(raw interface{}) bool {
	switch value := raw.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(value) != ""
	case []interface{}:
		return len(value) > 0
	case []map[string]interface{}:
		return len(value) > 0
	case map[string]interface{}:
		return len(value) > 0
	default:
		return true
	}
}

func estimateAnyTokens(value interface{}) int64 {
	switch typed := value.(type) {
	case string:
		return estimateTokens(typed)
	case []interface{}:
		var total int64
		for _, item := range typed {
			total += estimateAnyTokens(item)
		}
		return total
	case map[string]interface{}:
		var total int64
		for key, item := range typed {
			switch key {
			case "content", "text", "role", "name":
				total += estimateAnyTokens(item)
			}
		}
		return total
	default:
		return 0
	}
}

func estimateTokens(content string) int64 {
	text := strings.TrimSpace(content)
	if text == "" {
		return 0
	}
	var cjk int64
	var other int64
	for _, r := range text {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul) {
			cjk++
			continue
		}
		other++
	}
	tokens := cjk + int64(math.Ceil(float64(other)/4.0))
	if tokens <= 0 {
		return 1
	}
	return tokens
}

func asMap(raw interface{}) map[string]interface{} {
	if payload, ok := raw.(map[string]interface{}); ok {
		return payload
	}
	return map[string]interface{}{}
}

func asSlice(raw interface{}) []interface{} {
	if items, ok := raw.([]interface{}); ok {
		return items
	}
	return nil
}

func stringValue(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case fmt.Stringer:
		return typed.String()
	default:
		return ""
	}
}

func contentText(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return typed
	case []interface{}:
		var sb strings.Builder
		for _, item := range typed {
			part, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			sb.WriteString(stringValue(part["text"]))
		}
		return sb.String()
	default:
		return ""
	}
}

func streamChunkHasContent(chunk map[string]interface{}) bool {
	return streamContentDelta(chunk) != ""
}

func streamChunkHasReasoning(chunk map[string]interface{}) bool {
	return streamReasoningDelta(chunk) != ""
}

func streamContentDelta(chunk map[string]interface{}) string {
	choices, _ := chunk["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]interface{})
	delta, _ := choice["delta"].(map[string]interface{})
	return streamVisibleContentText(delta["content"])
}

func streamReasoningDelta(chunk map[string]interface{}) string {
	choices, _ := chunk["choices"].([]interface{})
	if len(choices) == 0 {
		return ""
	}
	choice, _ := choices[0].(map[string]interface{})
	delta, _ := choice["delta"].(map[string]interface{})
	if text := contentText(delta["reasoning_content"]); text != "" {
		return text
	}
	if text := contentText(delta["reasoning"]); text != "" {
		return text
	}
	return streamReasoningContentText(delta["content"])
}

func streamVisibleContentText(raw interface{}) string {
	switch typed := raw.(type) {
	case []interface{}:
		var sb strings.Builder
		for _, item := range typed {
			part, ok := item.(map[string]interface{})
			if !ok || isStreamReasoningContentType(part["type"]) {
				continue
			}
			sb.WriteString(stringValue(part["text"]))
		}
		return sb.String()
	default:
		return contentText(raw)
	}
}

func streamReasoningContentText(raw interface{}) string {
	switch typed := raw.(type) {
	case []interface{}:
		var sb strings.Builder
		for _, item := range typed {
			part, ok := item.(map[string]interface{})
			if !ok || !isStreamReasoningContentType(part["type"]) {
				continue
			}
			sb.WriteString(firstNonEmpty(stringValue(part["text"]), stringValue(part["content"])))
		}
		return sb.String()
	case map[string]interface{}:
		if !isStreamReasoningContentType(typed["type"]) {
			return ""
		}
		return firstNonEmpty(stringValue(typed["text"]), stringValue(typed["content"]))
	default:
		return ""
	}
}

func isStreamReasoningContentType(raw interface{}) bool {
	switch strings.TrimSpace(strings.ToLower(stringValue(raw))) {
	case "reasoning", "reasoning_content", "reasoning_text", "thinking":
		return true
	default:
		return false
	}
}

func makeStreamContentChunk(model string, content string) map[string]interface{} {
	return map[string]interface{}{
		"id":      "chatcmpl-openapi-content",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"content": content,
				},
			},
		},
	}
}

func makeStreamReasoningChunk(model string, content string) map[string]interface{} {
	return map[string]interface{}{
		"id":      "chatcmpl-openapi-reasoning",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []interface{}{
			map[string]interface{}{
				"index": 0,
				"delta": map[string]interface{}{
					"reasoning_content": content,
				},
			},
		},
	}
}

func makeStreamUsageChunk(model string, usage llm.Usage) map[string]interface{} {
	return map[string]interface{}{
		"id":      "chatcmpl-openapi-usage",
		"object":  "chat.completion.chunk",
		"created": time.Now().Unix(),
		"model":   model,
		"choices": []interface{}{},
		"usage":   usageResponseMap(usage),
	}
}
