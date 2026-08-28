package llm

import (
	"context"
	"encoding/base64"
	"sort"
	"strings"
)

// openAIResponsesAdapter 瀹炵幇 OpenAI Responses API锛圥OST /v1/responses锛夈€?
type openAIResponsesAdapter struct {
	client *Client
}

func (a *openAIResponsesAdapter) Name() string { return AdapterOpenAIResponses }

func (a *openAIResponsesAdapter) Generate(ctx context.Context, route RouteConfig, input GenerateInput) (*GenerateOutput, error) {
	route.Endpoint = EndpointResponses
	return a.client.generateOpenAICompatible(ctx, route, input)
}

func (a *openAIResponsesAdapter) GenerateStream(ctx context.Context, route RouteConfig, input GenerateInput, onEvent func(GenerateStreamEvent) error) (*GenerateOutput, error) {
	route.Endpoint = EndpointResponses
	return a.client.generateStreamOpenAICompatible(ctx, route, input, onEvent)
}

func (a *openAIResponsesAdapter) ListModels(ctx context.Context, route RouteConfig) ([]ModelItem, error) {
	return a.client.listModelsOpenAICompatible(ctx, route)
}

func buildResponsesRequestBody(
	adapter string,
	model string,
	input GenerateInput,
	messages []Message,
	providerTools []map[string]interface{},
	toolDefinitions []ToolDefinition,
	toolsEnabled bool,
	providerStreamOptions map[string]interface{},
	stream bool,
) map[string]interface{} {
	if adapter == AdapterOpenRouterResponses {
		return buildOpenRouterResponsesRequestBody(model, input, messages, providerTools, toolDefinitions, providerStreamOptions, stream)
	}
	promptCache := resolveOpenAIPromptCacheConfig(adapter, input)
	items := buildResponsesAPIInput(messages, &promptCache)
	payload := map[string]interface{}{
		"model":  strings.TrimSpace(model),
		"input":  items,
		"stream": stream,
	}
	if adapter == AdapterOpenAIResponses && input.Ephemeral {
		payload["store"] = false
	} else if adapter == AdapterOpenAIResponses && input.ResponsesBackground {
		payload["background"] = true
		payload["store"] = true
	}
	if instructions := strings.TrimSpace(input.Instructions); adapter == AdapterOpenAIResponses && instructions != "" {
		payload["instructions"] = instructions
	}
	if maxTokens := modelParamInt(input.Options, "max_output_tokens"); maxTokens > 0 {
		payload["max_output_tokens"] = maxTokens
	}
	applyOpenAICompatibleSamplingParams(payload, input.Options, false)
	applyOpenAIResponsesReasoningParams(payload, input.Options, adapter == AdapterOpenAIResponses)
	applyOpenAIResponsesTextParams(payload, input.Options, adapter == AdapterOpenAIResponses)
	webSearchTools := []map[string]interface{}{}
	if toolsEnabled && modelParamBool(input.Options, "web_search") && adapter == AdapterOpenAIResponses {
		webSearchTools = append(webSearchTools, map[string]interface{}{"type": "web_search"})
	}
	nativeTools := append([]map[string]interface{}{}, providerTools...)
	nativeTools = append(nativeTools, webSearchTools...)
	applyOpenAIPromptCacheRequestFields(payload, promptCache)
	appendToolDeclarations(payload, providerTools, webSearchTools, buildOpenAITools(toolDefinitions, false))
	// 鏈夌姸鎬佷細璇濓細鎻愪緵 previous_response_id 鏃舵湇鍔＄缁帴瀛樺偍鐨勫巻鍙诧紝
	// input 浠呭寘鍚湰杞柊娑堟伅锛岄伩鍏嶅叏閲忛噸浼犮€?
	if prevID := strings.TrimSpace(input.PreviousResponseID); !input.Ephemeral && prevID != "" {
		payload["previous_response_id"] = prevID
	}
	if streamOptions := responsesStreamOptions(providerStreamOptions); stream && len(streamOptions) > 0 {
		payload["stream_options"] = streamOptions
	}
	applyProviderOptions(payload, input.Options, responsesProtectedProviderOptionKeys(adapter, strings.TrimSpace(input.Instructions) != "")...)
	if supportsResponsesIncludeDefaults(adapter) {
		defaultIncludes := responsesDefaultIncludeValues(adapter, stream, nativeTools)
		appendResponseInclude(payload, responseIncludeValues(input.Options, defaultIncludes...)...)
	} else {
		appendResponseInclude(payload, responseIncludeValues(input.Options)...)
	}
	return payload
}

func responsesProtectedProviderOptionKeys(adapter string, hasManagedInstructions bool) []string {
	keys := []string{
		"contents",
		"include",
		"input",
		"messages",
		"model",
		"prompt_cache_key",
		"prompt_cache_options",
		"prompt_cache_retention",
		"previous_response_id",
		"reasoning",
		"response_format",
		"stream",
		"stream_options",
		"system",
		"systemInstruction",
		"text",
		"tools",
	}
	if adapter == AdapterOpenAIResponses {
		keys = append(keys, "background", "store")
	}
	if adapter != AdapterOpenAIResponses {
		keys = append(keys, "instructions", "metadata", "prompt")
	} else if hasManagedInstructions {
		keys = append(keys, "instructions")
	}
	return keys
}

func responsesStreamOptions(options map[string]interface{}) map[string]interface{} {
	if len(options) == 0 {
		return nil
	}
	result := map[string]interface{}{}
	if value, ok := options["include_obfuscation"]; ok {
		result["include_obfuscation"] = value
	}
	return result
}

func applyOpenAIResponsesReasoningParams(payload map[string]interface{}, options map[string]interface{}, defaultSummaryAuto bool) {
	reasoning := map[string]interface{}{}
	if existing := modelParamMap(options, "reasoning"); len(existing) > 0 {
		for key, value := range existing {
			reasoning[key] = value
		}
	}
	if effort := modelParamString(options, "reasoning_effort"); effort != "" {
		reasoning["effort"] = effort
	}
	if mode, ok := normalizedResponsesReasoningMode(reasoning["mode"]); ok {
		reasoning["mode"] = mode
	} else {
		delete(reasoning, "mode")
	}
	summaryConfigured := false
	if summary, ok := normalizedResponsesReasoningSummary(reasoning["summary"]); ok {
		summaryConfigured = true
		if responsesReasoningSummaryDisabled(summary) {
			delete(reasoning, "summary")
		} else {
			reasoning["summary"] = summary
		}
	} else {
		delete(reasoning, "summary")
	}
	if summary := modelParamString(options, "reasoning_summary"); summary != "" {
		summaryConfigured = true
		if responsesReasoningSummaryDisabled(summary) {
			delete(reasoning, "summary")
		} else {
			reasoning["summary"] = summary
		}
	} else if defaultSummaryAuto && !summaryConfigured {
		reasoning["summary"] = "auto"
	}
	mergeObjectParam(payload, "reasoning", reasoning)
}

func normalizedResponsesReasoningMode(raw interface{}) (string, bool) {
	value := strings.ToLower(strings.TrimSpace(getString(raw)))
	return value, value == "pro"
}

func normalizedResponsesReasoningSummary(raw interface{}) (string, bool) {
	value, ok := raw.(string)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return value, value != ""
}

func responsesReasoningSummaryDisabled(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "none")
}

func applyOpenAIResponsesTextParams(payload map[string]interface{}, options map[string]interface{}, allowVerbosity bool) {
	text := map[string]interface{}{}
	if existing := modelParamMap(options, "text"); len(existing) > 0 {
		for key, value := range existing {
			text[key] = value
		}
	}
	if format, ok := normalizedJSONResponseFormat(options); ok {
		text["format"] = format
	}
	if verbosity := modelParamString(options, "verbosity"); verbosity != "" && allowVerbosity {
		text["verbosity"] = verbosity
	}
	mergeObjectParam(payload, "text", text)
}

