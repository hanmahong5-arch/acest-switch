// Gateway Service - Standalone AI Provider Proxy
// Extracted from CodeSwitch for server deployment
package main

import (
	"codeswitch/services"
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Get configuration from environment
	port := getEnv("GATEWAY_PORT", "18100")
	newAPIEnabled := getEnv("NEW_API_ENABLED", "false") == "true"
	newAPIURL := getEnv("NEW_API_URL", "")
	newAPIToken := getEnv("NEW_API_TOKEN", "")
	enableBodyLog := getEnv("ENABLE_BODY_LOG", "false") == "true"

	log.Printf("[Gateway] Starting AI Provider Gateway Service")
	log.Printf("[Gateway] Port: %s", port)
	log.Printf("[Gateway] NEW-API Enabled: %v", newAPIEnabled)

	// Initialize services
	providerService := services.NewProviderService()
	providerRelay := services.NewProviderRelayService(providerService, ":"+port)

	// Configure options
	providerRelay.SetBodyLogEnabled(enableBodyLog)
	if enableBodyLog {
		log.Printf("[Gateway] Body logging enabled")
	}

	// Configure NEW-API mode
	if newAPIEnabled && newAPIURL != "" && newAPIToken != "" {
		providerRelay.SetNewAPIConfig(newAPIURL, newAPIToken)
		providerRelay.SetNewAPIEnabled(true)
		log.Printf("[Gateway] NEW-API gateway mode enabled: %s", newAPIURL)
	}

	// Initialize sync integration (optional)
	syncSettingsService := services.NewSyncSettingsService()
	services.InitSyncIntegration(syncSettingsService)
	if si := services.GetSyncIntegration(); si != nil {
		providerRelay.SetSyncIntegration(si)
		if si.IsEnabled() {
			log.Printf("[Gateway] Multi-device sync enabled")
		}
	}

	// Run data migrations
	providerRelay.RunMigrations()

	// Start the HTTP server
	go func() {
		if err := providerRelay.Start(); err != nil {
			log.Printf("[Gateway] Server error: %v", err)
		}
	}()

	log.Printf("[Gateway] Service started on port %s", port)

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Printf("[Gateway] Shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := providerRelay.Stop(); err != nil {
		log.Printf("[Gateway] Error stopping relay: %v", err)
	}

	if err := syncSettingsService.ServiceShutdown(); err != nil {
		log.Printf("[Gateway] Error stopping sync: %v", err)
	}

	<-ctx.Done()
	log.Printf("[Gateway] Service stopped")
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
