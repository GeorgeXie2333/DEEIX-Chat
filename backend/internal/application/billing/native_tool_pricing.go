package billing

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

const nativeToolPricingSource = "admin_config_with_default_fallback"

// NativeToolPricingInput 描述管理员保存的原生工具价格。金额单位为美元。
type NativeToolPricingInput struct {
	ToolKey  string
	PriceUSD float64
}

type nativeToolUsageAlias struct {
	providerProtocol string
	toolName         string
	matches          func(UsagePricingInput) bool
}

type nativeToolCatalogEntry struct {
	provider       string
	providerCode   string
	toolKey        string
	serviceName    string
	defaultNanousd int64
	unit           string
	priceLabel     string
	aliases        []nativeToolUsageAlias
}

func nativeToolCatalog() []nativeToolCatalogEntry {
	return []nativeToolCatalogEntry{
		{
			provider:       "OpenAI",
			providerCode:   "openai",
			toolKey:        "openaiWebSearchReasoning",
			serviceName:    "OpenAI Web search",
			defaultNanousd: nativeToolUSD001Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "openai_responses", toolName: "web_search", matches: isOpenAIWebSearchReasoningModel},
				{providerProtocol: "openai_responses", toolName: "web_search_preview", matches: isOpenAIWebSearchReasoningModel},
				{providerProtocol: "openai_chat_completions", toolName: "web_search", matches: isOpenAIWebSearchReasoningModel},
				{providerProtocol: "openai_chat_completions", toolName: "web_search_preview", matches: isOpenAIWebSearchReasoningModel},
			},
		},
		{
			provider:       "OpenAI",
			providerCode:   "openai",
			toolKey:        "openaiWebSearchStandard",
			serviceName:    "OpenAI Web search",
			defaultNanousd: nativeToolUSD0025Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "openai_responses", toolName: "web_search", matches: isOpenAIWebSearchStandardModel},
				{providerProtocol: "openai_responses", toolName: "web_search_preview", matches: isOpenAIWebSearchStandardModel},
				{providerProtocol: "openai_chat_completions", toolName: "web_search", matches: isOpenAIWebSearchStandardModel},
				{providerProtocol: "openai_chat_completions", toolName: "web_search_preview", matches: isOpenAIWebSearchStandardModel},
			},
		},
		{
			provider:     "OpenAI",
			providerCode: "openai",
			toolKey:      "openaiShell",
			serviceName:  "OpenAI Shell",
			unit:         "call",
			priceLabel:   "notMetered",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "openai_responses", toolName: "shell"},
			},
		},
		{
			provider:       "OpenAI",
			providerCode:   "openai",
			toolKey:        "openaiImageGeneration",
			serviceName:    "OpenAI Image generation",
			defaultNanousd: nativeToolUSD01Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "openai_responses", toolName: "image_generation"},
			},
		},
		{
			provider:     "OpenAI",
			providerCode: "openai",
			toolKey:      "openaiCodeInterpreter",
			serviceName:  "OpenAI Code interpreter",
			unit:         "call",
			priceLabel:   "notMetered",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "openai_responses", toolName: "code_interpreter"},
			},
		},
		{
			provider:       "Anthropic",
			providerCode:   "anthropic",
			toolKey:        "anthropicWebSearch",
			serviceName:    "Anthropic Web search",
			defaultNanousd: nativeToolUSD001Nanousd,
			unit:           "search",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "anthropic_messages", toolName: "web_search"},
			},
		},
		{
			provider:     "Anthropic",
			providerCode: "anthropic",
			toolKey:      "anthropicWebFetch",
			serviceName:  "Anthropic Web fetch",
			unit:         "call",
			priceLabel:   "included",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "anthropic_messages", toolName: "web_fetch"},
			},
		},
		{
			provider:     "Anthropic",
			providerCode: "anthropic",
			toolKey:      "anthropicCodeExecution",
			serviceName:  "Anthropic Code execution",
			unit:         "call",
			priceLabel:   "notMetered",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "anthropic_messages", toolName: "code_execution"},
			},
		},
		{
			provider:     "Anthropic",
			providerCode: "anthropic",
			toolKey:      "anthropicAdvisor",
			serviceName:  "Anthropic Advisor",
			unit:         "call",
			priceLabel:   "notMetered",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "anthropic_messages", toolName: "advisor"},
			},
		},
		{
			provider:     "Anthropic",
			providerCode: "anthropic",
			toolKey:      "anthropicToolSearch",
			serviceName:  "Anthropic Tool search",
			unit:         "call",
			priceLabel:   "included",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "anthropic_messages", toolName: "tool_search"},
				{providerProtocol: "anthropic_messages", toolName: "tool_search_tool_regex"},
				{providerProtocol: "anthropic_messages", toolName: "tool_search_tool_bm25"},
			},
		},
		{
			provider:       "xAI",
			providerCode:   "xai",
			toolKey:        "xaiWebSearch",
			serviceName:    "xAI Web Search",
			defaultNanousd: nativeToolUSD0005Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "xai_responses", toolName: "web_search"},
			},
		},
		{
			provider:       "xAI",
			providerCode:   "xai",
			toolKey:        "xaiXSearch",
			serviceName:    "xAI X Search",
			defaultNanousd: nativeToolUSD0005Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "xai_responses", toolName: "x_search"},
			},
		},
		{
			provider:       "xAI",
			providerCode:   "xai",
			toolKey:        "xaiCodeExecution",
			serviceName:    "xAI Code Execution",
			defaultNanousd: nativeToolUSD0005Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "xai_responses", toolName: "code_interpreter"},
				{providerProtocol: "xai_responses", toolName: "code_execution"},
			},
		},
		{
			provider:       "xAI",
			providerCode:   "xai",
			toolKey:        "xaiAttachmentSearch",
			serviceName:    "xAI File Attachments Search",
			defaultNanousd: nativeToolUSD001Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "xai_responses", toolName: "attachment_search"},
				{providerProtocol: "xai_responses", toolName: "file_attachment_search"},
				{providerProtocol: "xai_responses", toolName: "file_attachments_search"},
			},
		},
		{
			provider:       "xAI",
			providerCode:   "xai",
			toolKey:        "xaiCollectionsSearch",
			serviceName:    "xAI Collections Search / RAG",
			defaultNanousd: nativeToolUSD00025Nanousd,
			unit:           "call",
			aliases: []nativeToolUsageAlias{
				{providerProtocol: "xai_responses", toolName: "file_search"},
				{providerProtocol: "xai_responses", toolName: "collection_search"},
				{providerProtocol: "xai_responses", toolName: "collections_search"},
			},
		},
	}
}

