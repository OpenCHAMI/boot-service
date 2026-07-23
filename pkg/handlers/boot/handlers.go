// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Package boot provides boot API handlers for both modern and legacy,
// BSS-compatible endpoints
package boot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	apiv1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/controllers/bootscript"
)

// BootController interface for boot script generation
type BootController interface {
	GenerateBootScript(ctx context.Context, identifier string, profile string) (string, error)
}

// Handler handles boot API requests for both modern and legacy endpoints
type Handler struct {
	client     client.Client
	controller BootController
	logger     *log.Logger
}

// NewHandler creates a new boot API handler with standard controller
func NewHandler(c client.Client, logger *log.Logger) *Handler {
	controller := bootscript.NewBootScriptController(c, logger)
	return &Handler{
		client:     c,
		controller: controller,
		logger:     logger,
	}
}

// NewHandlerWithController creates a new boot API handler with a custom controller
func NewHandlerWithController(c client.Client, controller BootController, logger *log.Logger) *Handler {
	return &Handler{
		client:     c,
		controller: controller,
		logger:     logger,
	}
}

// RegisterModernRoutes registers modern boot API routes at root paths
// These are always available regardless of enable_legacy_api setting
func (h *Handler) RegisterModernRoutes(r chi.Router) {
	// Boot parameters endpoints
	r.Route("/bootparameters", func(r chi.Router) {
		r.Get("/", h.GetBootParameters)
		r.Post("/", h.CreateBootParameters)
		r.Put("/", h.UpdateBootParameters)
		r.Delete("/", h.DeleteBootParameters)
	})

	// Boot script endpoint
	r.Get("/bootscript", h.GetBootScript)

	// Service endpoints
	r.Route("/service", func(r chi.Router) {
		r.Get("/status", h.GetServiceStatus)
		r.Get("/version", h.GetServiceVersion)
	})
}

// RegisterLegacyRoutes registers legacy BSS API routes at /boot/v1
// These are ONLY available when enable_legacy_api: true
func (h *Handler) RegisterLegacyRoutes(r chi.Router) {
	r.Route("/boot/v1", func(r chi.Router) {
		// Boot parameters endpoints
		r.Route("/bootparameters", func(r chi.Router) {
			r.Get("/", h.GetBootParameters)
			r.Post("/", h.CreateBootParameters)
			r.Put("/", h.UpdateBootParameters)
			r.Delete("/", h.DeleteBootParameters)
		})

		// Boot script endpoint
		r.Get("/bootscript", h.GetBootScript)

		// Service endpoints
		r.Route("/service", func(r chi.Router) {
			r.Get("/status", h.GetServiceStatus)
			r.Get("/version", h.GetServiceVersion)
		})
	})
}

// GetBootParameters handles GET /bootparameters and GET /boot/v1/bootparameters
func (h *Handler) GetBootParameters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for filtering
	host := r.URL.Query().Get("host")
	mac := r.URL.Query().Get("mac")
	nid := r.URL.Query().Get("nid")
	name := r.URL.Query().Get("name")

	// Get all boot configurations
	configs, err := h.client.GetBootConfigurations(ctx)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to retrieve boot parameters", err.Error())
		return
	}

	// Filter configurations based on query parameters
	var filteredConfigs []apiv1.BootConfiguration
	if host != "" || mac != "" || nid != "" || name != "" {
		identifiers := ParseNodeIdentifiersFromQuery(host, mac, nid, name)
		filteredConfigs = h.filterConfigurationsByIdentifiers(configs, identifiers)
	} else {
		filteredConfigs = configs
	}

	// Convert to legacy format
	var legacyParams []BootParameters
	for _, config := range filteredConfigs {
		legacyParam := ConvertBootConfigurationToLegacy(&config)
		legacyParams = append(legacyParams, legacyParam)
	}

	response := BootParametersResponse{
		BootParameters: legacyParams,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// CreateBootParameters handles POST /bootparameters and POST /boot/v1/bootparameters
func (h *Handler) CreateBootParameters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req BootParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request format", err.Error())
		return
	}

	// Generate a name for the configuration
	name := h.generateConfigName(req)

	// Convert to modern BootConfiguration
	config := ConvertLegacyRequestToBootConfiguration(req)
	config.Metadata.Name = name

	// Create the configuration
	createReq := client.CreateBootConfigurationRequest{
		Spec: config.Spec,
	}
	createReq.Metadata.Name = name

	createdConfig, err := h.client.CreateBootConfiguration(ctx, createReq)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to create boot parameters", err.Error())
		return
	}

	// Convert back to legacy format and return
	legacyParam := ConvertBootConfigurationToLegacy(createdConfig)
	response := BootParametersResponse{
		BootParameters: []BootParameters{legacyParam},
	}

	h.writeJSON(w, http.StatusCreated, response)
}

