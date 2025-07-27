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
    "log"

    "github.com/gollm/internal/auth"
    "github.com/gollm/internal/config"
)

func main() {
    // Load configuration
    cfg, err := config.LoadConfig("gollm-config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Create key store and rotator
    keyStore, err := auth.NewKeyStoreFromConfig(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer keyStore.Close()

    rotator := auth.NewKeyRotator(cfg, keyStore)

    // Get next key for OpenAI
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
=== OpenAI Key Rotation Example ===
Provider: openai
  Iteration 1: Key=primary, Strategy=round_robin, LastUsed=14:30:15
  Iteration 2: Key=secondary, Strategy=round_robin, LastUsed=14:30:16
  Iteration 3: Key=primary, Strategy=round_robin, LastUsed=14:30:17

=== Key Validation Example ===
Provider: openai
  primary: ✓ Valid - Key is valid and active
  secondary: ✓ Valid - Key is valid and active

=== Usage Tracking Example ===
Provider: openai, Key: primary
  Recorded: Small completion - 1500 tokens, $0.045
  Recorded: Medium completion - 3000 tokens, $0.090
  Total usage: 3 requests, 5000 tokens, $0.150 cost
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
