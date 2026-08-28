package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	sharedsecurity "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/shared/security"
)

func TestLoadDefaultsUseBootstrapAdmin(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())

	cfg := Load()
	if cfg.Env != "prod" {
		t.Fatalf("expected default env prod, got %q", cfg.Env)
	}
	if cfg.AdminUsername != defaultAdminUsername {
		t.Fatalf("expected default admin username %q, got %q", defaultAdminUsername, cfg.AdminUsername)
	}
	if cfg.AdminDisplayName != defaultAdminDisplayName {
		t.Fatalf("expected default admin display name %q, got %q", defaultAdminDisplayName, cfg.AdminDisplayName)
	}
	if cfg.FileFullContextMaxBytes != DefaultFileFullContextMaxBytes {
		t.Fatalf("expected default full-context size %d, got %d", DefaultFileFullContextMaxBytes, cfg.FileFullContextMaxBytes)
	}
	if cfg.SSRFAllowedHosts != "" || cfg.SSRFAllowedCIDRs != "" {
		t.Fatalf("expected SSRF allowlist to be empty by default, hosts=%q CIDRs=%q", cfg.SSRFAllowedHosts, cfg.SSRFAllowedCIDRs)
	}
}

func TestLoadTreatsBlankAPPEnvAsUnset(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())
	t.Setenv("APP_ENV", " ")

	cfg := Load()
	if cfg.Env != "prod" {
		t.Fatalf("expected blank APP_ENV to default to prod, got %q", cfg.Env)
	}
}

func TestLoadNormalizesAPPEnvAliases(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "development", env: "development", want: "dev"},
		{name: "production", env: "production", want: "prod"},
		{name: "trim and case", env: " Production ", want: "prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanupConfigEnv(t)
			chdir(t, t.TempDir())
			t.Setenv("APP_ENV", tt.env)

			cfg := Load()
			if cfg.Env != tt.want {
				t.Fatalf("expected APP_ENV %q to normalize to %q, got %q", tt.env, tt.want, cfg.Env)
			}
		})
	}
}

func TestLoadNormalizesLegacyPostgresDSNTimeZone(t *testing.T) {
	cleanupConfigEnv(t)
	chdir(t, t.TempDir())
	t.Setenv("POSTGRES_DSN", "postgres://deeix_chat:secret%2Fvalue@postgres:5432/deeix_chat?sslmode=disable&TimeZone=Asia%2FShanghai")

	cfg := Load()
	if cfg.PostgresDSN != "postgres://deeix_chat:secret%2Fvalue@postgres:5432/deeix_chat?sslmode=disable&TimeZone=Asia/Shanghai" {
		t.Fatalf("expected legacy timezone to be normalized without decoding credentials, got %q", cfg.PostgresDSN)
	}
}

