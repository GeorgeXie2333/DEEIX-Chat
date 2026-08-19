package openapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/infra/llm"
	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

const defaultOpenAPIImageMaxBytes int64 = 20 * 1024 * 1024

var allowedOpenAPIImageMIMEs = map[string]struct{}{
	"image/gif":  {},
	"image/jpeg": {},
	"image/png":  {},
	"image/webp": {},
}

type chatImageResolver interface {
	ResolveChatImage(ctx context.Context, rawURL string) (llm.ContentPart, error)
}

type chatImageResolverFunc func(context.Context, string) (llm.ContentPart, error)

func (f chatImageResolverFunc) ResolveChatImage(ctx context.Context, rawURL string) (llm.ContentPart, error) {
	return f(ctx, rawURL)
}

// HTTPChatImageResolver downloads remote Chat Completions image_url inputs for
// upstream protocols that require inline image bytes.
type HTTPChatImageResolver struct {
	client   *http.Client
	policy   security.OutboundPolicy
	maxBytes int64
}

// NewHTTPChatImageResolver creates a safe resolver for OpenAI-compatible image_url inputs.
func NewHTTPChatImageResolver(policy security.OutboundPolicy, maxBytes int64) *HTTPChatImageResolver {
	if maxBytes <= 0 {
		maxBytes = defaultOpenAPIImageMaxBytes
	}
	return &HTTPChatImageResolver{
		client:   security.NewOutboundHTTPClient(policy, 60*time.Second),
		policy:   policy,
		maxBytes: maxBytes,
	}
}

func (r *HTTPChatImageResolver) ResolveChatImage(ctx context.Context, rawURL string) (llm.ContentPart, error) {
	imageURL := strings.TrimSpace(rawURL)
	parsed, err := url.Parse(imageURL)
	if err != nil || parsed == nil || parsed.Scheme == "" || parsed.Host == "" {
		return llm.ContentPart{}, fmt.Errorf("%w: invalid image_url", ErrInvalidRequest)
	}
	if parsed.User != nil || !strings.EqualFold(parsed.Scheme, "https") {
		return llm.ContentPart{}, fmt.Errorf("%w: image_url must be https", ErrInvalidRequest)
	}
	if err := security.ValidateOutboundHTTPURL(imageURL, r.policy); err != nil {
		return llm.ContentPart{}, fmt.Errorf("%w: unsafe image_url", ErrInvalidRequest)
	}
	client := r.client
	if client == nil {
		client = security.NewOutboundHTTPClient(r.policy, 60*time.Second)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return llm.ContentPart{}, err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := client.Do(req)
	if err != nil {
		return llm.ContentPart{}, err
	}
	defer resp.Body.Close() //nolint:errcheck
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return llm.ContentPart{}, fmt.Errorf("%w: image_url returned HTTP %d", ErrInvalidRequest, resp.StatusCode)
	}
	limit := r.maxBytes
	if limit <= 0 {
		limit = defaultOpenAPIImageMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return llm.ContentPart{}, err
	}
	if int64(len(data)) > limit {
		return llm.ContentPart{}, fmt.Errorf("%w: image_url too large", ErrInvalidRequest)
	}
	mimeType := normalizeImageMIME(resp.Header.Get("Content-Type"))
	if mimeType == "" {
		mimeType = normalizeImageMIME(http.DetectContentType(data))
	}
	if mimeType == "" {
		return llm.ContentPart{}, fmt.Errorf("%w: image_url content is not a supported image", ErrInvalidRequest)
	}
	return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: mimeType, Data: data}, nil
}

func buildGenerateInputFromChatCompletion(ctx context.Context, request map[string]interface{}, resolver chatImageResolver) (llm.GenerateInput, error) {
	messages, err := parseChatCompletionMessages(ctx, request["messages"], resolver)
	if err != nil {
		return llm.GenerateInput{}, err
	}
	tools, err := parseChatCompletionToolDefinitions(request["tools"], request["functions"])
	if err != nil {
		return llm.GenerateInput{}, err
	}
	options := cloneMap(request)
	for _, key := range []string{"model", "messages", "stream", "tools", "functions", "function_call"} {
		delete(options, key)
	}
	if _, ok := options["tool_choice"]; !ok {
		if toolChoice := legacyFunctionCallToolChoice(request["function_call"]); toolChoice != nil {
			options["tool_choice"] = toolChoice
		}
	}
	normalizeChatMaxTokens(options)
	return llm.GenerateInput{
		Messages: messages,
		Tools:    tools,
		Options:  options,
	}, nil
}

