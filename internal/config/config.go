package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
)

// RotationStrategy defines how API keys are rotated
type RotationStrategy string

const (
	RotationRoundRobin    RotationStrategy = "round_robin"
	RotationLeastUsed     RotationStrategy = "least_used"
	RotationCostOptimized RotationStrategy = "cost_optimized"
	RotationRandom        RotationStrategy = "random"
	RotationSingle        RotationStrategy = "single"
)

// APIKey represents a single API key configuration
type APIKey struct {
	Key        string    `yaml:"key" json:"key" mapstructure:"key"`
	Name       string    `yaml:"name" json:"name" mapstructure:"name"`
	RateLimit  int       `yaml:"rate_limit" json:"rate_limit" mapstructure:"rate_limit"`
	CostLimit  float64   `yaml:"cost_limit" json:"cost_limit" mapstructure:"cost_limit"`
	Enabled    bool      `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
	LastUsed   time.Time `yaml:"-" json:"-"` // runtime-only
	UsageCount int64     `yaml:"-" json:"-"`
	CostUsed   float64   `yaml:"-" json:"-"`
}

// ProviderConfig represents a provider's configuration
type ProviderConfig struct {
	APIKeys  []APIKey       `yaml:"api_keys" json:"api_keys" mapstructure:"api_keys"`
	Models   []ModelConfig  `yaml:"models" json:"models" mapstructure:"models"`
	Rotation RotationConfig `yaml:"rotation" json:"rotation" mapstructure:"rotation"`
}

// GetModelByName returns a model configuration by name
func (p *ProviderConfig) GetModelByName(name string) (*ModelConfig, error) {
	for _, model := range p.Models {
		if model.Name == name && model.Enabled {
			return &model, nil
		}
	}
	return nil, fmt.Errorf("model %s not found or not enabled", name)
}

// GetEnabledModels returns a list of enabled models
func (p *ProviderConfig) GetEnabledModels() []ModelConfig {
	var enabled []ModelConfig
	for _, model := range p.Models {
		if model.Enabled {
			enabled = append(enabled, model)
		}
	}
	return enabled
}

// IsValid checks if the API key is valid and enabled
func (k *APIKey) IsValid() bool {
	return k.Enabled && k.Key != "" && k.Name != ""
}

// CanUse checks if the key can be used based on limits
func (k *APIKey) CanUse() bool {
	if !k.IsValid() {
		return false
	}

	// Check daily cost limit
	if k.CostLimit > 0 && k.CostUsed >= k.CostLimit {
		return false
	}

	return true
}

// ModelConfig represents model-specific configuration
type ModelConfig struct {
	Name                  string  `yaml:"name" json:"name" mapstructure:"name"`
	InputCostPer1KTokens  float64 `yaml:"input_cost_per_1k_tokens" json:"input_cost_per_1k_tokens" mapstructure:"input_cost_per_1k_tokens"`
	OutputCostPer1KTokens float64 `yaml:"output_cost_per_1k_tokens" json:"output_cost_per_1k_tokens" mapstructure:"output_cost_per_1k_tokens"`
	MaxTokens             int     `yaml:"max_tokens" json:"max_tokens" mapstructure:"max_tokens"`
	Enabled               bool    `yaml:"enabled" json:"enabled" mapstructure:"enabled"`
}

// CalculateCost calculates the cost for given input/output tokens
func (m *ModelConfig) CalculateCost(inputTokens, outputTokens int) float64 {
	inputCost := (float64(inputTokens) / 1000.0) * m.InputCostPer1KTokens
	outputCost := (float64(outputTokens) / 1000.0) * m.OutputCostPer1KTokens
	return inputCost + outputCost
}

// RotationConfig defines key rotation behavior
type RotationConfig struct {
	Strategy        RotationStrategy `yaml:"strategy" json:"strategy"`
	Interval        string           `yaml:"interval" json:"interval"`
	HealthCheck     bool             `yaml:"health_check" json:"health_check"`
	FallbackEnabled bool             `yaml:"fallback_enabled" json:"fallback_enabled"`
}

// GetInterval returns the rotation interval as time.Duration
func (r *RotationConfig) GetInterval() (time.Duration, error) {
	if r.Interval == "" {
		return time.Hour, nil // default 1 hour
	}
	return time.ParseDuration(r.Interval)
}

// GetEnabledKeys returns only enabled API keys
func (p *ProviderConfig) GetEnabledKeys() []APIKey {
	var enabled []APIKey
	for _, key := range p.APIKeys {
		if key.IsValid() && key.CanUse() {
			enabled = append(enabled, key)
		}
	}
	return enabled
}

// GlobalConfig represents global configuration settings
type GlobalConfig struct {
	FallbackChain           []string         `yaml:"fallback_chain" json:"fallback_chain"`
	GlobalRateLimit         int              `yaml:"global_rate_limit" json:"global_rate_limit"`
	DailyCostLimit          float64          `yaml:"daily_cost_limit" json:"daily_cost_limit"`
	CostAlertThreshold      float64          `yaml:"cost_alert_threshold" json:"cost_alert_threshold"`
	EncryptKeys             bool             `yaml:"encrypt_keys" json:"encrypt_keys"`
	KeyValidation           bool             `yaml:"key_validation" json:"key_validation"`
	AuditLogging            bool             `yaml:"audit_logging" json:"audit_logging"`
	DefaultRotationStrategy RotationStrategy `yaml:"default_rotation_strategy" json:"default_rotation_strategy"`
	HealthCheckInterval     string           `yaml:"health_check_interval" json:"health_check_interval"`
	KeyTimeout              string           `yaml:"key_timeout" json:"key_timeout"`
}

// GetHealthCheckInterval returns the health check interval as time.Duration
func (g *GlobalConfig) GetHealthCheckInterval() (time.Duration, error) {
	if g.HealthCheckInterval == "" {
		return 5 * time.Minute, nil // default 5 minutes
	}
	return time.ParseDuration(g.HealthCheckInterval)
}

// GetKeyTimeout returns the key timeout as time.Duration
func (g *GlobalConfig) GetKeyTimeout() (time.Duration, error) {
	if g.KeyTimeout == "" {
		return 30 * time.Second, nil // default 30 seconds
	}
	return time.ParseDuration(g.KeyTimeout)
}

// Config represents the complete configuration structure
type Config struct {
	Providers map[string]ProviderConfig `yaml:"providers" json:"providers" mapstructure:"providers"`
	Global    GlobalConfig              `yaml:"global" json:"global" mapstructure:"global"`
}

// GetProvider returns a provider configuration by name
func (c *Config) GetProvider(name string) (*ProviderConfig, error) {
	provider, exists := c.Providers[name]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", name)
	}
	return &provider, nil
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set up viper
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Look for config in common locations
		viper.SetConfigName("gollmkit-config")
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.gollmkit")
		viper.AddConfigPath("/etc/gollmkit")
	}

	// Allow environment variable overrides
	viper.AutomaticEnv()
	viper.SetEnvPrefix("GOLLMKIT")

	// Read configuration
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, fmt.Errorf("config file not found: %w", err)
		}
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Load secrets from env (before validation)
	config.LoadFromEnvironment()

	// Validate after environment substitution
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// validateConfig performs basic validation on the configuration
func validateConfig(config *Config) error {
	if len(config.Providers) == 0 {
		return fmt.Errorf("at least one provider must be configured")
	}

	for providerName, provider := range config.Providers {

		if len(provider.APIKeys) == 0 {
			return fmt.Errorf("provider %s must have at least one API key", providerName)
		}

		if len(provider.Models) == 0 {
			return fmt.Errorf("provider %s must have at least one model", providerName)
		}

		// Validate API keys
		enabledKeyCount := 0
		for i, key := range provider.APIKeys {
			if key.Key == "" {
				return fmt.Errorf("provider %s: API key %d has empty key", providerName, i)
			}
			if key.Name == "" {
				return fmt.Errorf("provider %s: API key %d has empty name", providerName, i)
			}
			if key.Enabled {
				enabledKeyCount++
			}
		}

		if enabledKeyCount == 0 {
			return fmt.Errorf("provider %s must have at least one enabled API key", providerName)
		}

		// Validate models
		enabledModelCount := 0
		for i, model := range provider.Models {
			if model.Name == "" {
				return fmt.Errorf("provider %s: model %d has empty name", providerName, i)
			}
			if model.Enabled {
				enabledModelCount++
			}
		}

		if enabledModelCount == 0 {
			return fmt.Errorf("provider %s must have at least one enabled model", providerName)
		}
	}

	return nil
}

// SaveConfig saves the configuration to a file
func (c *Config) SaveConfig(configPath string) error {
	viper.SetConfigFile(configPath)

	// Set the config values
	viper.Set("providers", c.Providers)
	viper.Set("global", c.Global)

	return viper.WriteConfig()
}

// LoadFromEnvironment loads sensitive values from environment variables
func (c *Config) LoadFromEnvironment() {
	for providerName, provider := range c.Providers {
		for i, key := range provider.APIKeys {
			envKey := fmt.Sprintf("GOLLM_%s_API_KEY_%s",
				fmt.Sprintf("%s", providerName),
				fmt.Sprintf("%s", key.Name))

			if envValue := os.Getenv(envKey); envValue != "" {
				provider.APIKeys[i].Key = envValue
			}
		}
		c.Providers[providerName] = provider
	}
}
