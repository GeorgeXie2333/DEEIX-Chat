import enErrors from "@/i18n/messages/en-US/errors.json";
import jaErrors from "@/i18n/messages/ja-JP/errors.json";
import zhErrors from "@/i18n/messages/zh-CN/errors.json";
import { DEFAULT_LOCALE, LOCALE_COOKIE_NAME, normalizeAppLocale, resolveBrowserLocale, type AppLocale } from "@/i18n/config";
import { ApiError } from "@/shared/api/http-client";

const ERROR_MESSAGES: Record<AppLocale, unknown> = {
  "zh-CN": zhErrors,
  "en-US": enErrors,
  "ja-JP": jaErrors,
};

const FALLBACK_MESSAGES: Record<AppLocale, string> = {
  "zh-CN": "请求失败，请稍后重试。",
  "en-US": "Request failed. Please try again later.",
  "ja-JP": "リクエストに失敗しました。しばらくしてからもう一度お試しください。",
};

type RequestBodyFieldError = {
  field?: unknown;
  rule?: unknown;
  param?: unknown;
};

type RequestBodyErrorDetails = {
  fieldErrors?: unknown;
};

type RedemptionCodeErrorDetails = {
  reason?: unknown;
};

const REQUEST_FIELD_LABELS: Record<AppLocale, Record<string, string>> = {
  "en-US": {
    apiKeys: "API keys",
    avatarURL: "Avatar URL",
    baseURL: "Base URL",
    cbDurationMin: "Circuit duration",
    cbFailureThreshold: "Failure threshold",
    cbModelThreshold: "Model threshold",
    cbThresholdLogic: "Threshold logic",
    cbWindowMin: "Circuit window",
    compatible: "Compatibility mode",
    connectTimeoutMS: "Connect timeout",
    displayName: "Display name",
    email: "Email",
    headersJSON: "Headers JSON",
    locale: "Language",
    maskFileID: "Mask file",
    name: "Name",
    password: "Password",
    phone: "Phone",
    protocolDefaultsJSON: "Protocol defaults JSON",
    readTimeoutMS: "Read timeout",
    status: "Status",
    systemPrompt: "System prompt",
    streamIdleTimeoutMS: "Stream idle timeout",
    subscriptionExpiresAt: "Subscription expiry",
    subscriptionTier: "Subscription plan",
    timezone: "Timezone",
    username: "Username",
  },
  "zh-CN": {
    apiKeys: "API Keys",
    avatarURL: "头像地址",
    baseURL: "Base URL",
    cbDurationMin: "熔断时长",
    cbFailureThreshold: "失败阈值",
    cbModelThreshold: "模型阈值",
    cbThresholdLogic: "阈值逻辑",
    cbWindowMin: "统计窗口",
    compatible: "兼容模式",
    connectTimeoutMS: "连接超时",
    displayName: "昵称",
    email: "邮箱",
    headersJSON: "请求头 JSON",
    locale: "语言",
    maskFileID: "蒙版文件",
    name: "名称",
    password: "密码",
    phone: "手机号",
    protocolDefaultsJSON: "协议默认配置 JSON",
    readTimeoutMS: "读取超时",
    status: "状态",
    systemPrompt: "系统提示词",
    streamIdleTimeoutMS: "流式空闲超时",
    subscriptionExpiresAt: "订阅到期时间",
    subscriptionTier: "订阅方案",
    timezone: "时区",
    username: "用户名",
  },
  "ja-JP": {
    apiKeys: "API キー",
    avatarURL: "アバターURL",
    baseURL: "Base URL",
    cbDurationMin: "サーキット継続時間",
    cbFailureThreshold: "失敗しきい値",
    cbModelThreshold: "モデルしきい値",
    cbThresholdLogic: "しきい値ロジック",
    cbWindowMin: "サーキット集計期間",
    compatible: "互換モード",
    connectTimeoutMS: "接続タイムアウト",
    displayName: "表示名",
    email: "メールアドレス",
    headersJSON: "ヘッダーJSON",
    locale: "言語",
    maskFileID: "マスクファイル",
    name: "名前",
    password: "パスワード",
    phone: "電話番号",
    protocolDefaultsJSON: "プロトコル既定JSON",
    readTimeoutMS: "読み取りタイムアウト",
    status: "ステータス",
    systemPrompt: "システムプロンプト",
    streamIdleTimeoutMS: "ストリームアイドルタイムアウト",
    subscriptionExpiresAt: "サブスクリプション期限",
    subscriptionTier: "サブスクリプションプラン",
    timezone: "タイムゾーン",
    username: "ユーザー名",
  },
};

