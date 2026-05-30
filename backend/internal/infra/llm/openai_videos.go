package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	openAIVideoDefaultSize          = "1280x720"
	openAIVideoDefaultSeconds       = "4"
	openAIVideoPollInterval         = 2 * time.Second
	openAIMaxGeneratedVideoBodySize = 512 * 1024 * 1024
)

// openAIVideoGenerationsAdapter 负责 OpenAI Videos API 的视频生成端点。
type openAIVideoGenerationsAdapter struct {
	client *Client
}

func (a *openAIVideoGenerationsAdapter) Name() string { return AdapterOpenAIVideoGenerations }

// Generate 调用 OpenAI 视频生成接口，创建任务、轮询完成并下载 MP4。
func (a *openAIVideoGenerationsAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointVideoGenerations
	return a.client.generateOpenAIVideoGeneration(ctx, route, input, nil)
}

// GenerateStream 通过轮询 OpenAI Videos API 状态端点，把任务进度转成应用层流式事件。
func (a *openAIVideoGenerationsAdapter) GenerateStream(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	route.Endpoint = EndpointVideoGenerations
	return a.client.generateOpenAIVideoGeneration(ctx, route, input, onEvent)
}

// ListModels 复用 OpenAI 兼容 models 目录，供渠道校验和展示使用。
func (a *openAIVideoGenerationsAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

type openAIVideoJob struct {
	ID       string
	Status   string
	Progress *int
	Size     string
	Seconds  string
	Error    string
	RawJSON  string
}

func (c *Client) generateOpenAIVideoGeneration(
	ctx context.Context,
	route RouteConfig,
	input GenerateInput,
	onEvent func(GenerateStreamEvent) error,
) (*GenerateOutput, error) {
	requestURL := buildOpenAIRequestURL(route.BaseURL, EndpointVideoGenerations)
	if requestURL == "" {
		return nil, fmt.Errorf("invalid base url")
	}
	body, contentType, debugBody, err := buildOpenAIVideoGenerationMultipartRequest(route.UpstreamModel, input)
	if err != nil {
		return nil, err
	}

	requestCtx, cancel := context.WithTimeout(ctx, resolveReadTimeout(route.ReadTimeoutMS))
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, requestURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	setOpenAIAuthorizationAndHeaders(req, route)

	resp, err := c.httpClientForRoute(route).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck

	responseBody, err := readUpstreamBody(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, parseUpstreamError(resp.StatusCode, responseBody, upstreamDebugSnapshot(req, debugBody, resp, responseBody))
	}
	job, err := parseOpenAIVideoJob(responseBody)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(job.ID) == "" {
		return nil, fmt.Errorf("upstream returned video job without id")
	}
	if err = emitOpenAIVideoStatus(onEvent, job); err != nil {
		return nil, err
	}

	job, err = c.pollOpenAIVideoJob(requestCtx, route, job, onEvent)
	if err != nil {
		return nil, err
	}
	data, mimeType, err := c.downloadOpenAIVideoContent(requestCtx, route, job.ID)
	if err != nil {
		return nil, err
	}

	return &GenerateOutput{
		ResponseID: job.ID,
		GeneratedVideos: []GeneratedVideo{{
			ID:       job.ID,
			Data:     data,
			MIMEType: mimeType,
			Size:     job.Size,
			Seconds:  job.Seconds,
		}},
		RawJSON: job.RawJSON,
	}, nil
}

func buildOpenAIVideoGenerationMultipartRequest(model string, input GenerateInput) ([]byte, string, []byte, error) {
	prompt := buildOpenAIImageGenerationPrompt(input.Messages)
	if strings.TrimSpace(prompt) == "" {
		return nil, "", nil, fmt.Errorf("video prompt required")
	}
	formFields := map[string]string{
		"model":   strings.TrimSpace(model),
		"prompt":  strings.TrimSpace(prompt),
		"size":    resolveOpenAIVideoSize(model, modelParamString(input.Options, "size")),
		"seconds": resolveOpenAIVideoSeconds(modelParamString(input.Options, "seconds")),
	}

	images := collectImageInputParts(input.Messages)
	if len(images) > 1 {
		return nil, "", nil, fmt.Errorf("video generation accepts at most one input reference image")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	for key, value := range formFields {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if err := writer.WriteField(key, value); err != nil {
			return nil, "", nil, err
		}
	}
	if len(images) == 1 {
		image := images[0]
		fileName := strings.TrimSpace(image.FileName)
		if fileName == "" {
			fileName = "input-reference" + openAIImageFileExtension(image.MimeType)
		}
		if err := writeOpenAIMultipartFile(writer, "input_reference", fileName, image.MimeType, image.Data); err != nil {
			return nil, "", nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", nil, err
	}
	return body.Bytes(), writer.FormDataContentType(), buildOpenAIVideoGenerationDebugBody(formFields, len(images) == 1), nil
}

func resolveOpenAIVideoSeconds(raw string) string {
	switch strings.TrimSpace(raw) {
	case "4", "8", "12":
		return strings.TrimSpace(raw)
	default:
		return openAIVideoDefaultSeconds
	}
}

func resolveOpenAIVideoSize(model string, raw string) string {
	value := strings.TrimSpace(raw)
	for _, allowed := range openAIVideoSizeValues(model) {
		if value == allowed {
			return value
		}
	}
	return openAIVideoDefaultSize
}

func openAIVideoSizeValues(model string) []string {
	if strings.Contains(strings.ToLower(strings.TrimSpace(model)), "sora-2-pro") {
		return []string{"720x1280", "1280x720", "1024x1792", "1792x1024", "1080x1920", "1920x1080"}
	}
	return []string{"720x1280", "1280x720"}
}

func buildOpenAIVideoGenerationDebugBody(fields map[string]string, hasInputReference bool) []byte {
	payload := make(map[string]interface{}, len(fields)+2)
	for key, value := range fields {
		payload[key] = value
	}
	payload["input_reference"] = hasInputReference
	raw, err := json.Marshal(payload)
	if err != nil {
		return []byte(`{"multipart":true}`)
	}
	return raw
}

func (c *Client) pollOpenAIVideoJob(
	ctx context.Context,
	route RouteConfig,
	initial openAIVideoJob,
	onEvent func(GenerateStreamEvent) error,
) (openAIVideoJob, error) {
	job := initial
	for {
		switch strings.TrimSpace(strings.ToLower(job.Status)) {
		case "completed":
			return job, nil
		case "failed", "cancelled", "canceled", "expired":
			if strings.TrimSpace(job.Error) != "" {
				return job, fmt.Errorf("video generation %s: %s", job.Status, job.Error)
			}
			return job, fmt.Errorf("video generation %s", job.Status)
		}

		timer := time.NewTimer(openAIVideoPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return job, ctx.Err()
		case <-timer.C:
		}

		next, err := c.retrieveOpenAIVideoJob(ctx, route, job.ID)
		if err != nil {
			return job, err
		}
		if strings.TrimSpace(next.Size) == "" {
			next.Size = job.Size
		}
		if strings.TrimSpace(next.Seconds) == "" {
			next.Seconds = job.Seconds
		}
		if next.Progress == nil {
			next.Progress = job.Progress
		}
		job = next
		if err := emitOpenAIVideoStatus(onEvent, job); err != nil {
			return job, err
		}
	}
}

func (c *Client) retrieveOpenAIVideoJob(ctx context.Context, route RouteConfig, videoID string) (openAIVideoJob, error) {
	requestURL := buildOpenAIVideoResourceURL(route.BaseURL, videoID)
	if requestURL == "" {
		return openAIVideoJob{}, fmt.Errorf("invalid base url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return openAIVideoJob{}, err
	}
	setOpenAIAuthorizationAndHeaders(req, route)
	resp, err := c.httpClientForRoute(route).Do(req)
	if err != nil {
		return openAIVideoJob{}, err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := readUpstreamBody(resp.Body)
	if err != nil {
		return openAIVideoJob{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return openAIVideoJob{}, parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
	}
	return parseOpenAIVideoJob(body)
}

func (c *Client) downloadOpenAIVideoContent(ctx context.Context, route RouteConfig, videoID string) ([]byte, string, error) {
	baseURL := buildOpenAIVideoResourceURL(route.BaseURL, videoID)
	if baseURL == "" {
		return nil, "", fmt.Errorf("invalid base url")
	}
	requestURL := baseURL + "/content"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, "", err
	}
	setOpenAIAuthorizationAndHeaders(req, route)
	resp, err := c.httpClientForRoute(route).Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close() //nolint:errcheck
	body, err := readLimitedUpstreamBody(resp.Body, openAIMaxGeneratedVideoBodySize)
	if err != nil {
		return nil, "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", parseUpstreamError(resp.StatusCode, body, upstreamDebugSnapshot(req, nil, resp, body))
	}
	mimeType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if strings.Contains(mimeType, ";") {
		mimeType = strings.TrimSpace(strings.Split(mimeType, ";")[0])
	}
	if mimeType == "" || mimeType == "application/octet-stream" {
		mimeType = "video/mp4"
	}
	if !strings.HasPrefix(strings.ToLower(mimeType), "video/") {
		return nil, "", fmt.Errorf("video content has unexpected content type %q", mimeType)
	}
	return body, mimeType, nil
}

func parseOpenAIVideoJob(body []byte) (openAIVideoJob, error) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return openAIVideoJob{}, err
	}
	job := openAIVideoJob{
		ID:       strings.TrimSpace(getString(parsed["id"])),
		Status:   strings.TrimSpace(getString(parsed["status"])),
		Progress: getOptionalInt(parsed["progress"]),
		Size:     strings.TrimSpace(getString(parsed["size"])),
		Seconds:  strings.TrimSpace(getString(parsed["seconds"])),
		RawJSON:  string(body),
	}
	if errorPayload := asMap(parsed["error"]); len(errorPayload) > 0 {
		job.Error = firstNonEmptyString(getString(errorPayload["message"]), getString(errorPayload["code"]), getString(errorPayload["type"]))
	} else {
		job.Error = getString(parsed["error"])
	}
	return job, nil
}

func emitOpenAIVideoStatus(onEvent func(GenerateStreamEvent) error, job openAIVideoJob) error {
	if onEvent == nil {
		return nil
	}
	return onEvent(GenerateStreamEvent{
		ResponseID: strings.TrimSpace(job.ID),
		GeneratedVideoStatus: &GeneratedVideoStatus{
			ID:       strings.TrimSpace(job.ID),
			Status:   strings.TrimSpace(job.Status),
			Progress: job.Progress,
			Size:     strings.TrimSpace(job.Size),
			Seconds:  strings.TrimSpace(job.Seconds),
		},
	})
}

func getOptionalInt(raw interface{}) *int {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(getString(raw))
	if text == "" {
		return nil
	}
	value, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return nil
	}
	parsed := int(value)
	return &parsed
}

func buildOpenAIVideoResourceURL(baseURL string, videoID string) string {
	id := strings.TrimSpace(videoID)
	if id == "" {
		return ""
	}
	return buildVersionedEndpointURL(baseURL, "v1", "/videos/"+url.PathEscape(id))
}

func setOpenAIAuthorizationAndHeaders(req *http.Request, route RouteConfig) {
	if apiKey := strings.TrimSpace(route.APIKey); apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	setOpenRouterAttributionHeaders(req, route)
	setAdditionalHeaders(req, route.HeadersJSON)
}

func readLimitedUpstreamBody(reader io.Reader, limit int64) ([]byte, error) {
	if limit <= 0 {
		return readUpstreamBody(reader)
	}
	limited := io.LimitReader(reader, limit+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, errors.New("upstream response too large")
	}
	return body, nil
}