type responsesProtocolExtension struct {
	matchesAdapter                  func(adapter string) bool
	includeDefaults                 func(stream bool, tools []map[string]interface{}) []string
	serverToolIdentifierKeys        func() []string
	serverToolCallID                func(item map[string]interface{}, itemType string) (string, bool)
	isServerToolCallItem            func(item map[string]interface{}) bool
	isServerToolCallType            func(itemType string) bool
	normalizeServerSideToolUsageKey func(value string, original string) (string, bool)
}

func supportsResponsesIncludeDefaults(adapter string) bool {
	return adapter == AdapterOpenAIResponses || len(responsesProtocolExtensionsForAdapter(adapter)) > 0
}

func responsesDefaultIncludeValues(adapter string, stream bool, providerTools []map[string]interface{}) []string {
	values := []string{"reasoning.encrypted_content"}
	if adapter == AdapterOpenAIResponses {
		values = append(values, openAIResponsesDefaultIncludeValues(providerTools)...)
	}
	for _, extension := range responsesProtocolExtensionsForAdapter(adapter) {
		if extension.includeDefaults != nil {
			values = append(values, extension.includeDefaults(stream, providerTools)...)
		}
	}
	return appendUniqueStrings(nil, values...)
}

func openAIResponsesDefaultIncludeValues(tools []map[string]interface{}) []string {
	if !responsesToolsIncludeType(tools, "web_search") {
		return nil
	}
	return []string{"web_search_call.action.sources"}
}

func responsesToolsIncludeType(tools []map[string]interface{}, toolType string) bool {
	expected := strings.TrimSpace(toolType)
	if expected == "" {
		return false
	}
	for _, tool := range tools {
		if strings.TrimSpace(getString(tool["type"])) == expected {
			return true
		}
	}
	return false
}

func buildResponsesAPIInput(messages []Message, promptCache *openAIPromptCacheConfig) []map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(messages))
	for _, msg := range messages {
		if len(msg.ToolCalls) > 0 {
			for _, item := range msg.ToolCalls {
				args := strings.TrimSpace(item.ArgumentsJSON)
				if args == "" {
					args = "{}"
				}
				items = append(items, map[string]interface{}{
					"type":      "function_call",
					"call_id":   strings.TrimSpace(item.ToolCallID),
					"name":      strings.TrimSpace(item.ToolName),
					"arguments": args,
				})
			}
			continue
		}
		if len(msg.ToolResults) > 0 {
			for _, item := range msg.ToolResults {
				items = append(items, map[string]interface{}{
					"type":    "function_call_output",
					"call_id": strings.TrimSpace(item.ToolCallID),
					"output":  buildToolResultContent(item),
				})
			}
			continue
		}
		items = append(items, map[string]interface{}{
			"role":    normalizeRole(msg.Role),
			"content": buildResponsesAPIContent(msg, promptCache),
		})
	}
	return items
}

// buildResponsesAPIContent 将消息内容序列化为 Responses API 格式（content 数组）。
func buildResponsesAPIContent(msg Message, promptCache *openAIPromptCacheConfig) []map[string]interface{} {

	textType := responsesTextContentType(msg.Role)
	if len(msg.Parts) == 0 {
		block := map[string]interface{}{"type": textType, "text": msg.Content}
		appendOpenAIPromptCacheBreakpoint(block, msg.CacheControl, promptCache)
		return []map[string]interface{}{block}
	}
	parts := make([]map[string]interface{}, 0, len(msg.Parts))
	for _, part := range msg.Parts {
		switch part.Kind {
		case ContentPartImage:
			if normalizeRole(msg.Role) == "assistant" {
				continue
			}
			if len(part.Data) == 0 {
				continue
			}
			mime := strings.TrimSpace(part.MimeType)
			if mime == "" {
				mime = "image/jpeg"
			}
			b64 := base64.StdEncoding.EncodeToString(part.Data)
			block := map[string]interface{}{
				"type":      "input_image",
				"image_url": "data:" + mime + ";base64," + b64,
			}
			appendOpenAIPromptCacheBreakpoint(block, part.CacheControl, promptCache)
			parts = append(parts, block)
		default: // text, file
			text := part.Text
			if strings.TrimSpace(text) == "" {
				continue
			}
			block := map[string]interface{}{
				"type": textType,
				"text": text,
			}
			appendOpenAIPromptCacheBreakpoint(block, part.CacheControl, promptCache)
			parts = append(parts, block)
		}
	}
	if len(parts) == 0 {
		block := map[string]interface{}{"type": textType, "text": msg.Content}
		appendOpenAIPromptCacheBreakpoint(block, msg.CacheControl, promptCache)
		return []map[string]interface{}{block}
	}
	if msg.CacheControl != nil {
		for index := len(parts) - 1; index >= 0; index-- {
			if appendOpenAIPromptCacheBreakpoint(parts[index], msg.CacheControl, promptCache) {
				break
			}
		}
	}
	return parts
}

func responsesTextContentType(role string) string {
	if normalizeRole(role) == "assistant" {
		return "output_text"
	}
	return "input_text"
}

const responsesReasoningPartSeparator = "\n\n"

// responsesReasoningStreamState keeps upstream deltas append-only for callbacks while
// retaining authoritative done snapshots for the final GenerateOutput.
type responsesReasoningStreamPart struct {
	itemID         string
	outputIndex    int64
	hasOutputIndex bool
	partIndex      int64
	hasPartIndex   bool
	order          int
	kind           string
	text           string
	seeded         bool
	sawDelta       bool
	emitted        bool
	diverged       bool // The emitted callback text no longer prefixes the authoritative text.
	finalized      bool
}

type responsesReasoningStreamState struct {
	parts            []*responsesReasoningStreamPart
	itemID           string
	status           string
	signature        string
	encryptedContent string
}

func newResponsesReasoningStreamState() *responsesReasoningStreamState {
	return &responsesReasoningStreamState{
		parts: make([]*responsesReasoningStreamPart, 0),
	}
}

func responsesReasoningStateFor(result *GenerateOutput) *responsesReasoningStreamState {
	if result == nil {
		return nil
	}
	if state, ok := result.StreamState.(*responsesReasoningStreamState); ok && state != nil {
		return state
	}
	state := newResponsesReasoningStreamState()
	result.StreamState = state
	return state
}

func (s *responsesReasoningStreamState) partFor(
	kind string,
	itemID string,
	outputIndex int64,
	hasOutputIndex bool,
	partIndex int64,
	hasPartIndex bool,
	startNew bool,
) *responsesReasoningStreamPart {
	if s == nil {
		return nil
	}
	itemID = strings.TrimSpace(itemID)
	if hasPartIndex && !(startNew && itemID == "" && !hasOutputIndex) {
		if existing := s.bestMatchingPart(kind, itemID, outputIndex, hasOutputIndex, partIndex, true, true); existing != nil {
			existing.updateCoordinates(itemID, outputIndex, hasOutputIndex, partIndex, true)
			return existing
		}
	}
	if !hasPartIndex && !startNew {
		if existing := s.bestMatchingPart(kind, itemID, outputIndex, hasOutputIndex, partIndex, false, false); existing != nil {
			existing.updateCoordinates(itemID, outputIndex, hasOutputIndex, partIndex, false)
			return existing
		}
	}
	part := &responsesReasoningStreamPart{
		itemID:         itemID,
		outputIndex:    outputIndex,
		hasOutputIndex: hasOutputIndex,
		partIndex:      partIndex,
		hasPartIndex:   hasPartIndex,
		order:          len(s.parts),
		kind:           kind,
	}
	s.parts = append(s.parts, part)
	return part
}

