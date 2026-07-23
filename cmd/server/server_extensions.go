// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/controllers/bootscript"
	"github.com/openchami/boot-service/pkg/handlers/boot"
)

// registerCustomServerIntegrations keeps generated route wiring and legacy compatibility
// route setup together outside runServe's core startup flow.
func registerCustomServerIntegrations(r chi.Router, config Config, hsmClient *hsm.HSMClient, ctx context.Context) error {
	// Register UID prefixes used by generated handlers when creating resources.
	if err := registerResourcePrefixes(); err != nil {
		return fmt.Errorf("failed to register resource prefixes: %w", err)
	}

	// Register generated routes (modern API) - middleware already applied above.
	RegisterGeneratedRoutes(r)

	bootClient, err := client.NewClient(fmt.Sprintf("http://%s:%d", config.Host, config.Port),
		&http.Client{Timeout: 30 * time.Second}, client.DefaultLogger())
	if err != nil {
		return fmt.Errorf("failed to create boot script API client: %v", err)
	}

	logger := log.New(os.Stdout, "boot: ", log.LstdFlags)

	var bootHandler *boot.Handler

	if hsmClient != nil {
		// Use FlexibleBootScriptController with HSM provider.
		hsmIntegrationConfig := hsm.DefaultIntegrationConfig()
		hsmIntegrationConfig.HSMConfig.BaseURL = config.HSMURL
		hsmIntegrationConfig.HSMConfig.Timeout = 30 * time.Second
		hsmIntegrationConfig.SyncEnabled = config.HSMSyncEnabled
		hsmIntegrationConfig.SyncInterval = time.Duration(config.HSMSyncInterval) * time.Minute

		providerConfig := bootscript.ProviderConfig{
			Type:      "hsm",
			HSMConfig: &hsmIntegrationConfig,
			HSMClient: hsmClient,
		}

		controllerLogger := log.New(os.Stdout, "bootscript: ", log.LstdFlags)
		flexController, err := bootscript.NewFlexibleBootScriptController(*bootClient, providerConfig, controllerLogger)
		if err != nil {
			return fmt.Errorf("failed to create flexible controller with HSM: %v", err)
		}

		// Start background sync worker if enabled.
		if config.HSMSyncEnabled {
			go flexController.StartBackgroundSync(ctx)
			log.Printf("HSM background sync enabled (interval: %d minutes)", config.HSMSyncInterval)
		}

		bootHandler = boot.NewHandlerWithController(*bootClient, flexController, logger)
	} else {
		// Use standard controller with local storage.
		bootHandler = boot.NewHandler(*bootClient, logger)
	}

	// Always register "modern" boot API paths at /.
	bootHandler.RegisterModernRoutes(r)

	// Only register legacy BSS-compatible API if enable_legacy_api is true.
	// These live at /boot/v1/*.
	if config.EnableLegacyAPI {
		bootHandler.RegisterLegacyRoutes(r)
		if hsmClient != nil {
			log.Println("Legacy BSS API enabled with HSM integration at: /boot/v1/*")
		} else {
			log.Println("Legacy BSS API enabled at: /boot/v1/*")
		}
		log.Println("Note: Both modern and legacy endpoints are available for BSS compatibility")
	} else {
		log.Println("Legacy BSS API disabled (set enable_legacy_api to true to enable /boot/v1/* endpoints)")
	}

	return nil
}
