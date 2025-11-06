package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config represents the configuration structure for cmt.
type Config struct {
	// AI settings
	Model       string  `yaml:"model"`
	Temperature float64 `yaml:"temperature"`
	MaxTokens   int     `yaml:"max_tokens"`

	// Behavior settings
	AlwaysScope      bool   `yaml:"always_scope"`
	Verbose          bool   `yaml:"verbose"`
	SkipSecretScan   bool   `yaml:"skip_secret_scan"`
	CustomPromptPath string `yaml:"custom_prompt_path"`

	// UI settings
	ColorOutput bool   `yaml:"color_output"`
	Interactive bool   `yaml:"interactive"`
	EditorMode  string `yaml:"editor_mode"`  // "inline" or "external"

	// Preprocessing settings
	MaxDiffTokens   int  `yaml:"max_diff_tokens"`
	FilterBinary    bool `yaml:"filter_binary"`
	FilterMinified  bool `yaml:"filter_minified"`
	FilterGenerated bool `yaml:"filter_generated"`

	// Absorb settings
	AbsorbStrategy    string  `yaml:"absorb_strategy"`     // "fixup" (default) or "direct"
	AbsorbRange       string  `yaml:"absorb_range"`        // "unpushed" (default) or "branch-point"
	AbsorbAmbiguity   string  `yaml:"absorb_ambiguity"`    // "interactive" (default) or "best-match"
	AbsorbAutoCommit  bool    `yaml:"absorb_auto_commit"`  // true (default) - create commit for unmatched
	AbsorbConfidence  float64 `yaml:"absorb_confidence"`   // 0.7 (default) - min confidence threshold
}

// Default returns the default configuration.
func Default() *Config {
	return &Config{
		Model:           "claude-3-5-sonnet-latest",
		Temperature:     0.2,
		MaxTokens:       500,
		AlwaysScope:     false,
		Verbose:         false,
		SkipSecretScan:  false,
		ColorOutput:     true,
		Interactive:     true,
		EditorMode:      "inline",
		MaxDiffTokens:   16384,
		FilterBinary:    true,
		FilterMinified:  true,
		FilterGenerated: true,
		AbsorbStrategy:   "fixup",
		AbsorbRange:      "unpushed",
		AbsorbAmbiguity:  "interactive",
		AbsorbAutoCommit: true,
		AbsorbConfidence: 0.7,
	}
}

// LoadConfig loads configuration from multiple sources with the following precedence:
// 1. Environment variables (highest priority)
// 2. Local config file (.cmt.yml in current directory)
// 3. Global config file (~/.config/cmt/config.yml - XDG Base Directory)
// 4. Default values (lowest priority)
func LoadConfig() (*Config, error) {
	// Start with defaults
	config := Default()

	// Try to load global config (XDG Base Directory)
	homeDir, err := os.UserHomeDir()
	if err == nil {
		globalConfigPath := filepath.Join(homeDir, ".config", "cmt", "config.yml")
		if err := loadFromFile(globalConfigPath, config); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("error loading global config: %w", err)
		}
	}

	// Try to load local config
	localConfigPath := ".cmt.yml"
	if err := loadFromFile(localConfigPath, config); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("error loading local config: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(config)

	return config, nil
}