func (s *responsesReasoningStreamState) bestMatchingPart(
	kind string,
	itemID string,
	outputIndex int64,
	hasOutputIndex bool,
	partIndex int64,
	hasPartIndex bool,
	allowUnindexed bool,
) *responsesReasoningStreamPart {
	if s == nil {
		return nil
	}
	var best *responsesReasoningStreamPart
	bestScore := -1
	for _, part := range s.parts {
		if !responsesReasoningCoordinatesCompatible(part, kind, itemID, outputIndex, hasOutputIndex, partIndex, hasPartIndex) {
			continue
		}
		if hasPartIndex && !part.hasPartIndex && !allowUnindexed {
			continue
		}
		score := 0
		if itemID != "" && part.itemID == itemID {
			score += 4
		}
		if hasOutputIndex && part.hasOutputIndex && part.outputIndex == outputIndex {
			score += 2
		}
		if hasPartIndex && part.hasPartIndex && part.partIndex == partIndex {
			score += 8
		}
		if !part.finalized {
			score++
		}
		if score > bestScore || (score == bestScore && (best == nil || part.order > best.order)) {
			best = part
			bestScore = score
		}
	}
	return best
}

func responsesReasoningCoordinatesCompatible(
	part *responsesReasoningStreamPart,
	kind string,
	itemID string,
	outputIndex int64,
	hasOutputIndex bool,
	partIndex int64,
	hasPartIndex bool,
) bool {
	if part == nil || part.kind != kind {
		return false
	}
	if itemID != "" && part.itemID != "" && part.itemID != itemID {
		return false
	}
	if hasOutputIndex && part.hasOutputIndex && part.outputIndex != outputIndex {
		return false
	}
	return !hasPartIndex || !part.hasPartIndex || part.partIndex == partIndex
}

func (p *responsesReasoningStreamPart) updateCoordinates(itemID string, outputIndex int64, hasOutputIndex bool, partIndex int64, hasPartIndex bool) {
	if p == nil {
		return
	}
	if value := strings.TrimSpace(itemID); value != "" {
		p.itemID = value
	}
	if hasOutputIndex {
		p.outputIndex = outputIndex
		p.hasOutputIndex = true
	}
	if hasPartIndex {
		p.partIndex = partIndex
		p.hasPartIndex = true
	}
}

func (s *responsesReasoningStreamState) orderedParts(kind string) []*responsesReasoningStreamPart {
	if s == nil {
		return nil
	}
	parts := make([]*responsesReasoningStreamPart, 0, len(s.parts))
	for _, part := range s.parts {
		if part != nil && part.kind == kind {
			parts = append(parts, part)
		}
	}
	sort.SliceStable(parts, func(left int, right int) bool {
		a := parts[left]
		b := parts[right]
		if a.hasOutputIndex && b.hasOutputIndex && a.outputIndex != b.outputIndex {
			return a.outputIndex < b.outputIndex
		}
		if responsesReasoningSameItem(a, b) && a.hasPartIndex && b.hasPartIndex && a.partIndex != b.partIndex {
			return a.partIndex < b.partIndex
		}
		return a.order < b.order
	})
	return parts
}

func responsesReasoningSameItem(a *responsesReasoningStreamPart, b *responsesReasoningStreamPart) bool {
	if a == nil || b == nil || a.kind != b.kind {
		return false
	}
	if a.itemID != "" && b.itemID != "" {
		return a.itemID == b.itemID
	}
	return a.hasOutputIndex && b.hasOutputIndex && a.outputIndex == b.outputIndex
}

func (s *responsesReasoningStreamState) joinedText(kind string) string {
	texts := make([]string, 0)
	for _, part := range s.orderedParts(kind) {
		if part.text != "" {
			texts = append(texts, part.text)
		}
	}
	return strings.Join(texts, responsesReasoningPartSeparator)
}

func (s *responsesReasoningStreamState) hasPriorEmittedPart(target *responsesReasoningStreamPart) bool {
	if s == nil || target == nil {
		return false
	}
	for _, part := range s.orderedParts(target.kind) {
		if part == target {
			return false
		}
		if part.emitted {
			return true
		}
	}
	return false
}

func (s *responsesReasoningStreamState) appendEmission(part *responsesReasoningStreamPart, text string) string {
	if s == nil || part == nil || text == "" {
		return ""
	}
	if !part.emitted && s.hasPriorEmittedPart(part) {
		text = responsesReasoningPartSeparator + text
	}
	part.emitted = true
	return text
}

func (s *responsesReasoningStreamState) appendDelta(part *responsesReasoningStreamPart, delta string) string {
	if s == nil || part == nil || delta == "" {
		return ""
	}
	if part.seeded && !part.sawDelta {
		part.text = ""
		part.seeded = false
	}
	part.text += delta
	part.sawDelta = true
	if part.diverged {
		return ""
	}
	return s.appendEmission(part, delta)
}

func (s *responsesReasoningStreamState) reconcileSnapshot(part *responsesReasoningStreamPart, text string, finalized bool) string {
	if s == nil || part == nil {
		return ""
	}
	previous := part.text
	part.seeded = false
	part.finalized = part.finalized || finalized
	if previous == text {
		if !part.emitted && text != "" {
			return s.appendEmission(part, text)
		}
		return ""
	}
	part.text = text
	if !part.emitted {
		return s.appendEmission(part, text)
	}
	if part.diverged {
		return ""
	}
	if strings.HasPrefix(text, previous) {
		return s.appendEmission(part, text[len(previous):])
	}
	part.diverged = true
	return ""
}

func (s *responsesReasoningStreamState) seedSnapshot(part *responsesReasoningStreamPart, text string) {
	if s == nil || part == nil || text == "" || part.sawDelta || part.emitted {
		return
	}
	part.text = text
	part.seeded = true
}

func (s *responsesReasoningStreamState) updateMetadata(itemID string, status string, signature string, encryptedContent string) {
	if s == nil {
		return
	}
	if value := strings.TrimSpace(itemID); value != "" {
		s.itemID = value
	}
	if value := strings.TrimSpace(status); value != "" {
		s.status = value
	}
	if value := strings.TrimSpace(signature); value != "" {
		s.signature = value
	}
	if value := strings.TrimSpace(encryptedContent); value != "" {
		s.encryptedContent = value
	}
}

func (s *responsesReasoningStreamState) applyToResult(adapter string, result *GenerateOutput) {
	if s == nil || result == nil {
		return
	}
	reasoning := &ReasoningOutput{}
	if result.Reasoning != nil {
		reasoning.Signature = result.Reasoning.Signature
		reasoning.EncryptedContent = result.Reasoning.EncryptedContent
	}
	reasoning.ItemID = s.itemID
	reasoning.Status = s.status
	reasoning.Summary = s.joinedText("summary_text")
	reasoning.Text = s.joinedText("content_text")
	if s.signature != "" {
		reasoning.Signature = s.signature
	}
	if s.encryptedContent != "" {
		reasoning.EncryptedContent = s.encryptedContent
	}
	if adapter == AdapterOpenAIResponses {
		reasoning.Text = ""
	}
	if reasoningOutputEmpty(reasoning) {
		result.Reasoning = nil
		return
	}
	result.Reasoning = reasoning
}

func reasoningOutputEmpty(reasoning *ReasoningOutput) bool {
	return reasoning == nil || (strings.TrimSpace(reasoning.ItemID) == "" &&
		strings.TrimSpace(reasoning.Status) == "" &&
		reasoning.Summary == "" &&
		reasoning.Text == "" &&
		strings.TrimSpace(reasoning.Signature) == "" &&
		strings.TrimSpace(reasoning.EncryptedContent) == "")
}

