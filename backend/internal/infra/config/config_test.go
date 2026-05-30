package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
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
	legacy["gemini_generate_content"] = removeString(legacy["gemini_generate_content"], "thinkingConfig.thinkingLevel")
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
	if !containsString(normalized["gemini_generate_content"], "thinkingConfig.thinkingLevel") {
		t.Fatalf("expected normalized gemini allowlist to include thinkingConfig.thinkingLevel, got %#v", normalized["gemini_generate_content"])
	}
	if !containsString(normalized["gemini_generate_content"], "thinkingConfig.includeThoughts") {
		t.Fatalf("expected normalized gemini allowlist to include thinkingConfig.includeThoughts, got %#v", normalized["gemini_generate_content"])
	}
	if !containsString(normalized["openai_video_generations"], "seconds") || !containsString(normalized["openai_video_generations"], "size") {
		t.Fatalf("expected normalized OpenAI video allowlist to include seconds and size, got %#v", normalized["openai_video_generations"])
	}
}

func TestNormalizeModelOptionAllowedPathsJSONKeepsCustomPolicy(t *testing.T) {
	custom := `{"default":["temperature"],"anthropic_messages":["speed"]}`
	if got := NormalizeModelOptionAllowedPathsJSON(custom); got != custom {
		t.Fatalf("expected custom allowlist unchanged, got %s", got)
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
