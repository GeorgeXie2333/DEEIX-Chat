import type { UserDTO } from "@/shared/api/auth.types";
import type { BillingPlanDTO, BillingPlanPriceDTO } from "@/shared/api/billing.types";
import {
  formatBillingDisplayAmountFromUSD,
  formatBillingDisplayCompactAmountFromUSD,
  formatBillingDisplayPreciseAmountFromUSD,
  formatBillingDisplayUnitPriceFromUSD,
} from "@/shared/lib/billing-display";
import type { BillingDisplayOptions } from "@/shared/lib/billing-display";

const DEFAULT_BILLING_DISPLAY: BillingDisplayOptions = { currency: "USD" };
type PaymentProvider = "stripe" | "epay";

export function resolveDefaultPrice(plan: BillingPlanDTO | null | undefined): BillingPlanPriceDTO | null {
  const prices = plan?.prices ?? [];
  if (prices.length === 0) {
    return null;
  }
  return prices.find((item) => item.isDefault) || prices[0] || null;
}

export function formatPlanPrice(
  price: BillingPlanPriceDTO | null,
  intervalLabels: { lifetime: string; year: string; month: string },
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): string {
  if (!price) return "-";
  const currency = (price.currency || "USD").toUpperCase();
  const amount = currency === "USD"
    ? formatBillingDisplayAmountFromUSD((price.amountCents || 0) / 100, billingDisplay, { maximumFractionDigits: 2 })
    : new Intl.NumberFormat("en-US", {
      style: "currency",
      currency,
    }).format((price.amountCents || 0) / 100);
  if (price.billingInterval === "lifetime") return `${amount} / ${intervalLabels.lifetime}`;
  if (price.billingInterval === "year") return `${amount} / ${intervalLabels.year}`;
  return `${amount} / ${intervalLabels.month}`;
}

function effectivePaymentCurrency(provider: PaymentProvider): "USD" | "CNY" {
  if (provider === "epay") {
    return "CNY";
  }
  return "USD";
}

function formatCurrencyAmount(amount: number, currency: "USD" | "CNY"): string {
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency,
  }).format(Math.max(0, amount));
}

export function billingDisplayInputCurrency(billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): "USD" | "CNY" {
  const rate = Number(billingDisplay.usdToCnyRate);
  return billingDisplay.currency === "CNY" && Number.isFinite(rate) && rate > 0 ? "CNY" : "USD";
}

export function billingDisplayInputSymbol(billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): "$" | "¥" {
  return billingDisplayInputCurrency(billingDisplay) === "CNY" ? "¥" : "$";
}

export function billingDisplayAmountToUSD(amount: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): number {
  if (!Number.isFinite(amount) || amount <= 0) {
    return 0;
  }
  if (billingDisplayInputCurrency(billingDisplay) !== "CNY") {
    return amount;
  }
  return amount / Number(billingDisplay.usdToCnyRate);
}

export function billingDisplayAmountToMinorUnits(amount: number): number {
  if (!Number.isFinite(amount) || amount <= 0) {
    return 0;
  }
  return Math.round(amount * 100);
}

export function topUpInputAmountToUSD(
  amount: number,
  provider: PaymentProvider,
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): number {
  if (!Number.isFinite(amount) || amount <= 0) {
    return 0;
  }
  return provider === "stripe" ? amount : billingDisplayAmountToUSD(amount, billingDisplay);
}

export function topUpInputSymbol(
  provider: PaymentProvider,
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): "$" | "¥" {
  return provider === "stripe" ? "$" : billingDisplayInputSymbol(billingDisplay);
}

export function convertTopUpInputAmount(
  value: string,
  fromProvider: PaymentProvider,
  toProvider: PaymentProvider,
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): string {
  if (fromProvider === toProvider) {
    return value;
  }
  const amount = Number(value);
  const amountUSD = topUpInputAmountToUSD(amount, fromProvider, billingDisplay);
  if (amountUSD <= 0) {
    return value;
  }
  if (toProvider === "stripe" || billingDisplayInputCurrency(billingDisplay) === "USD") {
    return formatEditableMoneyAmount(amountUSD);
  }
  return formatEditableMoneyAmount(amountUSD * Number(billingDisplay.usdToCnyRate));
}

export type StripePaymentAmounts = {
  subtotalAmountCents: number;
  feeRateBasisPoints: number;
  feeAmountCents: number;
  totalAmountCents: number;
};