func TestNormalizePostgresDSNTimeZone(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "url",
			dsn:  "postgres://user:pass%2Fword@postgres:5432/db?sslmode=disable&TimeZone=Asia%2FShanghai",
			want: "postgres://user:pass%2Fword@postgres:5432/db?sslmode=disable&TimeZone=Asia/Shanghai",
		},
		{
			name: "key value",
			dsn:  "host=postgres user=deeix password=secret%2Fvalue dbname=deeix sslmode=disable TimeZone=Asia%2FShanghai",
			want: "host=postgres user=deeix password=secret%2Fvalue dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
		},
		{
			name: "already normalized",
			dsn:  "host=postgres user=deeix dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
			want: "host=postgres user=deeix dbname=deeix sslmode=disable TimeZone=Asia/Shanghai",
		},
		{
			name: "other percent encoded fields unchanged",
			dsn:  "postgres://user:pass@postgres:5432/db?application_name=DEEIX%2FChat&sslmode=disable",
			want: "postgres://user:pass@postgres:5432/db?application_name=DEEIX%2FChat&sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePostgresDSN(tt.dsn); got != tt.want {
				t.Fatalf("normalizePostgresDSN() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLoadReadsRepositoryRootConfigFromBackendWorkingDirectory(t *testing.T) {
	cleanupConfigEnv(t)

	root := filepath.Join(t.TempDir(), "repo")
	backendDir := filepath.Join(root, "backend")
	if err := os.MkdirAll(backendDir, 0o755); err != nil {
		t.Fatalf("create backend dir: %v", err)
	}
	if resolvedRoot, err := filepath.EvalSymlinks(root); err == nil {
		root = resolvedRoot
		backendDir = filepath.Join(root, "backend")
	}
	configPath := filepath.Join(root, "config.yaml")
	configBody := []byte(`
server:
  frontend_dist_dir: ./frontend/out
storage:
  local:
    root_dir: ./data/storage
geoip:
  database_path: ./data/geoip.mmdb
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write root config: %v", err)
	}
	chdir(t, backendDir)

	cfg := Load()
	if cfg.AdminUsername != defaultAdminUsername {
		t.Fatalf("expected built-in admin username, got %q", cfg.AdminUsername)
	}
	if cfg.AdminDisplayName != defaultAdminDisplayName {
		t.Fatalf("expected built-in admin display name, got %q", cfg.AdminDisplayName)
	}
	assertPath(t, "frontend dist", cfg.FrontendDistDir, filepath.Join(root, "frontend", "out"))
	assertPath(t, "storage root", cfg.StorageRootDir, filepath.Join(root, "data", "storage"))
	assertPath(t, "geoip database", cfg.GeoIPDatabasePath, filepath.Join(root, "data", "geoip.mmdb"))
}

func TestLoadReadsTurnstileSiteverifyURL(t *testing.T) {
	cleanupConfigEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := []byte(`
security:
  turnstile_siteverify_url: "https://turnstile.example.test/siteverify"
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := Load()
	if cfg.TurnstileSiteverifyURL != "https://turnstile.example.test/siteverify" {
		t.Fatalf("expected turnstile siteverify url from config, got %q", cfg.TurnstileSiteverifyURL)
	}

	t.Setenv("TURNSTILE_SITEVERIFY_URL", "https://turnstile-env.example.test/siteverify")
	cfg = Load()
	if cfg.TurnstileSiteverifyURL != "https://turnstile-env.example.test/siteverify" {
		t.Fatalf("expected turnstile siteverify url from env, got %q", cfg.TurnstileSiteverifyURL)
	}
}

func TestNormalizeModelOptionAllowedPathsJSONUpgradesLegacyDefault(t *testing.T) {
	var legacy map[string][]string
	if err := json.Unmarshal([]byte(DefaultModelOptionAllowedPathsJSON()), &legacy); err != nil {
		t.Fatalf("parse default model option paths: %v", err)
	}
	legacy["anthropic_messages"] = removeString(legacy["anthropic_messages"], "output_config.effort")
	legacy["openai_chat_completions"] = removeString(legacy["openai_chat_completions"], "reasoning_summary")
	legacy["openai_responses"] = removeString(legacy["openai_responses"], "reasoning.mode")
	legacy["gemini_generate_content"] = removeString(legacy["gemini_generate_content"], "thinkingConfig.thinkingLevel")
	legacy["google_image_generation"] = removeString(legacy["google_image_generation"], "generationConfig.thinkingConfig.thinkingLevel")
	delete(legacy, "gemini_interactions")
	delete(legacy, "openai_video_generations")
	rawLegacy, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy model option paths: %v", err)
	}

	var normalized map[string][]string
	if err = json.Unmarshal([]byte(NormalizeModelOptionAllowedPathsJSON(string(rawLegacy))), &normalized); err != nil {
		t.Fatalf("parse normalized model option paths: %v", err)
	}
	if !containsString(normalized["anthropic_messages"], "output_config.effort") {
		t.Fatalf("expected normalized anthropic allowlist to include output_config.effort, got %#v", normalized["anthropic_messages"])
	}
	if !containsString(normalized["openai_chat_completions"], "reasoning_summary") {
		t.Fatalf("expected normalized OpenAI Chat allowlist to include reasoning_summary, got %#v", normalized["openai_chat_completions"])
	}
	if !containsString(normalized["gemini_generate_content"], "thinkingConfig.thinkingLevel") {
		t.Fatalf("expected normalized gemini allowlist to include thinkingConfig.thinkingLevel, got %#v", normalized["gemini_generate_content"])
	}
	if !containsString(normalized["gemini_generate_content"], "thinkingConfig.includeThoughts") {
		t.Fatalf("expected normalized gemini allowlist to include thinkingConfig.includeThoughts, got %#v", normalized["gemini_generate_content"])
	}
	if !containsString(normalized["openai_responses"], "reasoning.mode") {
		t.Fatalf("expected normalized Responses allowlist to include reasoning.mode, got %#v", normalized["openai_responses"])
	}
	if !containsString(normalized["google_image_generation"], "generationConfig.thinkingConfig.thinkingLevel") {
		t.Fatalf("expected normalized Google image allowlist to include thinking level, got %#v", normalized["google_image_generation"])
	}
	if !containsString(normalized["gemini_interactions"], "generation_config.thinking_level") ||
		!containsString(normalized["gemini_interactions"], "response_format.aspect_ratio") ||
		!containsString(normalized["gemini_interactions"], "response_format.duration") ||
		!containsString(normalized["gemini_interactions"], "generation_config.video_config.task") {
		t.Fatalf("expected normalized Gemini Interactions allowlist to include advanced settings, got %#v", normalized["gemini_interactions"])
	}
	if !containsString(normalized["openai_video_generations"], "seconds") || !containsString(normalized["openai_video_generations"], "size") {
		t.Fatalf("expected normalized OpenAI video allowlist to include seconds and size, got %#v", normalized["openai_video_generations"])
	}
}

const preOpenRouterModelOptionAllowedPathsJSON = `{
  "anthropic_messages": [
    "speed",
    "top_k",
    "thinking.type",
    "thinking.budget_tokens",
    "output_config.effort"
  ],
  "default": [
    "temperature",
    "top_p",
    "max_tokens",
    "max_output_tokens",
    "max_completion_tokens",
    "stop",
    "response_format.type"
  ],
  "gemini_generate_content": [
    "generationConfig.temperature",
    "generationConfig.topP",
    "generationConfig.maxOutputTokens",
    "generationConfig.responseMimeType",
    "thinkingConfig.includeThoughts",
    "thinkingConfig.thinkingLevel"
  ],
  "google_image_generation": [
    "aspect_ratio",
    "aspectRatio",
    "image_size",
    "imageSize",
    "imageConfig.aspectRatio",
    "imageConfig.imageSize",
    "responseFormat.image.aspectRatio",
    "responseFormat.image.imageSize",
    "generationConfig.imageConfig.aspectRatio",
    "generationConfig.imageConfig.imageSize",
    "generationConfig.responseFormat.image.aspectRatio",
    "generationConfig.responseFormat.image.imageSize",
    "generationConfig.thinkingConfig.thinkingLevel"
  ],
  "openai_chat_completions": [
    "service_tier",
    "presence_penalty",
    "frequency_penalty",
    "reasoning_effort",
    "verbosity",
    "thinking.type",
    "stream_options.include_usage",
    "reasoning_summary"
  ],
  "openai_image_edits": [
    "background",
    "input_fidelity",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "user"
  ],
  "openai_image_generations": [
    "background",
    "moderation",
    "n",
    "output_compression",
    "output_format",
    "partial_images",
    "quality",
    "response_format",
    "size",
    "style",
    "user"
  ],
  "openai_responses": [
    "service_tier",
    "reasoning.effort",
    "reasoning.summary",
    "text.verbosity",
    "reasoning.mode"
  ],
  "openai_video_generations": [
    "seconds",
    "size"
  ],
  "xai_image": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "xai_image_edits": [
    "aspect_ratio",
    "n",
    "resolution",
    "response_format"
  ],
  "xai_responses": [
    "reasoning.effort"
  ]
}`

func TestNormalizeModelOptionAllowedPathsJSONUpgradesPreOpenRouterDefault(t *testing.T) {
	var normalized map[string][]string
	if err := json.Unmarshal([]byte(NormalizeModelOptionAllowedPathsJSON(preOpenRouterModelOptionAllowedPathsJSON)), &normalized); err != nil {
		t.Fatalf("parse normalized pre-OpenRouter model option paths: %v", err)
	}
	if !containsString(normalized["gemini_interactions"], "generation_config.thinking_level") ||
		!containsString(normalized["gemini_interactions"], "response_format.aspect_ratio") ||
		!containsString(normalized["gemini_interactions"], "response_format.duration") ||
		!containsString(normalized["gemini_interactions"], "generation_config.video_config.task") {
		t.Fatalf("expected pre-OpenRouter allowlist to gain Gemini Interactions settings, got %#v", normalized["gemini_interactions"])
	}

	var custom map[string][]string
	if err := json.Unmarshal([]byte(preOpenRouterModelOptionAllowedPathsJSON), &custom); err != nil {
		t.Fatalf("parse pre-OpenRouter model option paths: %v", err)
	}
	custom["openai_chat_completions"] = append(custom["openai_chat_completions"], "metadata.tenant")
	rawCustom, err := json.Marshal(custom)
	if err != nil {
		t.Fatalf("marshal customized pre-OpenRouter model option paths: %v", err)
	}
	var normalizedCustom map[string][]string
	if err = json.Unmarshal([]byte(NormalizeModelOptionAllowedPathsJSON(string(rawCustom))), &normalizedCustom); err != nil {
		t.Fatalf("parse normalized customized model option paths: %v", err)
	}
	if _, exists := normalizedCustom["gemini_interactions"]; exists {
		t.Fatalf("expected customized pre-OpenRouter allowlist to remain without Gemini Interactions")
	}
	if !containsString(normalizedCustom["openai_chat_completions"], "metadata.tenant") {
		t.Fatalf("expected customized path to be preserved, got %#v", normalizedCustom["openai_chat_completions"])
	}
}

func TestNormalizeModelOptionAllowedPathsJSONKeepsCustomPolicy(t *testing.T) {
	custom := `{"default":["temperature"],"anthropic_messages":["speed"]}`
	if got := NormalizeModelOptionAllowedPathsJSON(custom); got != custom {
		t.Fatalf("expected custom allowlist unchanged, got %s", got)
	}
}

func TestNormalizeModelOptionAllowedPathsJSONUpgradesLegacyGeminiInteractionsDuration(t *testing.T) {
	legacy := legacyGeminiInteractionsAllowedPaths()
	rawLegacy, err := json.Marshal(map[string][]string{"gemini_interactions": legacy})
	if err != nil {
		t.Fatalf("marshal legacy Gemini Interactions paths: %v", err)
	}

	var normalized map[string][]string
	if err = json.Unmarshal([]byte(NormalizeModelOptionAllowedPathsJSON(string(rawLegacy))), &normalized); err != nil {
		t.Fatalf("parse normalized Gemini Interactions paths: %v", err)
	}
	if !containsString(normalized["gemini_interactions"], "response_format.duration") ||
		containsString(normalized["gemini_interactions"], "responseFormat.duration") {
		t.Fatalf("expected legacy Gemini Interactions allowlist to gain the canonical duration path, got %#v", normalized["gemini_interactions"])
	}

	custom := append(append([]string{}, legacy...), "metadata.tenant")
	rawCustom, err := json.Marshal(map[string][]string{"gemini_interactions": custom})
	if err != nil {
		t.Fatalf("marshal custom Gemini Interactions paths: %v", err)
	}
	if got := NormalizeModelOptionAllowedPathsJSON(string(rawCustom)); got != string(rawCustom) {
		t.Fatalf("expected custom Gemini Interactions allowlist unchanged, got %s", got)
	}
}

func TestNormalizeModelOptionAllowedPathsJSONKeepsCustomizedOpenAIChatPolicy(t *testing.T) {
	custom := `{"openai_chat_completions":["service_tier","presence_penalty","frequency_penalty","reasoning_effort","verbosity","thinking.type","stream_options.include_usage","metadata.tenant"]}`
	if got := NormalizeModelOptionAllowedPathsJSON(custom); got != custom {
		t.Fatalf("expected customized OpenAI Chat allowlist unchanged, got %s", got)
	}
}

func TestNormalizeModelOptionAllowedPathsJSONKeepsCustomizedLegacyPolicyWithoutGeminiInteractions(t *testing.T) {
	var custom map[string][]string
	if err := json.Unmarshal([]byte(DefaultModelOptionAllowedPathsJSON()), &custom); err != nil {
		t.Fatalf("parse default model option paths: %v", err)
	}
	delete(custom, "gemini_interactions")
	custom["openai_chat_completions"] = append(custom["openai_chat_completions"], "metadata.tenant")
	rawCustom, err := json.Marshal(custom)
	if err != nil {
		t.Fatalf("marshal customized model option paths: %v", err)
	}
	if got := NormalizeModelOptionAllowedPathsJSON(string(rawCustom)); got != string(rawCustom) {
		t.Fatalf("expected customized legacy allowlist unchanged, got %s", got)
	}
}

func TestNormalizeModelOptionAllowedPathsJSONKeepsCurrentPolicyWithOpenRouterChatRemoved(t *testing.T) {
	var custom map[string][]string
	if err := json.Unmarshal([]byte(DefaultModelOptionAllowedPathsJSON()), &custom); err != nil {
		t.Fatalf("parse default model option paths: %v", err)
	}
	delete(custom, "gemini_interactions")
	delete(custom, "openrouter_chat_completions")
	rawCustom, err := json.Marshal(custom)
	if err != nil {
		t.Fatalf("marshal customized model option paths: %v", err)
	}
	var normalized map[string][]string
	if err = json.Unmarshal([]byte(NormalizeModelOptionAllowedPathsJSON(string(rawCustom))), &normalized); err != nil {
		t.Fatalf("parse normalized customized model option paths: %v", err)
	}
	if _, exists := normalized["gemini_interactions"]; exists {
		t.Fatalf("expected current allowlist with OpenRouter Chat removed to remain without Gemini Interactions")
	}
}

func TestDefaultNativeToolAllowedTypesIncludesGeminiTools(t *testing.T) {
	var rules map[string][]string
	if err := json.Unmarshal([]byte(DefaultNativeToolAllowedTypesJSON()), &rules); err != nil {
		t.Fatalf("parse native tool allowed types: %v", err)
	}
	if !containsString(rules["gemini_generate_content"], "google_search") {
		t.Fatalf("expected Gemini google_search in native tool defaults, got %#v", rules["gemini_generate_content"])
	}
	if !containsString(rules["gemini_generate_content"], "code_execution") {
		t.Fatalf("expected Gemini code_execution in native tool defaults, got %#v", rules["gemini_generate_content"])
	}
	if !containsString(rules["google_image_generation"], "google_search") {
		t.Fatalf("expected Google image google_search in native tool defaults, got %#v", rules["google_image_generation"])
	}
}

func TestLoadReadsSSRFAllowlistWithEnvironmentPriority(t *testing.T) {
	cleanupConfigEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := []byte(`
security:
  ssrf_protection_enabled: true
  ssrf_allowed_hosts: "new-api, host.docker.internal"
  ssrf_allowed_cidrs: "172.17.0.0/16, 10.20.0.0/16"
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := Load()
	if cfg.SSRFAllowedHosts != "new-api, host.docker.internal" {
		t.Fatalf("unexpected allowed hosts: %q", cfg.SSRFAllowedHosts)
	}
	if cfg.SSRFAllowedCIDRs != "172.17.0.0/16, 10.20.0.0/16" {
		t.Fatalf("unexpected allowed CIDRs: %q", cfg.SSRFAllowedCIDRs)
	}

	t.Setenv("SSRF_ALLOWED_HOSTS", "internal-api")
	t.Setenv("SSRF_ALLOWED_CIDRS", "192.168.50.0/24")
	cfg = Load()
	if cfg.SSRFAllowedHosts != "internal-api" || cfg.SSRFAllowedCIDRs != "192.168.50.0/24" {
		t.Fatalf("environment should override YAML allowlist: hosts=%q CIDRs=%q", cfg.SSRFAllowedHosts, cfg.SSRFAllowedCIDRs)
	}
}

func TestValidateRejectsInvalidSSRFAllowlist(t *testing.T) {
	for _, test := range []struct {
		name  string
		hosts string
		cidrs string
	}{
		{name: "host URL", hosts: "http://new-api:3000"},
		{name: "wildcard host", hosts: "*.internal"},
		{name: "metadata host", hosts: "metadata.google.internal"},
		{name: "invalid CIDR", cidrs: "172.17.0.1"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfigForEnv("dev")
			cfg.SSRFProtectionEnabled = true
			cfg.SSRFAllowedHosts = test.hosts
			cfg.SSRFAllowedCIDRs = test.cidrs
			if err := cfg.Validate(); !errors.Is(err, sharedsecurity.ErrInvalidOutboundPolicy) {
				t.Fatalf("expected invalid outbound policy, got %v", err)
			}
		})
	}
}

func TestValidateTurnstileSiteverifyURL(t *testing.T) {
	for _, test := range []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "default fallback", value: ""},
		{name: "public HTTPS", value: "https://turnstile.example.test/siteverify"},
		{name: "private administrator endpoint", value: "http://turnstile:8080/siteverify"},
		{name: "metadata endpoint", value: "http://169.254.169.254/latest/meta-data", wantErr: true},
		{name: "credentials", value: "http://user:password@turnstile:8080/siteverify", wantErr: true},
		{name: "unsupported scheme", value: "file:///etc/passwd", wantErr: true},
		{name: "relative URL", value: "/siteverify", wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := validConfigForEnv("dev")
			cfg.TurnstileSiteverifyURL = test.value
			err := cfg.Validate()
			if test.wantErr && err == nil {
				t.Fatal("expected invalid Turnstile endpoint to be rejected")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected Turnstile endpoint to be accepted, got %v", err)
			}
		})
	}
}

func TestTrustedAndStrictOutboundPoliciesRemainSeparated(t *testing.T) {
	cfg := validConfigForEnv("prod")
	cfg.SSRFProtectionEnabled = true
	cfg.SSRFAllowedHosts = "new-api"
	cfg.SSRFAllowedCIDRs = "172.17.0.0/16"

	trusted := cfg.TrustedOutboundPolicy()
	if err := sharedsecurity.ValidateOutboundHTTPURL("http://172.17.0.1:3000", trusted); err != nil {
		t.Fatalf("trusted integration policy should allow configured CIDR: %v", err)
	}
	strict := cfg.StrictOutboundPolicy()
	if err := sharedsecurity.ValidateOutboundHTTPURL("http://172.17.0.1:3000", strict); !errors.Is(err, sharedsecurity.ErrUnsafeOutboundURL) {
		t.Fatalf("external-content policy must not inherit the allowlist, got %v", err)
	}
}

func TestOutboundPolicyEnforcementBelongsToConfig(t *testing.T) {
	for _, test := range []struct {
		name    string
		env     string
		enabled bool
		blocked bool
	}{
		{name: "production enabled", env: "production", enabled: true, blocked: true},
		{name: "production disabled", env: "prod", enabled: false, blocked: false},
		{name: "development enabled", env: "development", enabled: true, blocked: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := Config{Env: test.env, SSRFProtectionEnabled: test.enabled}
			err := sharedsecurity.ValidateOutboundHTTPURL("http://127.0.0.1:8080", cfg.StrictOutboundPolicy())
			if test.blocked && !errors.Is(err, sharedsecurity.ErrUnsafeOutboundURL) {
				t.Fatalf("expected private target to be blocked, got %v", err)
			}
			if !test.blocked && err != nil {
				t.Fatalf("expected policy enforcement to be disabled, got %v", err)
			}
		})
	}
}

func TestLoadReadsBrandingFromConfig(t *testing.T) {
	cleanupConfigEnv(t)

	configPath := filepath.Join(t.TempDir(), "config.yaml")
	configBody := []byte(`
app:
  name: Example Backend
branding:
  title: Example Chat
  short_name: Example
  description: Example description
  logo_url: https://cdn.example.com/logo.svg
  favicon_url: https://cdn.example.com/favicon.ico
  pwa_icon_192_url: https://cdn.example.com/icon-192.png
  pwa_icon_512_url: https://cdn.example.com/icon-512.png
  pwa_maskable_icon_512_url: https://cdn.example.com/icon-maskable.png
  apple_touch_icon_180_url: https://cdn.example.com/apple-touch-icon.png
`)
	if err := os.WriteFile(configPath, configBody, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	t.Setenv("CONFIG_FILE", configPath)

	cfg := Load()
	if cfg.AppName != "Example Backend" || cfg.BrandTitle != "Example Chat" || cfg.BrandShortName != "Example" || cfg.BrandDescription != "Example description" {
		t.Fatalf("unexpected branding text: %+v", cfg)
	}
	if cfg.BrandLogoURL != "https://cdn.example.com/logo.svg" ||
		cfg.BrandFaviconURL != "https://cdn.example.com/favicon.ico" ||
		cfg.BrandPWAIcon192URL != "https://cdn.example.com/icon-192.png" ||
		cfg.BrandPWAIcon512URL != "https://cdn.example.com/icon-512.png" ||
		cfg.BrandPWAMaskableIcon512URL != "https://cdn.example.com/icon-maskable.png" ||
		cfg.BrandAppleTouchIcon180URL != "https://cdn.example.com/apple-touch-icon.png" {
		t.Fatalf("unexpected branding assets: %+v", cfg)
	}
}

func TestValidateAllowsOnlyDevAndProdEnvironment(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		wantErr bool
	}{
		{name: "dev", env: "dev"},
		{name: "prod", env: "prod"},
		{name: "development alias", env: "development"},
		{name: "production alias", env: "production"},
		{name: "staging rejected", env: "staging", wantErr: true},
		{name: "empty rejected", env: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfigForEnv(tt.env)
			err := cfg.Validate()
			if tt.wantErr && err == nil {
				t.Fatalf("Validate() error = nil, want error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("Validate() error = %v, want nil", err)
			}
		})
	}
}

func validConfigForEnv(env string) Config {
	return Config{
		Env:               env,
		StorageBackend:    "local",
		JWTSecret:         "test-jwt-secret-value",
		DataEncryptionKey: "test-data-encryption-key-value-32",
		CORSAllowOrigin:   "https://example.com",
		PublicAPIBaseURL:  "https://api.example.com",
		PublicWebBaseURL:  "https://example.com",
	}
}

func cleanupConfigEnv(t *testing.T) {
	t.Helper()
	keys := []string{
		"CONFIG_FILE",
		"APP_ENV",
		"FRONTEND_DIST_DIR",
		"STORAGE_ROOT_DIR",
		"GEOIP_DATABASE_PATH",
		"TURNSTILE_SITEVERIFY_URL",
		"SSRF_ALLOWED_HOSTS",
		"SSRF_ALLOWED_CIDRS",
		"POSTGRES_DSN",
	}
	for _, key := range keys {
		key := key
		original, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset %s: %v", key, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, original)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err = os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(previous)
	})
}

func assertPath(t *testing.T, label string, got string, want string) {
	t.Helper()
	gotPath := canonicalPath(t, got)
	wantPath := canonicalPath(t, want)
	if gotPath != wantPath {
		t.Fatalf("expected %s path %q, got %q", label, wantPath, gotPath)
	}
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	cleaned := filepath.Clean(path)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err == nil {
		return resolved
	}
	return cleaned
}
