package openapi

import (
	"context"
	"encoding/base64"
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
	env      string
	ssrf     bool
	maxBytes int64
}

// NewHTTPChatImageResolver creates a safe resolver for OpenAI-compatible image_url inputs.
func NewHTTPChatImageResolver(env string, ssrfProtectionEnabled bool, maxBytes int64) *HTTPChatImageResolver {
	if maxBytes <= 0 {
		maxBytes = defaultOpenAPIImageMaxBytes
	}
	return &HTTPChatImageResolver{
		client:   security.NewOutboundHTTPClient(env, ssrfProtectionEnabled, 60*time.Second),
		env:      strings.TrimSpace(env),
		ssrf:     ssrfProtectionEnabled,
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
	if err := security.ValidateOutboundHTTPURL(imageURL, r.env, r.ssrf); err != nil {
		return llm.ContentPart{}, fmt.Errorf("%w: unsafe image_url", ErrInvalidRequest)
	}
	client := r.client
	if client == nil {
		client = security.NewOutboundHTTPClient(r.env, r.ssrf, 60*time.Second)
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
	options := cloneMap(request)
	for _, key := range []string{"model", "messages", "stream"} {
		delete(options, key)
	}
	normalizeChatMaxTokens(options)
	return llm.GenerateInput{
		Messages: messages,
		Options:  options,
	}, nil
}

func normalizeChatMaxTokens(options map[string]interface{}) {
	if len(options) == 0 {
		return
	}
	if _, ok := options["max_output_tokens"]; ok {
		return
	}
	if value, ok := options["max_completion_tokens"]; ok {
		options["max_output_tokens"] = value
		return
	}
	if value, ok := options["max_tokens"]; ok {
		options["max_output_tokens"] = value
	}
}

func parseChatCompletionMessages(ctx context.Context, raw interface{}, resolver chatImageResolver) ([]llm.Message, error) {
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil, fmt.Errorf("%w: messages must be a non-empty array", ErrInvalidRequest)
	}
	messages := make([]llm.Message, 0, len(items))
	for _, rawItem := range items {
		item, ok := rawItem.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%w: message must be an object", ErrInvalidRequest)
		}
		role := strings.TrimSpace(stringValue(item["role"]))
		if role == "" {
			role = "user"
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
		messages = append(messages, message)
	}
	return messages, nil
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