func isOpenAIWebSearchStandardModel(input UsagePricingInput) bool {
	return !isOpenAIWebSearchReasoningModel(input)
}

func nativeToolCatalogByKey() map[string]nativeToolCatalogEntry {
	entries := nativeToolCatalog()
	result := make(map[string]nativeToolCatalogEntry, len(entries))
	for _, entry := range entries {
		result[entry.toolKey] = entry
	}
	return result
}

func nativeToolCatalogEntryForUsage(input UsagePricingInput, toolName string) (nativeToolCatalogEntry, bool) {
	protocol := strings.TrimSpace(input.ProviderProtocol)
	tool := strings.TrimSpace(toolName)
	if protocol == "" || tool == "" {
		return nativeToolCatalogEntry{}, false
	}
	for _, entry := range nativeToolCatalog() {
		for _, alias := range entry.aliases {
			if alias.providerProtocol != protocol || alias.toolName != tool {
				continue
			}
			if alias.matches != nil && !alias.matches(input) {
				continue
			}
			return entry, true
		}
	}
	return nativeToolCatalogEntry{}, false
}

func nativeToolEffectivePriceNanousd(entry nativeToolCatalogEntry, overrides map[string]int64) int64 {
	if overrides != nil {
		if value, ok := overrides[entry.toolKey]; ok {
			return value
		}
	}
	return entry.defaultNanousd
}