export function calculateStripePaymentAmounts(
  subtotalAmountCents: number,
  feeRatePercent: number,
): StripePaymentAmounts {
  const subtotal = Number.isSafeInteger(subtotalAmountCents) && subtotalAmountCents > 0
    ? subtotalAmountCents
    : 0;
  const basisPoints = normalizeStripeFeeRateBasisPoints(feeRatePercent);
  const quotient = Math.floor(subtotal / 10_000);
  const remainder = subtotal % 10_000;
  const fee = quotient * basisPoints + Math.floor((remainder * basisPoints + 5_000) / 10_000);
  return {
    subtotalAmountCents: subtotal,
    feeRateBasisPoints: basisPoints,
    feeAmountCents: fee,
    totalAmountCents: subtotal + fee,
  };
}

export function formatUSDFromCents(amountCents: number): string {
  return formatCurrencyAmount(Math.max(0, amountCents) / 100, "USD");
}

export function formatStripeFeeRatePercent(feeRatePercent: number): string {
  return `${normalizeStripeFeeRateBasisPoints(feeRatePercent) / 100}%`;
}

function normalizeStripeFeeRateBasisPoints(value: number): number {
  if (!Number.isFinite(value) || value < 0 || value > 100) {
    return 0;
  }
  return Math.round(value * 100);
}

function formatEditableMoneyAmount(value: number): string {
  return value.toFixed(2).replace(/\.?0+$/, "");
}

export function formatProviderPaymentAmountFromUSD(
  amountUSD: number,
  provider: PaymentProvider,
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): string {
  const currency = effectivePaymentCurrency(provider);
  if (currency === "USD") {
    return formatCurrencyAmount(amountUSD, "USD");
  }
  const rate = Number(billingDisplay.usdToCnyRate);
  const amount = Number.isFinite(rate) && rate > 0 ? amountUSD * rate : amountUSD * 7.2;
  return formatCurrencyAmount(amount, "CNY");
}

export function formatPlanCredit(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  return formatBillingDisplayAmountFromUSD(value, billingDisplay, { maximumFractionDigits: 2 });
}

export function formatAccountBalance(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  return formatBillingDisplayPreciseAmountFromUSD(value, billingDisplay);
}

export function formatUsageCost(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  return formatBillingDisplayCompactAmountFromUSD(value, billingDisplay);
}

export function formatTooltipUsageCost(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  return formatBillingDisplayPreciseAmountFromUSD(value, billingDisplay);
}

export function formatTooltipUnitPrice(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  return formatBillingDisplayUnitPriceFromUSD(value, billingDisplay);
}

export function nanousdToUSD(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  return value / 1_000_000_000;
}

export function formatUsageSummaryCost(value: number, billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY): string {
  if (!Number.isFinite(value) || value <= 0) {
    return formatBillingDisplayAmountFromUSD(0, billingDisplay, { maximumFractionDigits: 4 });
  }
  const compact = formatBillingDisplayCompactAmountFromUSD(value, billingDisplay, 0.0001);
  if (compact.startsWith("< ")) {
    return compact;
  }
  return formatBillingDisplayAmountFromUSD(value, billingDisplay, {
    minimumFractionDigits: 0,
    maximumFractionDigits: 4,
  });
}

export function formatUsageAxisTokens(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1_000_000) return `${(value / 1_000_000).toLocaleString("en-US", { maximumFractionDigits: 1 })}M`;
  if (value >= 1_000) return `${Math.round(value / 1_000).toLocaleString("en-US")}K`;
  return Math.round(value).toLocaleString("en-US");
}

export function formatLatency(value: number | null | undefined): string {
  if (!Number.isFinite(value ?? NaN) || (value ?? 0) <= 0) return "-";
  const ms = value ?? 0;
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toLocaleString("en-US", { maximumFractionDigits: 2 })}s`;
}

export function formatUsageTrendLatency(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return formatLatency(value);
}

export function formatTokenCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "-";
  return value.toLocaleString("en-US");
}

export function formatFormulaTokenCount(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return "0";
  return value.toLocaleString("en-US");
}

export function formatDay(value: string | null | undefined): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${month}/${day}`;
}

export function formatMonthLabel(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, { month: "short" }).format(date);
}

export function formatFullMonthLabel(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, { year: "numeric", month: "long" }).format(date);
}