func responsesEventIndex(parsed map[string]interface{}, key string) (int64, bool) {
	if parsed == nil {
		return 0, false
	}
	raw, ok := parsed[key]
	if !ok || raw == nil || strings.TrimSpace(getString(raw)) == "" {
		return 0, false
	}
	return toInt64(raw), true
}

func responsesReasoningKind(eventType string) string {
	if strings.Contains(eventType, "summary") {
		return "summary_text"
	}
	return "content_text"
}

func responsesReasoningPartForEvent(state *responsesReasoningStreamState, eventType string, parsed map[string]interface{}, startNew bool) *responsesReasoningStreamPart {
	if state == nil {
		return nil
	}
	kind := responsesReasoningKind(eventType)
	itemID := firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id"))
	outputIndex, hasOutputIndex := responsesEventIndex(parsed, "output_index")
	partKey := "content_index"
	if kind == "summary_text" {
		partKey = "summary_index"
	}
	partIndex, hasPartIndex := responsesEventIndex(parsed, partKey)
	return state.partFor(kind, itemID, outputIndex, hasOutputIndex, partIndex, hasPartIndex, startNew)
}

func applyResponsesStreamEvent(
	adapter string,
	eventName string,
	parsed map[string]interface{},
	rawBody string,
	result *GenerateOutput,
	onEvent func(GenerateStreamEvent) error,
) error {
	eventType := strings.TrimSpace(getString(parsed["type"]))
	if eventType == "" {
		eventType = strings.TrimSpace(eventName)
	}

	if call, ok := parseResponsesServerToolStatusEvent(eventType, parsed); ok {
		appendUniqueToolCall(&result.ServerToolCalls, call)
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				ServerToolCall: &call,
				ResponseID:     result.ResponseID,
			})
		}
		return nil
	}
	if isResponsesImageGenerationPartialEvent(eventType) {
		return emitResponsesImageGenerationPartial(result, parsed, onEvent)
	}

	switch eventType {
	case "response.created":
		var eventErr error
		if responseID := strings.TrimSpace(getStringFromPath(parsed, "response", "id")); responseID != "" {
			result.ResponseID = responseID
			if onEvent != nil {
				eventErr = onEvent(GenerateStreamEvent{ResponseID: responseID})
			}
		}
		if serviceTier := strings.TrimSpace(getStringFromPath(parsed, "response", "service_tier")); serviceTier != "" {
			result.Usage.ServiceTier = serviceTier
		}
		return eventErr
	case "response.output_item.added", "response.output_item.in_progress":
		return mergeResponsesStreamOutputItem(adapter, eventType, result, parsed, false, onEvent)
	case "response.output_item.done":
		return mergeResponsesStreamOutputItem(adapter, eventType, result, parsed, true, onEvent)
	case "response.custom_tool_call_input.delta", "response.custom_tool_call_input.done":
		return mergeResponsesCustomToolInputEvent(result, parsed, onEvent)
	case "response.output_text.delta":
		delta := getString(parsed["delta"])
		if delta == "" {
			return nil
		}
		result.Text += delta
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			})
		}
	case "response.output_text.done":
		text := firstNonEmptyString(getString(parsed["text"]), getString(parsed["delta"]))
		if text != "" && !strings.Contains(result.Text, text) {
			result.Text += text
		}
	case "response.refusal.delta":
		delta := getString(parsed["delta"])
		if delta == "" {
			return nil
		}
		result.Text += delta
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Delta:      delta,
				ResponseID: result.ResponseID,
			})
		}
	case "response.refusal.done":
		text := firstNonEmptyString(getString(parsed["refusal"]), getString(parsed["text"]), getString(parsed["delta"]))
		if text != "" && !strings.Contains(result.Text, text) {
			result.Text += text
		}
	case "response.reasoning_summary_part.added":
		state := responsesReasoningStateFor(result)
		responsesReasoningPartForEvent(state, eventType, parsed, true)
		itemID, _, signature, encryptedContent := responsesReasoningEventMetadata(parsed)
		state.updateMetadata(itemID, "", signature, encryptedContent)
		state.applyToResult(adapter, result)
	case "response.reasoning_summary_text.delta", "response.reasoning_text.delta", "response.thinking.delta":
		if shouldSuppressResponsesRawReasoning(adapter, eventType) {
			return nil
		}
		reasoning := parseResponsesReasoningDelta(eventType, parsed)
		if reasoning == nil || reasoning.Text == "" {
			return nil
		}
		state := responsesReasoningStateFor(result)
		part := responsesReasoningPartForEvent(state, eventType, parsed, false)
		state.updateMetadata(reasoning.ItemID, "", reasoning.Signature, reasoning.EncryptedContent)
		reasoning.Text = state.appendDelta(part, reasoning.Text)
		state.applyToResult(adapter, result)
		if reasoning.Text == "" {
			return nil
		}
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Reasoning:  reasoning,
				ResponseID: result.ResponseID,
			})
		}
	case "response.reasoning_summary_text.done", "response.reasoning_summary_part.done", "response.reasoning_text.done", "response.thinking.done":
		if shouldSuppressResponsesRawReasoning(adapter, eventType) {
			return nil
		}
		reasoning := parseResponsesReasoningDone(eventType, parsed)
		state := responsesReasoningStateFor(result)
		part := responsesReasoningPartForEvent(state, eventType, parsed, false)
		finalized := strings.HasSuffix(eventType, "_part.done") || eventType == "response.thinking.done" || eventType == "response.reasoning_text.done"
		if part != nil && finalized {
			part.finalized = true
		}
		if reasoning == nil {
			state.applyToResult(adapter, result)
			return nil
		}
		state.updateMetadata(reasoning.ItemID, "", reasoning.Signature, reasoning.EncryptedContent)
		reasoning.Text = state.reconcileSnapshot(part, reasoning.Text, finalized)
		state.applyToResult(adapter, result)
		if reasoning.Text != "" && onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Reasoning:  reasoning,
				ResponseID: result.ResponseID,
			})
		}
	case "response.completed":
		response := asMap(parsed["response"])
		state := responsesReasoningStateFor(result)
		reasoningEvents := reconcileResponsesReasoningOutput(adapter, eventType, state, response["output"])
		output := buildGenerateOutputFromParsedForAdapter(EndpointResponses, adapter, response, false)
		mergeGenerateOutput(result, output)
		state.applyToResult(adapter, result)
		if err := emitResponsesReasoningEvents(result, reasoningEvents, onEvent); err != nil {
			return err
		}
		if output.Usage != (Usage{}) && onEvent != nil {
			return onEvent(GenerateStreamEvent{
				Usage:      output.Usage,
				ResponseID: result.ResponseID,
			})
		}
	case "response.failed", "response.error":
		return parseResponsesStreamErrorEvent(parsed, rawBody)
	}

	return nil
}

func shouldSuppressResponsesRawReasoning(adapter string, eventType string) bool {
	return adapter == AdapterOpenAIResponses && strings.HasPrefix(strings.TrimSpace(eventType), "response.reasoning_text.")
}

func responsesReasoningEventMetadata(parsed map[string]interface{}) (itemID string, status string, signature string, encryptedContent string) {
	return firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id")),
		firstNonEmptyString(getString(parsed["status"]), getStringFromPath(parsed, "item", "status")),
		firstNonEmptyString(getString(parsed["signature"]), getStringFromPath(parsed, "item", "signature")),
		firstNonEmptyString(getString(parsed["encrypted_content"]), getStringFromPath(parsed, "item", "encrypted_content"))
}

func reconcileResponsesReasoningOutput(
	adapter string,
	eventType string,
	state *responsesReasoningStreamState,
	rawOutput interface{},
) []*ReasoningDelta {
	events := make([]*ReasoningDelta, 0)
	for outputIndex, raw := range asSlice(rawOutput) {
		item := asMap(raw)
		if strings.TrimSpace(getString(item["type"])) != "reasoning" {
			continue
		}
		events = append(events, reconcileResponsesReasoningItem(
			adapter,
			eventType,
			state,
			item,
			int64(outputIndex),
			true,
			true,
		)...)
	}
	return events
}

