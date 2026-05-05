// SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

// Main entry point for the OpenCHAMI Boot Service
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/clients/hsm"
)

// Config holds all configuration for the boot service
type Config struct {
	// Server Configuration
	Port         int    `mapstructure:"port"`
	Host         string `mapstructure:"host"`
	ReadTimeout  int    `mapstructure:"read_timeout"`
	WriteTimeout int    `mapstructure:"write_timeout"`
	IdleTimeout  int    `mapstructure:"idle_timeout"`

	// Storage Configuration
	DataDir     string `mapstructure:"data_dir"`
	StorageType string `mapstructure:"storage_type"` // file, database

	// Feature Flags
	EnableAuth      bool `mapstructure:"enable_auth"`
	EnableMetrics   bool `mapstructure:"enable_metrics"`
	EnableLegacyAPI bool `mapstructure:"enable_legacy_api"`
	MetricsPort     int  `mapstructure:"metrics_port"`

	// Authentication Configuration (when enabled)
	TokenSmithURL                       string `mapstructure:"tokensmith_url"`
	TokenSmithBootstrapToken            string `mapstructure:"tokensmith_bootstrap_token"`
	TokenSmithTargetService             string `mapstructure:"tokensmith_target_service"`
	TokenSmithBootstrapPolicyScopesHint string `mapstructure:"tokensmith_bootstrap_policy_scopes_hint"`
	TokenSmithScopesLegacy              string `mapstructure:"tokensmith_scopes"`
	TokenSmithRefreshSkewSec            int    `mapstructure:"tokensmith_refresh_skew_sec"`
	JWKSEndpoint                        string `mapstructure:"jwks_endpoint"`

	// Hardware State Manager Configuration (when enabled)
	HSMURL          string `mapstructure:"hsm_url"`
	HSMSyncEnabled  bool   `mapstructure:"hsm_sync_enabled"`
	HSMSyncInterval int    `mapstructure:"hsm_sync_interval"` // in minutes
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port:                                8080,
		Host:                                "0.0.0.0",
		ReadTimeout:                         30,
		WriteTimeout:                        30,
		IdleTimeout:                         120,
		DataDir:                             "./data",
		StorageType:                         "file",
		EnableAuth:                          false,
		EnableMetrics:                       false,
		EnableLegacyAPI:                     true,
		MetricsPort:                         9090,
		TokenSmithURL:                       "",
		TokenSmithBootstrapToken:            "",
		TokenSmithTargetService:             "hsm",
		TokenSmithBootstrapPolicyScopesHint: "",
		TokenSmithScopesLegacy:              "",
		TokenSmithRefreshSkewSec:            120,
		JWKSEndpoint:                        "",
		HSMURL:                              "",
		HSMSyncEnabled:                      true,
		HSMSyncInterval:                     5, // 5 minutes
	}
}

var rootCmd = &cobra.Command{
	Use:   "boot-service",
	Short: "OpenCHAMI Boot Service",
	Long:  "A microservice for managing node boot configurations in OpenCHAMI",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the boot service server",
	Long:  "Start the HTTP server for the boot service with configurable features",
	RunE:  runServe,
}

func init() {
	// Server configuration flags
	serveCmd.Flags().Int("port", 8080, "Port to listen on")
	serveCmd.Flags().String("host", "0.0.0.0", "Host to bind to")
	serveCmd.Flags().Int("read-timeout", 30, "Read timeout in seconds")
	serveCmd.Flags().Int("write-timeout", 30, "Write timeout in seconds")
	serveCmd.Flags().Int("idle-timeout", 120, "Idle timeout in seconds")

	// Storage configuration flags
	serveCmd.Flags().String("data-dir", "./data", "Directory for file storage")
	serveCmd.Flags().String("storage-type", "file", "Storage backend: file or database")

	// Feature flags
	serveCmd.Flags().Bool("enable-auth", false, "Enable authentication with TokenSmith")
	serveCmd.Flags().Bool("enable-metrics", false, "Enable Prometheus metrics")
	serveCmd.Flags().Bool("enable-legacy-api", true, "Enable legacy BSS API compatibility")
	serveCmd.Flags().Int("metrics-port", 9090, "Port for metrics endpoint")

	// Authentication configuration flags
	serveCmd.Flags().String("tokensmith_url", "", "TokenSmith service URL for authentication")
	serveCmd.Flags().String("tokensmith-bootstrap-token", "", "Bootstrap token used to exchange HSM service tokens")
	serveCmd.Flags().String("tokensmith-target-service", "hsm", "Target service audience for HSM service token exchange")
	serveCmd.Flags().String("tokensmith-bootstrap-policy-scopes-hint", "", "Comma-separated scope hint from bootstrap token policy used for diagnostics only")
	serveCmd.Flags().String("tokensmith-scopes", "", "Deprecated alias for --tokensmith-bootstrap-policy-scopes-hint")
	serveCmd.Flags().Int("tokensmith-refresh-skew-sec", 120, "Refresh service tokens when this many seconds remain before expiry")
	serveCmd.Flags().String("jwks-endpoint", "", "JWKS endpoint for JWT validation")

	// Hardware State Manager configuration flags
	serveCmd.Flags().String("hsm-url", "", "Hardware State Manager service URL (enables HSM when provided)")
	serveCmd.Flags().Bool("hsm-sync-enabled", true, "Enable background sync with HSM")
	serveCmd.Flags().Int("hsm-sync-interval", 5, "HSM sync interval in minutes")

	// Bind flags to viper
	viper.BindPFlags(serveCmd.Flags()) //nolint:errcheck

	// Add commands
	rootCmd.AddCommand(serveCmd)
}

