// SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/openchami/boot-service/internal/storage"
	bootclient "github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/handlers/boot"
)

func TestBindFlagsWithUnderscoreKeys_ConfigValuesBeatUnchangedFlagDefaults(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("enable-auth", false, "Enable authentication with TokenSmith")
	flags.Bool("enable-metrics", false, "Enable Prometheus metrics")
	flags.Bool("enable-legacy-api", true, "Enable legacy BSS API compatibility")
	flags.String("tokensmith-url", "", "TokenSmith service URL for authentication")
	flags.String("tokensmith-target-service", "hsm", "Target service audience for HSM service token exchange")
	flags.String("hsm-url", "", "Hardware State Manager service URL")
	flags.Bool("hsm-sync-enabled", true, "Enable background sync with HSM")

	v := viper.New()
	if err := bindFlagsWithUnderscoreKeys(v, flags); err != nil {
		t.Fatalf("bindFlagsWithUnderscoreKeys failed: %v", err)
	}

	v.SetConfigType("yaml")
	configYAML := `
enable_auth: true
tokensmith_url: http://tokensmith:8080
tokensmith_target_service: smd
hsm_url: http://smd:27779
hsm_sync_enabled: true
enable_legacy_api: true
enable_metrics: false
`
	if err := v.ReadConfig(strings.NewReader(configYAML)); err != nil {
		t.Fatalf("ReadConfig failed: %v", err)
	}

	config := DefaultConfig()
	if err := v.Unmarshal(&config); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !config.EnableAuth {
		t.Fatal("expected enable_auth config value to override unchanged --enable-auth default")
	}
	if config.TokenSmithURL != "http://tokensmith:8080" {
		t.Fatalf("expected tokensmith_url from config, got %q", config.TokenSmithURL)
	}
	if config.TokenSmithTargetService != "smd" {
		t.Fatalf("expected tokensmith_target_service from config, got %q", config.TokenSmithTargetService)
	}
	if config.HSMURL != "http://smd:27779" {
		t.Fatalf("expected hsm_url config value to override unchanged --hsm-url default, got %q", config.HSMURL)
	}
	if !config.HSMSyncEnabled {
		t.Fatal("expected hsm_sync_enabled from config")
	}
	if !config.EnableLegacyAPI {
		t.Fatal("expected enable_legacy_api from config")
	}
	if config.EnableMetrics {
		t.Fatal("expected enable_metrics to remain false")
	}
}

func TestBindFlagsWithUnderscoreKeys_ChangedHyphenatedFlagsUseUnderscoreKeys(t *testing.T) {
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.Bool("enable-auth", false, "Enable authentication with TokenSmith")
	flags.String("hsm-url", "", "Hardware State Manager service URL")

	if err := flags.Set("enable-auth", "true"); err != nil {
		t.Fatalf("Set enable-auth failed: %v", err)
	}
	if err := flags.Set("hsm-url", "http://smd:27779"); err != nil {
		t.Fatalf("Set hsm-url failed: %v", err)
	}

	v := viper.New()
	if err := bindFlagsWithUnderscoreKeys(v, flags); err != nil {
		t.Fatalf("bindFlagsWithUnderscoreKeys failed: %v", err)
	}

	if !v.GetBool("enable_auth") {
		t.Fatal("expected --enable-auth to bind to enable_auth")
	}
	if got := v.GetString("hsm_url"); got != "http://smd:27779" {
		t.Fatalf("expected --hsm-url to bind to hsm_url, got %q", got)
	}
}

func newGeneratedRouterForTest(t *testing.T) http.Handler {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "data")
	if err := storage.InitFileBackend(dataDir); err != nil {
		t.Fatalf("failed to initialize file backend: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RedirectSlashes)
	RegisterGeneratedRoutes(r)

	return r
}

func TestGeneratedRoutesSupportSlashlessCollectionPaths(t *testing.T) {
	server := httptest.NewServer(newGeneratedRouterForTest(t))
	defer server.Close()

	tests := []struct {
		name string
		path string
	}{
		{name: "BMCs", path: "/bmcs"},
		{name: "BootConfigurations", path: "/bootconfigurations"},
		{name: "Nodes", path: "/nodes"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatalf("GET %s failed: %v", tc.path, err)
			}
			defer resp.Body.Close() //nolint:errcheck

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("GET %s returned status %d, want %d", tc.path, resp.StatusCode, http.StatusOK)
			}
		})
	}
}