func reconcileResponsesReasoningItem(
	adapter string,
	eventType string,
	state *responsesReasoningStreamState,
	item map[string]interface{},
	outputIndex int64,
	hasOutputIndex bool,
	authoritative bool,
) []*ReasoningDelta {
	if state == nil || strings.TrimSpace(getString(item["type"])) != "reasoning" {
		return nil
	}
	itemID := strings.TrimSpace(getString(item["id"]))
	status := strings.TrimSpace(getString(item["status"]))
	signature := strings.TrimSpace(getString(item["signature"]))
	encryptedContent := strings.TrimSpace(getString(item["encrypted_content"]))
	state.updateMetadata(itemID, status, signature, encryptedContent)

	events := make([]*ReasoningDelta, 0)
	reconcileParts := func(kind string, fragments []string) {
		for partIndex, text := range fragments {
			if text == "" && !authoritative {
				continue
			}
			part := state.partFor(kind, itemID, outputIndex, hasOutputIndex, int64(partIndex), true, false)
			if !authoritative {
				state.seedSnapshot(part, text)
				continue
			}
			emitted := state.reconcileSnapshot(part, text, true)
			if emitted == "" {
				continue
			}
			events = append(events, &ReasoningDelta{
				EventType:        eventType,
				ItemID:           itemID,
				Status:           status,
				Kind:             kind,
				Text:             emitted,
				Signature:        signature,
				EncryptedContent: encryptedContent,
			})
		}
	}

	summaryParts := responsesReasoningFragments(item["summary"])
	if len(summaryParts) == 0 {
		summaryParts = responsesReasoningFragments(item["summary_text"])
	}
	reconcileParts("summary_text", summaryParts)
	if adapter != AdapterOpenAIResponses {
		contentParts := responsesReasoningFragments(item["content"])
		if len(contentParts) == 0 {
			contentParts = responsesReasoningFragments(item["text"])
		}
		reconcileParts("content_text", contentParts)
	}
	return events
}

func responsesReasoningFragments(raw interface{}) []string {
	if values, ok := raw.([]interface{}); ok {
		parts := make([]string, len(values))
		for index, value := range values {
			parts[index] = extractReasoningDeltaText(value)
		}
		return parts
	}
	if text := extractReasoningDeltaText(raw); text != "" {
		return []string{text}
	}
	return nil
}

