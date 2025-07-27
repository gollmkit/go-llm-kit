package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gollmkit/gollmkit/internal/config"
)

// KeyStore defines the interface for API key storage and management
type KeyStore interface {
	// StoreKey stores an API key securely
	StoreKey(ctx context.Context, provider, keyName, key string) error

	// GetKey retrieves an API key
	GetKey(ctx context.Context, provider, keyName string) (string, error)

	// DeleteKey removes an API key
	DeleteKey(ctx context.Context, provider, keyName string) error

	// ListKeys returns all key names for a provider
	ListKeys(ctx context.Context, provider string) ([]string, error)

	// IsHealthy checks if a key is healthy and valid
	IsHealthy(ctx context.Context, provider, keyName string) (bool, error)

	// UpdateUsage updates key usage statistics
	UpdateUsage(ctx context.Context, provider, keyName string, tokens int, cost float64) error

	// GetUsage returns key usage statistics
	GetUsage(ctx context.Context, provider, keyName string) (*KeyUsage, error)

	// Close closes the keystore connection
	Close() error
}

// KeyUsage represents usage statistics for an API key
type KeyUsage struct {
	LastUsed   time.Time `json:"last_used"`
	UsageCount int64     `json:"usage_count"`
	TokensUsed int64     `json:"tokens_used"`
	CostUsed   float64   `json:"cost_used"`
	DailyCost  float64   `json:"daily_cost"`
	ErrorCount int64     `json:"error_count"`
	LastError  string    `json:"last_error,omitempty"`
}

// MemoryKeyStore is an in-memory implementation of KeyStore for development/testing
type MemoryKeyStore struct {
	mu        sync.RWMutex
	keys      map[string]map[string]string    // provider -> keyName -> encryptedKey
	usage     map[string]map[string]*KeyUsage // provider -> keyName -> usage
	health    map[string]map[string]bool      // provider -> keyName -> healthy
	encryptor *KeyEncryptor
}

// NewMemoryKeyStore creates a new in-memory key store
func NewMemoryKeyStore(encryptionKey string) *MemoryKeyStore {
	var encryptor *KeyEncryptor
	if encryptionKey != "" {
		encryptor = NewKeyEncryptor(encryptionKey)
	}

	return &MemoryKeyStore{
		keys:      make(map[string]map[string]string),
		usage:     make(map[string]map[string]*KeyUsage),
		health:    make(map[string]map[string]bool),
		encryptor: encryptor,
	}
}

// StoreKey stores an API key securely
func (m *MemoryKeyStore) StoreKey(ctx context.Context, provider, keyName, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keys[provider] == nil {
		m.keys[provider] = make(map[string]string)
		m.usage[provider] = make(map[string]*KeyUsage)
		m.health[provider] = make(map[string]bool)
	}

	var storedKey string
	var err error

	if m.encryptor != nil {
		storedKey, err = m.encryptor.Encrypt(key)
		if err != nil {
			return fmt.Errorf("failed to encrypt key: %w", err)
		}
	} else {
		storedKey = key
	}

	m.keys[provider][keyName] = storedKey
	m.usage[provider][keyName] = &KeyUsage{
		LastUsed:   time.Now(),
		UsageCount: 0,
		TokensUsed: 0,
		CostUsed:   0,
		DailyCost:  0,
		ErrorCount: 0,
	}
	m.health[provider][keyName] = true

	return nil
}

// GetKey retrieves an API key
func (m *MemoryKeyStore) GetKey(ctx context.Context, provider, keyName string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providerKeys, exists := m.keys[provider]
	if !exists {
		return "", fmt.Errorf("provider %s not found", provider)
	}

	encryptedKey, exists := providerKeys[keyName]
	if !exists {
		return "", fmt.Errorf("key %s not found for provider %s", keyName, provider)
	}

	if m.encryptor != nil {
		return m.encryptor.Decrypt(encryptedKey)
	}

	return encryptedKey, nil
}

// DeleteKey removes an API key
func (m *MemoryKeyStore) DeleteKey(ctx context.Context, provider, keyName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keys[provider] != nil {
		delete(m.keys[provider], keyName)
		delete(m.usage[provider], keyName)
		delete(m.health[provider], keyName)
	}

	return nil
}

// ListKeys returns all key names for a provider
func (m *MemoryKeyStore) ListKeys(ctx context.Context, provider string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	providerKeys, exists := m.keys[provider]
	if !exists {
		return []string{}, nil
	}

	keys := make([]string, 0, len(providerKeys))
	for keyName := range providerKeys {
		keys = append(keys, keyName)
	}

	return keys, nil
}