// loadFromFile loads configuration from a YAML file.
func loadFromFile(path string, config *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("error parsing config file %s: %w", path, err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to the config.
func applyEnvOverrides(config *Config) {
	// AI settings
	if model := os.Getenv("CMT_MODEL"); model != "" {
		config.Model = model
	}
	if temp := os.Getenv("CMT_TEMPERATURE"); temp != "" {
		if val, err := strconv.ParseFloat(temp, 64); err == nil {
			config.Temperature = val
		}
	}
	if maxTokens := os.Getenv("CMT_MAX_TOKENS"); maxTokens != "" {
		if val, err := strconv.Atoi(maxTokens); err == nil {
			config.MaxTokens = val
		}
	}

	// Behavior settings
	if alwaysScope := os.Getenv("CMT_ALWAYS_SCOPE"); alwaysScope != "" {
		config.AlwaysScope = parseBool(alwaysScope)
	}
	if verbose := os.Getenv("CMT_VERBOSE"); verbose != "" {
		config.Verbose = parseBool(verbose)
	}
	if skipScan := os.Getenv("CMT_SKIP_SECRET_SCAN"); skipScan != "" {
		config.SkipSecretScan = parseBool(skipScan)
	}
	if customPrompt := os.Getenv("CMT_CUSTOM_PROMPT_PATH"); customPrompt != "" {
		config.CustomPromptPath = customPrompt
	}

	// UI settings
	if colorOutput := os.Getenv("CMT_COLOR_OUTPUT"); colorOutput != "" {
		config.ColorOutput = parseBool(colorOutput)
	}
	if interactive := os.Getenv("CMT_INTERACTIVE"); interactive != "" {
		config.Interactive = parseBool(interactive)
	}
	if editorMode := os.Getenv("CMT_EDITOR_MODE"); editorMode != "" {
		config.EditorMode = editorMode
	}

	// Preprocessing settings
	if maxDiffTokens := os.Getenv("CMT_MAX_DIFF_TOKENS"); maxDiffTokens != "" {
		if val, err := strconv.Atoi(maxDiffTokens); err == nil {
			config.MaxDiffTokens = val
		}
	}
	if filterBinary := os.Getenv("CMT_FILTER_BINARY"); filterBinary != "" {
		config.FilterBinary = parseBool(filterBinary)
	}
	if filterMinified := os.Getenv("CMT_FILTER_MINIFIED"); filterMinified != "" {
		config.FilterMinified = parseBool(filterMinified)
	}
	if filterGenerated := os.Getenv("CMT_FILTER_GENERATED"); filterGenerated != "" {
		config.FilterGenerated = parseBool(filterGenerated)
	}

	// Absorb settings
	if absorbStrategy := os.Getenv("CMT_ABSORB_STRATEGY"); absorbStrategy != "" {
		config.AbsorbStrategy = absorbStrategy
	}
	if absorbRange := os.Getenv("CMT_ABSORB_RANGE"); absorbRange != "" {
		config.AbsorbRange = absorbRange
	}
	if absorbAmbiguity := os.Getenv("CMT_ABSORB_AMBIGUITY"); absorbAmbiguity != "" {
		config.AbsorbAmbiguity = absorbAmbiguity
	}
	if absorbAutoCommit := os.Getenv("CMT_ABSORB_AUTO_COMMIT"); absorbAutoCommit != "" {
		config.AbsorbAutoCommit = parseBool(absorbAutoCommit)
	}
	if absorbConfidence := os.Getenv("CMT_ABSORB_CONFIDENCE"); absorbConfidence != "" {
		if val, err := strconv.ParseFloat(absorbConfidence, 64); err == nil {
			config.AbsorbConfidence = val
		}
	}
}

// parseBool parses a string as a boolean value.
func parseBool(s string) bool {
	switch s {
	case "1", "true", "True", "TRUE", "yes", "Yes", "YES", "on", "On", "ON":
		return true
	default:
		return false
	}
}

// Save saves the configuration to a file.
// If global is true, saves to ~/.config/gac/config.yml (XDG Base Directory), otherwise saves to .gac.yml
func (c *Config) Save(global bool) error {
	var configPath string

	if global {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("error getting home directory: %w", err)
		}
		configDir := filepath.Join(homeDir, ".config", "cmt")
		// Create config directory if it doesn't exist
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("error creating config directory: %w", err)
		}
		configPath = filepath.Join(configDir, "config.yml")
	} else {
		configPath = ".cmt.yml"
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("error marshaling config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// Get retrieves a configuration value by key.
func (c *Config) Get(key string) (interface{}, error) {
	switch key {
	// AI settings
	case "model":
		return c.Model, nil
	case "temperature":
		return c.Temperature, nil
	case "max_tokens":
		return c.MaxTokens, nil
	// Behavior settings
	case "always_scope":
		return c.AlwaysScope, nil
	case "verbose":
		return c.Verbose, nil
	case "skip_secret_scan":
		return c.SkipSecretScan, nil
	case "custom_prompt_path":
		return c.CustomPromptPath, nil
	// UI settings
	case "color_output":
		return c.ColorOutput, nil
	case "interactive":
		return c.Interactive, nil
	case "editor_mode":
		return c.EditorMode, nil
	// Preprocessing settings
	case "max_diff_tokens":
		return c.MaxDiffTokens, nil
	case "filter_binary":
		return c.FilterBinary, nil
	case "filter_minified":
		return c.FilterMinified, nil
	case "filter_generated":
		return c.FilterGenerated, nil
	// Absorb settings
	case "absorb_strategy":
		return c.AbsorbStrategy, nil
	case "absorb_range":
		return c.AbsorbRange, nil
	case "absorb_ambiguity":
		return c.AbsorbAmbiguity, nil
	case "absorb_auto_commit":
		return c.AbsorbAutoCommit, nil
	case "absorb_confidence":
		return c.AbsorbConfidence, nil
	default:
		return nil, fmt.Errorf("unknown configuration key: %s", key)
	}
}

// Set updates a configuration value by key.
func (c *Config) Set(key string, value string) error {
	switch key {
	// AI settings
	case "model":
		c.Model = value
	case "temperature":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid temperature value: %s", value)
		}
		c.Temperature = val
	case "max_tokens":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_tokens value: %s", value)
		}
		c.MaxTokens = val
	// Behavior settings
	case "always_scope":
		c.AlwaysScope = parseBool(value)
	case "verbose":
		c.Verbose = parseBool(value)
	case "skip_secret_scan":
		c.SkipSecretScan = parseBool(value)
	case "custom_prompt_path":
		c.CustomPromptPath = value
	// UI settings
	case "color_output":
		c.ColorOutput = parseBool(value)
	case "interactive":
		c.Interactive = parseBool(value)
	case "editor_mode":
		// Validate editor mode value
		if value != "inline" && value != "external" {
			return fmt.Errorf("invalid editor_mode value: %s (must be inline or external)", value)
		}
		c.EditorMode = value
	// Preprocessing settings
	case "max_diff_tokens":
		val, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("invalid max_diff_tokens value: %s", value)
		}
		c.MaxDiffTokens = val
	case "filter_binary":
		c.FilterBinary = parseBool(value)
	case "filter_minified":
		c.FilterMinified = parseBool(value)
	case "filter_generated":
		c.FilterGenerated = parseBool(value)
	// Absorb settings
	case "absorb_strategy":
		if value != "fixup" && value != "direct" {
			return fmt.Errorf("invalid absorb_strategy value: %s (must be fixup or direct)", value)
		}
		c.AbsorbStrategy = value
	case "absorb_range":
		if value != "unpushed" && value != "branch-point" {
			return fmt.Errorf("invalid absorb_range value: %s (must be unpushed or branch-point)", value)
		}
		c.AbsorbRange = value
	case "absorb_ambiguity":
		if value != "interactive" && value != "best-match" {
			return fmt.Errorf("invalid absorb_ambiguity value: %s (must be interactive or best-match)", value)
		}
		c.AbsorbAmbiguity = value
	case "absorb_auto_commit":
		c.AbsorbAutoCommit = parseBool(value)
	case "absorb_confidence":
		val, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid absorb_confidence value: %s", value)
		}
		if val < 0.0 || val > 1.0 {
			return fmt.Errorf("absorb_confidence must be between 0.0 and 1.0")
		}
		c.AbsorbConfidence = val
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}
	return nil
}