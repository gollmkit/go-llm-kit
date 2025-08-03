# GoLLMKit API Key Management System

This document describes the API key management feature implementation for GoLLM, focusing on secure key storage, intelligent rotation strategies, and comprehensive validation.

## 🚀 Quick Start

### 1. Installation

```bash
go mod init your-project
go get github.com/gollm
```

### 2. Configuration Setup

Create a `gollm-config.yaml` file:

```yaml
providers:
  openai:
    api_keys:
      - key: "sk-proj-example1..."
        name: "primary"
        rate_limit: 1000
        cost_limit: 100.0
        enabled: true
      - key: "sk-proj-example2..."
        name: "secondary"
        rate_limit: 800
        cost_limit: 75.0
        enabled: true
    rotation:
      strategy: "round_robin"
      interval: "1h"
      health_check: true
      fallback_enabled: true

global:
  encrypt_keys: true
  key_validation: true
  daily_cost_limit: 500.0
```

### 3. Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gollmkit/gollmkit/internal/auth"
    "github.com/gollmkit/gollmkit/internal/config"
    "github.com/gollmkit/gollmkit/internal/providers"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("gollmkit-config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Create key store, rotator and validator
    keyStore, err := auth.NewKeyStoreFromConfig(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer keyStore.Close()

    rotator := auth.NewKeyRotator(cfg, keyStore)
    validator := auth.NewKeyValidator()

    ctx := context.Background()

    // Create unified provider for all LLM interactions
    provider := providers.NewUnifiedProvider(cfg, rotator, validator)

    // Example 1: Simple completion with OpenAI
    opts := providers.DefaultOptions(providers.OpenAI)
    opts.MaxTokens = 50
    resp, err := provider.Invoke(ctx, "Tell me a short joke", opts)
    if err != nil {
        log.Printf("OpenAI error: %v", err)
    } else {
        fmt.Printf("Response: %s\n", resp.Content)
        fmt.Printf("Model: %s, Tokens: %d\n", resp.Model, resp.Usage.TotalTokens)
    }

    // Example 2: Chat with Anthropic
    messages := []providers.Message{
        {Role: "system", Content: "You are a helpful assistant."},
        {Role: "user", Content: "What's the capital of France?"},
    }
    resp, err = provider.Chat(ctx, messages, providers.RequestOptions{
        Provider:    providers.Anthropic,
        Model:       "claude-3-sonnet-20240229",
        Temperature: 0.5,
    })
    if err != nil {
        log.Printf("Anthropic error: %v", err)
    } else {
        fmt.Printf("Response: %s\n", resp.Content)
        fmt.Printf("Model: %s, Tokens: %d\n", resp.Model, resp.Usage.TotalTokens)
    }

    // Example 3: Using Gemini
    fmt.Println("\n=== Gemini Example ===")
    // You can either use DefaultOptions
    geminiOpts := providers.DefaultOptions(providers.Gemini)
    // Or create custom options
    geminiCustomOpts := providers.RequestOptions{
        Provider:    providers.Gemini,
        Model:       "gemini-2.5-flash",
        MaxTokens:   100,
        Temperature: 0.3,
    }

    // Simple completion with default options
    resp, err = provider.Invoke(ctx, "Explain quantum computing in simple terms", geminiOpts)
    if err != nil {
        log.Printf("Gemini error: %v", err)
    } else {
        fmt.Printf("Gemini Response: %s\n", resp.Content)
        fmt.Printf("Model: %s, Tokens: %d\n", resp.Model, resp.Usage.TotalTokens)
    }

    // Chat with custom options
    geminiMessages := []providers.Message{
        {Role: "user", Content: "What are the key differences between quantum and classical computers?"},
    }
    resp, err = provider.Chat(ctx, geminiMessages, geminiCustomOpts)
    if err != nil {
        log.Printf("Gemini chat error: %v", err)
    } else {
        fmt.Printf("Gemini Chat Response: %s\n", resp.Content)
        fmt.Printf("Model: %s, Tokens: %d\n", resp.Model, resp.Usage.TotalTokens)
    }
}
    ctx := context.Background()
    selection, err := rotator.GetNextKey(ctx, "openai")
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Selected key: %s, Strategy: %s",
        selection.KeyName, selection.Strategy)
}
```

## 🏗️ Architecture Overview

### Core Components

1. **Configuration System** (`internal/config/`)

   - YAML-based configuration with Viper
   - Environment variable overrides
   - Comprehensive validation

2. **Key Storage** (`internal/auth/keystore.go`)

   - Secure encrypted storage
   - In-memory implementation (extensible to database)
   - Usage tracking and statistics

3. **Key Rotation** (`internal/auth/rotation.go`)

   - Multiple rotation strategies
   - Intelligent failover
   - Cost optimization

4. **Key Validation** (`internal/auth/validator.go`)
   - Format validation
   - Live API validation
   - Health monitoring

## 🎯 Key Features and Examples

### 1. Unified Provider Interface

The unified provider interface simplifies interactions with different LLM providers:

```go
provider := providers.NewUnifiedProvider(cfg, rotator, validator)

