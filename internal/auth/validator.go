package auth

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// KeyValidator handles API key validation for different providers
type KeyValidator struct {
	httpClient *http.Client
}

// NewKeyValidator creates a new key validator
func NewKeyValidator() *KeyValidator {
	return &KeyValidator{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ValidationResult represents the result of key validation
type ValidationResult struct {
	Valid     bool                   `json:"valid"`
	Provider  string                 `json:"provider"`
	KeyName   string                 `json:"key_name"`
	Message   string                 `json:"message,omitempty"`
	CheckedAt time.Time              `json:"checked_at"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ValidateKey validates an API key for a specific provider
func (kv *KeyValidator) ValidateKey(ctx context.Context, provider, keyName, apiKey string) (*ValidationResult, error) {
	result := &ValidationResult{
		Provider:  provider,
		KeyName:   keyName,
		CheckedAt: time.Now(),
		Metadata:  make(map[string]interface{}),
	}

	// First, check key format
	if !kv.isValidKeyFormat(provider, apiKey) {
		result.Valid = false
		result.Message = "Invalid key format"
		return result, nil
	}

	// Then, perform live validation if possible
	switch strings.ToLower(provider) {
	case "openai":
		return kv.validateOpenAIKey(ctx, result, apiKey)
	case "anthropic":
		return kv.validateAnthropicKey(ctx, result, apiKey)
	case "gemini", "google":
		return kv.validateGeminiKey(ctx, result, apiKey)
	default:
		result.Valid = true
		result.Message = "Format validation passed (live validation not implemented)"
		return result, nil
	}
}

// isValidKeyFormat checks if the API key format is valid for the provider
func (kv *KeyValidator) isValidKeyFormat(provider, apiKey string) bool {
	switch strings.ToLower(provider) {
	case "openai":
		// OpenAI keys typically start with "sk-" and are 51 characters long
		// New format: sk-proj-... (longer)
		openAIPattern := regexp.MustCompile(`^sk-[a-zA-Z0-9]{48}$|^sk-proj-[a-zA-Z0-9-_]{43,}$`)
		return openAIPattern.MatchString(apiKey)

	case "anthropic":
		// Anthropic keys start with "sk-ant-"
		anthropicPattern := regexp.MustCompile(`^sk-ant-[a-zA-Z0-9-_]{93,}$`)
		return anthropicPattern.MatchString(apiKey)

	case "gemini", "google":
		// Google AI keys typically start with "AIza"
		geminiPattern := regexp.MustCompile(`^AIza[a-zA-Z0-9_-]{35}$`)
		return geminiPattern.MatchString(apiKey)

	default:
		// For unknown providers, just check it's not empty
		return strings.TrimSpace(apiKey) != ""
	}
}

// validateOpenAIKey validates an OpenAI API key by making a test request
func (kv *KeyValidator) validateOpenAIKey(ctx context.Context, result *ValidationResult, apiKey string) (*ValidationResult, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.openai.com/v1/models", nil)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("User-Agent", "GoLLM/1.0")

	resp, err := kv.httpClient.Do(req)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("Request failed: %s", err.Error())
		return result, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		result.Valid = true
		result.Message = "Key is valid and active"

		// Try to extract organization info from headers
		if org := resp.Header.Get("openai-organization"); org != "" {
			result.Metadata["organization"] = org
		}

	case http.StatusUnauthorized:
		result.Valid = false
		result.Message = "Invalid or expired API key"

	case http.StatusTooManyRequests:
		result.Valid = true
		result.Message = "Key is valid but rate limited"
		result.Metadata["rate_limited"] = true

	case http.StatusForbidden:
		result.Valid = false
		result.Message = "Key lacks required permissions"

	default:
		result.Valid = false
		result.Message = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
	}

	return result, nil
}

// validateAnthropicKey validates an Anthropic API key
func (kv *KeyValidator) validateAnthropicKey(ctx context.Context, result *ValidationResult, apiKey string) (*ValidationResult, error) {
	// Anthropic doesn't have a models endpoint, so we'll make a minimal completion request
	reqBody := strings.NewReader(`{
		"model": "claude-3-haiku-20240307",
		"max_tokens": 1,
		"messages": [{"role": "user", "content": "Hi"}]
	}`)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", reqBody)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "GoLLM/1.0")

	resp, err := kv.httpClient.Do(req)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("Request failed: %s", err.Error())
		return result, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		result.Valid = true
		result.Message = "Key is valid and active"

	case http.StatusUnauthorized:
		result.Valid = false
		result.Message = "Invalid or expired API key"

	case http.StatusTooManyRequests:
		result.Valid = true
		result.Message = "Key is valid but rate limited"
		result.Metadata["rate_limited"] = true

	case http.StatusForbidden:
		result.Valid = false
		result.Message = "Key lacks required permissions"

	case http.StatusBadRequest:
		// Bad request might still mean the key is valid
		result.Valid = true
		result.Message = "Key appears valid (request format issue)"

	default:
		result.Valid = false
		result.Message = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
	}

	return result, nil
}

// validateGeminiKey validates a Google Gemini API key
func (kv *KeyValidator) validateGeminiKey(ctx context.Context, result *ValidationResult, apiKey string) (*ValidationResult, error) {
	// Use the models list endpoint for validation
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models?key=%s", apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return result, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "GoLLM/1.0")

	resp, err := kv.httpClient.Do(req)
	if err != nil {
		result.Valid = false
		result.Message = fmt.Sprintf("Request failed: %s", err.Error())
		return result, nil
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		result.Valid = true
		result.Message = "Key is valid and active"

	case http.StatusUnauthorized, http.StatusForbidden:
		result.Valid = false
		result.Message = "Invalid or expired API key"

	case http.StatusTooManyRequests:
		result.Valid = true
		result.Message = "Key is valid but rate limited"
		result.Metadata["rate_limited"] = true

	case http.StatusBadRequest:
		result.Valid = false
		result.Message = "Invalid request (possibly malformed key)"

	default:
		result.Valid = false
		result.Message = fmt.Sprintf("Unexpected status code: %d", resp.StatusCode)
	}

	return result, nil
}

// ValidateAllKeys validates all keys for all providers in the configuration
func (kv *KeyValidator) ValidateAllKeys(ctx context.Context, keyStore KeyStore, providers map[string][]string) (map[string]map[string]*ValidationResult, error) {
	results := make(map[string]map[string]*ValidationResult)

	for provider, keyNames := range providers {
		results[provider] = make(map[string]*ValidationResult)

		for _, keyName := range keyNames {
			apiKey, err := keyStore.GetKey(ctx, provider, keyName)
			if err != nil {
				results[provider][keyName] = &ValidationResult{
					Valid:     false,
					Provider:  provider,
					KeyName:   keyName,
					Message:   fmt.Sprintf("Failed to retrieve key: %s", err.Error()),
					CheckedAt: time.Now(),
				}
				continue
			}

			result, err := kv.ValidateKey(ctx, provider, keyName, apiKey)
			if err != nil {
				results[provider][keyName] = &ValidationResult{
					Valid:     false,
					Provider:  provider,
					KeyName:   keyName,
					Message:   fmt.Sprintf("Validation error: %s", err.Error()),
					CheckedAt: time.Now(),
				}
				continue
			}

			results[provider][keyName] = result
		}
	}

	return results, nil
}

// HealthChecker performs periodic health checks on API keys
type HealthChecker struct {
	validator *KeyValidator
	keyStore  KeyStore
	interval  time.Duration
	stopCh    chan struct{}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(keyStore KeyStore, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		validator: NewKeyValidator(),
		keyStore:  keyStore,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

// Start begins periodic health checking
func (hc *HealthChecker) Start(ctx context.Context, providers map[string][]string) {
	ticker := time.NewTicker(hc.interval)
	defer ticker.Stop()

	// Perform initial health check
	go hc.performHealthCheck(ctx, providers)

	for {
		select {
		case <-ticker.C:
			go hc.performHealthCheck(ctx, providers)
		case <-hc.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// performHealthCheck performs a health check on all keys
func (hc *HealthChecker) performHealthCheck(ctx context.Context, providers map[string][]string) {
	results, err := hc.validator.ValidateAllKeys(ctx, hc.keyStore, providers)
	if err != nil {
		return // Log error in production
	}

	// Update health status in key store
	for provider, providerResults := range results {
		for keyName, result := range providerResults {
			if memStore, ok := hc.keyStore.(*MemoryKeyStore); ok {
				memStore.SetHealth(ctx, provider, keyName, result.Valid)
				if !result.Valid {
					memStore.RecordError(ctx, provider, keyName, result.Message)
				}
			}
		}
	}
}

// GetHealthStatus returns the current health status of all keys
func (hc *HealthChecker) GetHealthStatus(ctx context.Context, providers map[string][]string) (map[string]map[string]bool, error) {
	status := make(map[string]map[string]bool)

	for provider, keyNames := range providers {
		status[provider] = make(map[string]bool)
		for _, keyName := range keyNames {
			healthy, err := hc.keyStore.IsHealthy(ctx, provider, keyName)
			if err != nil {
				status[provider][keyName] = false
			} else {
				status[provider][keyName] = healthy
			}
		}
	}

	return status, nil
}