func normalizeChatMaxTokens(options map[string]interface{}) {
	if len(options) == 0 {
		return
	}
	normalizeMaxTokenTarget(options, "max_output_tokens", "max_completion_tokens", "max_tokens")
}

func parseChatCompletionMessages(ctx context.Context, raw interface{}, resolver chatImageResolver) ([]llm.Message, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("%w: messages must be a non-empty array", ErrInvalidRequest)
	}
	messages := make([]llm.Message, 0, len(items))
	state := chatToolMessageState{
		nameByID: make(map[string]string),
		idByName: make(map[string]string),
	}
	for index, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: message must be an object", ErrInvalidRequest)
		}
		role := strings.TrimSpace(stringValue(item["role"]))
		if role == "" {
			role = "user"
		}
		if isChatToolResultRole(role) {
			message, err := parseChatToolResultMessage(item, role, &state)
			if err != nil {
				return nil, err
			}
			messages = append(messages, message)
			continue
		}
		message := llm.Message{Role: role}
		content := item["content"]
		switch typed := content.(type) {
		case nil:
			message.Content = ""
		case string:
			message.Content = typed
		case []interface{}:
			parts, err := parseChatCompletionContentParts(ctx, role, typed, resolver)
			if err != nil {
				return nil, err
			}
			message.Parts = parts
		default:
			return nil, fmt.Errorf("%w: message content must be text or content parts", ErrInvalidRequest)
		}
		if strings.EqualFold(strings.TrimSpace(role), "assistant") {
			toolCalls, err := parseAssistantToolCalls(item, index)
			if err != nil {
				return nil, err
			}
			message.ToolCalls = toolCalls
			state.rememberToolCalls(toolCalls)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

type chatToolMessageState struct {
	nameByID map[string]string
	idByName map[string]string
}

func (s *chatToolMessageState) rememberToolCalls(toolCalls []llm.ToolCall) {
	if s == nil {
		return
	}
	for _, call := range toolCalls {
		id := strings.TrimSpace(call.ToolCallID)
		name := strings.TrimSpace(call.ToolName)
		if id != "" && name != "" {
			s.nameByID[id] = name
			s.idByName[name] = id
		}
	}
}

func isChatToolResultRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "tool", "function":
		return true
	default:
		return false
	}
}

func parseChatToolResultMessage(item map[string]interface{}, role string, state *chatToolMessageState) (llm.Message, error) {
	toolName := strings.TrimSpace(stringValue(item["name"]))
	toolCallID := strings.TrimSpace(stringValue(item["tool_call_id"]))
	if toolCallID == "" && strings.EqualFold(strings.TrimSpace(role), "function") && toolName != "" && state != nil {
		toolCallID = strings.TrimSpace(state.idByName[toolName])
	}
	if toolName == "" && toolCallID != "" && state != nil {
		toolName = strings.TrimSpace(state.nameByID[toolCallID])
	}
	if toolCallID == "" || toolName == "" {
		return llm.Message{}, fmt.Errorf("%w: tool result must reference a known tool call", ErrInvalidRequest)
	}
	return llm.Message{
		Role: "tool",
		ToolResults: []llm.ToolResult{{
			ToolCallID: toolCallID,
			ToolName:   toolName,
			OutputJSON: chatToolContentString(item["content"]),
			Status:     "success",
		}},
	}, nil
}

func parseAssistantToolCalls(item map[string]interface{}, messageIndex int) ([]llm.ToolCall, error) {
	modern, err := parseModernAssistantToolCalls(item["tool_calls"], messageIndex)
	if err != nil {
		return nil, err
	}
	if len(modern) > 0 {
		return modern, nil
	}
	legacy := asMap(item["function_call"])
	if len(legacy) == 0 {
		return nil, nil
	}
	name := strings.TrimSpace(stringValue(legacy["name"]))
	if name == "" {
		return nil, fmt.Errorf("%w: function_call.name is required", ErrInvalidRequest)
	}
	args := normalizeOpenAPIJSONString(legacy["arguments"])
	if args == "" {
		args = "{}"
	}
	return []llm.ToolCall{{
		ToolCallID:       stableChatToolCallID(messageIndex, 0),
		ToolType:         "function",
		ToolName:         name,
		ArgumentsJSON:    args,
		ThoughtSignature: chatToolCallThoughtSignature(legacy, nil, stableChatToolCallID(messageIndex, 0)),
		Status:           "requested",
	}}, nil
}

