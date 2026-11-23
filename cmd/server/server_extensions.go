// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
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

	"github.com/openchami/boot-service/pkg/clients/hsm"
	"github.com/openchami/boot-service/pkg/controllers/bootscript"
	"github.com/openchami/boot-service/pkg/declarativeboot"
	"github.com/openchami/boot-service/pkg/handlers/boot"
)

// bootRoutes holds the boot handler so its routes can join the router's public
// and protected groups separately.
type bootRoutes struct {
	handler   *boot.Handler
	legacyAPI bool
	manager   *declarativeboot.Manager
}

// newBootRoutes builds the boot handler outside runServe's core startup flow and
// starts HSM background sync when configured.
func newBootRoutes(config Config, hsmClient *hsm.HSMClient, ctx context.Context, managers ...*declarativeboot.Manager) (*bootRoutes, error) {
	bootClient, err := newInternalAPIClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create boot script API client: %w", err)
	}

	logger := log.New(os.Stdout, "boot: ", log.LstdFlags)

	var controller bootscript.ScriptGenerator
	var nodes bootscript.DeclarativeNodeResolver
	buildInfo := boot.WithBuildInfo(boot.BuildInfo{
		Version: serviceVersion,
		Commit:  serviceCommit,
		Date:    serviceDate,
	})

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
			return nil, fmt.Errorf("failed to create flexible controller with HSM: %w", err)
		}

		// Start background sync worker if enabled.
		if config.HSMSyncEnabled {
			go flexController.StartBackgroundSync(ctx)
			log.Printf("HSM background sync enabled (interval: %d minutes)", config.HSMSyncInterval)
		}

		controller = flexController
		nodes = hsm.NewDeclarativeNodeProvider(hsmClient)
	} else {
		// Use standard controller with local storage.
		controller = bootscript.NewBootScriptController(*bootClient, logger)
		nodes = bootscript.NewDeclarativeLocalNodeProvider(*bootClient)
	}
	if len(managers) != 0 && managers[0] != nil {
		declarative := bootscript.NewDeclarativeController(bootscript.DeclarativeOptions{Legacy: controller, Nodes: nodes, Revisions: declarativeboot.NewRevisionService(managers[0])})
		controller = declarative
	}
	bootHandler := boot.NewHandlerWithController(*bootClient, controller, logger, buildInfo)

	// Only the legacy BSS-compatible API at /boot/v1/* depends on enable_legacy_api.
	if config.EnableLegacyAPI {
		if hsmClient != nil {
			log.Println("Legacy BSS API enabled with HSM integration at: /boot/v1/*")
		} else {
			log.Println("Legacy BSS API enabled at: /boot/v1/*")
		}
		log.Println("Note: Both modern and legacy endpoints are available for BSS compatibility")
	} else {
		log.Println("Legacy BSS API disabled (set enable_legacy_api to true to enable /boot/v1/* endpoints)")
	}

	routes := &bootRoutes{handler: bootHandler, legacyAPI: config.EnableLegacyAPI}
	if len(managers) != 0 {
		routes.manager = managers[0]
	}
	return routes, nil
}

func (b *bootRoutes) registerPublic(r chi.Router) {
	r.Group(func(public chi.Router) {
		public.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				next.ServeHTTP(w, r.WithContext(bootscript.WithRequestProfile(r.Context(), r.URL.Query().Get("profile"))))
			})
		})
		b.handler.RegisterModernPublicRoutes(public)
		if b.legacyAPI {
			b.handler.RegisterLegacyPublicRoutes(public)
		}
	})
}

func (b *bootRoutes) registerProtected(r chi.Router) {
	b.handler.RegisterModernProtectedRoutes(r)
	if b.legacyAPI {
		b.handler.RegisterLegacyProtectedRoutes(r)
	}
}
