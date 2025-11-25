package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	// Check AI settings
	if cfg.Model != "claude-3-5-sonnet-latest" {
		t.Errorf("expected default model to be claude-3-5-sonnet-latest, got %s", cfg.Model)
	}
	if cfg.Temperature != 0.2 {
		t.Errorf("expected default temperature to be 0.2, got %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 500 {
		t.Errorf("expected default max_tokens to be 500, got %d", cfg.MaxTokens)
	}

	// Check behavior settings
	if cfg.AlwaysScope != false {
		t.Error("expected default always_scope to be false")
	}
	if cfg.Verbose != false {
		t.Error("expected default verbose to be false")
	}
	if cfg.SkipSecretScan != false {
		t.Error("expected default skip_secret_scan to be false")
	}

	// Check UI settings
	if cfg.ColorOutput != true {
		t.Error("expected default color_output to be true")
	}
	if cfg.Interactive != true {
		t.Error("expected default interactive to be true")
	}
}

func TestParseBool(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"True", true},
		{"TRUE", true},
		{"yes", true},
		{"Yes", true},
		{"YES", true},
		{"on", true},
		{"On", true},
		{"ON", true},
		{"0", false},
		{"false", false},
		{"False", false},
		{"FALSE", false},
		{"no", false},
		{"No", false},
		{"NO", false},
		{"off", false},
		{"Off", false},
		{"OFF", false},
		{"", false},
		{"invalid", false},
	}

	for _, tc := range tests {
		result := parseBool(tc.input)
		if result != tc.expected {
			t.Errorf("parseBool(%q) = %v, expected %v", tc.input, result, tc.expected)
		}
	}
}

func TestEnvOverrides(t *testing.T) {
	// Save and restore environment
	envVars := []string{
		"CMT_MODEL",
		"CMT_TEMPERATURE",
		"CMT_MAX_TOKENS",
		"CMT_ALWAYS_SCOPE",
		"CMT_VERBOSE",
		"CMT_SKIP_SECRET_SCAN",
		"CMT_CUSTOM_PROMPT_PATH",
		"CMT_COLOR_OUTPUT",
		"CMT_INTERACTIVE",
	}

	oldEnv := make(map[string]string)
	for _, key := range envVars {
		oldEnv[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		for key, value := range oldEnv {
			if value != "" {
				os.Setenv(key, value)
			}
		}
	}()

	// Set test environment variables
	os.Setenv("CMT_MODEL", "haiku-4.5")
	os.Setenv("CMT_TEMPERATURE", "0.8")
	os.Setenv("CMT_MAX_TOKENS", "1000")
	os.Setenv("CMT_ALWAYS_SCOPE", "true")
	os.Setenv("CMT_VERBOSE", "yes")
	os.Setenv("CMT_SKIP_SECRET_SCAN", "1")
	os.Setenv("CMT_CUSTOM_PROMPT_PATH", "/custom/prompt.txt")
	os.Setenv("CMT_COLOR_OUTPUT", "false")
	os.Setenv("CMT_INTERACTIVE", "no")

	cfg := Default()
	applyEnvOverrides(cfg)

	// Check AI settings
	if cfg.Model != "haiku-4.5" {
		t.Errorf("expected model to be haiku-4.5, got %s", cfg.Model)
	}
	if cfg.Temperature != 0.8 {
		t.Errorf("expected temperature to be 0.8, got %f", cfg.Temperature)
	}
	if cfg.MaxTokens != 1000 {
		t.Errorf("expected max_tokens to be 1000, got %d", cfg.MaxTokens)
	}

	// Check behavior settings
	if cfg.AlwaysScope != true {
		t.Error("expected always_scope to be true")
	}
	if cfg.Verbose != true {
		t.Error("expected verbose to be true")
	}
	if cfg.SkipSecretScan != true {
		t.Error("expected skip_secret_scan to be true")
	}
	if cfg.CustomPromptPath != "/custom/prompt.txt" {
		t.Errorf("expected custom_prompt_path to be /custom/prompt.txt, got %s", cfg.CustomPromptPath)
	}

	// Check UI settings
	if cfg.ColorOutput != false {
		t.Error("expected color_output to be false")
	}
	if cfg.Interactive != false {
		t.Error("expected interactive to be false")
	}
}

func TestGetSet(t *testing.T) {
	cfg := Default()

	// Test Get
	tests := []struct {
		key      string
		expected interface{}
		hasError bool
	}{
		{"model", "claude-3-5-sonnet-latest", false},
		{"temperature", 0.2, false},
		{"max_tokens", 500, false},
		{"always_scope", false, false},
		{"verbose", false, false},
		{"skip_secret_scan", false, false},
		{"custom_prompt_path", "", false},
		{"color_output", true, false},
		{"interactive", true, false},
		{"invalid_key", nil, true},
	}

	for _, tc := range tests {
		value, err := cfg.Get(tc.key)
		if tc.hasError {
			if err == nil {
				t.Errorf("expected error for key %q", tc.key)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for key %q: %v", tc.key, err)
			}
			if value != tc.expected {
				t.Errorf("Get(%q) = %v, expected %v", tc.key, value, tc.expected)
			}
		}
	}

	// Test Set
	setTests := []struct {
		key      string
		value    string
		expected interface{}
		hasError bool
	}{
		{"model", "opus-3", "opus-3", false},
		{"temperature", "0.5", 0.5, false},
		{"temperature", "invalid", 0.5, true},
		{"max_tokens", "750", 750, false},
		{"max_tokens", "not-a-number", 750, true},
		{"always_scope", "true", true, false},
		{"verbose", "yes", true, false},
		{"skip_secret_scan", "1", true, false},
		{"custom_prompt_path", "/new/path", "/new/path", false},
		{"color_output", "false", false, false},
		{"interactive", "no", false, false},
		{"invalid_key", "value", nil, true},
	}

	for _, tc := range setTests {
		err := cfg.Set(tc.key, tc.value)
		if tc.hasError {
			if err == nil {
				t.Errorf("expected error for Set(%q, %q)", tc.key, tc.value)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error for Set(%q, %q): %v", tc.key, tc.value, err)
			} else {
				// Verify the value was set
				actual, _ := cfg.Get(tc.key)
				if actual != tc.expected {
					t.Errorf("after Set(%q, %q), Get returned %v, expected %v", tc.key, tc.value, actual, tc.expected)
				}
			}
		}
	}
}

func TestSaveAndLoad(t *testing.T) {
	// Create a temp directory for testing
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, ".cmt.yml")

	// Create a config with custom values
	cfg := &Config{
		Model:            "test-model",
		Temperature:      0.7,
		MaxTokens:        750,
		AlwaysScope:      true,
		Verbose:          true,
		SkipSecretScan:   true,
		CustomPromptPath: "/test/prompt",
		ColorOutput:      false,
		Interactive:      false,
	}

	// Change to temp directory for local save
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tempDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)

	// Save config locally
	if err := cfg.Save(false); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Check file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("config file was not created")
	}

	// Load config
	loaded := Default()
	if err := loadFromFile(".cmt.yml", loaded); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify loaded values
	if loaded.Model != cfg.Model {
		t.Errorf("loaded model = %s, expected %s", loaded.Model, cfg.Model)
	}
	if loaded.Temperature != cfg.Temperature {
		t.Errorf("loaded temperature = %f, expected %f", loaded.Temperature, cfg.Temperature)
	}
	if loaded.MaxTokens != cfg.MaxTokens {
		t.Errorf("loaded max_tokens = %d, expected %d", loaded.MaxTokens, cfg.MaxTokens)
	}
	if loaded.AlwaysScope != cfg.AlwaysScope {
		t.Errorf("loaded always_scope = %v, expected %v", loaded.AlwaysScope, cfg.AlwaysScope)
	}
	if loaded.Verbose != cfg.Verbose {
		t.Errorf("loaded verbose = %v, expected %v", loaded.Verbose, cfg.Verbose)
	}
	if loaded.SkipSecretScan != cfg.SkipSecretScan {
		t.Errorf("loaded skip_secret_scan = %v, expected %v", loaded.SkipSecretScan, cfg.SkipSecretScan)
	}
	if loaded.CustomPromptPath != cfg.CustomPromptPath {
		t.Errorf("loaded custom_prompt_path = %s, expected %s", loaded.CustomPromptPath, cfg.CustomPromptPath)
	}
	if loaded.ColorOutput != cfg.ColorOutput {
		t.Errorf("loaded color_output = %v, expected %v", loaded.ColorOutput, cfg.ColorOutput)
	}
	if loaded.Interactive != cfg.Interactive {
		t.Errorf("loaded interactive = %v, expected %v", loaded.Interactive, cfg.Interactive)
	}
}

