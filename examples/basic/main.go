package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/gollmkit/gollmkit/internal/auth"
	"github.com/gollmkit/gollmkit/internal/config"
)

func main() {
	// Load configuration from YAML file
	cfg, err := config.LoadConfig("gollmkit-config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create key store
	keyStore, err := auth.NewKeyStoreFromConfig(cfg)
	if err != nil {
		log.Fatalf("Failed to create key store: %v", err)
	}
	defer keyStore.Close()

	// Create key rotator
	rotator := auth.NewKeyRotator(cfg, keyStore)

	ctx := context.Background()

	// Example 1: Get next key for OpenAI
	fmt.Println("=== OpenAI Key Rotation Example ===")
	demonstrateKeyRotation(ctx, rotator, "openai", 5)

	// Example 2: Get next key for Anthropic
	fmt.Println("\n=== Anthropic Key Rotation Example ===")
	demonstrateKeyRotation(ctx, rotator, "anthropic", 3)

	// Example 3: Validate all keys
	fmt.Println("\n=== Key Validation Example ===")
	demonstrateKeyValidation(ctx, keyStore, cfg)

	// Example 4: Track usage and costs
	fmt.Println("\n=== Usage Tracking Example ===")
	demonstrateUsageTracking(ctx, rotator)

	// Example 5: Health checking
	fmt.Println("\n=== Health Checking Example ===")
	demonstrateHealthChecking(ctx, keyStore, cfg)

	// Example 6: Statistics
	fmt.Println("\n=== Statistics Example ===")
	demonstrateStatistics(ctx, rotator)
}

// demonstrateKeyRotation shows how key rotation works
func demonstrateKeyRotation(ctx context.Context, rotator *auth.KeyRotator, provider string, iterations int) {
	fmt.Printf("Provider: %s\n", provider)

	for i := 0; i < iterations; i++ {
		selection, err := rotator.GetNextKey(ctx, provider)
		if err != nil {
			fmt.Printf("  Error getting key: %v\n", err)
			continue
		}

		fmt.Printf("  Iteration %d: Key=%s, Strategy=%s, LastUsed=%s\n",
			i+1,
			selection.KeyName,
			selection.Strategy,
			selection.LastUsed.Format("15:04:05"))

		// Simulate usage
		err = rotator.RecordUsage(ctx, provider, selection.KeyName, 1000, 0.05)
		if err != nil {
			fmt.Printf("    Error recording usage: %v\n", err)
		}

		// Small delay to show time differences
		time.Sleep(100 * time.Millisecond)
	}
}

// demonstrateKeyValidation shows how to validate API keys
func demonstrateKeyValidation(ctx context.Context, keyStore auth.KeyStore, cfg *config.Config) {
	validator := auth.NewKeyValidator()

	// Build provider-key mapping
	providers := make(map[string][]string)
	for providerName := range cfg.Providers {
		keyNames, err := keyStore.ListKeys(ctx, providerName)
		if err != nil {
			fmt.Printf("Error listing keys for %s: %v\n", providerName, err)
			continue
		}
		providers[providerName] = keyNames
	}

	// Validate all keys
	results, err := validator.ValidateAllKeys(ctx, keyStore, providers)
	if err != nil {
		fmt.Printf("Error validating keys: %v\n", err)
		return
	}

	// Display results
	for provider, providerResults := range results {
		fmt.Printf("Provider: %s\n", provider)
		for keyName, result := range providerResults {
			status := "✓ Valid"
			if !result.Valid {
				status = "✗ Invalid"
			}
			fmt.Printf("  %s: %s - %s\n", keyName, status, result.Message)
		}
	}
}

// demonstrateUsageTracking shows usage tracking functionality
func demonstrateUsageTracking(ctx context.Context, rotator *auth.KeyRotator) {
	// Simulate some usage
	providers := []string{"openai", "anthropic"}

	for _, provider := range providers {
		// Get a key
		selection, err := rotator.GetNextKey(ctx, provider)
		if err != nil {
			fmt.Printf("Error getting key for %s: %v\n", provider, err)
			continue
		}

		// Simulate different usage patterns
		usageScenarios := []struct {
			tokens int
			cost   float64
			desc   string
		}{
			{1500, 0.045, "Small completion"},
			{3000, 0.090, "Medium completion"},
			{500, 0.015, "Quick query"},
		}

		fmt.Printf("Provider: %s, Key: %s\n", provider, selection.KeyName)

		for _, scenario := range usageScenarios {
			err = rotator.RecordUsage(ctx, provider, selection.KeyName, scenario.tokens, scenario.cost)
			if err != nil {
				fmt.Printf("  Error recording usage: %v\n", err)
				continue
			}
			fmt.Printf("  Recorded: %s - %d tokens, $%.3f\n",
				scenario.desc, scenario.tokens, scenario.cost)
		}

		// Get updated usage stats
		usage, err := rotator.GetKeyStatistics(ctx, provider)
		if err != nil {
			fmt.Printf("  Error getting statistics: %v\n", err)
			continue
		}

		if keyUsage, exists := usage[selection.KeyName]; exists {
			fmt.Printf("  Total usage: %d requests, %d tokens, $%.3f cost\n",
				keyUsage.UsageCount, keyUsage.TokensUsed, keyUsage.CostUsed)
		}
	}
}

// demonstrateHealthChecking shows health checking functionality
func demonstrateHealthChecking(ctx context.Context, keyStore auth.KeyStore, cfg *config.Config) {
	// Create health checker
	healthChecker := auth.NewHealthChecker(keyStore, 30*time.Second)

	// Build provider-key mapping
	providers := make(map[string][]string)
	for providerName := range cfg.Providers {
		keyNames, err := keyStore.ListKeys(ctx, providerName)
		if err != nil {
			continue
		}
		providers[providerName] = keyNames
	}

	// Get current health status
	healthStatus, err := healthChecker.GetHealthStatus(ctx, providers)
	if err != nil {
		fmt.Printf("Error getting health status: %v\n", err)
		return
	}

	// Display health status
	for provider, keys := range healthStatus {
		fmt.Printf("Provider: %s\n", provider)
		for keyName, healthy := range keys {
			status := "✓ Healthy"
			if !healthy {
				status = "✗ Unhealthy"
			}
			fmt.Printf("  %s: %s\n", keyName, status)
		}
	}

	// Note: In a real application, you would start the health checker
	// with healthChecker.Start(ctx, providers) in a separate goroutine
}

// demonstrateStatistics shows statistics functionality
func demonstrateStatistics(ctx context.Context, rotator *auth.KeyRotator) {
	providers := []string{"openai", "anthropic"}

	for _, provider := range providers {
		// Get provider statistics
		stats, err := rotator.GetProviderStatistics(ctx, provider)
		if err != nil {
			fmt.Printf("Error getting statistics for %s: %v\n", provider, err)
			continue
		}

		fmt.Printf("Provider: %s\n", provider)
		fmt.Printf("  Total Keys: %d\n", stats.TotalKeys)
		fmt.Printf("  Healthy Keys: %d\n", stats.HealthyKeys)
		fmt.Printf("  Total Cost: $%.3f\n", stats.TotalCost)
		fmt.Printf("  Total Tokens: %d\n", stats.TotalTokens)
		fmt.Printf("  Total Requests: %d\n", stats.TotalRequests)

		// Show per-key stats
		for keyName, keyStats := range stats.KeyStats {
			healthStatus := "Healthy"
			if !keyStats.Healthy {
				healthStatus = "Unhealthy"
			}
			fmt.Printf("    %s: %s, %d requests, $%.3f cost\n",
				keyName, healthStatus, keyStats.Usage.UsageCount, keyStats.Usage.CostUsed)
		}

		// Get rotation status
		rotationStatus, err := rotator.GetRotationStatus(ctx, provider)
		if err != nil {
			fmt.Printf("  Error getting rotation status: %v\n", err)
			continue
		}

		fmt.Printf("  Rotation Strategy: %s\n", rotationStatus.Strategy)
		if rotationStatus.Strategy == config.RotationRoundRobin {
			fmt.Printf("  Current Index: %d\n", rotationStatus.CurrentIndex)
		}
		if !rotationStatus.LastRotation.IsZero() {
			fmt.Printf("  Last Rotation: %s\n", rotationStatus.LastRotation.Format("15:04:05"))
		}
	}
}
