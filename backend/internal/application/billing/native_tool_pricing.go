package billing

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/nativetool"
)

// NativeToolPricingInput 描述管理员保存的原生工具价格。金额单位为美元。
type NativeToolPricingInput struct {
	ToolKey  string
	PriceUSD float64
}

// NativeToolPricingOverridesFromUSD 校验并转换管理员提交的美元价格覆盖。
func NativeToolPricingOverridesFromUSD(items []NativeToolPricingInput) (map[string]nativetool.PricingOverride, error) {
	defaults := nativetool.PricingOverridesFromDefinitions(nativetool.PricingDefinitions())
	results := make(map[string]nativetool.PricingOverride, len(items))
	for _, item := range items {
		toolKey := strings.TrimSpace(item.ToolKey)
		defaultOverride, ok := defaults[toolKey]
		if !ok {
			return nil, fmt.Errorf("%w: unknown native tool %q", ErrInvalidNativeToolPricing, toolKey)
		}
		if item.PriceUSD < 0 || math.IsNaN(item.PriceUSD) || math.IsInf(item.PriceUSD, 0) {
			return nil, fmt.Errorf("%w: invalid price for %q", ErrInvalidNativeToolPricing, toolKey)
		}
		priceNanousd := usdToNanousd(item.PriceUSD)
		defaultOverride.PriceNanousd = priceNanousd
		defaultOverride.PriceLabel = ""
		defaultOverride.Billable = priceNanousd > 0
		results[toolKey] = defaultOverride
	}
	return results, nil
}

// ParseNativeToolPricingOverridesJSON 解析系统设置中保存的原生工具价格覆盖。
func ParseNativeToolPricingOverridesJSON(raw string) (map[string]nativetool.PricingOverride, error) {
	overrides, err := nativetool.ParsePricingOverridesJSON(raw)
	if err == nil {
		return overrides, nil
	}

	// 兼容旧版本保存的 {"toolKey": priceNanousd} 形态。
	value := strings.TrimSpace(raw)
	if value == "" {
		return map[string]nativetool.PricingOverride{}, nil
	}
	var legacy map[string]int64
	if legacyErr := json.Unmarshal([]byte(value), &legacy); legacyErr != nil {
		return nil, fmt.Errorf("%w: invalid native tool pricing json", ErrInvalidNativeToolPricing)
	}
	defaults := nativetool.PricingOverridesFromDefinitions(nativetool.PricingDefinitions())
	converted := make(map[string]nativetool.PricingOverride, len(legacy))
	for key, priceNanousd := range legacy {
		toolKey := strings.TrimSpace(key)
		defaultOverride, ok := defaults[toolKey]
		if !ok {
			return nil, fmt.Errorf("%w: unknown native tool %q", ErrInvalidNativeToolPricing, toolKey)
		}
		if priceNanousd < 0 {
			return nil, fmt.Errorf("%w: invalid price for %q", ErrInvalidNativeToolPricing, toolKey)
		}
		defaultOverride.PriceNanousd = priceNanousd
		defaultOverride.PriceLabel = ""
		defaultOverride.Billable = priceNanousd > 0
		converted[toolKey] = defaultOverride
	}
	return converted, nil
}

// MarshalNativeToolPricingOverridesJSON 将覆盖价格规范化为系统设置 JSON。
func MarshalNativeToolPricingOverridesJSON(overrides map[string]nativetool.PricingOverride) (string, error) {
	return nativetool.PricingOverridesJSON(overrides)
}