// Use default options for each provider
openaiOpts := providers.DefaultOptions(providers.OpenAI)
anthropicOpts := providers.DefaultOptions(providers.Anthropic)
geminiOpts := providers.DefaultOptions(providers.Gemini)

// Single prompt completion
resp, err := provider.Invoke(ctx, prompt, openaiOpts)

// Chat completion
resp, err = provider.Chat(ctx, messages, anthropicOpts)
```

### 2. Key Management Features

#### A. Key Rotation

```go
// Demonstrate key rotation with different strategies
func ExampleKeyRotation(ctx context.Context, rotator *auth.KeyRotator) {
    // Get next key using configured rotation strategy
    selection, err := rotator.GetNextKey(ctx, "openai")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Selected: %s, Strategy: %s\n",
        selection.KeyName, selection.Strategy)
}
```

#### B. Key Validation

```go
// Validate all configured keys
func ExampleKeyValidation(ctx context.Context, keyStore auth.KeyStore, cfg *config.Config) {
    validator := auth.NewKeyValidator()

    // Get all provider keys
    providers := make(map[string][]string)
    for provider := range cfg.Providers {
        keys, _ := keyStore.ListKeys(ctx, provider)
        providers[provider] = keys
    }

    // Validate all keys
    results, _ := validator.ValidateAllKeys(ctx, keyStore, providers)
    for provider, validations := range results {
        for key, result := range validations {
            fmt.Printf("%s.%s: %v - %s\n",
                provider, key, result.Valid, result.Message)
        }
    }
}
```

#### C. Usage Tracking

```go
// Track and monitor key usage
func ExampleUsageTracking(ctx context.Context, rotator *auth.KeyRotator) {
    // Record usage for a key
    err := rotator.RecordUsage(ctx, "openai", "primary", 1000, 0.03)
    if err != nil {
        log.Printf("Error recording usage: %v", err)
    }

    // Get usage statistics
    stats, _ := rotator.GetKeyStatistics(ctx, "openai")
    for key, usage := range stats {
        fmt.Printf("Key: %s\n", key)
        fmt.Printf("  Requests: %d\n", usage.UsageCount)
        fmt.Printf("  Tokens: %d\n", usage.TokensUsed)
        fmt.Printf("  Cost: $%.3f\n", usage.CostUsed)
    }
}
```

#### D. Health Monitoring

```go
// Monitor key health
func ExampleHealthMonitoring(ctx context.Context, keyStore auth.KeyStore) {
    checker := auth.NewHealthChecker(keyStore, 5*time.Minute)

    // Get current health status
    providers := map[string][]string{
        "openai": {"primary", "secondary"},
        "anthropic": {"primary"},
    }

    status, _ := checker.GetHealthStatus(ctx, providers)
    for provider, keys := range status {
        for key, healthy := range keys {
            fmt.Printf("%s.%s: %v\n", provider, key, healthy)
        }
    }

    // In production, start continuous monitoring:
    // go checker.Start(ctx, providers)
}
```

## 🔄 Rotation Strategies

### 1. Round Robin (`round_robin`)

Cycles through keys in order, ensuring even distribution.

```yaml
rotation:
  strategy: "round_robin"
  interval: "1h"
```

**Use Case**: Equal load distribution across all keys.

### 2. Least Used (`least_used`)

Selects the key with the lowest usage count.

```yaml
rotation:
  strategy: "least_used"
  interval: "30m"
```

**Use Case**: Balancing usage when keys have different capacities.

### 3. Cost Optimized (`cost_optimized`)

Chooses the key with the lowest current daily cost.

```yaml
rotation:
  strategy: "cost_optimized"
  interval: "15m"
```

**Use Case**: Minimizing overall costs across multiple keys.

### 4. Random (`random`)

Randomly selects from available keys.

```yaml
rotation:
  strategy: "random"
```

**Use Case**: Avoiding predictable patterns.

### 5. Single Key (`single`)

Always uses the first available key.

```yaml
rotation:
  strategy: "single"
