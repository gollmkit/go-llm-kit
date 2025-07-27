package auth

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/gollmkit/gollmkit/internal/config"
)

// KeyRotator manages API key rotation strategies
type KeyRotator struct {
	mu          sync.RWMutex
	config      *config.Config
	keyStore    KeyStore
	lastUsed    map[string]map[string]time.Time // provider -> keyName -> lastUsed
	rotationIdx map[string]int                  // provider -> current rotation index
	rand        *rand.Rand
}

// NewKeyRotator creates a new key rotator
func NewKeyRotator(cfg *config.Config, keyStore KeyStore) *KeyRotator {
	return &KeyRotator{
		config:      cfg,
		keyStore:    keyStore,
		lastUsed:    make(map[string]map[string]time.Time),
		rotationIdx: make(map[string]int),
		rand:        rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// KeySelection represents a selected API key with metadata
type KeySelection struct {
	Provider   string
	KeyName    string
	Key        string
	RateLimit  int
	CostLimit  float64
	UsageCount int64
	LastUsed   time.Time
	Strategy   config.RotationStrategy
}

// GetNextKey returns the next API key based on rotation strategy
func (kr *KeyRotator) GetNextKey(ctx context.Context, provider string) (*KeySelection, error) {
	kr.mu.Lock()
	defer kr.mu.Unlock()

	providerConfig, err := kr.config.GetProvider(provider)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	enabledKeys := providerConfig.GetEnabledKeys()
	if len(enabledKeys) == 0 {
		return nil, fmt.Errorf("no enabled keys available for provider %s", provider)
	}

	var selectedKey *config.APIKey
	var keyName string

	switch providerConfig.Rotation.Strategy {
	case config.RotationRoundRobin:
		selectedKey, keyName = kr.selectRoundRobin(provider, enabledKeys)
	case config.RotationLeastUsed:
		selectedKey, keyName, err = kr.selectLeastUsed(ctx, provider, enabledKeys)
	case config.RotationCostOptimized:
		selectedKey, keyName, err = kr.selectCostOptimized(ctx, provider, enabledKeys)
	case config.RotationRandom:
		selectedKey, keyName = kr.selectRandom(enabledKeys)
	case config.RotationSingle:
		selectedKey, keyName = kr.selectSingle(enabledKeys)
	default:
		selectedKey, keyName = kr.selectRoundRobin(provider, enabledKeys)
	}

	if err != nil {
		return nil, fmt.Errorf("key selection failed: %w", err)
	}

	if selectedKey == nil {
		return nil, fmt.Errorf("no suitable key found for provider %s", provider)
	}

	// Health check if enabled
	if providerConfig.Rotation.HealthCheck {
		healthy, err := kr.keyStore.IsHealthy(ctx, provider, keyName)
		if err != nil {
			return nil, fmt.Errorf("health check failed: %w", err)
		}
		if !healthy {
			// Try fallback if enabled
			if providerConfig.Rotation.FallbackEnabled && len(enabledKeys) > 1 {
				return kr.getFallbackKey(ctx, provider, keyName, enabledKeys)
			}
			return nil, fmt.Errorf("selected key %s is unhealthy and no fallback available", keyName)
		}
	}

	// Get the actual key value
	keyValue, err := kr.keyStore.GetKey(ctx, provider, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key: %w", err)
	}

	// Get usage statistics
	usage, err := kr.keyStore.GetUsage(ctx, provider, keyName)
	if err != nil {
		// If usage doesn't exist, create default
		usage = &KeyUsage{
			LastUsed:   time.Now(),
			UsageCount: 0,
		}
	}

	// Update last used time
	kr.updateLastUsed(provider, keyName)

	return &KeySelection{
		Provider:   provider,
		KeyName:    keyName,
		Key:        keyValue,
		RateLimit:  selectedKey.RateLimit,
		CostLimit:  selectedKey.CostLimit,
		UsageCount: usage.UsageCount,
		LastUsed:   usage.LastUsed,
		Strategy:   providerConfig.Rotation.Strategy,
	}, nil
}

// selectRoundRobin implements round-robin key selection
func (kr *KeyRotator) selectRoundRobin(provider string, keys []config.APIKey) (*config.APIKey, string) {
	if len(keys) == 0 {
		return nil, ""
	}

	// Initialize rotation index if not exists
	if _, exists := kr.rotationIdx[provider]; !exists {
		kr.rotationIdx[provider] = 0
	}

	// Get current index and increment for next time
	idx := kr.rotationIdx[provider]
	kr.rotationIdx[provider] = (idx + 1) % len(keys)

	selectedKey := &keys[idx]
	return selectedKey, selectedKey.Name
}

// selectLeastUsed implements least-used key selection
func (kr *KeyRotator) selectLeastUsed(ctx context.Context, provider string, keys []config.APIKey) (*config.APIKey, string, error) {
	if len(keys) == 0 {
		return nil, "", fmt.Errorf("no keys available")
	}

	var bestKey *config.APIKey
	var bestKeyName string
	minUsage := int64(-1)
	oldestLastUsed := time.Now()

	for _, key := range keys {
		usage, err := kr.keyStore.GetUsage(ctx, provider, key.Name)
		if err != nil {
			// If no usage data, consider it as least used
			bestKey = &key
			bestKeyName = key.Name
			break
		}

		// Select based on usage count, then by oldest last used time
		if minUsage == -1 || usage.UsageCount < minUsage ||
			(usage.UsageCount == minUsage && usage.LastUsed.Before(oldestLastUsed)) {
			minUsage = usage.UsageCount
			oldestLastUsed = usage.LastUsed
			bestKey = &key
			bestKeyName = key.Name
		}
	}

	return bestKey, bestKeyName, nil
}

// selectCostOptimized implements cost-optimized key selection
func (kr *KeyRotator) selectCostOptimized(ctx context.Context, provider string, keys []config.APIKey) (*config.APIKey, string, error) {
	if len(keys) == 0 {
		return nil, "", fmt.Errorf("no keys available")
	}

	var bestKey *config.APIKey
	var bestKeyName string
	lowestCost := float64(-1)

	for _, key := range keys {
		usage, err := kr.keyStore.GetUsage(ctx, provider, key.Name)
		if err != nil {
			// If no usage data, consider it as lowest cost
			bestKey = &key
			bestKeyName = key.Name
			continue
		}

		// Select key with lowest daily cost usage
		if lowestCost == -1 || usage.DailyCost < lowestCost {
			// Also check if key hasn't exceeded its daily limit
			if key.CostLimit <= 0 || usage.DailyCost < key.CostLimit {
				lowestCost = usage.DailyCost
				bestKey = &key
				bestKeyName = key.Name
			}
		}
	}

	if bestKey == nil {
		return nil, "", fmt.Errorf("all keys have exceeded their cost limits")
	}

	return bestKey, bestKeyName, nil
}

// selectRandom implements random key selection
func (kr *KeyRotator) selectRandom(keys []config.APIKey) (*config.APIKey, string) {
	if len(keys) == 0 {
		return nil, ""
	}

	idx := kr.rand.Intn(len(keys))
	selectedKey := &keys[idx]
	return selectedKey, selectedKey.Name
}

// selectSingle implements single key selection (first available)
func (kr *KeyRotator) selectSingle(keys []config.APIKey) (*config.APIKey, string) {
	if len(keys) == 0 {
		return nil, ""
	}

	selectedKey := &keys[0]
	return selectedKey, selectedKey.Name
}

// getFallbackKey gets a fallback key when primary selection fails
func (kr *KeyRotator) getFallbackKey(ctx context.Context, provider, excludeKey string, keys []config.APIKey) (*KeySelection, error) {
	// Filter out the failed key
	var fallbackKeys []config.APIKey
	for _, key := range keys {
		if key.Name != excludeKey {
			// Check if key is healthy
			if healthy, err := kr.keyStore.IsHealthy(ctx, provider, key.Name); err == nil && healthy {
				fallbackKeys = append(fallbackKeys, key)
			}
		}
	}

	if len(fallbackKeys) == 0 {
		return nil, fmt.Errorf("no healthy fallback keys available")
	}

	// Use round-robin for fallback selection
	selectedKey, keyName := kr.selectRoundRobin(provider, fallbackKeys)
	if selectedKey == nil {
		return nil, fmt.Errorf("fallback selection failed")
	}

	keyValue, err := kr.keyStore.GetKey(ctx, provider, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve fallback key: %w", err)
	}

	usage, _ := kr.keyStore.GetUsage(ctx, provider, keyName)
	if usage == nil {
		usage = &KeyUsage{LastUsed: time.Now()}
	}

	kr.updateLastUsed(provider, keyName)

	return &KeySelection{
		Provider:   provider,
		KeyName:    keyName,
		Key:        keyValue,
		RateLimit:  selectedKey.RateLimit,
		CostLimit:  selectedKey.CostLimit,
		UsageCount: usage.UsageCount,
		LastUsed:   usage.LastUsed,
		Strategy:   config.RotationRoundRobin, // Fallback uses round-robin
	}, nil
}

// updateLastUsed updates the last used time for a key
func (kr *KeyRotator) updateLastUsed(provider, keyName string) {
	if kr.lastUsed[provider] == nil {
		kr.lastUsed[provider] = make(map[string]time.Time)
	}
	kr.lastUsed[provider][keyName] = time.Now()
}

// RecordUsage records usage for a key and updates statistics
func (kr *KeyRotator) RecordUsage(ctx context.Context, provider, keyName string, tokens int, cost float64) error {
	return kr.keyStore.UpdateUsage(ctx, provider, keyName, tokens, cost)
}

// RecordError records an error for a key
func (kr *KeyRotator) RecordError(ctx context.Context, provider, keyName, errorMsg string) error {
	if memStore, ok := kr.keyStore.(*MemoryKeyStore); ok {
		return memStore.RecordError(ctx, provider, keyName, errorMsg)
	}
	return fmt.Errorf("error recording not supported by this keystore implementation")
}

// GetKeyStatistics returns statistics for all keys of a provider
func (kr *KeyRotator) GetKeyStatistics(ctx context.Context, provider string) (map[string]*KeyUsage, error) {
	keyNames, err := kr.keyStore.ListKeys(ctx, provider)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]*KeyUsage)
	for _, keyName := range keyNames {
		usage, err := kr.keyStore.GetUsage(ctx, provider, keyName)
		if err != nil {
			continue // Skip keys without usage data
		}
		stats[keyName] = usage
	}

	return stats, nil
}

