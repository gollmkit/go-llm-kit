// Package providers implements a unified interface for various LLM providers
package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/gollmkit/gollmkit/internal/auth"
	"github.com/gollmkit/gollmkit/internal/config"
)

// Common errors
var (
	ErrInvalidModel   = errors.New("invalid model")
	ErrKeyRotation    = errors.New("key rotation failed")
	ErrInvalidConfig  = errors.New("invalid configuration")
	ErrResponseFormat = errors.New("invalid response format")
)

// ProviderType identifies the LLM provider
type ProviderType string

const (
	OpenAI    ProviderType = "openai"
	Anthropic ProviderType = "anthropic"
	Gemini    ProviderType = "gemini"
)

// Message represents a chat message
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// RequestOptions contains common options for LLM requests
type RequestOptions struct {
	Provider    ProviderType `json:"provider,omitempty"`
	Model       string       `json:"model,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float32      `json:"temperature,omitempty"`
	TopP        float32      `json:"top_p,omitempty"`
	Stop        []string     `json:"stop,omitempty"`
	Stream      bool         `json:"stream,omitempty"`
}

// CompletionResponse represents a unified response format
type CompletionResponse struct {
	Content      string                 `json:"content"`
	Model        string                 `json:"model"`
	Usage        TokenUsage             `json:"usage"`
	ProviderName string                 `json:"provider_name"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TokenUsage tracks token usage for billing
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// DefaultOptions returns default RequestOptions for a provider
func DefaultOptions(provider ProviderType) RequestOptions {
	switch provider {
	case OpenAI:
		return RequestOptions{
			Provider:    OpenAI,
			Model:       "gpt-3.5-turbo",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	case Anthropic:
		return RequestOptions{
			Provider:    Anthropic,
			Model:       "claude-3-sonnet-20240229",
			Temperature: 0.7,
			MaxTokens:   4000,
		}
	case Gemini:
		return RequestOptions{
			Provider:    Gemini,
			Model:       "gemini-2.0-flash",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	default:
		return RequestOptions{
			Provider:    OpenAI,
			Model:       "gpt-3.5-turbo",
			Temperature: 0.7,
			MaxTokens:   2000,
		}
	}
}

// LLMProvider is the main interface for interacting with LLMs
type LLMProvider interface {
	Invoke(ctx context.Context, prompt string, opts RequestOptions) (*CompletionResponse, error)
	Chat(ctx context.Context, messages []Message, opts RequestOptions) (*CompletionResponse, error)
}

// BaseProvider contains common functionality for all providers
type BaseProvider struct {
	config    *config.Config
	rotator   *auth.KeyRotator
	validator *auth.KeyValidator
	client    *http.Client
}

// NewBaseProvider creates a new base provider with common functionality
func NewBaseProvider(cfg *config.Config, rotator *auth.KeyRotator, validator *auth.KeyValidator) *BaseProvider {
	return &BaseProvider{
		config:    cfg,
		rotator:   rotator,
		validator: validator,
		client:    &http.Client{},
	}
}

// validateModel checks if the model is valid for the given provider
func (p *BaseProvider) validateModel(provider ProviderType, model string) error {
	if model == "" {
		return fmt.Errorf("%w: model name cannot be empty", ErrInvalidModel)
	}

	providerCfg, err := p.config.GetProvider(string(provider))
	if err != nil {
		return fmt.Errorf("%w: %s provider not configured", ErrInvalidConfig, provider)
	}

	if _, err := providerCfg.GetModelByName(model); err != nil {
		return fmt.Errorf("%w: %s", ErrInvalidModel, model)
	}
	return nil
}

// getNextKey gets the next valid API key using the rotator
func (p *BaseProvider) getNextKey(ctx context.Context, provider ProviderType) (*auth.KeySelection, error) {
	key, err := p.rotator.GetNextKey(ctx, string(provider))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyRotation, err)
	}
	return key, nil
}

// recordUsage records token usage for the key
func (p *BaseProvider) recordUsage(ctx context.Context, provider ProviderType, keyName string, usage TokenUsage) error {
	cost := float64(usage.TotalTokens) * 0.001 // Default cost per 1k tokens
	return p.rotator.RecordUsage(ctx, string(provider), keyName, usage.TotalTokens, cost)
}

// recordError records an error for a key
func (p *BaseProvider) recordError(ctx context.Context, provider ProviderType, keyName string, err error) {
	if err != nil {
		p.rotator.RecordError(ctx, string(provider), keyName, err.Error())
	}
}

// UnifiedProvider is the unified LLM provider that handles all provider types
type UnifiedProvider struct {
	*BaseProvider
}

// NewUnifiedProvider creates a new unified LLM provider
func NewUnifiedProvider(cfg *config.Config, rotator *auth.KeyRotator, validator *auth.KeyValidator) *UnifiedProvider {
	return &UnifiedProvider{
		BaseProvider: NewBaseProvider(cfg, rotator, validator),
	}
}

// Invoke sends a single prompt to the LLM
func (p *UnifiedProvider) Invoke(ctx context.Context, prompt string, opts RequestOptions) (*CompletionResponse, error) {
	if opts.Provider == "" {
		opts.Provider = OpenAI
	}

	messages := []Message{{Role: "user", Content: prompt}}
	return p.Chat(ctx, messages, opts)
}

// mergeOptions merges request options with configuration and defaults
func (p *UnifiedProvider) mergeOptions(provider ProviderType, opts RequestOptions) (RequestOptions, error) {
	// Get provider configuration
	providerCfg, err := p.config.GetProvider(string(provider))
	if err != nil {
		return opts, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
	}

	// Start with a clean options struct containing what we know
	result := RequestOptions{
		Provider: provider,
		Model:    opts.Model,
		// Copy request values as-is - they take highest priority
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		TopP:        opts.TopP,
		Stop:        opts.Stop,
		Stream:      opts.Stream,
	}

	// Get model configuration if specified
	var modelCfg *config.ModelConfig
	if opts.Model != "" {
		// First try to find the specified model in config
		modelCfg, err = providerCfg.GetModelByName(opts.Model)
		if err != nil {
			// Model not found in config, that's okay, we'll use the user-specified model
			result.Model = opts.Model
		}
	}

	// If no model specified or found, use first enabled model from config
	if modelCfg == nil && opts.Model == "" {
		enabledModels := providerCfg.GetEnabledModels()
		if len(enabledModels) > 0 {
			modelCfg = &enabledModels[0]
		}
	}

	// Apply config values if available for any unset values
	if modelCfg != nil {
		if result.Model == "" {
			result.Model = modelCfg.Name
		}
		if result.MaxTokens == 0 {
			result.MaxTokens = modelCfg.MaxTokens
		}
	}

	// Get defaults for this provider
	defaults := DefaultOptions(provider)

	// Apply defaults for any remaining unset values
	if result.Model == "" {
		result.Model = defaults.Model
	}
	if result.MaxTokens == 0 {
		result.MaxTokens = defaults.MaxTokens
	}
	if result.Temperature == 0 {
		result.Temperature = defaults.Temperature
	}
	if result.TopP == 0 {
		result.TopP = defaults.TopP
	}

	return result, nil
}

// Chat sends a series of messages to the LLM
func (p *UnifiedProvider) Chat(ctx context.Context, messages []Message, opts RequestOptions) (*CompletionResponse, error) {
	if opts.Provider == "" {
		opts.Provider = OpenAI
	}

	// Merge options with configuration and defaults
	mergedOpts, err := p.mergeOptions(opts.Provider, opts)
	if err != nil {
		return nil, err
	}
	opts = mergedOpts

	if err := p.validateModel(opts.Provider, opts.Model); err != nil {
		return nil, err
	}

	key, err := p.getNextKey(ctx, opts.Provider)
	if err != nil {
		return nil, err
	}

	switch opts.Provider {
	case OpenAI:
		return p.callOpenAI(ctx, messages, opts, key)
	case Anthropic:
		return p.callAnthropic(ctx, messages, opts, key)
	case Gemini:
		return p.callGemini(ctx, messages, opts, key)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", opts.Provider)
	}
}

func (p *UnifiedProvider) callOpenAI(ctx context.Context, messages []Message, opts RequestOptions, key *auth.KeySelection) (*CompletionResponse, error) {
	reqBody := map[string]interface{}{
		"model":       opts.Model,
		"messages":    messages,
		"max_tokens":  opts.MaxTokens,
		"temperature": opts.Temperature,
		"top_p":       opts.TopP,
		"stop":        opts.Stop,
		"stream":      opts.Stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key.Key)

	resp, err := p.client.Do(req)
	if err != nil {
		p.recordError(ctx, OpenAI, key.KeyName, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("OpenAI API error: %d", resp.StatusCode)
		p.recordError(ctx, OpenAI, key.KeyName, err)
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseFormat, err)
	}

	choices, ok := result["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		return nil, fmt.Errorf("%w: missing choices in response", ErrResponseFormat)
	}

	usage, ok := result["usage"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: missing usage in response", ErrResponseFormat)
	}

	tokenUsage := TokenUsage{
		PromptTokens:     int(usage["prompt_tokens"].(float64)),
		CompletionTokens: int(usage["completion_tokens"].(float64)),
		TotalTokens:      int(usage["total_tokens"].(float64)),
	}

	if err := p.recordUsage(ctx, OpenAI, key.KeyName, tokenUsage); err != nil {
		return nil, err
	}

	msgContent, ok := choices[0].(map[string]interface{})["message"].(map[string]interface{})["content"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: invalid message format in response", ErrResponseFormat)
	}

	return &CompletionResponse{
		Content:      msgContent,
		Model:        opts.Model,
		Usage:        tokenUsage,
		ProviderName: string(OpenAI),
		Metadata:     result,
	}, nil
}

func (p *UnifiedProvider) callAnthropic(ctx context.Context, messages []Message, opts RequestOptions, key *auth.KeySelection) (*CompletionResponse, error) {
	reqBody := map[string]interface{}{
		"model":          opts.Model,
		"messages":       messages,
		"max_tokens":     opts.MaxTokens,
		"temperature":    opts.Temperature,
		"top_p":          opts.TopP,
		"stop_sequences": opts.Stop,
		"stream":         opts.Stream,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key.Key)
	req.Header.Set("anthropic-version", "2024-01-01")

	resp, err := p.client.Do(req)
	if err != nil {
		p.recordError(ctx, Anthropic, key.KeyName, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Anthropic API error: %d", resp.StatusCode)
		p.recordError(ctx, Anthropic, key.KeyName, err)
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseFormat, err)
	}

	content, ok := result["content"].([]interface{})
	if !ok || len(content) == 0 {
		return nil, fmt.Errorf("%w: missing content in response", ErrResponseFormat)
	}

	text, ok := content[0].(map[string]interface{})["text"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: invalid content format in response", ErrResponseFormat)
	}

	usage, ok := result["usage"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: missing usage in response", ErrResponseFormat)
	}

	tokenUsage := TokenUsage{
		PromptTokens:     int(usage["input_tokens"].(float64)),
		CompletionTokens: int(usage["output_tokens"].(float64)),
		TotalTokens:      int(usage["input_tokens"].(float64)) + int(usage["output_tokens"].(float64)),
	}

	if err := p.recordUsage(ctx, Anthropic, key.KeyName, tokenUsage); err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Content:      text,
		Model:        opts.Model,
		Usage:        tokenUsage,
		ProviderName: string(Anthropic),
		Metadata:     result,
	}, nil
}

