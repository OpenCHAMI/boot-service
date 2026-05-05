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
	"github.com/openchami/boot-service/pkg/handlers/legacy"
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
		&http.Client{Timeout: 30 * time.Second})
	if err != nil {
		return fmt.Errorf("failed to create boot script API client: %v", err)
	}

	logger := log.New(os.Stdout, "legacy: ", log.LstdFlags)

	var legacyHandler *legacy.LegacyHandler

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

		legacyHandler = legacy.NewLegacyHandlerWithController(*bootClient, flexController, logger)
	} else {
		// Use standard controller with local storage.
		legacyHandler = legacy.NewLegacyHandler(*bootClient, logger)
	}

	// Register node bootscript endpoint always; keep additional legacy BSS
	// compatibility endpoints behind enable_legacy_api.
	if config.EnableLegacyAPI {
		legacyHandler.RegisterRoutes(r)
		if hsmClient != nil {
			log.Println("Legacy BSS API enabled with HSM integration at: /boot/v1/")
		} else {
			log.Println("Legacy BSS API enabled at: /boot/v1/")
		}
	} else {
		legacyHandler.RegisterBootScriptRoute(r)
		log.Println("Boot script endpoint enabled at: /boot/v1/bootscript")
	}

	return nil
}
