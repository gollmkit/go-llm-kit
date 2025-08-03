# GoLLMKit

[![Go Version](https://img.shields.io/badge/Go-1.19+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/gollmkit/gollmkit)](https://goreportcard.com/report/github.com/gollmkit/gollmkit)
[![Documentation](https://godoc.org/github.com/gollmkit/gollmkit?status.svg)](https://godoc.org/github.com/gollmkit/gollmkit)

A comprehensive Go library for managing multiple LLM (Large Language Model) providers with intelligent API key management, rotation strategies, and unified interfaces. GoLLMKit simplifies working with OpenAI, Anthropic, Google Gemini, and other LLM providers while providing enterprise-grade features like cost optimization, health monitoring, and secure key management.

## 📋 Table of Contents

- [Features](#-features)
- [Installation](#-installation)
- [Quick Start](#-quick-start)
- [Configuration](#-configuration)
- [Usage](#-usage)
  - [Basic Usage](#basic-usage)
  - [Advanced Examples](#advanced-examples)
- [Key Management](#-key-management)
  - [Rotation Strategies](#rotation-strategies)
  - [Health Monitoring](#health-monitoring)
  - [Usage Tracking](#usage-tracking)
- [Providers](#-providers)
- [Security](#-security)
- [API Reference](#-api-reference)
- [Examples](#-examples)
- [Contributing](#-contributing)
- [License](#-license)

## ✨ Features

### 🔑 **Intelligent Key Management**

- **Multiple API Keys**: Support for multiple keys per provider with automatic rotation
- **Smart Rotation**: 5 different rotation strategies (round-robin, least-used, cost-optimized, random, single)
- **Health Monitoring**: Automatic key validation and health checks
- **Usage Tracking**: Real-time monitoring of requests, tokens, and costs

### 🔒 **Enterprise Security**

- **Encrypted Storage**: AES-GCM encryption for stored API keys
- **Environment Integration**: Secure key loading from environment variables
- **Access Control**: Fine-grained permissions and validation

### 🚀 **Multi-Provider Support**

- **OpenAI**: GPT-3.5, GPT-4, and newer models
- **Anthropic**: Claude 3 Sonnet, Haiku, and Opus
- **Google Gemini**: Gemini Pro, Vision, and Flash models
- **Unified Interface**: Single API for all providers

### 📊 **Cost & Performance Optimization**

- **Cost Tracking**: Real-time cost monitoring with configurable limits
- **Rate Limiting**: Built-in rate limiting per key
- **Fallback Chains**: Automatic provider failover
- **Load Balancing**: Intelligent request distribution

## 📦 Installation

```bash
go get github.com/gollmkit/gollmkit
```

### Requirements

- Go 1.19 or higher
- Valid API keys for desired LLM providers

## 🚀 Quick Start

### 1. Create Configuration File

Create a `gollmkit-config.yaml` file in your project root:

```yaml
providers:
  openai:
    api_keys:
      - key: "sk-proj-your-key-here..."
        name: "primary"
        rate_limit: 1000
        cost_limit: 100.0
        enabled: true
    rotation:
      strategy: "round_robin"
      interval: "1h"

global:
  encrypt_keys: true
  daily_cost_limit: 500.0
```

### 2. Basic Implementation

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

    // Initialize components
    keyStore, err := auth.NewKeyStoreFromConfig(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer keyStore.Close()

    rotator := auth.NewKeyRotator(cfg, keyStore)
    validator := auth.NewKeyValidator()

    // Create unified provider
    provider := providers.NewUnifiedProvider(cfg, rotator, validator)

    ctx := context.Background()

    // Simple completion
    response, err := provider.Invoke(ctx, "Tell me a joke", providers.RequestOptions{
        Provider: providers.OpenAI,
        Model:    "gpt-3.5-turbo",
        MaxTokens: 100,
    })

    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Response: %s\n", response.Content)
    fmt.Printf("Tokens used: %d\n", response.Usage.TotalTokens)
}
```

## ⚙️ Configuration

### Configuration Structure

GoLLMKit uses a hierarchical configuration system with three levels of precedence:

1. **Request Options** (highest priority)
2. **Configuration File** (medium priority)
3. **Default Values** (fallback)

### Complete Configuration Example

```yaml
# Provider-specific settings
providers:
  openai:
    api_keys:
      - key: "sk-proj-example1..."
        name: "primary"
        rate_limit: 1000 # requests/hour
        cost_limit: 100.0 # USD/day
        enabled: true
      - key: "sk-proj-example2..."
        name: "secondary"
        rate_limit: 800
        cost_limit: 75.0
        enabled: true

    models:
      - name: "gpt-4"
        input_cost_per_1k_tokens: 0.03
        output_cost_per_1k_tokens: 0.06
        max_tokens: 8192
        enabled: true

    rotation:
      strategy: "round_robin" # round_robin, least_used, cost_optimized, random, single
      interval: "1h" # rotation check interval
      health_check: true # enable health monitoring
      fallback_enabled: true # enable automatic failover

  anthropic:
    api_keys:
      - key: "sk-ant-example..."
        name: "primary"
        rate_limit: 500
        cost_limit: 80.0
        enabled: true

    rotation:
      strategy: "least_used"
      interval: "30m"

  gemini:
    api_keys:
      - key: "your-gemini-key..."
        name: "primary"
        rate_limit: 600
        cost_limit: 50.0
        enabled: true

# Global settings
global:
  fallback_chain: ["openai", "anthropic", "gemini"]
  global_rate_limit: 2000
  daily_cost_limit: 500.0
  cost_alert_threshold: 0.8
  encrypt_keys: true
  key_validation: true
  audit_logging: true
  health_check_interval: "5m"
  key_timeout: "30s"
```

### Environment Variable Override

```bash
# Override API keys securely
export GOLLM_OPENAI_API_KEY_PRIMARY="sk-proj-your-key..."
export GOLLM_ANTHROPIC_API_KEY_PRIMARY="sk-ant-your-key..."
export GOLLM_GEMINI_API_KEY_PRIMARY="your-gemini-key..."
```

## 💻 Usage

### Basic Usage

#### Simple Text Completion

```go
// Using OpenAI
response, err := provider.Invoke(ctx, "Explain quantum computing", providers.RequestOptions{
    Provider:    providers.OpenAI,
    Model:       "gpt-4",
    MaxTokens:   200,
    Temperature: 0.7,
})

// Using Anthropic
response, err = provider.Invoke(ctx, "Write a haiku about programming", providers.RequestOptions{
    Provider: providers.Anthropic,
    Model:    "claude-3-sonnet-20240229",
})
```

#### Chat Completion

```go
messages := []providers.Message{
    {Role: "system", Content: "You are a helpful coding assistant."},
    {Role: "user", Content: "How do I implement a binary search in Go?"},
}

response, err := provider.Chat(ctx, messages, providers.RequestOptions{
    Provider:    providers.OpenAI,
    Model:       "gpt-4",
    MaxTokens:   500,
    Temperature: 0.3,
})
```

### Advanced Examples

#### Multi-Provider Fallback

```go
// Try OpenAI first, fallback to Anthropic if it fails
providers := []providers.ProviderType{providers.OpenAI, providers.Anthropic}

for _, prov := range providers {
    response, err := provider.Invoke(ctx, prompt, providers.RequestOptions{
        Provider: prov,
        Model:    getModelForProvider(prov),
    })

    if err == nil {
        fmt.Printf("Success with %s: %s\n", prov, response.Content)
        break
    }
    log.Printf("Provider %s failed: %v", prov, err)
}
```

#### Streaming Responses

```go
stream, err := provider.InvokeStream(ctx, "Tell me a long story", providers.RequestOptions{
    Provider: providers.OpenAI,
    Model:    "gpt-4",
    Stream:   true,
})

if err != nil {
    log.Fatal(err)
}

for chunk := range stream {
    if chunk.Error != nil {
        log.Printf("Stream error: %v", chunk.Error)
        break
    }
    fmt.Print(chunk.Content)
}
```

## 🔑 Key Management

### Rotation Strategies

#### 1. Round Robin

Cycles through keys evenly:

```yaml
rotation:
  strategy: "round_robin"
  interval: "1h"
```

#### 2. Least Used

Selects key with lowest usage:

```yaml
rotation:
  strategy: "least_used"
  interval: "30m"
```

#### 3. Cost Optimized

Chooses key with lowest daily cost:

```yaml
rotation:
  strategy: "cost_optimized"
  interval: "15m"
```

#### 4. Random

Random key selection:

```yaml
rotation:
  strategy: "random"
```

#### 5. Single Key

Uses primary key only:

```yaml
rotation:
  strategy: "single"
```

### Health Monitoring

```go
// Create health checker
healthChecker := auth.NewHealthChecker(keyStore, 5*time.Minute)

// Get health status
providers := map[string][]string{
    "openai": {"primary", "secondary"},
    "anthropic": {"primary"},
}

healthStatus, err := healthChecker.GetHealthStatus(ctx, providers)
for provider, keys := range healthStatus {
    for keyName, isHealthy := range keys {
        status := "✓ Healthy"
        if !isHealthy {
            status = "✗ Unhealthy"
        }
        fmt.Printf("%s.%s: %s\n", provider, keyName, status)
    }
}
```

### Usage Tracking

```go
// Record usage
err := rotator.RecordUsage(ctx, "openai", "primary", 1500, 0.045)

// Get statistics
stats, err := rotator.GetKeyStatistics(ctx, "openai")
for keyName, usage := range stats {
    fmt.Printf("Key: %s\n", keyName)
    fmt.Printf("  Requests: %d\n", usage.UsageCount)
    fmt.Printf("  Tokens: %d\n", usage.TokensUsed)
    fmt.Printf("  Cost: $%.3f\n", usage.CostUsed)
}
```

## 🏢 Providers

### Supported Providers

| Provider          | Models                         | Features         |
| ----------------- | ------------------------------ | ---------------- |
| **OpenAI**        | GPT-3.5, GPT-4, GPT-4 Turbo    | Chat, Completion |
| **Anthropic**     | Claude 3 (Sonnet, Haiku, Opus) | Chat, Completion |
| **Google Gemini** | Gemini Pro, Flash              | Chat, Completion |

### Provider-Specific Configuration

```go
// OpenAI with specific settings
openaiOpts := providers.RequestOptions{
    Provider:     providers.OpenAI,
    Model:        "gpt-4",
    Temperature:  0.7,
    MaxTokens:    2000,
    TopP:         0.9,
    TopP:         0.9,
}

// Anthropic with message options
anthropicOpts := providers.RequestOptions{
    Provider:    providers.Anthropic,
    Model:       "claude-3-sonnet-20240229",
    MaxTokens:   1000,
    Temperature: 0.7,
}

// Gemini with model options
geminiOpts := providers.RequestOptions{
    Provider:    providers.Gemini,
    Model:       "gemini-2.0-flash",
    MaxTokens:   1500,
    Temperature: 0.7,
    TopP:        0.9,
}
```

## 🔒 Security

### Key Encryption

Keys are automatically encrypted using AES-GCM:

```go
// Encryption is enabled by default in configuration
global:
  encrypt_keys: true
```

### Best Practices

1. **Environment Variables**: Store sensitive keys in environment variables
2. **File Permissions**: Restrict access to configuration files
3. **Audit Logging**: Enable comprehensive request logging
4. **Key Rotation**: Regularly rotate API keys
5. **Cost Monitoring**: Set up cost alerts and limits

### Access Control

```go
// Validate keys before use
validator := auth.NewKeyValidator()
result, err := validator.ValidateKey(ctx, keyStore, "openai", "primary")
if !result.Valid {
    log.Printf("Key validation failed: %s", result.Message)
}
```

## 📖 API Reference

### Core Types

```go
// Request options for LLM calls
type RequestOptions struct {
    Provider         ProviderType
    Model           string
    MaxTokens       int
    Temperature     float64
    TopP           float64
    Stream         bool
    SystemPrompt   string
}

// Response from LLM providers
type Response struct {
    Content    string
    Model      string
    Usage      Usage
    Provider   ProviderType
    Metadata   map[string]interface{}
}

// Usage statistics
type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    Cost            float64
}
```

### Main Interfaces

```go
// Unified provider interface
type UnifiedProvider interface {
    Invoke(ctx context.Context, prompt string, opts RequestOptions) (*Response, error)
    Chat(ctx context.Context, messages []Message, opts RequestOptions) (*Response, error)
    InvokeStream(ctx context.Context, prompt string, opts RequestOptions) (<-chan StreamChunk, error)
}

// Key management interface
type KeyRotator interface {
    GetNextKey(ctx context.Context, provider string) (*KeySelection, error)
    RecordUsage(ctx context.Context, provider, keyName string, tokens int, cost float64) error
    GetKeyStatistics(ctx context.Context, provider string) (map[string]*KeyUsage, error)
}
```

## 📚 Examples

### Complete Working Example

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
    // Initialize GoLLMKit
    cfg, err := config.LoadConfig("gollmkit-config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    keyStore, err := auth.NewKeyStoreFromConfig(cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer keyStore.Close()

    rotator := auth.NewKeyRotator(cfg, keyStore)
    validator := auth.NewKeyValidator()
    provider := providers.NewUnifiedProvider(cfg, rotator, validator)

    ctx := context.Background()

    // Example 1: Simple completion
    fmt.Println("=== Simple Completion ===")
    response, err := provider.Invoke(ctx, "What is the capital of France?", providers.RequestOptions{
        Provider: providers.OpenAI,
        Model:    "gpt-3.5-turbo",
    })
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        fmt.Printf("Answer: %s\n", response.Content)
        fmt.Printf("Tokens: %d, Cost: $%.4f\n", response.Usage.TotalTokens, response.Usage.Cost)
    }

    // Example 2: Chat conversation
    fmt.Println("\n=== Chat Conversation ===")
    messages := []providers.Message{
        {Role: "system", Content: "You are a helpful programming tutor."},
        {Role: "user", Content: "Explain what a closure is in programming."},
    }

    response, err = provider.Chat(ctx, messages, providers.RequestOptions{
        Provider:    providers.Anthropic,
        Model:       "claude-3-sonnet-20240229",
        MaxTokens:   300,
        Temperature: 0.7,
    })
    if err != nil {
        log.Printf("Error: %v", err)
    } else {
        fmt.Printf("Explanation: %s\n", response.Content)
    }

    // Example 3: Key rotation demonstration
    fmt.Println("\n=== Key Rotation ===")
    for i := 0; i < 3; i++ {
        selection, err := rotator.GetNextKey(ctx, "openai")
        if err != nil {
            log.Printf("Error getting key: %v", err)
            continue
        }
        fmt.Printf("Iteration %d: Using key '%s' with strategy '%s'\n",
                  i+1, selection.KeyName, selection.Strategy)
    }

    // Example 4: Usage statistics
    fmt.Println("\n=== Usage Statistics ===")
    stats, err := rotator.GetKeyStatistics(ctx, "openai")
    if err != nil {
        log.Printf("Error getting stats: %v", err)
    } else {
        for keyName, usage := range stats {
            fmt.Printf("Key '%s': %d requests, %d tokens, $%.4f cost\n",
                      keyName, usage.UsageCount, usage.TokensUsed, usage.CostUsed)
        }
    }
}
```

For more examples, see the [examples directory](examples/).

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/gollmkit/gollmkit.git
cd gollmkit

# Install dependencies
go mod tidy

# Run tests
go test ./...

# Run examples
go run examples/basic/main.go
```

### Submitting Changes

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 🐛 Issues and Support

- **Bug Reports**: [GitHub Issues](https://github.com/gollmkit/gollmkit/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/gollmkit/gollmkit/discussions)
- **Documentation**: [Wiki](https://github.com/gollmkit/gollmkit/wiki)

## 🗺️ Roadmap

- [ ] **Database Backend**: PostgreSQL/MySQL support for key storage
- [ ] **Distributed Locking**: Multi-instance deployment support
- [ ] **Advanced Analytics**: ML-based cost optimization
- [ ] **Webhook Integration**: Real-time notifications
- [ ] **More Providers**: Cohere, Replicate, Hugging Face
- [ ] **GraphQL API**: Alternative API interface
- [ ] **Kubernetes Operator**: Native K8s integration

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [OpenAI](https://openai.com) for GPT models
- [Anthropic](https://anthropic.com) for Claude models
- [Google](https://ai.google.dev) for Gemini models
- The Go community for excellent tooling and libraries

---

**GoLLMKit** - Simplifying LLM integration for Go developers

[Documentation](https://godoc.org/github.com/gollmkit/gollmkit) • [Examples](examples/) • [Contributing](CONTRIBUTING.md) • [License](LICENSE)