func emitResponsesReasoningEvents(
	result *GenerateOutput,
	events []*ReasoningDelta,
	onEvent func(GenerateStreamEvent) error,
) error {
	if onEvent == nil {
		return nil
	}
	for _, reasoning := range events {
		if reasoning == nil || reasoning.Text == "" {
			continue
		}
		if err := onEvent(GenerateStreamEvent{
			Reasoning:  reasoning,
			ResponseID: result.ResponseID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func parseResponsesServerToolStatusEvent(eventType string, parsed map[string]interface{}) (ToolCall, bool) {
	value := strings.TrimSpace(eventType)
	if !strings.HasPrefix(value, "response.") {
		return ToolCall{}, false
	}
	status := ""
	for _, suffix := range []string{".in_progress", ".searching", ".generating", ".completed", ".failed", ".error"} {
		if strings.HasSuffix(value, suffix) {
			status = strings.TrimPrefix(suffix, ".")
			value = strings.TrimSuffix(strings.TrimPrefix(value, "response."), suffix)
			break
		}
	}
	if status == "" || !isResponsesServerToolCallType(value) {
		return ToolCall{}, false
	}
	item := cloneMap(asMap(parsed["item"]))
	if len(item) == 0 {
		item = make(map[string]interface{})
	}
	mergeMapValueIfEmpty(item, "type", value)
	mergeMapValueIfEmpty(item, "status", status)
	for _, key := range responseServerToolIdentifierKeys() {
		mergeMapValueIfEmpty(item, key, parsed[key])
	}
	for _, key := range []string{"action", "arguments", "input", "query", "output", "outputs", "results", "search_results", "sources", "citations", "data", "items", "content", "response", "result", "error"} {
		mergeMapValueIfEmpty(item, key, parsed[key])
	}
	return parseResponseServerToolCall(item), true
}

func responseServerToolIdentifierKeys() []string {
	keys := []string{"item_id", "id", "call_id", "tool_call_id"}
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.serverToolIdentifierKeys != nil {
			keys = append(keys, extension.serverToolIdentifierKeys()...)
		}
	}
	return keys
}

func mergeResponsesStreamOutputItem(
	adapter string,
	eventType string,
	result *GenerateOutput,
	parsed map[string]interface{},
	authoritative bool,
	onEvent func(GenerateStreamEvent) error,
) error {
	item := asMap(parsed["item"])
	if result == nil || len(item) == 0 {
		return nil
	}
	if strings.TrimSpace(getString(item["type"])) == "reasoning" {
		state := responsesReasoningStateFor(result)
		outputIndex, hasOutputIndex := responsesEventIndex(parsed, "output_index")
		events := reconcileResponsesReasoningItem(adapter, eventType, state, item, outputIndex, hasOutputIndex, authoritative)
		state.applyToResult(adapter, result)
		return emitResponsesReasoningEvents(result, events, onEvent)
	}
	if !isResponsesServerToolCallItem(item) {
		mergeResponsesOutputItem(result, item, false)
		return nil
	}
	call := parseResponseServerToolCall(item)
	appendUniqueToolCall(&result.ServerToolCalls, call)
	result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
	if err := mergeResponsesImageGenerationToolItem(result, item, onEvent); err != nil {
		return err
	}
	if onEvent == nil {
		return nil
	}
	return onEvent(GenerateStreamEvent{
		ServerToolCall: &call,
		ResponseID:     result.ResponseID,
	})
}

func mergeResponsesCustomToolInputEvent(
	result *GenerateOutput,
	parsed map[string]interface{},
	onEvent func(GenerateStreamEvent) error,
) error {
	if result == nil {
		return nil
	}
	itemID := firstNonEmptyString(getString(parsed["item_id"]), getString(parsed["call_id"]), getString(parsed["id"]))
	if itemID == "" {
		return nil
	}
	input := firstNonEmptyString(getString(parsed["delta"]), getString(parsed["input"]))
	done := strings.HasSuffix(strings.TrimSpace(getString(parsed["type"])), ".done")
	if call, ok := updateToolCallInput(&result.ServerToolCalls, itemID, input, done); ok {
		if onEvent != nil {
			return onEvent(GenerateStreamEvent{
				ServerToolCall: &call,
				ResponseID:     result.ResponseID,
			})
		}
		return nil
	}
	updateToolCallInput(&result.ToolCalls, itemID, input, done)
	return nil
}

func parseResponsesStreamErrorEvent(parsed map[string]interface{}, rawBody string) error {
	errorPayload := asMap(parsed["error"])
	if len(errorPayload) == 0 {
		errorPayload = asMap(asMap(parsed["response"])["error"])
	}
	statusCode := streamErrorStatusCode(parsed, errorPayload)
	message := firstNonEmptyString(
		getString(errorPayload["message"]),
		getString(errorPayload["msg"]),
		getString(parsed["message"]),
		"responses stream returned an error event",
	)
	return &UpstreamError{
		StatusCode: statusCode,
		Message:    message,
		Body:       rawBody,
	}
}

func parseResponsesOutput(adapter string, parsed map[string]interface{}, result *GenerateOutput) {
	result.Text = getString(parsed["output_text"])
	outputItems := asSlice(parsed["output"])
	textChunks := make([]string, 0, len(outputItems))
	reasoningItems := make([]*ReasoningOutput, 0)

	for _, raw := range outputItems {
		item := asMap(raw)
		if strings.TrimSpace(getString(item["type"])) == "reasoning" {
			reasoningItems = append(reasoningItems, parseReasoningOutputItem(item))
			continue
		}
		if chunk := mergeResponsesOutputItem(result, item, true); chunk != "" {
			textChunks = append(textChunks, chunk)
		}
	}
	result.Reasoning = combineResponsesReasoningOutputs(reasoningItems)

	if result.Text == "" && len(textChunks) > 0 {
		result.Text = strings.Join(textChunks, "")
	}

	mergeResponsesTopLevelToolCalls(result, parsed["tool_calls"])

	result.Usage = parseOpenAICompatibleUsageForAdapter(adapter, parsed)
	result.ServerSideToolUsage = parseServerSideToolUsage(parsed)
	result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(parsed)...)
	suppressOpenAIResponsesRawReasoning(adapter, result)
}

func mergeResponsesTopLevelToolCalls(result *GenerateOutput, raw interface{}) {
	if result == nil {
		return
	}
	for _, value := range asSlice(raw) {
		item := asMap(value)
		if len(item) == 0 {
			continue
		}
		if isResponsesServerToolCallItem(item) {
			appendUniqueToolCall(&result.ServerToolCalls, parseResponseServerToolCall(item))
			result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
			continue
		}
		itemType := strings.TrimSpace(getString(item["type"]))
		if isResponsesClientToolCallType(itemType) {
			appendUniqueToolCall(&result.ToolCalls, parseResponseToolCall(item))
		}
	}
}

func mergeReasoningDeltaOutput(dst **ReasoningOutput, delta *ReasoningDelta) {
	if delta == nil {
		return
	}
	if *dst == nil {
		*dst = &ReasoningOutput{}
	}
	if strings.TrimSpace(delta.ItemID) != "" {
		(*dst).ItemID = strings.TrimSpace(delta.ItemID)
	}
	if strings.TrimSpace(delta.Status) != "" {
		(*dst).Status = strings.TrimSpace(delta.Status)
	}
	switch delta.Kind {
	case "summary_text":
		(*dst).Summary += delta.Text
	default:
		(*dst).Text += delta.Text
	}
	if strings.TrimSpace(delta.Signature) != "" {
		(*dst).Signature = strings.TrimSpace(delta.Signature)
	}
	if strings.TrimSpace(delta.EncryptedContent) != "" {
		(*dst).EncryptedContent = strings.TrimSpace(delta.EncryptedContent)
	}
}

func parseResponsesReasoningDelta(eventType string, parsed map[string]interface{}) *ReasoningDelta {
	text := extractReasoningDeltaText(parsed["delta"])
	if text == "" {
		return nil
	}

	kind := "content_text"
	if strings.Contains(eventType, "summary") {
		kind = "summary_text"
	}

	return &ReasoningDelta{
		EventType:        eventType,
		ItemID:           firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id")),
		Status:           firstNonEmptyString(getString(parsed["status"]), getStringFromPath(parsed, "item", "status")),
		Kind:             kind,
		Text:             text,
		Signature:        firstNonEmptyString(getString(parsed["signature"]), getStringFromPath(parsed, "item", "signature")),
		EncryptedContent: firstNonEmptyString(getString(parsed["encrypted_content"]), getStringFromPath(parsed, "item", "encrypted_content")),
	}
}

func parseResponsesReasoningDone(eventType string, parsed map[string]interface{}) *ReasoningDelta {
	text, hasSnapshot := responsesReasoningDoneText(parsed)
	if !hasSnapshot {
		return nil
	}

	kind := "content_text"
	if strings.Contains(eventType, "summary") {
		kind = "summary_text"
	}

	return &ReasoningDelta{
		EventType:        eventType,
		ItemID:           firstNonEmptyString(getString(parsed["item_id"]), getStringFromPath(parsed, "item", "id")),
		Status:           firstNonEmptyString(getString(parsed["status"]), getStringFromPath(parsed, "item", "status")),
		Kind:             kind,
		Text:             text,
		Signature:        firstNonEmptyString(getString(parsed["signature"]), getStringFromPath(parsed, "item", "signature")),
		EncryptedContent: firstNonEmptyString(getString(parsed["encrypted_content"]), getStringFromPath(parsed, "item", "encrypted_content")),
	}
}

func responsesReasoningDoneText(parsed map[string]interface{}) (string, bool) {
	found := false
	for _, key := range []string{"text", "summary", "delta", "part"} {
		raw, ok := parsed[key]
		if !ok {
			continue
		}
		found = true
		if value := extractReasoningDeltaText(raw); value != "" {
			return value, true
		}
	}
	return "", found
}

func suppressOpenAIResponsesRawReasoning(adapter string, result *GenerateOutput) {
	if adapter != AdapterOpenAIResponses || result == nil || result.Reasoning == nil {
		return
	}
	result.Reasoning.Text = ""
	if strings.TrimSpace(result.Reasoning.Summary) != "" ||
		strings.TrimSpace(result.Reasoning.ItemID) != "" ||
		strings.TrimSpace(result.Reasoning.Status) != "" ||
		strings.TrimSpace(result.Reasoning.Signature) != "" ||
		strings.TrimSpace(result.Reasoning.EncryptedContent) != "" {
		return
	}
	result.Reasoning = nil
}

func parseReasoningOutputItem(item map[string]interface{}) *ReasoningOutput {
	if len(item) == 0 {
		return nil
	}

	summaryParts := responsesReasoningFragments(item["summary"])
	if len(summaryParts) == 0 {
		summaryParts = responsesReasoningFragments(item["summary_text"])
	}
	contentParts := responsesReasoningFragments(item["content"])
	if len(contentParts) == 0 {
		contentParts = responsesReasoningFragments(item["text"])
	}

	result := &ReasoningOutput{
		ItemID:           getString(item["id"]),
		Status:           getString(item["status"]),
		Summary:          joinResponsesReasoningFragments(summaryParts),
		Text:             joinResponsesReasoningFragments(contentParts),
		Signature:        getString(item["signature"]),
		EncryptedContent: getString(item["encrypted_content"]),
	}
	if reasoningOutputEmpty(result) {
		return nil
	}
	return result
}

func joinResponsesReasoningFragments(parts []string) string {
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			values = append(values, part)
		}
	}
	return strings.Join(values, responsesReasoningPartSeparator)
}

func combineResponsesReasoningOutputs(items []*ReasoningOutput) *ReasoningOutput {
	combined := &ReasoningOutput{}
	summaries := make([]string, 0, len(items))
	contents := make([]string, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.Summary != "" {
			summaries = append(summaries, item.Summary)
		}
		if item.Text != "" {
			contents = append(contents, item.Text)
		}
		if value := strings.TrimSpace(item.ItemID); value != "" {
			combined.ItemID = value
		}
		if value := strings.TrimSpace(item.Status); value != "" {
			combined.Status = value
		}
		if value := strings.TrimSpace(item.Signature); value != "" {
			combined.Signature = value
		}
		if value := strings.TrimSpace(item.EncryptedContent); value != "" {
			combined.EncryptedContent = value
		}
	}
	combined.Summary = strings.Join(summaries, responsesReasoningPartSeparator)
	combined.Text = strings.Join(contents, responsesReasoningPartSeparator)
	if reasoningOutputEmpty(combined) {
		return nil
	}
	return combined
}