func parseModernAssistantToolCalls(raw interface{}, messageIndex int) ([]llm.ToolCall, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: tool_calls must be an array", ErrInvalidRequest)
	}
	toolCalls := make([]llm.ToolCall, 0, len(items))
	for index, rawItem := range items {
		payload, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: tool_call must be an object", ErrInvalidRequest)
		}
		toolType := strings.TrimSpace(stringValue(payload["type"]))
		if toolType == "" {
			toolType = "function"
		}
		if toolType != "function" {
			return nil, fmt.Errorf("%w: only function tool calls are supported", ErrInvalidRequest)
		}
		function := asMap(payload["function"])
		name := strings.TrimSpace(stringValue(function["name"]))
		if name == "" {
			return nil, fmt.Errorf("%w: tool_call.function.name is required", ErrInvalidRequest)
		}
		args := normalizeOpenAPIJSONString(function["arguments"])
		if args == "" {
			args = "{}"
		}
		id := strings.TrimSpace(stringValue(payload["id"]))
		if id == "" {
			id = stableChatToolCallID(messageIndex, index)
		}
		toolCalls = append(toolCalls, llm.ToolCall{
			ToolCallID:       id,
			ToolType:         toolType,
			ToolName:         name,
			ArgumentsJSON:    args,
			ThoughtSignature: chatToolCallThoughtSignature(payload, function, id),
			Status:           "requested",
		})
	}
	return toolCalls, nil
}

func chatToolCallThoughtSignature(payload map[string]interface{}, function map[string]interface{}, toolCallID string) string {
	return firstNonEmpty(
		strings.TrimSpace(stringValue(payload["thought_signature"])),
		strings.TrimSpace(stringValue(payload["thoughtSignature"])),
		strings.TrimSpace(stringValue(function["thought_signature"])),
		strings.TrimSpace(stringValue(function["thoughtSignature"])),
		openAPIToolCallThoughtSignatureFromID(toolCallID),
	)
}

func stableChatToolCallID(messageIndex int, callIndex int) string {
	return fmt.Sprintf("call_%d_%d", messageIndex, callIndex)
}

func parseChatCompletionToolDefinitions(rawTools interface{}, rawFunctions interface{}) ([]llm.ToolDefinition, error) {
	tools, err := parseModernToolDefinitions(rawTools)
	if err != nil {
		return nil, err
	}
	legacy, err := parseLegacyFunctionDefinitions(rawFunctions)
	if err != nil {
		return nil, err
	}
	tools = append(tools, legacy...)
	return tools, nil
}

func parseModernToolDefinitions(raw interface{}) ([]llm.ToolDefinition, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: tools must be an array", ErrInvalidRequest)
	}
	tools := make([]llm.ToolDefinition, 0, len(items))
	for _, rawItem := range items {
		payload, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: tool must be an object", ErrInvalidRequest)
		}
		toolType := strings.TrimSpace(stringValue(payload["type"]))
		if toolType == "" {
			toolType = "function"
		}
		if toolType != "function" {
			continue
		}
		definition, err := parseToolFunctionDefinition(asMap(payload["function"]))
		if err != nil {
			return nil, err
		}
		tools = append(tools, definition)
	}
	return tools, nil
}

func parseLegacyFunctionDefinitions(raw interface{}) ([]llm.ToolDefinition, error) {
	if raw == nil {
		return nil, nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: functions must be an array", ErrInvalidRequest)
	}
	tools := make([]llm.ToolDefinition, 0, len(items))
	for _, rawItem := range items {
		payload, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: function must be an object", ErrInvalidRequest)
		}
		definition, err := parseToolFunctionDefinition(payload)
		if err != nil {
			return nil, err
		}
		tools = append(tools, definition)
	}
	return tools, nil
}

func parseToolFunctionDefinition(function map[string]interface{}) (llm.ToolDefinition, error) {
	name := strings.TrimSpace(stringValue(function["name"]))
	if name == "" {
		return llm.ToolDefinition{}, fmt.Errorf("%w: function tool name is required", ErrInvalidRequest)
	}
	schema, err := marshalRawJSON(function["parameters"])
	if err != nil {
		return llm.ToolDefinition{}, fmt.Errorf("%w: function parameters must be JSON", ErrInvalidRequest)
	}
	return llm.ToolDefinition{
		Name:        name,
		Description: strings.TrimSpace(stringValue(function["description"])),
		InputSchema: schema,
	}, nil
}

func legacyFunctionCallToolChoice(raw interface{}) interface{} {
	if raw == nil {
		return nil
	}
	value := strings.TrimSpace(stringValue(raw))
	switch value {
	case "auto", "none":
		return value
	}
	payload := asMap(raw)
	name := strings.TrimSpace(stringValue(payload["name"]))
	if name == "" {
		return nil
	}
	return map[string]interface{}{
		"type": "function",
		"function": map[string]interface{}{
			"name": name,
		},
	}
}

