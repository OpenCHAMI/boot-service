package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/openchami/boot-service/internal/storage"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/handlers/legacy"
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
	TokenSmithURL string `mapstructure:"tokensmith_url"`
	JWKSEndpoint  string `mapstructure:"jwks_endpoint"`

	// Hardware State Manager Configuration (when enabled)
	HSMURL string `mapstructure:"hsm_url"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() Config {
	return Config{
		Port:            8080,
		Host:            "0.0.0.0",
		ReadTimeout:     30,
		WriteTimeout:    30,
		IdleTimeout:     120,
		DataDir:         "./data",
		StorageType:     "file",
		EnableAuth:      false,
		EnableMetrics:   false,
		EnableLegacyAPI: true,
		MetricsPort:     9090,
		TokenSmithURL:   "",
		JWKSEndpoint:    "",
		HSMURL:          "",
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
	serveCmd.Flags().Int("read_timeout", 30, "Read timeout in seconds")
	serveCmd.Flags().Int("write_timeout", 30, "Write timeout in seconds")
	serveCmd.Flags().Int("idle_timeout", 120, "Idle timeout in seconds")

	// Storage configuration flags
	serveCmd.Flags().String("data_dir", "./data", "Directory for file storage")
	serveCmd.Flags().String("storage_type", "file", "Storage backend: file or database")

	// Feature flags
	serveCmd.Flags().Bool("enable_auth", false, "Enable authentication with TokenSmith")
	serveCmd.Flags().Bool("enable_metrics", false, "Enable Prometheus metrics")
	serveCmd.Flags().Bool("enable_legacy_api", true, "Enable legacy BSS API compatibility")
	serveCmd.Flags().Int("metrics_port", 9090, "Port for metrics endpoint")

	// Authentication configuration flags
	serveCmd.Flags().String("tokensmith_url", "", "TokenSmith service URL for authentication")
	serveCmd.Flags().String("jwks_endpoint", "", "JWKS endpoint for JWT validation")

	// Hardware State Manager configuration flags
	serveCmd.Flags().String("hsm_url", "", "Hardware State Manager service URL (enables HSM when provided)")

	// Bind flags to viper
	viper.BindPFlags(serveCmd.Flags())

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
	viper.AutomaticEnv()

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

func runServe(cmd *cobra.Command, args []string) error {
	// Load configuration
	config := DefaultConfig()
	if err := viper.Unmarshal(&config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	// Validate configuration
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

	// Setup router
	r := chi.NewRouter()

	// Add all middleware first, before any routes
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(time.Duration(config.ReadTimeout) * time.Second))

	// Register health check
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"boot-service"}`))
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

	// Register generated routes (modern API) - middleware already applied above
	RegisterGeneratedRoutes(r)

	// Register legacy BSS API routes if enabled
	if config.EnableLegacyAPI {
		bootClient, err := client.NewClient(fmt.Sprintf("http://%s:%d", config.Host, config.Port),
			&http.Client{Timeout: 30 * time.Second})
		if err != nil {
			return fmt.Errorf("failed to create client for legacy API: %v", err)
		}

		logger := log.New(os.Stdout, "legacy: ", log.LstdFlags)
		legacyHandler := legacy.NewLegacyHandler(*bootClient, logger)
		legacyHandler.RegisterRoutes(r)
		log.Println("Legacy BSS API enabled at: /boot/v1/")
	}

	// Configure server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", config.Host, config.Port),
		Handler:      r,
		ReadTimeout:  time.Duration(config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(config.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(config.IdleTimeout) * time.Second,
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
	// Note: HSM is auto-enabled when hsm-url is provided, no explicit validation needed
	return nil
}

func startMetricsServer(config Config) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", metricsHandler)

	metricsAddr := fmt.Sprintf(":%d", config.MetricsPort)
	log.Printf("Metrics server starting on %s", metricsAddr)

	if err := http.ListenAndServe(metricsAddr, mux); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}

func metricsHandler(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement Prometheus metrics here
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("# Metrics endpoint - implementation pending\n"))
}