func mergeReasoningOutput(dst **ReasoningOutput, src *ReasoningOutput) {
	if src == nil {
		return
	}
	if *dst == nil {
		value := *src
		*dst = &value
		return
	}
	if strings.TrimSpace(src.ItemID) != "" {
		(*dst).ItemID = strings.TrimSpace(src.ItemID)
	}
	if strings.TrimSpace(src.Status) != "" {
		(*dst).Status = strings.TrimSpace(src.Status)
	}
	if strings.TrimSpace(src.Summary) != "" {
		(*dst).Summary = strings.TrimSpace(src.Summary)
	}
	if strings.TrimSpace(src.Text) != "" {
		(*dst).Text = strings.TrimSpace(src.Text)
	}
	if strings.TrimSpace(src.Signature) != "" {
		(*dst).Signature = strings.TrimSpace(src.Signature)
	}
	if strings.TrimSpace(src.EncryptedContent) != "" {
		(*dst).EncryptedContent = strings.TrimSpace(src.EncryptedContent)
	}
}

func mergeResponsesOutputItem(result *GenerateOutput, item map[string]interface{}, collectText bool) string {
	if result == nil || len(item) == 0 {
		return ""
	}
	itemType := strings.TrimSpace(getString(item["type"]))
	switch {
	case itemType == "reasoning":
		mergeReasoningOutput(&result.Reasoning, parseReasoningOutputItem(item))
	case isResponsesServerToolCallItem(item):
		appendUniqueToolCall(&result.ServerToolCalls, parseResponseServerToolCall(item))
		result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
		_ = mergeResponsesImageGenerationToolItem(result, item, nil)
	case isResponsesClientToolCallType(itemType):
		appendUniqueToolCall(&result.ToolCalls, parseResponseToolCall(item))
	default:
		result.Citations = appendUniqueStrings(result.Citations, parseResponseCitations(item)...)
		if collectText {
			return extractOutputTextChunk(item)
		}
	}
	return ""
}

func parseResponseToolCall(item map[string]interface{}) ToolCall {
	arguments := normalizeJSONString(item["arguments"])
	if arguments == "" {
		arguments = normalizeJSONString(item["input"])
	}
	if arguments == "" {
		arguments = "{}"
	}

	toolCallID := strings.TrimSpace(getString(item["call_id"]))
	if toolCallID == "" {
		toolCallID = strings.TrimSpace(getString(item["id"]))
	}

	toolName := strings.TrimSpace(getString(item["name"]))
	if toolName == "" {
		toolName = strings.TrimSpace(getStringFromPath(item, "function", "name"))
	}

	toolType := strings.TrimSpace(getString(item["type"]))
	if toolType == "" {
		toolType = "function"
	}

	status := strings.TrimSpace(getString(item["status"]))
	if status == "" {
		status = "requested"
	}

	return ToolCall{
		ToolCallID:    toolCallID,
		ToolType:      toolType,
		ToolName:      toolName,
		ArgumentsJSON: arguments,
		Status:        status,
	}
}

func parseResponseServerToolCall(item map[string]interface{}) ToolCall {
	itemType := strings.TrimSpace(getString(item["type"]))
	if itemType == "" {
		itemType = "server_tool_call"
	}
	toolCallID := responseServerToolCallID(item, itemType)
	toolName := firstNonEmptyString(getString(item["name"]), responseServerToolNameFromType(itemType))
	status := firstNonEmptyString(getString(item["status"]), "completed")
	actionInputJSON, actionOutputJSON := splitResponsesServerToolAction(item["action"])
	inputJSON := firstNonEmptyString(
		actionInputJSON,
		normalizeJSONString(item["arguments"]),
		normalizeJSONString(item["input"]),
		normalizeJSONString(item["query"]),
	)
	outputJSON := firstNonEmptyString(
		normalizeJSONString(item["output"]),
		normalizeJSONString(item["outputs"]),
		normalizeJSONString(item["results"]),
		normalizeJSONString(item["search_results"]),
		normalizeJSONString(item["sources"]),
		normalizeJSONString(item["citations"]),
		normalizeJSONString(item["data"]),
		normalizeJSONString(item["items"]),
		normalizeJSONString(item["content"]),
		normalizeJSONString(item["response"]),
		normalizeJSONString(item["result"]),
		actionOutputJSON,
	)
	if isResponsesImageGenerationCallType(itemType) {
		outputJSON = responsesImageGenerationToolOutputJSON(item, outputJSON)
	}
	errorJSON := normalizeJSONString(item["error"])
	return ToolCall{
		ToolCallID:    toolCallID,
		ToolType:      itemType,
		ToolName:      toolName,
		ArgumentsJSON: inputJSON,
		Status:        status,
		OutputJSON:    outputJSON,
		ErrorJSON:     errorJSON,
	}
}

func responseServerToolCallID(item map[string]interface{}, itemType string) string {
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.serverToolCallID == nil {
			continue
		}
		if toolCallID, ok := extension.serverToolCallID(item, itemType); ok {
			return toolCallID
		}
	}
	return firstNonEmptyString(
		getString(item["item_id"]),
		getString(item["call_id"]),
		getString(item["id"]),
		getString(item["tool_call_id"]),
	)
}

func responseServerToolNameFromType(itemType string) string {
	value := strings.TrimSpace(itemType)
	for _, suffix := range []string{"_call_output", "_call"} {
		value = strings.TrimSuffix(value, suffix)
	}
	return value
}

func splitResponsesServerToolAction(raw interface{}) (string, string) {
	action := asMap(raw)
	if len(action) == 0 {
		return normalizeJSONString(raw), ""
	}
	input := cloneMap(action)
	delete(input, "sources")
	output := make(map[string]interface{})
	if query := strings.TrimSpace(getString(action["query"])); query != "" {
		output["query"] = query
	}
	if actionType := strings.TrimSpace(getString(action["type"])); actionType != "" {
		output["type"] = actionType
	}
	if sources := asSlice(action["sources"]); len(sources) > 0 {
		output["sources"] = sources
	}
	outputJSON := ""
	if len(output) > 0 && len(asSlice(output["sources"])) > 0 {
		outputJSON = normalizeJSONString(output)
	}
	return normalizeJSONString(input), outputJSON
}

func isResponsesServerToolCallItem(item map[string]interface{}) bool {
	itemType := strings.TrimSpace(getString(item["type"]))
	if isResponsesServerToolCallType(itemType) {
		return true
	}
	for _, extension := range allResponsesProtocolExtensions() {
		if extension.isServerToolCallItem != nil && extension.isServerToolCallItem(item) {
			return true
		}
	}
	return false
}

func isResponsesServerToolCallType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "web_search_call",
		"web_search_call_output",
		"file_search_call",
		"file_search_call_output",
		"code_interpreter_call",
		"code_interpreter_call_output",
		"code_execution_call",
		"code_execution_call_output",
		"collections_search_call",
		"collections_search_call_output",
		"attachment_search_call",
		"attachment_search_call_output",
		"computer_call",
		"computer_call_output",
		"mcp_call",
		"mcp_call_output",
		"image_generation_call",
		"image_generation_call_output",
		"shell_call",
		"shell_call_output",
		"local_shell_call",
		"local_shell_call_output":
		return true
	default:
		for _, extension := range allResponsesProtocolExtensions() {
			if extension.isServerToolCallType != nil && extension.isServerToolCallType(itemType) {
				return true
			}
		}
		return false
	}
}

func isResponsesImageGenerationCallType(itemType string) bool {
	switch strings.TrimSpace(itemType) {
	case "image_generation_call", "image_generation_call_output":
		return true
	default:
		return false
	}
}

func isResponsesImageGenerationPartialEvent(eventType string) bool {
	return strings.TrimSpace(eventType) == "response.image_generation_call.partial_image"
}