```

**Use Case**: Simple setups with backup keys only.

## 🔒 Security Features

### Encryption

- AES-GCM encryption for stored keys
- SHA256-based key derivation
- Configurable encryption password

### Key Validation

- Format validation for all providers
- Live API validation
- Health status tracking

### Environment Integration

```bash
# Override keys via environment variables
export GOLLM_OPENAI_API_KEY_PRIMARY="sk-proj-newkey..."
export GOLLM_ANTHROPIC_API_KEY_PRIMARY="sk-ant-newkey..."
```

## 📊 Usage Tracking & Analytics

### Key Usage Statistics

```go
// Get usage statistics for a provider
stats, err := rotator.GetKeyStatistics(ctx, "openai")
for keyName, usage := range stats {
    fmt.Printf("Key: %s, Requests: %d, Cost: $%.3f\n",
        keyName, usage.UsageCount, usage.CostUsed)
}
```

### Provider Statistics

```go
// Get aggregated provider statistics
providerStats, err := rotator.GetProviderStatistics(ctx, "openai")
fmt.Printf("Total Cost: $%.2f, Healthy Keys: %d/%d\n",
    providerStats.TotalCost,
    providerStats.HealthyKeys,
    providerStats.TotalKeys)
```

## 🏥 Health Monitoring

### Automated Health Checks

```go
// Create and start health checker
healthChecker := auth.NewHealthChecker(keyStore, 5*time.Minute)

// Build provider-key mapping
providers := make(map[string][]string)
for providerName := range cfg.Providers {
    keyNames, _ := keyStore.ListKeys(ctx, providerName)
    providers[providerName] = keyNames
}

// Start health monitoring (in production)
go healthChecker.Start(ctx, providers)
```

### Health Status Check

```go
healthStatus, err := healthChecker.GetHealthStatus(ctx, providers)
for provider, keys := range healthStatus {
    for keyName, healthy := range keys {
        status := "✓ Healthy"
        if !healthy {
            status = "✗ Unhealthy"
        }
        fmt.Printf("%s.%s: %s\n", provider, keyName, status)
    }
}
```

## 🛠️ Configuration Reference

### Complete Configuration Example

```yaml
providers:
  openai:
    api_keys:
      - key: "sk-proj-example1..."
        name: "primary"
        rate_limit: 1000 # requests per hour
        cost_limit: 100.0 # dollars per day
        enabled: true

    models:
      - name: "gpt-4"
        input_cost_per_1k_tokens: 0.03
        output_cost_per_1k_tokens: 0.06
        max_tokens: 8192
        enabled: true

    rotation:
      strategy: "round_robin"
      interval: "1h"
      health_check: true
      fallback_enabled: true

  anthropic:
    api_keys:
      - key: "sk-ant-example1..."
        name: "primary"
        rate_limit: 500
        cost_limit: 80.0
        enabled: true

    models:
      - name: "claude-3-sonnet-20240229"
        input_cost_per_1k_tokens: 0.003
        output_cost_per_1k_tokens: 0.015
        max_tokens: 4096
        enabled: true

    rotation:
      strategy: "least_used"
      interval: "30m"
      health_check: true
      fallback_enabled: true

  gemini:
    api_keys:
      - key: "YOUR_GEMINI_API_KEY..."
        name: "primary"
        rate_limit: 600
        cost_limit: 50.0
        enabled: true
      - key: "YOUR_BACKUP_GEMINI_KEY..."
        name: "backup"
        rate_limit: 400
        cost_limit: 30.0
        enabled: true

    models:
      - name: "gemini-2.0-flash"
        input_cost_per_1k_tokens: 0.001
        output_cost_per_1k_tokens: 0.002
        max_tokens: 32768
        enabled: true
      - name: "gemini-pro-vision"
        input_cost_per_1k_tokens: 0.002
        output_cost_per_1k_tokens: 0.003
        max_tokens: 16384
        enabled: true

    rotation:
      strategy: "cost_optimized"
      interval: "15m"
      health_check: true
      fallback_enabled: true

global:
  fallback_chain: ["openai", "anthropic", "gemini"]
  global_rate_limit: 2000
  daily_cost_limit: 500.0
  cost_alert_threshold: 0.8
  encrypt_keys: true
  key_validation: true
  audit_logging: true
  default_rotation_strategy: "round_robin"
  health_check_interval: "5m"
  key_timeout: "30s"