func main() {
	// Setup configuration
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc/boot-service/")
	viper.AddConfigPath("$HOME/.boot-service")

	// Enable environment variable overrides
	viper.SetEnvPrefix("BOOT_SERVICE")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	// Register aliases for flags with dashes to work with mapstructure tags that use underscores
	viper.RegisterAlias("hsm_url", "hsm-url")
	viper.RegisterAlias("hsm_sync_enabled", "hsm-sync-enabled")
	viper.RegisterAlias("hsm_sync_interval", "hsm-sync-interval")
	viper.RegisterAlias("tokensmith_bootstrap_token", "tokensmith-bootstrap-token")
	viper.RegisterAlias("tokensmith_target_service", "tokensmith-target-service")
	viper.RegisterAlias("tokensmith_bootstrap_policy_scopes_hint", "tokensmith-bootstrap-policy-scopes-hint")
	viper.RegisterAlias("tokensmith_scopes", "tokensmith-scopes")
	viper.RegisterAlias("tokensmith_refresh_skew_sec", "tokensmith-refresh-skew-sec")

	// Standardized TokenSmith env vars for cross-service UX consistency.
	viper.BindEnv("tokensmith_url", "TOKENSMITH_URL")                                                   //nolint:errcheck
	viper.BindEnv("tokensmith_bootstrap_token", "TOKENSMITH_BOOTSTRAP_TOKEN")                           //nolint:errcheck
	viper.BindEnv("tokensmith_target_service", "TOKENSMITH_TARGET_SERVICE")                             //nolint:errcheck
	viper.BindEnv("tokensmith_bootstrap_policy_scopes_hint", "TOKENSMITH_BOOTSTRAP_POLICY_SCOPES_HINT") //nolint:errcheck
	viper.BindEnv("tokensmith_scopes", "TOKENSMITH_SCOPES")                                             //nolint:errcheck
	viper.BindEnv("tokensmith_refresh_skew_sec", "TOKENSMITH_REFRESH_SKEW_SEC")                         //nolint:errcheck

	// Read config file if present
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			log.Printf("Error reading config file: %v", err)
		}
	}

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func runServe(cmd *cobra.Command, args []string) error { //nolint:revive
	// Load configuration
	config := DefaultConfig()
	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	} // Validate configuration
	if err := validateConfig(config); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// Print startup configuration
	log.Printf("Starting boot service with configuration:")
	log.Printf("  Server: %s:%d", config.Host, config.Port)
	log.Printf("  Storage: %s (%s)", config.StorageType, config.DataDir)
	log.Printf("  Features: auth=%v, hsm=%v, metrics=%v, legacy-api=%v",
		config.EnableAuth, config.HSMURL != "", config.EnableMetrics, config.EnableLegacyAPI)

	// Initialize storage backend
	if err := storage.InitFileBackend(config.DataDir); err != nil {
		return fmt.Errorf("failed to initialize storage: %v", err)
	}

	// Setup graceful shutdown context early so it can be used for background workers
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize HSM client if configured
	// When HSM URL is provided, the service will use FlexibleBootScriptController
	// with HSM as the node provider for boot script generation
	var hsmClient *hsm.HSMClient
	var serviceTokenManager *hsm.ServiceTokenManager
	if config.HSMURL != "" {
		var err error
		hsmConfig := hsm.DefaultHSMConfig()
		hsmConfig.BaseURL = config.HSMURL

		hsmLogger := log.New(os.Stdout, "smd: ", log.LstdFlags)

		serviceTokenManager, err = initializeHSMServiceTokenManager(ctx, config, hsmLogger)
		if err != nil {
			return err
		}
		if serviceTokenManager != nil {
			hsmConfig.ServiceTokenManager = serviceTokenManager
		}

		hsmClient, err = hsm.NewHSMClient(hsmConfig, hsmLogger)
		if err != nil {
			return fmt.Errorf("failed to initialize HSM client: %w", err)
		}

		// Test HSM connectivity
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := hsmClient.Health(ctx); err != nil {
			log.Printf("Warning: HSM health check failed: %v", err)
			log.Printf("HSM integration will be available but may not be functional")
		} else {
			log.Printf("HSM integration enabled and healthy at: %s", config.HSMURL)
		}
	}

	if serviceTokenManager != nil {
		go serviceTokenManager.StartAutoRefresh(ctx)
	}

	// Setup router
	r := chi.NewRouter()

	// Add all middleware first, before any routes
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// Generated chi routes are registered with trailing slashes, while existing
	// clients (including our legacy compatibility handler's internal client)
	// use slashless resource paths. RedirectSlashes preserves that compatibility
	// without hand-editing generated route registrations.
	r.Use(middleware.RedirectSlashes)
	r.Use(middleware.Timeout(time.Duration(config.ReadTimeout) * time.Second))

	// Register health check
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) { //nolint:revive
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"boot-service"}`)) //nolint:errcheck
	})

	// Setup metrics endpoint if enabled (before other routes)
	if config.EnableMetrics {
		// Add metrics to main router
		r.Route("/metrics", func(r chi.Router) {
			r.Get("/", metricsHandler)
		})

		// Start separate metrics server
		go startMetricsServer(config)
	}

	if err := registerCustomServerIntegrations(r, config, hsmClient, ctx); err != nil {
		return err
	}

	// Configure server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
	}

	// Setup graceful shutdown handler

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		cancel()
	}()

	// Start server
	log.Printf("Server starting on %s", server.Addr)
	log.Println("Modern API available at: /nodes, /bootconfigurations")
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %v", err)
	}

	<-ctx.Done()
	log.Println("Server stopped")
	return nil
}

func validateConfig(config Config) error {
	if config.Port <= 0 || config.Port > 65535 {
		return fmt.Errorf("invalid port: %d", config.Port)
	}
	if config.EnableAuth && config.TokenSmithURL == "" {
		return fmt.Errorf("tokensmith-url is required when auth is enabled")
	}
	if config.TokenSmithRefreshSkewSec < 0 {
		return fmt.Errorf("tokensmith-refresh-skew-sec must be >= 0")
	}
	// Note: HSM is auto-enabled when hsm-url is provided, no explicit validation needed
	return nil
}

func tokenSmithScopeHintCSV(config Config) string {
	if strings.TrimSpace(config.TokenSmithBootstrapPolicyScopesHint) != "" {
		return config.TokenSmithBootstrapPolicyScopesHint
	}

	// Backward compatibility for legacy key/env/flag names.
	return config.TokenSmithScopesLegacy
}

func parseScopeHintCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	scopes := make([]string, 0, len(parts))
	for _, part := range parts {
		scope := strings.TrimSpace(part)
		if scope != "" {
			scopes = append(scopes, scope)
		}
	}

	if len(scopes) == 0 {
		return nil
	}

	return scopes
}

func initializeHSMServiceTokenManager(ctx context.Context, config Config, hsmLogger *log.Logger) (*hsm.ServiceTokenManager, error) {
	if strings.TrimSpace(config.TokenSmithURL) == "" {
		return nil, nil
	}

	if !config.EnableAuth {
		log.Printf("INFO: tokensmith URL ignored, auth disabled")
		return nil, nil
	}

	bootstrapToken := strings.TrimSpace(config.TokenSmithBootstrapToken)
	bootstrapSource := "config"
	if bootstrapToken == "" {
		bootstrapToken = strings.TrimSpace(os.Getenv("TOKENSMITH_BOOTSTRAP_TOKEN"))
		bootstrapSource = "env:TOKENSMITH_BOOTSTRAP_TOKEN"
	}
	if bootstrapToken == "" {
		return nil, fmt.Errorf("tokensmith bootstrap token is required when both hsm-url and tokensmith_url are set")
	}

	tokenConfig := hsm.DefaultTokenExchangeConfig()
	tokenConfig.TokenSmithURL = config.TokenSmithURL
	tokenConfig.BootstrapToken = bootstrapToken
	tokenConfig.TargetService = strings.TrimSpace(config.TokenSmithTargetService)
	tokenConfig.Scopes = parseScopeHintCSV(tokenSmithScopeHintCSV(config))
	tokenConfig.RefreshBefore = time.Duration(config.TokenSmithRefreshSkewSec) * time.Second

	tokenEndpoint := strings.TrimRight(tokenConfig.TokenSmithURL, "/") + "/oauth/token"
	log.Printf("HSM token exchange config: endpoint=%s target=%s scope_hint=%v bootstrap_token_present=%v bootstrap_token_source=%s",
		tokenEndpoint,
		tokenConfig.TargetService,
		tokenConfig.Scopes,
		bootstrapToken != "",
		bootstrapSource,
	)

	serviceTokenManager := hsm.NewServiceTokenManager(tokenConfig, hsmLogger)
	initialTokenCtx, initialTokenCancel := context.WithTimeout(ctx, 10*time.Second)
	defer initialTokenCancel()
	if err := serviceTokenManager.Initialize(initialTokenCtx); err != nil {
		return nil, fmt.Errorf("failed to initialize HSM service token manager: %w", err)
	}

	log.Printf("HSM auth enabled via TokenSmith service-token exchange (target=%s)", tokenConfig.TargetService)
	return serviceTokenManager, nil
}

func startMetricsServer(config Config) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	metricsAddr := fmt.Sprintf("%s:%d", config.Host, config.MetricsPort)
	log.Printf("Metrics server starting on %s", metricsAddr)

	if err := http.ListenAndServe(metricsAddr, mux); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) { //nolint:revive
	promhttp.Handler().ServeHTTP(w, r)
}