// IsHealthy checks if a key is healthy and valid
func (m *MemoryKeyStore) IsHealthy(ctx context.Context, provider, keyName string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.health[provider] == nil {
		return false, fmt.Errorf("provider %s not found", provider)
	}

	healthy, exists := m.health[provider][keyName]
	if !exists {
		return false, fmt.Errorf("key %s not found for provider %s", keyName, provider)
	}

	return healthy, nil
}

// UpdateUsage updates key usage statistics
func (m *MemoryKeyStore) UpdateUsage(ctx context.Context, provider, keyName string, tokens int, cost float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.usage[provider] == nil {
		return fmt.Errorf("provider %s not found", provider)
	}

	usage, exists := m.usage[provider][keyName]
	if !exists {
		return fmt.Errorf("key %s not found for provider %s", keyName, provider)
	}

	usage.LastUsed = time.Now()
	usage.UsageCount++
	usage.TokensUsed += int64(tokens)
	usage.CostUsed += cost

	// Reset daily cost if it's a new day
	now := time.Now()
	if usage.LastUsed.Day() != now.Day() {
		usage.DailyCost = cost
	} else {
		usage.DailyCost += cost
	}

	return nil
}

// GetUsage returns key usage statistics
func (m *MemoryKeyStore) GetUsage(ctx context.Context, provider, keyName string) (*KeyUsage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.usage[provider] == nil {
		return nil, fmt.Errorf("provider %s not found", provider)
	}

	usage, exists := m.usage[provider][keyName]
	if !exists {
		return nil, fmt.Errorf("key %s not found for provider %s", keyName, provider)
	}

	// Return a copy to prevent external modification
	return &KeyUsage{
		LastUsed:   usage.LastUsed,
		UsageCount: usage.UsageCount,
		TokensUsed: usage.TokensUsed,
		CostUsed:   usage.CostUsed,
		DailyCost:  usage.DailyCost,
		ErrorCount: usage.ErrorCount,
		LastError:  usage.LastError,
	}, nil
}

// SetHealth sets the health status of a key
func (m *MemoryKeyStore) SetHealth(ctx context.Context, provider, keyName string, healthy bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.health[provider] == nil {
		return fmt.Errorf("provider %s not found", provider)
	}

	m.health[provider][keyName] = healthy
	return nil
}

// RecordError records an error for a key
func (m *MemoryKeyStore) RecordError(ctx context.Context, provider, keyName, errorMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.usage[provider] == nil {
		return fmt.Errorf("provider %s not found", provider)
	}

	usage, exists := m.usage[provider][keyName]
	if !exists {
		return fmt.Errorf("key %s not found for provider %s", keyName, provider)
	}

	usage.ErrorCount++
	usage.LastError = errorMsg

	// Mark as unhealthy if too many errors
	if usage.ErrorCount > 5 {
		m.health[provider][keyName] = false
	}

	return nil
}

// Close closes the keystore connection
func (m *MemoryKeyStore) Close() error {
	// Nothing to close for memory store
	return nil
}

// KeyEncryptor handles encryption/decryption of API keys
type KeyEncryptor struct {
	key []byte
}

// NewKeyEncryptor creates a new key encryptor
func NewKeyEncryptor(password string) *KeyEncryptor {
	// Create a 32-byte key from password using SHA256
	hash := sha256.Sum256([]byte(password))
	return &KeyEncryptor{key: hash[:]}
}

// Encrypt encrypts a string using AES-GCM
func (e *KeyEncryptor) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a string using AES-GCM
func (e *KeyEncryptor) Decrypt(ciphertext string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext_bytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext_bytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// NewKeyStoreFromConfig creates a KeyStore from configuration
func NewKeyStoreFromConfig(cfg *config.Config) (KeyStore, error) {
	// For now, we only support memory store
	// In production, this could be extended to support database backends

	var encryptionKey string
	if cfg.Global.EncryptKeys {
		// In production, this should come from environment or secure vault
		encryptionKey = "default-encryption-key-change-in-production"
	}

	store := NewMemoryKeyStore(encryptionKey)

	// Populate store with keys from config
	ctx := context.Background()
	for providerName, provider := range cfg.Providers {
		for _, apiKey := range provider.APIKeys {
			if err := store.StoreKey(ctx, providerName, apiKey.Name, apiKey.Key); err != nil {
				return nil, fmt.Errorf("failed to store key %s for provider %s: %w",
					apiKey.Name, providerName, err)
			}
		}
	}

	return store, nil
}