func TestGlobalSave(t *testing.T) {
	// Create a temp directory to act as home
	tempHome := t.TempDir()

	// Save original HOME and restore later
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", oldHome)

	cfg := Default()
	cfg.Model = "global-test-model"

	// Save globally
	if err := cfg.Save(true); err != nil {
		t.Fatalf("failed to save global config: %v", err)
	}

	// Check file exists (XDG Base Directory)
	globalPath := filepath.Join(tempHome, ".config", "cmt", "config.yml")
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		t.Fatal("global config file was not created")
	}

	// Load and verify
	loaded := Default()
	if err := loadFromFile(globalPath, loaded); err != nil {
		t.Fatalf("failed to load global config: %v", err)
	}

	if loaded.Model != "global-test-model" {
		t.Errorf("loaded model = %s, expected global-test-model", loaded.Model)
	}
}

func TestLoadConfigPrecedence(t *testing.T) {
	// Create temp directories
	tempHome := t.TempDir()
	tempWork := t.TempDir()

	// Save original environment
	oldHome := os.Getenv("HOME")
	oldModel := os.Getenv("CMT_MODEL")
	oldWd, _ := os.Getwd()

	// Clean up environment
	os.Setenv("HOME", tempHome)
	os.Unsetenv("CMT_MODEL")
	os.Chdir(tempWork)

	defer func() {
		os.Setenv("HOME", oldHome)
		if oldModel != "" {
			os.Setenv("CMT_MODEL", oldModel)
		}
		os.Chdir(oldWd)
	}()

	// Create global config
	globalCfg := &Config{Model: "global-model"}
	globalCfg.Save(true)

	// Test 1: Only global config
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Model != "global-model" {
		t.Errorf("expected global-model, got %s", cfg.Model)
	}

	// Create local config
	localCfg := &Config{Model: "local-model"}
	localCfg.Save(false)

	// Test 2: Local overrides global
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Model != "local-model" {
		t.Errorf("expected local-model, got %s", cfg.Model)
	}

	// Set environment variable
	os.Setenv("CMT_MODEL", "env-model")

	// Test 3: Environment overrides local
	cfg, err = LoadConfig()
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	if cfg.Model != "env-model" {
		t.Errorf("expected env-model, got %s", cfg.Model)
	}
}