const SETTINGS_FIELD_LABELS: Record<AppLocale, Record<string, string>> = {
  "en-US": {
    "auth:auto_link_verified_email": "Auto-link same email",
    "auth:email_login_enabled": "Email sign-in",
    "auth:email_registration_allowed_domains": "Allowed email domains",
    "auth:email_registration_block_plus_alias": "Block plus aliases",
    "auth:email_registration_enabled": "Email registration",
    "auth:email_verification_enabled": "Email verification",
    "auth:password_reset_enabled": "Password reset",
    "auth:login_default_next_path": "Default redirect path",
    "auth:login_lock_minutes": "Lock duration",
    "auth:login_max_failures": "Login failure limit",
    "auth:rate_limit_enabled": "Platform rate limit",
    "auth:rate_limit_rpm": "User API rate limit",
    "auth:public_auth_rate_limit_rpm": "Public auth rate limit",
    "auth:refresh_token_ttl_hours": "Refresh token TTL",
    "auth:smtp_from": "SMTP sender",
    "auth:smtp_host": "SMTP host",
    "auth:smtp_password": "SMTP password",
    "auth:smtp_port": "SMTP port",
    "auth:smtp_username": "SMTP username",
    "auth:third_party_login_enabled": "Third-party sign-in",
    "auth:token_ttl_hours": "Access token TTL",
    "auth:turnstile_registration_enabled": "Turnstile registration verification",
    "auth:turnstile_secret_key": "Turnstile Secret Key",
    "auth:turnstile_site_key": "Turnstile Site Key",
    "auth:username_login_enabled": "Username sign-in",
    "billing:epay_gateway_url": "EPay gateway URL",
    "billing:epay_key": "EPay key",
    "billing:epay_minimum_top_up_amount_usd": "EPay minimum top-up",
    "billing:epay_pid": "EPay merchant ID",
    "billing:epay_types": "EPay payment types",
    "billing:mode": "Billing mode",
    "billing:payment_providers": "Payment providers",
    "billing:prepaid_amount_usd": "Per-request reservation",
    "billing:stripe_fee_rate_percent": "Stripe surcharge rate",
    "billing:stripe_minimum_top_up_amount_usd": "Stripe minimum top-up",
    "billing:stripe_publishable_key": "Stripe publishable key",
    "billing:stripe_secret_key": "Stripe secret key",
    "billing:stripe_webhook_secret": "Stripe webhook secret",
    "billing:usd_to_cny_rate": "USD to CNY rate",
    "chat:model_option_allowed_paths": "Model option allowlist",
    "chat:default_system_prompt": "Global default system prompt",
    "chat:model_option_denied_paths": "Model option denylist",
    "chat:model_option_policy_mode": "Model option policy",
    "chat:prompt_sensitive_words": "Sensitive word dictionary",
    "file:embedding_enabled": "Embedding",
    "file:full_context_limit_enabled": "Full-text injection limits",
    "file:file_full_context_max_bytes": "Full-text size limit",
    "file:full_context_max_tokens": "Full-text token limit",
    "file:full_context_pdf_max_pages": "Full-text page limit",
    "mcp:mcp_enable": "MCP",
  },
  "zh-CN": {
    "auth:auto_link_verified_email": "同邮箱自动绑定",
    "auth:email_login_enabled": "邮箱登录",
    "auth:email_registration_allowed_domains": "邮箱注册域名白名单",
    "auth:email_registration_block_plus_alias": "禁止邮箱 + 别名",
    "auth:email_registration_enabled": "邮箱注册",
    "auth:email_verification_enabled": "邮箱验证",
    "auth:password_reset_enabled": "重置密码",
    "auth:login_default_next_path": "登录后默认跳转路径",
    "auth:login_lock_minutes": "锁定时长",
    "auth:login_max_failures": "登录失败阈值",
    "auth:rate_limit_enabled": "平台限流",
    "auth:rate_limit_rpm": "用户接口限流",
    "auth:public_auth_rate_limit_rpm": "公开鉴权限流",
    "auth:refresh_token_ttl_hours": "刷新令牌有效期",
    "auth:smtp_from": "SMTP 发件人",
    "auth:smtp_host": "SMTP 主机",
    "auth:smtp_password": "SMTP 密码",
    "auth:smtp_port": "SMTP 端口",
    "auth:smtp_username": "SMTP 用户名",
    "auth:third_party_login_enabled": "第三方登录",
    "auth:token_ttl_hours": "访问令牌有效期",
    "auth:turnstile_registration_enabled": "注册人机验证",
    "auth:turnstile_secret_key": "Turnstile Secret Key",
    "auth:turnstile_site_key": "Turnstile Site Key",
    "auth:username_login_enabled": "用户名登录",
    "billing:epay_gateway_url": "易支付网关地址",
    "billing:epay_key": "易支付商户密钥",
    "billing:epay_minimum_top_up_amount_usd": "易支付最低充值金额",
    "billing:epay_pid": "易支付商户 ID",
    "billing:epay_types": "易支付支付方式",
    "billing:mode": "计费模式",
    "billing:payment_providers": "支付渠道",
    "billing:prepaid_amount_usd": "单次预留金额",
    "billing:stripe_fee_rate_percent": "Stripe 额外手续费率",
    "billing:stripe_minimum_top_up_amount_usd": "Stripe 最低充值金额",
    "billing:stripe_publishable_key": "Stripe Publishable Key",
    "billing:stripe_secret_key": "Stripe Secret Key",
    "billing:stripe_webhook_secret": "Stripe Webhook Secret",
    "billing:usd_to_cny_rate": "美元人民币汇率",
    "chat:model_option_allowed_paths": "模型参数白名单",
    "chat:default_system_prompt": "全局默认系统提示词",
    "chat:model_option_denied_paths": "模型参数黑名单",
    "chat:model_option_policy_mode": "模型参数透传策略",
    "chat:prompt_sensitive_words": "敏感词库",
    "file:embedding_enabled": "向量服务",
    "file:full_context_limit_enabled": "全文注入限制",
    "file:file_full_context_max_bytes": "全文大小上限",
    "file:full_context_max_tokens": "全文 Token 上限",
    "file:full_context_pdf_max_pages": "全文页数上限",
    "mcp:mcp_enable": "MCP",
  },
  "ja-JP": {
    "auth:auto_link_verified_email": "同じメールを自動連携",
    "auth:email_login_enabled": "メールログイン",
    "auth:email_registration_allowed_domains": "登録可能なメールドメイン",
    "auth:email_registration_block_plus_alias": "プラス別名を禁止",
    "auth:email_registration_enabled": "メール登録",
    "auth:email_verification_enabled": "メール認証",
    "auth:login_default_next_path": "ログイン後の既定遷移先",
    "auth:login_lock_minutes": "ロック時間",
    "auth:login_max_failures": "ログイン失敗上限",
    "auth:login_page_title": "ログインページタイトル",
    "auth:public_auth_rate_limit_rpm": "公開認証レート制限",
    "auth:refresh_token_ttl_hours": "更新トークン有効期間",
    "auth:smtp_from": "SMTP 送信者",
    "auth:smtp_host": "SMTP ホスト",
    "auth:smtp_password": "SMTP パスワード",
    "auth:smtp_port": "SMTP ポート",
    "auth:smtp_username": "SMTP ユーザー名",
    "auth:third_party_login_enabled": "外部ログイン",
    "auth:token_ttl_hours": "アクセストークン有効期間",
    "auth:turnstile_registration_enabled": "登録時の人間確認",
    "auth:turnstile_secret_key": "Turnstile Secret Key",
    "auth:turnstile_site_key": "Turnstile Site Key",
    "auth:username_login_enabled": "ユーザー名ログイン",
    "billing:epay_gateway_url": "EPay ゲートウェイ URL",
    "billing:epay_key": "EPay キー",
    "billing:epay_minimum_top_up_amount_usd": "EPay 最低チャージ金額",
    "billing:epay_pid": "EPay 加盟店 ID",
    "billing:epay_types": "EPay 支払い方法",
    "billing:mode": "課金モード",
    "billing:payment_providers": "決済プロバイダー",
    "billing:prepaid_amount_usd": "前払い金額",
    "billing:stripe_fee_rate_percent": "Stripe 追加手数料率",
    "billing:stripe_minimum_top_up_amount_usd": "Stripe 最低チャージ金額",
    "billing:stripe_publishable_key": "Stripe 公開キー",
    "billing:stripe_secret_key": "Stripe シークレットキー",
    "billing:stripe_webhook_secret": "Stripe Webhook シークレット",
    "billing:usd_to_cny_rate": "USD/CNY レート",
    "chat:model_option_allowed_paths": "モデルオプション許可リスト",
    "chat:default_system_prompt": "グローバル既定システムプロンプト",
    "chat:model_option_denied_paths": "モデルオプション拒否リスト",
    "chat:model_option_native_tool_types": "ネイティブツール種別",
    "chat:model_option_policy_mode": "モデルオプションポリシー",
    "chat:prompt_sensitive_words": "センシティブワード辞書",
    "file:embedding_enabled": "埋め込み",
    "file:full_context_limit_enabled": "全文注入制限",
    "file:file_full_context_max_bytes": "全文バイト上限",
    "file:full_context_max_tokens": "全文トークン上限",
    "file:full_context_pdf_max_pages": "全文ページ上限",
    "mcp:mcp_enable": "MCP",
  },
};