// UpdateBootParameters handles PUT /bootparameters and PUT /boot/v1/bootparameters
func (h *Handler) UpdateBootParameters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req BootParametersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request format", err.Error())
		return
	}

	// For update, we need to find existing configurations that match the identifiers
	// This is a simplified implementation - in a real scenario, you might want more sophisticated matching
	configs, err := h.client.GetBootConfigurations(ctx)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to retrieve existing configurations", err.Error())
		return
	}

	// Find configurations that match any of the provided identifiers
	identifiers := append(req.Hosts, req.Macs...)
	identifiers = append(identifiers, req.Nids...)

	matchingConfigs := h.filterConfigurationsByIdentifiers(configs, identifiers)

	if len(matchingConfigs) == 0 {
		h.writeError(w, http.StatusNotFound, "No matching boot parameters found", "")
		return
	}

	// Update the first matching configuration (simplified approach)
	configToUpdate := matchingConfigs[0]
	updateReq := client.UpdateBootConfigurationRequest{
		Spec: apiv1.BootConfigurationSpec{
			Hosts:    req.Hosts,
			MACs:     req.Macs,
			Groups:   configToUpdate.Spec.Groups, // Preserve existing groups
			Kernel:   req.Kernel,
			Initrd:   req.Initrd,
			Params:   req.Params,
			Priority: configToUpdate.Spec.Priority, // Preserve existing priority
		},
	}

	// Convert string NIDs to int32
	for _, nidStr := range req.Nids {
		if nid, err := strconv.Atoi(nidStr); err == nil {
			updateReq.Spec.NIDs = append(updateReq.Spec.NIDs, int32(nid))
		}
	}

	updatedConfig, err := h.client.UpdateBootConfiguration(ctx, configToUpdate.Metadata.UID, updateReq)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to update boot parameters", err.Error())
		return
	}

	// Convert back to legacy format and return
	legacyParam := ConvertBootConfigurationToLegacy(updatedConfig)
	response := BootParametersResponse{
		BootParameters: []BootParameters{legacyParam},
	}

	h.writeJSON(w, http.StatusOK, response)
}

// DeleteBootParameters handles DELETE /bootparameters and DELETE /boot/v1/bootparameters
func (h *Handler) DeleteBootParameters(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters to identify which configurations to delete
	host := r.URL.Query().Get("host")
	mac := r.URL.Query().Get("mac")
	nid := r.URL.Query().Get("nid")
	name := r.URL.Query().Get("name")

	if host == "" && mac == "" && nid == "" && name == "" {
		h.writeError(w, http.StatusBadRequest, "Missing identifier", "At least one identifier (host, mac, nid, or name) must be provided")
		return
	}

	// Get all configurations and filter by identifiers
	configs, err := h.client.GetBootConfigurations(ctx)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to retrieve configurations", err.Error())
		return
	}

	identifiers := ParseNodeIdentifiersFromQuery(host, mac, nid, name)
	matchingConfigs := h.filterConfigurationsByIdentifiers(configs, identifiers)

	if len(matchingConfigs) == 0 {
		h.writeError(w, http.StatusNotFound, "No matching boot parameters found", "")
		return
	}

	// Delete all matching configurations
	var deletedConfigs []BootParameters
	for _, config := range matchingConfigs {
		err := h.client.DeleteBootConfiguration(ctx, config.Metadata.UID)
		if err != nil {
			h.logger.Printf("Warning: Failed to delete configuration %s: %v", config.Metadata.UID, err)
			continue
		}
		legacyParam := ConvertBootConfigurationToLegacy(&config)
		deletedConfigs = append(deletedConfigs, legacyParam)
	}

	response := BootParametersResponse{
		BootParameters: deletedConfigs,
	}

	h.writeJSON(w, http.StatusOK, response)
}