// NativeToolPricingKeys 返回当前支持配置价格的原生工具 key 集合。
func NativeToolPricingKeys() map[string]struct{} {
	entries := nativeToolCatalog()
	result := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		result[entry.toolKey] = struct{}{}
	}
	return result
}

// ListNativeToolDefaultPricing 返回当前内置的原生工具默认价格目录。
func ListNativeToolDefaultPricing() []NativeToolPricingView {
	return ListNativeToolPricing(nil)
}

// ListNativeToolPricing 返回叠加管理员覆盖后的原生工具价格目录。
func ListNativeToolPricing(overrides map[string]int64) []NativeToolPricingView {
	entries := nativeToolCatalog()
	results := make([]NativeToolPricingView, 0, len(entries))
	for _, entry := range entries {
		priceNanousd := nativeToolEffectivePriceNanousd(entry, overrides)
		_, customized := overrides[entry.toolKey]
		priceLabel := strings.TrimSpace(entry.priceLabel)
		if priceNanousd <= 0 && priceLabel == "" {
			priceLabel = "notMetered"
		}
		results = append(results, NativeToolPricingView{
			Provider:            entry.provider,
			ToolKey:             entry.toolKey,
			PriceNanousd:        priceNanousd,
			PriceUSD:            nanousdToUSD(priceNanousd),
			DefaultPriceNanousd: entry.defaultNanousd,
			DefaultPriceUSD:     nanousdToUSD(entry.defaultNanousd),
			Unit:                entry.unit,
			PriceLabel:          priceLabel,
			Billable:            priceNanousd > 0,
			Customized:          customized,
		})
	}
	return results
}

// NativeToolPricingOverridesFromUSD 校验并转换管理员提交的美元价格覆盖。
func NativeToolPricingOverridesFromUSD(items []NativeToolPricingInput) (map[string]int64, error) {
	catalog := nativeToolCatalogByKey()
	results := make(map[string]int64, len(items))
	for _, item := range items {
		toolKey := strings.TrimSpace(item.ToolKey)
		if _, ok := catalog[toolKey]; !ok {
			return nil, fmt.Errorf("%w: unknown native tool %q", ErrInvalidNativeToolPricing, toolKey)
		}
		if item.PriceUSD < 0 || math.IsNaN(item.PriceUSD) || math.IsInf(item.PriceUSD, 0) {
			return nil, fmt.Errorf("%w: invalid price for %q", ErrInvalidNativeToolPricing, toolKey)
		}
		results[toolKey] = usdToNanousd(item.PriceUSD)
	}
	return results, nil
}

// ParseNativeToolPricingOverridesJSON 解析系统设置中保存的纳美元价格覆盖。
func ParseNativeToolPricingOverridesJSON(raw string) (map[string]int64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]int64{}, nil
	}
	var parsed map[string]int64
	if err := json.Unmarshal([]byte(value), &parsed); err != nil {
		return nil, fmt.Errorf("%w: invalid native tool pricing json", ErrInvalidNativeToolPricing)
	}
	return normalizeNativeToolPricingOverrides(parsed)
}

func normalizeNativeToolPricingOverrides(items map[string]int64) (map[string]int64, error) {
	catalog := nativeToolCatalogByKey()
	results := make(map[string]int64, len(items))
	for key, value := range items {
		toolKey := strings.TrimSpace(key)
		if _, ok := catalog[toolKey]; !ok {
			return nil, fmt.Errorf("%w: unknown native tool %q", ErrInvalidNativeToolPricing, toolKey)
		}
		if value < 0 {
			return nil, fmt.Errorf("%w: invalid price for %q", ErrInvalidNativeToolPricing, toolKey)
		}
		results[toolKey] = value
	}
	return results, nil
}

// MarshalNativeToolPricingOverridesJSON 将覆盖价格规范化为系统设置 JSON。
func MarshalNativeToolPricingOverridesJSON(overrides map[string]int64) (string, error) {
	normalized, err := normalizeNativeToolPricingOverrides(overrides)
	if err != nil {
		return "", err
	}
	if len(normalized) == 0 {
		return "{}", nil
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}