```

### Configuration Fields

#### API Key Configuration

- `key`: The actual API key
- `name`: Friendly name for the key
- `rate_limit`: Requests per hour limit
- `cost_limit`: Daily cost limit in dollars
- `enabled`: Whether the key is active

#### Rotation Configuration

- `strategy`: Rotation algorithm to use
- `interval`: How often to consider rotation
- `health_check`: Enable health validation
- `fallback_enabled`: Enable automatic failover

#### Global Settings

- `fallback_chain`: Provider priority order
- `encrypt_keys`: Enable key encryption
- `key_validation`: Enable validation
- `daily_cost_limit`: Global cost limit

## 🧪 Testing Strategy

### Unit Tests

```bash
# Run unit tests
go test ./internal/config/...
go test ./internal/auth/...
```

### Integration Tests

```bash
# Test with actual API keys (use test keys)
go run examples/basic/main.go
```

### Load Testing

```go
// Simulate concurrent key requests
func TestConcurrentKeyRotation(t *testing.T) {
    // Implementation for testing concurrent access
}
```

## 🚀 Running the Example

1. **Setup Configuration**:

   ```bash
   cp gollm-config.yaml.example gollm-config.yaml
   # Edit with your actual API keys
   ```

2. **Set Environment Variables** (optional):

   ```bash
   export GOLLM_OPENAI_API_KEY_PRIMARY="your-openai-key"
   export GOLLM_ANTHROPIC_API_KEY_PRIMARY="your-anthropic-key"
   ```

3. **Run Example**:
   ```bash
   cd examples/basic
   go run main.go
   ```

## 🎯 Expected Output

```
=== OpenAI Completion Example ===
OpenAI Response: Why did the scarecrow win an award? Because he was outstanding in his field!
Model: gpt-3.5-turbo, Tokens: 28

=== Anthropic Chat Example ===
Anthropic Response: The capital of France is Paris.
Model: claude-3-sonnet-20240229, Tokens: 15

=== Gemini Example ===
Gemini Response: Quantum computing is like having a super-powerful calculator that can solve certain complex problems much faster than regular computers. Instead of using regular bits (0s and 1s), it uses quantum bits or "qubits" that can exist in multiple states at once, kind of like being able to be in multiple places at the same time.
Model: gemini-2.0-flash, Tokens: 42

Gemini Chat Response: Here are the key differences between quantum and classical computers:
1. Information Storage: Classical computers use bits (0 or 1), while quantum computers use qubits that can exist in multiple states simultaneously.
2. Processing Power: Quantum computers can solve certain complex problems exponentially faster.
3. Error Handling: Classical computers are more stable, while quantum computers are more sensitive to environmental factors.
4. Applications: Classical computers excel at everyday tasks, while quantum computers are better for specific problems like cryptography and molecular modeling.
Model: gemini-2.0-flash, Tokens: 89

=== Key Rotation Example ===
Provider: openai
  Iteration 1: Key=primary, Strategy=round_robin, LastUsed=14:30:15
  Iteration 2: Key=secondary, Strategy=round_robin, LastUsed=14:30:16

=== Key Validation Example ===
Provider: openai
  primary: ✓ Valid - Key is valid and active
  secondary: ✓ Valid - Key is valid and active
Provider: anthropic
  primary: ✓ Valid - Key is valid and active

=== Usage Tracking Example ===
Provider: openai, Key: primary
  Recorded: Small completion - 1500 tokens, $0.045
  Recorded: Medium completion - 3000 tokens, $0.090
  Recorded: Quick query - 500 tokens, $0.015
  Total usage: 3 requests, 5000 tokens, $0.150 cost

=== Health Status Example ===
Provider: openai
  primary: ✓ Healthy
  secondary: ✓ Healthy
Provider: anthropic
  primary: ✓ Healthy

=== Statistics Example ===
Provider: openai
  Total Keys: 2
  Healthy Keys: 2
  Total Cost: $0.150
  Total Tokens: 5000
  Total Requests: 3
    primary: Healthy, 2 requests, $0.105 cost
    secondary: Healthy, 1 requests, $0.045 cost
  Rotation Strategy: round_robin
  Current Index: 1
```

## 🛡️ Production Considerations

### Security Best Practices

1. **Environment Variables**: Store sensitive keys in environment variables
2. **Encryption**: Always enable key encryption in production
3. **Access Control**: Limit file permissions on config files
4. **Audit Logging**: Enable comprehensive logging

### Performance Optimization

1. **Caching**: Key selection results are cached appropriately
2. **Connection Pooling**: HTTP client reuse for validation
3. **Concurrent Safety**: All operations are thread-safe

### Monitoring & Alerting

1. **Cost Monitoring**: Set up alerts for cost thresholds
2. **Health Monitoring**: Monitor key health status
3. **Usage Analytics**: Track usage patterns and optimize

## 🔮 Future Enhancements

1. **Database Backend**: PostgreSQL/MySQL support for key storage
2. **Distributed Locking**: For multi-instance deployments
3. **Advanced Analytics**: ML-based cost optimization
4. **Webhook Integration**: Real-time notifications
5. **Key Lifecycle Management**: Automatic key rotation and renewal

## 🤝 Contributing

This is the foundational API key management system for GoLLM. Future enhancements will build upon this solid foundation to provide enterprise-grade AI tooling for Go applications.

## 📄 License

MIT License - see LICENSE file for details.
