// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// HSM integration service for boot script service
package hsm

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/resources/node"
)

// IntegrationService provides HSM integration for the boot service
type IntegrationService struct {
	hsmClient    *HSMClient
	bootClient   client.Client
	logger       *log.Logger
	syncEnabled  bool
	syncInterval time.Duration
}

// IntegrationConfig holds configuration for HSM integration
type IntegrationConfig struct {
	HSMConfig    HSMConfig     `json:"hsm"`
	SyncEnabled  bool          `json:"syncEnabled"`
	SyncInterval time.Duration `json:"syncInterval"`
}

// DefaultIntegrationConfig returns default integration configuration
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		HSMConfig:    DefaultHSMConfig(),
		SyncEnabled:  true,
		SyncInterval: 5 * time.Minute,
	}
}

// NewIntegrationService creates a new HSM integration service
func NewIntegrationService(config IntegrationConfig, bootClient client.Client, logger *log.Logger) *IntegrationService {
	if logger == nil {
		logger = log.New(log.Writer(), "hsm-integration: ", log.LstdFlags)
	}

	hsmClient := NewHSMClient(config.HSMConfig, logger)

	return &IntegrationService{
		hsmClient:    hsmClient,
		bootClient:   bootClient,
		logger:       logger,
		syncEnabled:  config.SyncEnabled,
		syncInterval: config.SyncInterval,
	}
}

// SyncNodesFromHSM synchronizes node data from HSM to the boot service
func (s *IntegrationService) SyncNodesFromHSM(ctx context.Context) error {
	s.logger.Printf("Starting HSM node synchronization")

	// Get components from HSM
	components, err := s.hsmClient.GetComponents(ctx)
	if err != nil {
		return fmt.Errorf("failed to get components from HSM: %w", err)
	}

	// Filter for compute nodes
	var computeNodes []HSMComponent
	for _, comp := range components {
		if comp.Type == "Node" && (comp.Role == "Compute" || comp.Role == "Application") {
			computeNodes = append(computeNodes, comp)
		}
	}

	s.logger.Printf("Found %d compute nodes in HSM", len(computeNodes))

	// Get ethernet interfaces for MAC address mapping
	interfaces, err := s.hsmClient.GetEthernetInterfaces(ctx)
	if err != nil {
		return fmt.Errorf("failed to get ethernet interfaces from HSM: %w", err)
	}

	// Create MAC address lookup map
	macMap := make(map[string]string) // componentID -> MAC
	for _, iface := range interfaces {
		if iface.Type == "Node" {
			macMap[iface.ComponentID] = iface.MACAddress
		}
	}

	// Get existing nodes from boot service
	existingNodes, err := s.bootClient.GetNodes(ctx)
	if err != nil {
		return fmt.Errorf("failed to get existing nodes: %w", err)
	}

	// Create lookup map for existing nodes
	existingMap := make(map[string]*node.Node)
	for i := range existingNodes {
		existingMap[existingNodes[i].Spec.XName] = &existingNodes[i]
	}

	// Sync each compute node
	var created, updated, skipped int
	for _, comp := range computeNodes {
		err := s.syncNode(ctx, comp, macMap, existingMap)
		if err != nil {
			s.logger.Printf("Warning: Failed to sync node %s: %v", comp.ID, err)
			continue
		}

		// Track what we did
		if existing, exists := existingMap[comp.ID]; exists {
			if s.needsUpdate(comp, macMap, existing) {
				updated++
			} else {
				skipped++
			}
		} else {
			created++
		}
	}

	s.logger.Printf("HSM sync complete: %d created, %d updated, %d skipped", created, updated, skipped)
	return nil
}

// syncNode synchronizes a single node from HSM
func (s *IntegrationService) syncNode(ctx context.Context, comp HSMComponent, macMap map[string]string, existingMap map[string]*node.Node) error {
	// Check if node already exists
	existing, exists := existingMap[comp.ID]

	// Get MAC address for this component
	bootMAC := macMap[comp.ID]

	// Create node spec from HSM data
	nodeSpec := node.NodeSpec{
		XName:   comp.ID,
		NID:     comp.NID,
		BootMAC: bootMAC,
		Role:    comp.Role,
		SubRole: comp.SubRole,
		Groups:  []string{}, // Will be populated from inventory service later
	}

	if exists {
		// Update existing node if needed
		if s.needsUpdate(comp, macMap, existing) {
			updateReq := client.UpdateNodeRequest{
				NodeSpec: nodeSpec,
			}

			_, err := s.bootClient.UpdateNode(ctx, existing.Metadata.UID, updateReq)
			if err != nil {
				return fmt.Errorf("failed to update node %s: %w", comp.ID, err)
			}

			s.logger.Printf("Updated node %s from HSM", comp.ID)
		}
	} else {
		// Create new node
		createReq := client.CreateNodeRequest{
			Name:     comp.ID,
			NodeSpec: nodeSpec,
		}

		_, err := s.bootClient.CreateNode(ctx, createReq)
		if err != nil {
			return fmt.Errorf("failed to create node %s: %w", comp.ID, err)
		}

		s.logger.Printf("Created node %s from HSM", comp.ID)
	}

	return nil
}