// GetBootScript handles GET /bootscript and GET /boot/v1/bootscript
func (h *Handler) GetBootScript(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters for node identification
	host := r.URL.Query().Get("host")
	mac := r.URL.Query().Get("mac")
	nid := r.URL.Query().Get("nid")

	// Create boot script request
	req := BootScriptRequest{
		Host:   host,
		Mac:    mac,
		Nid:    nid,
		Format: r.URL.Query().Get("format"), // defaults to "ipxe"
	}

	// Extract the node identifier
	identifier := ExtractNodeIdentifier(req)
	if identifier == "" {
		h.writeError(w, http.StatusBadRequest, "Missing node identifier", "At least one node identifier (host, mac, or nid) must be provided")
		return
	}

	// Generate the boot script using our boot logic
	// Ignore profile query parameter and always auto-resolve best configuration.
	// Profile selection is driven by matching score and priority within boot logic.
	script, err := h.controller.GenerateBootScript(ctx, identifier, "")
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, "Failed to generate boot script", err.Error())
		return
	}

	// Return the script as plain text (iPXE format)
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(script)) //nolint:errcheck
}

// GetServiceStatus handles GET /service/status and GET /boot/v1/service/status
func (h *Handler) GetServiceStatus(w http.ResponseWriter, r *http.Request) { //nolint:revive
	status := CreateServiceStatus("2.0.0-fabrica")
	h.writeJSON(w, http.StatusOK, status)
}

// GetServiceVersion handles GET /service/version and GET /boot/v1/service/version
func (h *Handler) GetServiceVersion(w http.ResponseWriter, r *http.Request) { //nolint:revive
	version := CreateServiceVersion("2.0.0-fabrica", "2025-10-08", "main")
	h.writeJSON(w, http.StatusOK, version)
}

// Helper methods

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Printf("Error encoding JSON response: %v", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, title, detail string) {
	errorResp := CreateErrorResponse(status, title, detail)
	h.writeJSON(w, status, errorResp)
}

func (h *Handler) generateConfigName(req BootParametersRequest) string {
	// Generate a name based on the first identifier
	if len(req.Hosts) > 0 {
		return fmt.Sprintf("legacy-%s", strings.ReplaceAll(req.Hosts[0], ".", "-"))
	}
	if len(req.Macs) > 0 {
		return fmt.Sprintf("legacy-%s", strings.ReplaceAll(req.Macs[0], ":", "-"))
	}
	if len(req.Nids) > 0 {
		return fmt.Sprintf("legacy-nid-%s", req.Nids[0])
	}
	return fmt.Sprintf("legacy-config-%d", len(req.Hosts)+len(req.Macs)+len(req.Nids))
}

func (h *Handler) filterConfigurationsByIdentifiers(configs []apiv1.BootConfiguration, identifiers []string) []apiv1.BootConfiguration {
	var matching []apiv1.BootConfiguration

	for _, config := range configs {
		if h.configMatchesIdentifiers(config, identifiers) {
			matching = append(matching, config)
		}
	}

	return matching
}

func (h *Handler) configMatchesIdentifiers(config apiv1.BootConfiguration, identifiers []string) bool {
	for _, identifier := range identifiers {
		// Check hosts
		for _, host := range config.Spec.Hosts {
			if host == identifier {
				return true
			}
		}

		// Check MACs
		for _, mac := range config.Spec.MACs {
			if mac == identifier {
				return true
			}
		}

		// Check NIDs
		if nid, err := strconv.Atoi(identifier); err == nil {
			for _, configNid := range config.Spec.NIDs {
				if int32(nid) == configNid {
					return true
				}
			}
		}

		// Check groups
		for _, group := range config.Spec.Groups {
			if group == identifier {
				return true
			}
		}
	}

	return false
}