func marshalRawJSON(raw interface{}) (json.RawMessage, error) {
	if raw == nil {
		return nil, nil
	}
	if text := strings.TrimSpace(stringValue(raw)); text != "" {
		return json.RawMessage(text), nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func normalizeOpenAPIJSONString(raw interface{}) string {
	if raw == nil {
		return ""
	}
	if text := strings.TrimSpace(stringValue(raw)); text != "" {
		return text
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return ""
	}
	return string(data)
}

func chatToolContentString(raw interface{}) string {
	if text := strings.TrimSpace(stringValue(raw)); text != "" {
		return text
	}
	if raw == nil {
		return "{}"
	}
	return normalizeOpenAPIJSONString(raw)
}

func parseChatCompletionContentParts(ctx context.Context, role string, rawParts []interface{}, resolver chatImageResolver) ([]llm.ContentPart, error) {
	parts := make([]llm.ContentPart, 0, len(rawParts))
	for _, rawPart := range rawParts {
		part, ok := rawPart.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: content part must be an object", ErrInvalidRequest)
		}
		partType := strings.ToLower(strings.TrimSpace(stringValue(part["type"])))
		switch partType {
		case "text", "input_text":
			text := stringValue(part["text"])
			if strings.TrimSpace(text) != "" {
				parts = append(parts, llm.ContentPart{Kind: llm.ContentPartText, Text: text})
			}
		case "image_url", "input_image":
			if strings.TrimSpace(role) != "user" {
				return nil, fmt.Errorf("%w: images are only supported in user messages", ErrInvalidRequest)
			}
			imageURL := chatImageURLString(part["image_url"])
			if imageURL == "" {
				imageURL = stringValue(part["image_url"])
			}
			if imageURL == "" {
				imageURL = stringValue(part["url"])
			}
			imagePart, err := parseChatImagePart(ctx, imageURL, resolver)
			if err != nil {
				return nil, err
			}
			parts = append(parts, imagePart)
		default:
			return nil, fmt.Errorf("%w: unsupported content part type", ErrInvalidRequest)
		}
	}
	return parts, nil
}

func chatImageURLString(raw interface{}) string {
	switch typed := raw.(type) {
	case string:
		return strings.TrimSpace(typed)
	case map[string]interface{}:
		return strings.TrimSpace(stringValue(typed["url"]))
	default:
		return ""
	}
}

func parseChatImagePart(ctx context.Context, rawURL string, resolver chatImageResolver) (llm.ContentPart, error) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return llm.ContentPart{}, fmt.Errorf("%w: image_url is required", ErrInvalidRequest)
	}
	if strings.HasPrefix(strings.ToLower(value), "data:") {
		return parseDataURLImagePart(value)
	}
	if resolver == nil {
		return llm.ContentPart{}, fmt.Errorf("%w: remote image_url resolver is unavailable", ErrInvalidRequest)
	}
	return resolver.ResolveChatImage(ctx, value)
}

func parseDataURLImagePart(value string) (llm.ContentPart, error) {
	header, encoded, ok := strings.Cut(value, ",")
	if !ok {
		return llm.ContentPart{}, fmt.Errorf("%w: invalid image data URL", ErrInvalidRequest)
	}
	header = strings.TrimSpace(header)
	if !strings.Contains(strings.ToLower(header), ";base64") {
		return llm.ContentPart{}, fmt.Errorf("%w: image data URL must be base64", ErrInvalidRequest)
	}
	mimeType := strings.TrimPrefix(strings.Split(header, ";")[0], "data:")
	mimeType = normalizeImageMIME(mimeType)
	if mimeType == "" {
		return llm.ContentPart{}, fmt.Errorf("%w: unsupported image MIME", ErrInvalidRequest)
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		if raw, rawErr := base64.RawStdEncoding.DecodeString(strings.TrimSpace(encoded)); rawErr == nil {
			data = raw
		} else {
			return llm.ContentPart{}, err
		}
	}
	if len(data) == 0 {
		return llm.ContentPart{}, fmt.Errorf("%w: image data URL is empty", ErrInvalidRequest)
	}
	return llm.ContentPart{Kind: llm.ContentPartImage, MimeType: mimeType, Data: data}, nil
}

func normalizeImageMIME(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if parsed, _, err := mime.ParseMediaType(value); err == nil {
		value = parsed
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if _, ok := allowedOpenAPIImageMIMEs[value]; !ok {
		return ""
	}
	return value
}