export function toErrorMessagePath(errorCode: string): string[] {
  return errorCode
    .trim()
    .split(".")
    .filter(Boolean)
    .map((segment) => segment.replace(/_([a-z])/g, (_, char: string) => char.toUpperCase()));
}

function isInternalErrorKey(message: string): boolean {
  return /^errors\.[a-zA-Z0-9_.]+$/.test(message.trim());
}

function readClientLocale(): AppLocale {
  if (typeof document === "undefined") {
    return DEFAULT_LOCALE;
  }
  const cookieValue = document.cookie
    .split(";")
    .map((item) => item.trim())
    .find((item) => item.startsWith(`${LOCALE_COOKIE_NAME}=`))
    ?.slice(LOCALE_COOKIE_NAME.length + 1);
  if (cookieValue) {
    return normalizeAppLocale(decodeURIComponent(cookieValue));
  }
  return typeof navigator === "undefined"
    ? DEFAULT_LOCALE
    : resolveBrowserLocale(navigator.languages?.length ? navigator.languages : [navigator.language]);
}

function lookupErrorMessage(locale: AppLocale, errorCode: string): string | undefined {
  let current: unknown = ERROR_MESSAGES[locale];
  for (const segment of toErrorMessagePath(errorCode)) {
    if (!current || typeof current !== "object" || !Object.hasOwn(current, segment)) {
      return undefined;
    }
    current = (current as Record<string, unknown>)[segment];
  }
  return typeof current === "string" ? current : undefined;
}