func TestGeneratedClientWorksAgainstSlashlessCollectionPaths(t *testing.T) {
	server := httptest.NewServer(newGeneratedRouterForTest(t))
	defer server.Close()

	client, err := bootclient.NewClient(server.URL, server.Client(), bootclient.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	ctx := context.Background()

	if _, err := client.GetBMCs(ctx); err != nil {
		t.Fatalf("GetBMCs failed against slashless path: %v", err)
	}

	if _, err := client.GetBootConfigurations(ctx); err != nil {
		t.Fatalf("GetBootConfigurations failed against slashless path: %v", err)
	}

	if _, err := client.GetNodes(ctx); err != nil {
		t.Fatalf("GetNodes failed against slashless path: %v", err)
	}
}

func TestBootScriptEndpointAvailabilityByLegacyFlag(t *testing.T) {
	tests := []struct {
		name                     string
		enableLegacyAPI          bool
		expectedModernBootScript int
		expectedLegacyBootScript int
		expectedModernBootParams int
		expectedLegacyBootParams int
		expectedModernService    int
		expectedLegacyService    int
	}{
		{
			name:                     "LegacyDisabled_OnlyModernRoutes",
			enableLegacyAPI:          false,
			expectedModernBootScript: http.StatusOK,
			expectedLegacyBootScript: http.StatusNotFound,
			expectedModernBootParams: http.StatusOK,
			expectedLegacyBootParams: http.StatusNotFound,
			expectedModernService:    http.StatusOK,
			expectedLegacyService:    http.StatusNotFound,
		},
		{
			name:                     "LegacyEnabled_BothModernAndLegacyRoutes",
			enableLegacyAPI:          true,
			expectedModernBootScript: http.StatusOK,
			expectedLegacyBootScript: http.StatusOK,
			expectedModernBootParams: http.StatusOK,
			expectedLegacyBootParams: http.StatusOK,
			expectedModernService:    http.StatusOK,
			expectedLegacyService:    http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := newRouterWithLegacyModeForTest(t, tc.enableLegacyAPI)
			server := httptest.NewServer(router)
			defer server.Close()

			// Test modern bootscript endpoint
			modernBootScriptResp, err := http.Get(server.URL + "/bootscript?mac=aa:bb:cc:dd:ee:ff")
			if err != nil {
				t.Fatalf("GET modern bootscript failed: %v", err)
			}
			defer modernBootScriptResp.Body.Close() //nolint:errcheck

			if modernBootScriptResp.StatusCode != tc.expectedModernBootScript {
				t.Errorf("GET /bootscript returned %d, want %d", modernBootScriptResp.StatusCode, tc.expectedModernBootScript)
			}

			// Test legacy bootscript endpoint
			legacyBootScriptResp, err := http.Get(server.URL + "/boot/v1/bootscript?mac=aa:bb:cc:dd:ee:ff")
			if err != nil {
				t.Fatalf("GET legacy bootscript failed: %v", err)
			}
			defer legacyBootScriptResp.Body.Close() //nolint:errcheck

			if legacyBootScriptResp.StatusCode != tc.expectedLegacyBootScript {
				t.Errorf("GET /boot/v1/bootscript returned %d, want %d", legacyBootScriptResp.StatusCode, tc.expectedLegacyBootScript)
			}

			// Test modern bootparameters endpoint
			modernBootParamsResp, err := http.Get(server.URL + "/bootparameters")
			if err != nil {
				t.Fatalf("GET modern bootparameters failed: %v", err)
			}
			defer modernBootParamsResp.Body.Close() //nolint:errcheck

			if modernBootParamsResp.StatusCode != tc.expectedModernBootParams {
				t.Errorf("GET /bootparameters returned %d, want %d", modernBootParamsResp.StatusCode, tc.expectedModernBootParams)
			}

			// Test legacy bootparameters endpoint
			legacyBootParamsResp, err := http.Get(server.URL + "/boot/v1/bootparameters")
			if err != nil {
				t.Fatalf("GET legacy bootparameters failed: %v", err)
			}
			defer legacyBootParamsResp.Body.Close() //nolint:errcheck

			if legacyBootParamsResp.StatusCode != tc.expectedLegacyBootParams {
				t.Errorf("GET /boot/v1/bootparameters returned %d, want %d", legacyBootParamsResp.StatusCode, tc.expectedLegacyBootParams)
			}

			// Test modern service status endpoint
			modernServiceResp, err := http.Get(server.URL + "/service/status")
			if err != nil {
				t.Fatalf("GET modern service status failed: %v", err)
			}
			defer modernServiceResp.Body.Close() //nolint:errcheck

			if modernServiceResp.StatusCode != tc.expectedModernService {
				t.Errorf("GET /service/status returned %d, want %d", modernServiceResp.StatusCode, tc.expectedModernService)
			}

			// Test legacy service status endpoint
			legacyServiceResp, err := http.Get(server.URL + "/boot/v1/service/status")
			if err != nil {
				t.Fatalf("GET legacy service status failed: %v", err)
			}
			defer legacyServiceResp.Body.Close() //nolint:errcheck

			if legacyServiceResp.StatusCode != tc.expectedLegacyService {
				t.Errorf("GET /boot/v1/service/status returned %d, want %d", legacyServiceResp.StatusCode, tc.expectedLegacyService)
			}
		})
	}
}