// needsUpdate checks if a node needs to be updated based on HSM data
func (s *IntegrationService) needsUpdate(comp HSMComponent, macMap map[string]string, existing *node.Node) bool {
	// Check if NID changed
	if comp.NID != existing.Spec.NID {
		return true
	}

	// Check if Role changed
	if comp.Role != existing.Spec.Role {
		return true
	}

	// Check if SubRole changed
	if comp.SubRole != existing.Spec.SubRole {
		return true
	}

	// Check if MAC address changed
	bootMAC := macMap[comp.ID]
	if bootMAC != existing.Spec.BootMAC {
		return true
	}

	return false
}

// ResolveNodeByIdentifier resolves a node using HSM as fallback
func (s *IntegrationService) ResolveNodeByIdentifier(ctx context.Context, identifier string) (*node.Node, error) {
	// First try to find in our local database
	nodes, err := s.bootClient.GetNodes(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get nodes from boot service: %w", err)
	}

	// Try to match by XName, NID, or MAC
	for i := range nodes {
		n := &nodes[i]
		if n.Spec.XName == identifier {
			return n, nil
		}
		if fmt.Sprintf("%d", n.Spec.NID) == identifier {
			return n, nil
		}
		if n.Spec.BootMAC == identifier {
			return n, nil
		}
	}

	// If not found locally, try HSM as fallback
	s.logger.Printf("Node %s not found locally, checking HSM", identifier)

	// Try to find by component ID
	comp, err := s.hsmClient.GetComponent(ctx, identifier)
	if err == nil {
		return s.convertHSMComponentToNode(ctx, comp)
	}

	// Try to find by MAC address
	comp, err = s.hsmClient.GetComponentByMAC(ctx, identifier)
	if err == nil {
		return s.convertHSMComponentToNode(ctx, comp)
	}

	return nil, fmt.Errorf("node %s not found in boot service or HSM", identifier)
}

// convertHSMComponentToNode converts an HSM component to a Node (for fallback scenarios)
func (s *IntegrationService) convertHSMComponentToNode(ctx context.Context, comp *HSMComponent) (*node.Node, error) {
	// Get MAC address
	interfaces, err := s.hsmClient.GetEthernetInterfaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
	}

	var bootMAC string
	for _, iface := range interfaces {
		if iface.ComponentID == comp.ID {
			bootMAC = iface.MACAddress
			break
		}
	}

	// Create a temporary node representation (not persisted)
	return &node.Node{
		Spec: node.NodeSpec{
			XName:   comp.ID,
			NID:     comp.NID,
			BootMAC: bootMAC,
			Role:    comp.Role,
			SubRole: comp.SubRole,
		},
	}, nil
}

// HealthCheck performs health checks on HSM connectivity
func (s *IntegrationService) HealthCheck(ctx context.Context) error {
	return s.hsmClient.Health(ctx)
}

// StartSyncWorker starts a background worker to periodically sync with HSM
func (s *IntegrationService) StartSyncWorker(ctx context.Context) {
	if !s.syncEnabled {
		s.logger.Printf("HSM sync disabled")
		return
	}

	s.logger.Printf("Starting HSM sync worker (interval: %v)", s.syncInterval)

	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	// Do initial sync
	if err := s.SyncNodesFromHSM(ctx); err != nil {
		s.logger.Printf("Initial HSM sync failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("HSM sync worker stopped")
			return

		case <-ticker.C:
			if err := s.SyncNodesFromHSM(ctx); err != nil {
				s.logger.Printf("HSM sync failed: %v", err)
			}
		}
	}
}

// GetHSMStats returns detailed HSM integration statistics
func (s *IntegrationService) GetHSMStats(ctx context.Context) (map[string]interface{}, error) {
	// Get HSM client stats directly
	hsmStats := s.hsmClient.GetStats(ctx)

	stats := map[string]interface{}{
		"hsm_integration_enabled": true,
		"hsm_client_stats":        hsmStats,
		"sync_enabled":            s.syncEnabled,
		"sync_interval":           s.syncInterval.String(),
	}

	return stats, nil
}

// GetStats returns HSM integration statistics (implements NodeProvider interface)
func (s *IntegrationService) GetStats(ctx context.Context) map[string]interface{} {
	// Get HSM client stats directly
	hsmStats := s.hsmClient.GetStats(ctx)

	stats := map[string]interface{}{
		"hsm_integration_enabled": true,
		"hsm_client_stats":        hsmStats,
		"sync_enabled":            s.syncEnabled,
		"sync_interval":           s.syncInterval.String(),
	}

	return stats
}