function isRequestBodyErrorDetails(details: unknown): details is RequestBodyErrorDetails {
  return Boolean(details && typeof details === "object" && "fieldErrors" in details);
}

function isRequestBodyFieldError(item: unknown): item is RequestBodyFieldError {
  return Boolean(item && typeof item === "object" && "field" in item && "rule" in item);
}

function resolveRequestFieldLabel(locale: AppLocale, field: string): string {
  return REQUEST_FIELD_LABELS[locale][field] ?? field;
}

function resolveRequestFieldError(locale: AppLocale, item: RequestBodyFieldError): string | undefined {
  const field = typeof item.field === "string" ? item.field.trim() : "";
  const rule = typeof item.rule === "string" ? item.rule.trim() : "";
  const param = typeof item.param === "string" ? item.param.trim() : "";
  if (!field || !rule) return undefined;

  const label = resolveRequestFieldLabel(locale, field);
  if (locale === "ja-JP") {
    switch (rule) {
      case "required":
      case "required_without":
        return `${label}は必須です。`;
      case "min":
        return `${label}は${param}文字以上で入力してください。`;
      case "max":
        return `${label}は${param}文字以内で入力してください。`;
      case "len":
        return `${label}は${param}文字で入力してください。`;
      case "email":
        return `${label}の形式が正しくありません。`;
      case "url":
        return `${label}は https://api.example.com のような完全な URL で入力してください。`;
      case "oneof":
        return `${label}は次のいずれかである必要があります：${param}。`;
      default:
        return `${label}が無効です。`;
    }
  }
  if (locale === "zh-CN") {
    switch (rule) {
      case "required":
      case "required_without":
        return `${label}不能为空。`;
      case "min":
        return `${label}至少 ${param} 个字符。`;
      case "max":
        return `${label}不能超过 ${param} 个字符。`;
      case "len":
        return `${label}长度必须是 ${param} 个字符。`;
      case "email":
        return `${label}格式不正确。`;
      case "url":
        return `${label}必须是完整 URL，例如 https://api.example.com。`;
      case "oneof":
        return `${label}必须是以下值之一：${param}。`;
      default:
        return `${label}参数无效。`;
    }
  }

  switch (rule) {
    case "required":
    case "required_without":
      return `${label} is required.`;
    case "min":
      return `${label} must be at least ${param} characters.`;
    case "max":
      return `${label} must be at most ${param} characters.`;
    case "len":
      return `${label} must be ${param} characters.`;
    case "email":
      return `${label} must be a valid email address.`;
    case "url":
      return `${label} must be a full URL, for example https://api.example.com.`;
    case "oneof":
      return `${label} must be one of: ${param}.`;
    default:
      return `${label} is invalid.`;
  }
}

function resolveRequestBodyValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (error.errorCode !== "request.invalid_body") return undefined;
  if (!isRequestBodyErrorDetails(error.details) || !Array.isArray(error.details.fieldErrors)) return undefined;

  const messages = error.details.fieldErrors
    .filter(isRequestBodyFieldError)
    .map((item) => resolveRequestFieldError(locale, item))
    .filter((item): item is string => Boolean(item));

  return messages.length > 0 ? messages.join(locale === "zh-CN" || locale === "ja-JP" ? "" : " ") : undefined;
}

function resolveSettingsFieldLabel(locale: AppLocale, key: string): string {
  return SETTINGS_FIELD_LABELS[locale][key] ?? key;
}

function resolveSettingsReason(locale: AppLocale, label: string, reason: string): string {
  const normalized = reason.trim();
  if (!normalized) return "";
  if (locale === "ja-JP") {
    const integerRange = normalized.match(/^must be an integer between (.+) and (.+)$/);
    if (integerRange) return `${label}は${integerRange[1]}から${integerRange[2]}までの整数で入力してください。`;
    const optionalZeroRange = normalized.match(/^must be empty, 0, or between (.+) and (.+)$/);
    if (optionalZeroRange) return `${label}は空、0、または${optionalZeroRange[1]}から${optionalZeroRange[2]}までの値にしてください。`;
    const range = normalized.match(/^must be between (.+) and (.+)$/);
    if (range) return `${label}は${range[1]}から${range[2]}までの値にしてください。`;
    const optionalMin = normalized.match(/^must be empty or >= (.+)$/);
    if (optionalMin) return `${label}は空、または${optionalMin[1]}以上にしてください。`;
    const min = normalized.match(/^must be >= (.+)$/);
    if (min) return `${label}は${min[1]}以上にしてください。`;
    const maxLength = normalized.match(/^length must be <= (.+)$/);
    if (maxLength) return `${label}は${maxLength[1]}文字以内で入力してください。`;
    const oneOf = normalized.match(/^must be one of: (.+)$/);
    if (oneOf) return `${label}は次のいずれかである必要があります：${oneOf[1]}。`;
    const only = normalized.match(/^must contain only: (.+)$/);
    if (only) return `${label}に含められるのは次のみです：${only[1]}。`;
    const invalidDomain = normalized.match(/^contains invalid domain: (.+)$/);
    if (invalidDomain) return `${label}に無効なドメインが含まれています：${invalidDomain[1]}。`;
    const invalidMime = normalized.match(/^contains invalid mime: (.+)$/);
    if (invalidMime) return `${label}に無効な MIME タイプが含まれています：${invalidMime[1]}。`;
    switch (normalized) {
      case "cannot be empty":
      case "is required":
        return `${label}は必須です。`;
      case "must be a local path":
        return `${label}は /chat のようなサイト内パスで入力してください。`;
      case "must be bool":
        return `${label}は true または false で入力してください。`;
      case "must start with http:// or https://":
        return `${label}は http:// または https:// で始まる必要があります。`;
      case "must be a json array":
        return `${label}は JSON 配列で入力してください。`;
      case "must contain 1-10 payment types":
        return `${label}には1から10個の支払い方法を含めてください。`;
      case "items require name and type":
        return `${label}の各項目には name と type が必要です。`;
      case "item is too long":
        return `${label}の項目が長すぎます。`;
      case "type contains invalid characters":
        return `${label}の type に無効な文字が含まれています。`;
      case "type must be unique":
        return `${label}の type は重複できません。`;
      default:
        return `${label}：${normalized}`;
    }
  }
  if (locale === "zh-CN") {
    const integerRange = normalized.match(/^must be an integer between (.+) and (.+)$/);
    if (integerRange) return `${label}必须是 ${integerRange[1]} 到 ${integerRange[2]} 之间的整数。`;
    const optionalZeroRange = normalized.match(/^must be empty, 0, or between (.+) and (.+)$/);
    if (optionalZeroRange) return `${label}必须留空、填 0，或在 ${optionalZeroRange[1]} 到 ${optionalZeroRange[2]} 之间。`;
    const range = normalized.match(/^must be between (.+) and (.+)$/);
    if (range) return `${label}必须在 ${range[1]} 到 ${range[2]} 之间。`;
    const optionalMin = normalized.match(/^must be empty or >= (.+)$/);
    if (optionalMin) return `${label}必须留空，或大于等于 ${optionalMin[1]}。`;
    const min = normalized.match(/^must be >= (.+)$/);
    if (min) return `${label}必须大于等于 ${min[1]}。`;
    const maxLength = normalized.match(/^length must be <= (.+)$/);
    if (maxLength) return `${label}长度不能超过 ${maxLength[1]} 个字符。`;
    const oneOf = normalized.match(/^must be one of: (.+)$/);
    if (oneOf) return `${label}必须是以下值之一：${oneOf[1]}。`;
    const only = normalized.match(/^must contain only: (.+)$/);
    if (only) return `${label}只能包含：${only[1]}。`;
    const invalidDomain = normalized.match(/^contains invalid domain: (.+)$/);
    if (invalidDomain) return `${label}包含无效域名：${invalidDomain[1]}。`;
    const invalidMime = normalized.match(/^contains invalid mime: (.+)$/);
    if (invalidMime) return `${label}包含无效 MIME 类型：${invalidMime[1]}。`;
    switch (normalized) {
      case "cannot be empty":
      case "is required":
        return `${label}不能为空。`;
      case "must be a local path":
        return `${label}必须是站内路径，例如 /chat。`;
      case "must be bool":
        return `${label}必须是 true 或 false。`;
      case "must start with http:// or https://":
        return `${label}必须以 http:// 或 https:// 开头。`;
      case "must be a json array":
        return `${label}必须是 JSON 数组。`;
      case "must contain 1-10 payment types":
        return `${label}必须包含 1 到 10 个支付方式。`;
      case "items require name and type":
        return `${label}每一项都必须包含 name 和 type。`;
      case "item is too long":
        return `${label}单项内容过长。`;
      case "type contains invalid characters":
        return `${label}的 type 包含无效字符。`;
      case "type must be unique":
        return `${label}的 type 不能重复。`;
      default:
        return `${label}：${normalized}`;
    }
  }

  const optionalZeroRange = normalized.match(/^must be empty, 0, or between (.+) and (.+)$/);
  if (optionalZeroRange) {
    return `${label} must be empty, 0, or between ${optionalZeroRange[1]} and ${optionalZeroRange[2]}.`;
  }
  const optionalMin = normalized.match(/^must be empty or >= (.+)$/);
  if (optionalMin) {
    return `${label} must be empty or at least ${optionalMin[1]}.`;
  }

  return `${label}: ${normalized}.`;
}

function resolveSettingsValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (!error.errorCode?.startsWith("settings.")) return undefined;
  const raw = (error.rawMessage || error.message || "").trim();
  if (!raw || /^invalid .+ settings?\.?$/i.test(raw) || /^invalid setting value\.?$/i.test(raw)) {
    return undefined;
  }
  const detail = raw.replace(/^invalid setting:\s*/i, "").trim();
  const dependencyMessages: Record<AppLocale, Record<string, string>> = {
    "en-US": {
      "auth:third_party_login_enabled must be enabled before disabling username and email login": "Enable third-party sign-in before disabling both username and email sign-in.",
      "embedding service must be enabled and configured before enabling rag or semantic enhancement": "Enable and configure embedding before enabling RAG or semantic context.",
    },
    "zh-CN": {
      "auth:third_party_login_enabled must be enabled before disabling username and email login": "关闭用户名和邮箱登录前，必须先启用第三方登录。",
      "embedding service must be enabled and configured before enabling rag or semantic enhancement": "启用 RAG 或语义增强前，必须先启用并配置向量服务。",
    },
    "ja-JP": {
      "auth:third_party_login_enabled must be enabled before disabling username and email login": "ユーザー名ログインとメールログインの両方を無効にする前に、外部ログインを有効にしてください。",
      "embedding service must be enabled and configured before enabling rag or semantic enhancement": "RAGまたはセマンティックコンテキストを有効にする前に、埋め込みサービスを有効化して設定してください。",
    },
  };
  const dependencyMessage = dependencyMessages[locale][detail.toLowerCase()];
  if (dependencyMessage) return dependencyMessage;

  const match = detail.match(/^([a-z]+:[a-z0-9_]+)\s+(.+)$/);
  if (!match) return detail;
  return resolveSettingsReason(locale, resolveSettingsFieldLabel(locale, match[1]), match[2]);
}