func newRouterWithLegacyModeForTest(t *testing.T, enableLegacyAPI bool) http.Handler {
	t.Helper()

	dataDir := filepath.Join(t.TempDir(), "data")
	if err := storage.InitFileBackend(dataDir); err != nil {
		t.Fatalf("failed to initialize file backend: %v", err)
	}

	backendServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nodes":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"apiVersion":"boot.openchami.io/v1","kind":"Node","metadata":{},"spec":{"xname":"x0c0s0b0n0","nid":42,"bootMAC":"aa:bb:cc:dd:ee:ff","hostname":"node-42"}}]`))
		case "/bootconfigurations":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"apiVersion":"boot.openchami.io/v1","kind":"BootConfiguration","metadata":{"name":"default-config"},"spec":{"kernel":"http://files.example.com/vmlinuz-default","params":"console=ttyS0,115200"}}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(backendServer.Close)

	bootClient, err := bootclient.NewClient(backendServer.URL, backendServer.Client(), bootclient.DefaultLogger())
	if err != nil {
		t.Fatalf("failed to create boot client: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.RedirectSlashes)
	RegisterGeneratedRoutes(r)

	bootHandler := boot.NewHandler(*bootClient, log.New(io.Discard, "", 0))

	// Always register modern routes
	bootHandler.RegisterModernRoutes(r)

	// Conditionally register legacy routes
	if enableLegacyAPI {
		bootHandler.RegisterLegacyRoutes(r)
	}

	return r
}

func TestInitializeHSMServiceTokenManager_IgnoresTokenSmithWhenAuthDisabled(t *testing.T) {
	t.Setenv("TOKENSMITH_BOOTSTRAP_TOKEN", "")

	var buf bytes.Buffer
	originalWriter := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(originalWriter)

	manager, err := initializeHSMServiceTokenManager(context.Background(), Config{
		EnableAuth:    false,
		TokenSmithURL: "http://tokensmith.example",
		HSMURL:        "http://hsm.example",
	}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatalf("expected no error when auth is disabled, got %v", err)
	}
	if manager != nil {
		t.Fatal("expected no token manager when auth is disabled")
	}
	if !strings.Contains(buf.String(), "INFO: tokensmith URL ignored, auth disabled") {
		t.Fatalf("expected auth-disabled info log, got %q", buf.String())
	}
}

func TestInitializeHSMServiceTokenManager_RequiresBootstrapTokenWhenAuthEnabled(t *testing.T) {
	t.Setenv("TOKENSMITH_BOOTSTRAP_TOKEN", "")

	manager, err := initializeHSMServiceTokenManager(context.Background(), Config{
		EnableAuth:    true,
		TokenSmithURL: "http://tokensmith.example",
		HSMURL:        "http://hsm.example",
	}, log.New(io.Discard, "", 0))
	if err == nil {
		t.Fatal("expected error when auth is enabled and bootstrap token is missing")
	}
	if manager != nil {
		t.Fatal("expected no token manager on error")
	}
	if !strings.Contains(err.Error(), "tokensmith bootstrap token is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