func emitResponsesImageGenerationPartial(
	result *GenerateOutput,
	parsed map[string]interface{},
	onEvent func(GenerateStreamEvent) error,
) error {
	if onEvent == nil {
		return nil
	}
	b64 := strings.TrimSpace(getString(parsed["partial_image_b64"]))
	if b64 == "" {
		return nil
	}
	image := GeneratedImage{
		B64JSON:  b64,
		MIMEType: openAIImageMIMEType(getString(parsed["output_format"])),
	}
	index := int(firstNonZero(
		getInt64FromPath(parsed, "partial_image_index"),
		getInt64FromPath(parsed, "index"),
	))
	return onEvent(GenerateStreamEvent{
		GeneratedImage:        &image,
		GeneratedImageIndex:   int64(index),
		GeneratedImagePartial: true,
		ResponseID:            responseIDForStreamEvent(result, parsed),
	})
}

func mergeResponsesImageGenerationToolItem(
	result *GenerateOutput,
	item map[string]interface{},
	onEvent func(GenerateStreamEvent) error,
) error {
	if result == nil || !isResponsesImageGenerationCallType(getString(item["type"])) {
		return nil
	}
	for _, image := range parseResponsesImageGenerationImages(item) {
		index, appended := appendUniqueGeneratedImage(&result.GeneratedImages, image)
		if !appended || onEvent == nil {
			continue
		}
		eventImage := image
		if err := onEvent(GenerateStreamEvent{
			GeneratedImage:      &eventImage,
			GeneratedImageIndex: int64(index),
			ResponseID:          result.ResponseID,
		}); err != nil {
			return err
		}
	}
	return nil
}

func responseIDForStreamEvent(result *GenerateOutput, parsed map[string]interface{}) string {
	if result != nil && strings.TrimSpace(result.ResponseID) != "" {
		return strings.TrimSpace(result.ResponseID)
	}
	return firstNonEmptyString(getString(parsed["response_id"]), getStringFromPath(parsed, "response", "id"), getString(parsed["id"]))
}

func parseResponsesImageGenerationImages(item map[string]interface{}) []GeneratedImage {
	if len(item) == 0 {
		return nil
	}
	if !isResponsesImageGenerationCallType(getString(item["type"])) {
		return nil
	}
	revisedPrompt := firstNonEmptyString(getString(item["revised_prompt"]), getString(item["revisedPrompt"]))
	outputFormat := getString(item["output_format"])
	if b64 := strings.TrimSpace(getString(item["result"])); b64 != "" {
		return []GeneratedImage{{
			B64JSON:       b64,
			MIMEType:      openAIImageMIMEType(outputFormat),
			RevisedPrompt: strings.TrimSpace(revisedPrompt),
		}}
	}
	image, ok := parseOpenAIImagePayload(asMap(item["result"]), outputFormat)
	if !ok {
		image, ok = parseOpenAIImagePayload(item, outputFormat)
	}
	if !ok {
		return nil
	}
	if image.RevisedPrompt == "" {
		image.RevisedPrompt = strings.TrimSpace(revisedPrompt)
	}
	return []GeneratedImage{image}
}

func appendUniqueGeneratedImage(items *[]GeneratedImage, image GeneratedImage) (int, bool) {
	if items == nil {
		return -1, false
	}
	if strings.TrimSpace(image.URL) == "" && strings.TrimSpace(image.B64JSON) == "" {
		return -1, false
	}
	for index, existing := range *items {
		if image.URL != "" && strings.TrimSpace(existing.URL) == strings.TrimSpace(image.URL) {
			return index, false
		}
		if image.B64JSON != "" && strings.TrimSpace(existing.B64JSON) == strings.TrimSpace(image.B64JSON) {
			return index, false
		}
	}
	*items = append(*items, image)
	return len(*items) - 1, true
}

func responsesImageGenerationToolOutputJSON(item map[string]interface{}, fallback string) string {
	if len(parseResponsesImageGenerationImages(item)) == 0 {
		return fallback
	}
	payload := map[string]interface{}{
		"type":            "image_generation_call",
		"result":          "[redacted]",
		"output_format":   firstNonEmptyString(getString(item["output_format"]), "png"),
		"image_generated": true,
	}
	for _, key := range []string{"background", "quality", "revised_prompt", "size", "status"} {
		if value, ok := item[key]; ok {
			payload[key] = value
		}
	}
	if result := asMap(item["result"]); len(result) > 0 {
		if imageURL := strings.TrimSpace(getString(result["url"])); imageURL != "" {
			payload["url"] = imageURL
		}
	}
	return normalizeJSONString(payload)
}

func isResponsesClientToolCallType(itemType string) bool {
	value := strings.TrimSpace(itemType)
	if value == "" {
		return false
	}
	if value == "function_call" || value == "tool_call" || value == "custom_tool_call" {
		return true
	}
	return strings.HasSuffix(value, "_tool_call") && !isResponsesServerToolCallType(value)
}

func parseResponseCitations(parsed map[string]interface{}) []string {
	if len(parsed) == 0 {
		return nil
	}
	result := make([]string, 0)
	for _, key := range []string{"citations", "sources", "urls"} {
		for _, raw := range asSlice(parsed[key]) {
			if text := firstNonEmptyString(getString(raw), getStringFromPath(asMap(raw), "url"), getStringFromPath(asMap(raw), "uri")); text != "" {
				result = append(result, text)
			}
		}
	}
	collectResponseCitationURLs(parsed, &result)
	return appendUniqueStrings(nil, result...)
}

func collectResponseCitationURLs(raw interface{}, result *[]string) {
	if result == nil {
		return
	}
	switch value := raw.(type) {
	case []interface{}:
		for _, item := range value {
			collectResponseCitationURLs(item, result)
		}
	case map[string]interface{}:
		itemType := strings.TrimSpace(getString(value["type"]))
		switch itemType {
		case "url_citation", "file_citation":
			if text := firstNonEmptyString(
				getString(value["url"]),
				getString(value["uri"]),
				getStringFromPath(value, "url_citation", "url"),
				getStringFromPath(value, "file_citation", "url"),
			); text != "" {
				*result = append(*result, text)
			}
		}
		if text := firstNonEmptyString(getString(value["url"]), getString(value["uri"])); text != "" {
			*result = append(*result, text)
		}
		for _, key := range []string{"action", "content", "annotations", "output", "outputs", "results", "search_results", "sources", "citations", "url_citation", "file_citation", "data", "items", "response", "result"} {
			collectResponseCitationURLs(value[key], result)
		}
	}
}

func parseServerSideToolUsage(parsed map[string]interface{}) map[string]int64 {
	usage := asMap(parsed["usage"])
	if len(usage) == 0 {
		return nil
	}
	raw := asMap(usage["server_side_tool_usage"])
	if len(raw) == 0 {
		raw = asMap(usage["tool_usage"])
	}
	if len(raw) == 0 {
		raw = asMap(usage["server_side_tool_usage_details"])
	}
	if len(raw) == 0 {
		return nil
	}
	result := make(map[string]int64, len(raw))
	for key, value := range raw {
		if normalized := normalizeServerSideToolUsageKey(key); normalized != "" {
			result[normalized] = toInt64(value)
		}
	}
	return result
}

func normalizeServerSideToolUsageKey(key string) string {
	value := strings.TrimSpace(key)
	value = strings.TrimSuffix(value, "_calls")
	value = strings.TrimSuffix(value, "_call")
	switch value {
	case "web_search", "code_interpreter", "image_generation", "shell", "file_search", "mcp", "document_search":
		return value
	default:
		for _, extension := range allResponsesProtocolExtensions() {
			if extension.normalizeServerSideToolUsageKey == nil {
				continue
			}
			if normalized, ok := extension.normalizeServerSideToolUsageKey(value, key); ok {
				return normalized
			}
		}
		return key
	}
}

func extractOutputTextChunk(item map[string]interface{}) string {
	if chunk := extractContentText(item["content"]); chunk != "" {
		return chunk
	}
	if chunk := getString(item["text"]); chunk != "" {
		return chunk
	}
	return ""
}