func (p *UnifiedProvider) callGemini(ctx context.Context, messages []Message, opts RequestOptions, key *auth.KeySelection) (*CompletionResponse, error) {
	var combinedContent string
	for _, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		combinedContent += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	reqBody := map[string]interface{}{
		"contents": []map[string]interface{}{{
			"parts": []map[string]interface{}{{
				"text": combinedContent,
			}},
		}},
		"generationConfig": map[string]interface{}{
			"temperature":     opts.Temperature,
			"topP":            opts.TopP,
			"maxOutputTokens": opts.MaxTokens,
			"stopSequences":   opts.Stop,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1/models/%s:generateContent?key=%s",
		url.PathEscape(opts.Model),
		url.QueryEscape(key.Key))

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		p.recordError(ctx, Gemini, key.KeyName, err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Gemini API error: %d", resp.StatusCode)
		p.recordError(ctx, Gemini, key.KeyName, err)
		return nil, err
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrResponseFormat, err)
	}

	candidates, ok := result["candidates"].([]interface{})
	if !ok || len(candidates) == 0 {
		return nil, fmt.Errorf("%w: missing candidates in response", ErrResponseFormat)
	}

	content, ok := candidates[0].(map[string]interface{})["content"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%w: invalid content format in response", ErrResponseFormat)
	}

	parts, ok := content["parts"].([]interface{})
	if !ok || len(parts) == 0 {
		return nil, fmt.Errorf("%w: missing parts in response", ErrResponseFormat)
	}

	text, ok := parts[0].(map[string]interface{})["text"].(string)
	if !ok {
		return nil, fmt.Errorf("%w: invalid text format in response", ErrResponseFormat)
	}

	usage := TokenUsage{
		PromptTokens:     int(result["usageMetadata"].(map[string]interface{})["promptTokenCount"].(float64)),
		CompletionTokens: int(result["usageMetadata"].(map[string]interface{})["candidatesTokenCount"].(float64)),
		TotalTokens:      int(result["usageMetadata"].(map[string]interface{})["totalTokenCount"].(float64)),
	}

	if err := p.recordUsage(ctx, Gemini, key.KeyName, usage); err != nil {
		return nil, err
	}

	return &CompletionResponse{
		Content:      text,
		Model:        opts.Model,
		Usage:        usage,
		ProviderName: string(Gemini),
		Metadata:     result,
	}, nil
}