function isRedemptionCodeErrorDetails(details: unknown): details is RedemptionCodeErrorDetails {
  return Boolean(details && typeof details === "object" && "reason" in details);
}

function resolveRedemptionCodeValidationMessage(error: ApiError, locale: AppLocale): string | undefined {
  if (error.errorCode !== "billing.invalid_redemption_code") return undefined;
  if (!isRedemptionCodeErrorDetails(error.details) || typeof error.details.reason !== "string") return undefined;
  const reason = error.details.reason.trim();
  if (!reason) return undefined;
  return lookupErrorMessage(locale, `billing.redemption_validation.${reason}`);
}

export function resolveLocalizedErrorMessage(error: unknown, fallback?: string): string {
  const locale = readClientLocale();
  if (error instanceof ApiError && error.errorCode) {
    const validationMessage = resolveRequestBodyValidationMessage(error, locale);
    if (validationMessage) {
      return validationMessage;
    }

    const settingsValidationMessage = resolveSettingsValidationMessage(error, locale);
    if (settingsValidationMessage) {
      return settingsValidationMessage;
    }

    const redemptionCodeValidationMessage = resolveRedemptionCodeValidationMessage(error, locale);
    if (redemptionCodeValidationMessage) {
      return redemptionCodeValidationMessage;
    }

    const translated = lookupErrorMessage(locale, error.errorCode);
    if (translated) {
      return translated;
    }
  }

  if (error instanceof Error) {
    const message = error.message.trim();
    if (isInternalErrorKey(message)) {
      const translated = lookupErrorMessage(locale, message.replace(/^errors\./, ""));
      if (translated) {
        return translated;
      }
    }
    if (message && !isInternalErrorKey(message)) {
      return message;
    }
  }

  return fallback || FALLBACK_MESSAGES[locale];
}