// GetProviderStatistics returns aggregated statistics for a provider
func (kr *KeyRotator) GetProviderStatistics(ctx context.Context, provider string) (*ProviderStats, error) {
	keyStats, err := kr.GetKeyStatistics(ctx, provider)
	if err != nil {
		return nil, err
	}

	stats := &ProviderStats{
		Provider:      provider,
		TotalKeys:     len(keyStats),
		HealthyKeys:   0,
		TotalCost:     0,
		TotalTokens:   0,
		TotalRequests: 0,
		KeyStats:      make(map[string]*KeyStats),
	}

	for keyName, usage := range keyStats {
		healthy, _ := kr.keyStore.IsHealthy(ctx, provider, keyName)
		if healthy {
			stats.HealthyKeys++
		}

		stats.TotalCost += usage.CostUsed
		stats.TotalTokens += usage.TokensUsed
		stats.TotalRequests += usage.UsageCount

		stats.KeyStats[keyName] = &KeyStats{
			Name:     keyName,
			Healthy:  healthy,
			Usage:    usage,
			LastUsed: usage.LastUsed,
		}
	}

	return stats, nil
}

// ProviderStats represents aggregated statistics for a provider
type ProviderStats struct {
	Provider      string               `json:"provider"`
	TotalKeys     int                  `json:"total_keys"`
	HealthyKeys   int                  `json:"healthy_keys"`
	TotalCost     float64              `json:"total_cost"`
	TotalTokens   int64                `json:"total_tokens"`
	TotalRequests int64                `json:"total_requests"`
	KeyStats      map[string]*KeyStats `json:"key_stats"`
}

