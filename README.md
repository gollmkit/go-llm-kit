# GoLLMKit - AI-First Go Library

GoLLMKit is an AI-first Go library that provides unified LLM management, intelligent cost optimization, and advanced RAG capabilities. Built specifically for production environments that require enterprise-grade features like API key rotation, cost tracking, and provider failover.

## 🚀 Key Features

### **Phase 1 - Core LLM Management** ✅

- **Multi-Provider Support**: OpenAI, Anthropic, Google Gemini (easily extensible)
- **Intelligent API Key Rotation**: Round-robin, weighted, random, and failover algorithms
- **Advanced Failover**: Automatic provider switching with configurable retry logic
- **Cost Tracking**: Real-time cost estimation and budget monitoring
- **Production Ready**: Comprehensive error handling, logging, and monitoring

### **Coming Soon**

- **Advanced RAG System**: Pre/during/post retrieval techniques
- **Built-in AI Features**: Chat with memory, summarization, math solver
- **Analytics Dashboard**: Web and CLI dashboards for usage monitoring
- **Enterprise Features**: Role-based access, audit logs, compliance tools

## 📦 Installation

```bash
# Initialize your Go module
go mod init your-project

# Install GoLLMKit
go get github.com/gollmkit/gollmkit
```

## 🏃‍♂️ Quick Start

### 1. Environment Setup

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-your-api-key-here"

# Or set multiple keys for rotation
export OPENAI_API_KEY_1="sk-primary-key"
export OPENAI_API_KEY_2="sk-backup-key"
```

### 2. Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/gollmkit/gollmkit/pkg/gollmkit"
)

func main() {
    // Create client with default settings
    client, err := gollmkit.New(gollmkit.DefaultClientOptions())
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Create a request
    request := &gollmkit.Request{
        SystemPrompt: "You are a helpful assistant.",
        UserPrompt:   "Explain Go interfaces in simple terms.",
        MaxTokens:    200,
    }

    // Send request
    ctx := context.Background()
    response, err := client.Complete(ctx, request)
    if err != nil {
        log.Fatal(err)
    }

    // Use the response
    fmt.Printf("Response: %s\n", response.Content)
    fmt.Printf("Provider: %s\n", response.Provider.Name)
    fmt.Printf("Tokens: %d\n", response.Usage.TotalTokens)
    fmt.Printf("Cost: $%.6f\n", response.Provider.Cost)
}
```

### 3. Configuration File

Create `gollmkit.yaml`:

```yaml
client:
  default_provider: "openai"
  enabled_providers: ["openai"]
  rotation_algorithm: "round_robin" # round_robin, weighted, random, failover
  default_max_tokens: 1000
  default_temperature: 0.7

providers:
  openai:
    name: "openai"
    enabled: true
    default_model: "gpt-3.5-turbo"
    api_keys:
      - id: "primary"
        key: "sk-your-primary-key"
        name: "Primary Key"
        weight: 70 # 70% of requests
        priority: 1 # Highest priority for failover
        enabled: true
      - id: "backup"
        key: "sk-your-backup-key"
        name: "Backup Key"
        weight: 30 # 30% of requests
        priority: 2 # Lower priority
        enabled: true

analytics:
  enabled: true
  storage_path: "./gollmkit_analytics.db"

logging:
  level: "info"
  format: "text"
```

## 🛠 CLI Tool

GoLLMKit includes a powerful CLI tool for testing and management:

```bash
# Build the CLI
go build -o gollmkit ./cmd/gollmkit

# Initialize configuration
./gollmkit config init

# Test a simple chat
./gollmkit chat "What is Go programming language?"

# Complete a prompt with specific options
./gollmkit complete "Explain microservices" \
  --system "You are a software architect" \
  --max-tokens 500 \
  --temperature 0.3

# List available providers
./gollmkit providers list

# Get detailed provider information
./gollmkit providers info openai

# Validate all API keys
./gollmkit validate

# Show usage statistics
./gollmkit stats
```

## 📚 Advanced Usage

### Multiple Providers with Failover

```go
request := &gollmkit.Request{
    UserPrompt:         "Explain quantum computing",
    MaxTokens:          300,
    PreferredProviders: []string{"openai"},           // Try OpenAI first
    FallbackProviders:  []string{"anthropic", "gemini"}, // Then try these
}

response, err := client.Complete(ctx, request)
```

### JSON Output Format

```go
request := &gollmkit.Request{
    SystemPrompt: "You are a data analyst. Always respond with valid JSON.",
    UserPrompt:   "Analyze the pros and cons of microservices. Use JSON format with 'pros' and 'cons' arrays.",
    OutputFormat: gollmkit.OutputJSON,
    Temperature:  0.1, // Low temperature for structured output
}
```

### Batch Processing

```go
// Process multiple requests concurrently
requests := []*gollmkit.Request{
    {UserPrompt: "Explain REST APIs", MaxTokens: 100},
    {UserPrompt: "What is GraphQL?", MaxTokens: 100},
    {UserPrompt: "Compare REST vs GraphQL", MaxTokens: 150},
}

// Use goroutines for concurrent processing
for _, req := range requests {
    go func(r *gollmkit.Request) {
        response, err := client.Complete(ctx, r)
        // Handle response...
    }(req)
}
```

### API Key Validation

```go
// Validate all configured API keys
results, err := client.ValidateAPIKeys(ctx)
for providerName, providerResults := range results {
    for _, result := range providerResults {
        if result.Valid {
            fmt.Printf("✅ %s: %s is valid\n", providerName, result.KeyID)
        } else {
            fmt.Printf("❌ %s: %s failed - %s\n", providerName, result.KeyID, result.Error)
        }
    }
}
```

## 🔧 Configuration Options

### Rotation Algorithms

1. **Round Robin** (`round_robin`): Cycles through keys sequentially
2. **Weighted** (`weighted`): Distributes requests based on key weights
3. **Random** (`random`): Randomly selects keys
4. **Failover** (`failover`): Uses priority order, switches on failure