export function formatShortDate(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

export function formatMediumDate(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}

export function formatUsageLogTime(value: string | null | undefined, locale: string): string {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return "-";
  return new Intl.DateTimeFormat(locale, {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
}

export function modelDisplayLabel(model: { platformModelName?: string }): string {
  return model.platformModelName?.trim() || "-";
}

export function isFreePlan(plan: BillingPlanDTO | null | undefined): boolean {
  return plan?.code?.trim() === "free";
}

export function isCurrentBillingPlan(
  plan: BillingPlanDTO,
  currentPlan: BillingPlanDTO | null,
  viewer: UserDTO | null,
): boolean {
  return currentPlan?.id === plan.id || viewer?.subscriptionPlanID === plan.id || viewer?.subscriptionTier === plan.code;
}

export function planRank(plan: BillingPlanDTO | null | undefined): number {
  if (!plan) return 0;
  if (Number.isFinite(plan.sortOrder) && plan.sortOrder > 0) {
    return plan.sortOrder;
  }
  if (plan.code === "ultra") return 40;
  if (plan.code === "max") return 30;
  if (plan.code === "pro") return 20;
  if (plan.code === "free") return 10;
  return plan.periodCreditUSD;
}

export type PlanActionKind = "current" | "renew" | "upgrade" | "subscribe" | "switch" | "freeBlocked" | "unavailable";

export type PlanActionLabels = {
  current: string;
  unavailable: string;
  renew: string;
  subscribe: string;
  switch: string;
  upgrade: string;
  freeBlocked: string;
};

export function resolvePlanActionKind(
  plan: BillingPlanDTO,
  price: BillingPlanPriceDTO | null,
  isCurrent: boolean,
  currentPlan: BillingPlanDTO | null,
  protectedPaidPlanRank: number,
): PlanActionKind {
  const targetRank = planRank(plan);
  if (isCurrent) {
    if (isFreePlan(plan)) return "current";
    return price ? "renew" : "current";
  }
  if (isFreePlan(plan) && protectedPaidPlanRank > 0) return "freeBlocked";
  if (!price) return "unavailable";
  if (!price.amountCents) return "switch";
  if (!currentPlan || isFreePlan(currentPlan)) return "subscribe";

  if (protectedPaidPlanRank > targetRank) return "renew";
  const comparison = targetRank - planRank(currentPlan);
  if (comparison > 0) return "upgrade";
  return "renew";
}

export function resolvePlanActionLabel(action: PlanActionKind, labels: PlanActionLabels): string {
  return labels[action];
}

export function resolvePlanButtonVariant(action: PlanActionKind): "default" | "outline" | "secondary" {
  if (action === "current") return "secondary";
  if (action === "freeBlocked" || action === "unavailable" || action === "switch") return "outline";
  return "default";
}

export function resolvePaymentProviderLabel(provider: string | undefined, fallback: string): string {
  if (provider === "stripe") return "Stripe";
  if (provider === "epay") return "EPay";
  return fallback;
}

export function resolveEPayTypeLabel(type: string, labels: { alipay: string; wxpay: string; qqpay: string; custom: (type: string) => string }): string {
  if (type === "alipay") return labels.alipay;
  if (type === "wxpay") return labels.wxpay;
  if (type === "qqpay") return labels.qqpay;
  return labels.custom(type);
}

export function resolvePlanFeatures(
  plan: BillingPlanDTO,
  labels: { monthlyCredit: (credit: string) => string; freeModelsNotIncluded: string },
  billingDisplay: BillingDisplayOptions = DEFAULT_BILLING_DISPLAY,
): string[] {
  const fallback = [
    labels.monthlyCredit(formatPlanCredit(plan.periodCreditUSD, billingDisplay)),
    labels.freeModelsNotIncluded,
  ];
  try {
    const parsed = JSON.parse(plan.featureJSON || "null") as unknown;
    if (Array.isArray(parsed)) {
      const features = parsed.filter((item): item is string => typeof item === "string" && item.trim().length > 0);
      return features.length > 0 ? features : fallback;
    }
    if (parsed && typeof parsed === "object" && Array.isArray((parsed as { features?: unknown }).features)) {
      const features = ((parsed as { features: unknown[] }).features).filter((item): item is string => typeof item === "string" && item.trim().length > 0);
      return features.length > 0 ? features : fallback;
    }
  } catch {
    // ignore invalid admin-entered feature JSON
  }
  return plan.description ? [plan.description, ...fallback] : fallback;
}