// KeyStats represents statistics for a single key
type KeyStats struct {
	Name     string    `json:"name"`
	Healthy  bool      `json:"healthy"`
	Usage    *KeyUsage `json:"usage"`
	LastUsed time.Time `json:"last_used"`
}

// RotationStatus represents the current rotation status
type RotationStatus struct {
	Provider      string                  `json:"provider"`
	Strategy      config.RotationStrategy `json:"strategy"`
	CurrentIndex  int                     `json:"current_index,omitempty"`
	AvailableKeys []string                `json:"available_keys"`
	LastRotation  time.Time               `json:"last_rotation"`
}

// GetRotationStatus returns the current rotation status for a provider
func (kr *KeyRotator) GetRotationStatus(ctx context.Context, provider string) (*RotationStatus, error) {
	kr.mu.RLock()
	defer kr.mu.RUnlock()

	providerConfig, err := kr.config.GetProvider(provider)
	if err != nil {
		return nil, err
	}

	keyNames, err := kr.keyStore.ListKeys(ctx, provider)
	if err != nil {
		return nil, err
	}

	status := &RotationStatus{
		Provider:      provider,
		Strategy:      providerConfig.Rotation.Strategy,
		AvailableKeys: keyNames,
	}

	if idx, exists := kr.rotationIdx[provider]; exists {
		status.CurrentIndex = idx
	}

	// Get last rotation time from last used times
	if providerLastUsed, exists := kr.lastUsed[provider]; exists {
		for _, lastUsed := range providerLastUsed {
			if lastUsed.After(status.LastRotation) {
				status.LastRotation = lastUsed
			}
		}
	}

	return status, nil
}
