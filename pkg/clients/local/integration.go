// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Local integration service for YAML-based node management
package local

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/openchami/boot-service/pkg/client"
	"github.com/openchami/boot-service/pkg/resources/node"
)

// IntegrationService provides local YAML-based node management
type IntegrationService struct {
	yamlProvider *YAMLNodeProvider
	bootClient   client.Client
	logger       *log.Logger
	config       IntegrationConfig
}

// IntegrationConfig configures the local YAML integration
type IntegrationConfig struct {
	YAMLFile     string        `yaml:"yaml_file"`
	AutoReload   bool          `yaml:"auto_reload"`
	SyncEnabled  bool          `yaml:"sync_enabled"`
	SyncInterval time.Duration `yaml:"sync_interval"`
}

// DefaultIntegrationConfig returns a default configuration
func DefaultIntegrationConfig() IntegrationConfig {
	return IntegrationConfig{
		YAMLFile:     "nodes.yaml",
		AutoReload:   true,
		SyncEnabled:  false, // Usually not needed for local files
		SyncInterval: 1 * time.Minute,
	}
}

// NewIntegrationService creates a new local integration service
func NewIntegrationService(config IntegrationConfig, bootClient client.Client, logger *log.Logger) (*IntegrationService, error) {
	// Create YAML provider
	yamlProvider, err := NewYAMLNodeProvider(config.YAMLFile, config.AutoReload, logger)
	if err != nil {
		return nil, fmt.Errorf("creating YAML provider: %w", err)
	}

	return &IntegrationService{
		yamlProvider: yamlProvider,
		bootClient:   bootClient,
		logger:       logger,
		config:       config,
	}, nil
}

// ResolveNodeByIdentifier resolves a node identifier using the YAML file
func (s *IntegrationService) ResolveNodeByIdentifier(ctx context.Context, identifier string) (*node.Node, error) {
	s.logger.Printf("Resolving node identifier %s using YAML file", identifier)

	// Get node from YAML
	yamlNode, err := s.yamlProvider.GetNodeByIdentifier(ctx, identifier)
	if err != nil {
		return nil, fmt.Errorf("node not found in YAML: %w", err)
	}

	// Create and return node resource
	nodeResource := &node.Node{
		Spec: node.NodeSpec{
			XName:   yamlNode.XName,
			Role:    yamlNode.Role,
			SubRole: yamlNode.SubRole,
			BootMAC: yamlNode.BootMAC,
		},
		Status: node.NodeStatus{
			State: yamlNode.State,
		},
	}

	// Add NID if present
	if yamlNode.NID > 0 {
		nodeResource.Spec.NID = int32(yamlNode.NID)
	}

	return nodeResource, nil
}

// SyncNodesFromYAML synchronizes all nodes from YAML to the boot service
func (s *IntegrationService) SyncNodesFromYAML(ctx context.Context) error {
	s.logger.Printf("Starting YAML to boot service sync")

	// Get all nodes from YAML
	yamlNodes, err := s.yamlProvider.GetAllNodes(ctx)
	if err != nil {
		return fmt.Errorf("getting nodes from YAML: %w", err)
	}

	s.logger.Printf("Found %d nodes in YAML file", len(yamlNodes))

	// Convert and sync each node
	syncCount := 0
	for _, yamlNode := range yamlNodes {

		// Check if node exists in boot service
		existingNode, err := s.bootClient.GetNode(ctx, yamlNode.XName)
		if err != nil {
			// Node doesn't exist, create it
			createReq := client.CreateNodeRequest{
				NodeSpec: node.NodeSpec{
					XName:   yamlNode.XName,
					Role:    yamlNode.Role,
					SubRole: yamlNode.SubRole,
					BootMAC: yamlNode.BootMAC,
					NID:     int32(yamlNode.NID),
				},
				Name: yamlNode.XName,
			}

			_, err = s.bootClient.CreateNode(ctx, createReq)
			if err != nil {
				s.logger.Printf("Failed to create node %s: %v", yamlNode.XName, err)
				continue
			}
			s.logger.Printf("Created node %s from YAML", yamlNode.XName)
			syncCount++
		} else {
			// Node exists, update if different
			if s.shouldUpdateNode(existingNode, yamlNode) {
				updateReq := client.UpdateNodeRequest{
					NodeSpec: node.NodeSpec{
						XName:   yamlNode.XName,
						Role:    yamlNode.Role,
						SubRole: yamlNode.SubRole,
						BootMAC: yamlNode.BootMAC,
						NID:     int32(yamlNode.NID),
					},
					Name: yamlNode.XName,
				}

				_, err = s.bootClient.UpdateNode(ctx, yamlNode.XName, updateReq)
				if err != nil {
					s.logger.Printf("Failed to update node %s: %v", yamlNode.XName, err)
					continue
				}
				s.logger.Printf("Updated node %s from YAML", yamlNode.XName)
				syncCount++
			}
		}
	}

	s.logger.Printf("YAML sync completed: %d nodes synchronized", syncCount)
	return nil
}

// StartSyncWorker starts a background worker to sync from YAML periodically
func (s *IntegrationService) StartSyncWorker(ctx context.Context) {
	if !s.config.SyncEnabled {
		s.logger.Printf("YAML sync worker disabled in configuration")
		return
	}

	s.logger.Printf("Starting YAML sync worker with interval %v", s.config.SyncInterval)

	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()

	// Initial sync
	if err := s.SyncNodesFromYAML(ctx); err != nil {
		s.logger.Printf("Initial YAML sync failed: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Printf("YAML sync worker stopping due to context cancellation")
			return
		case <-ticker.C:
			if err := s.SyncNodesFromYAML(ctx); err != nil {
				s.logger.Printf("YAML sync failed: %v", err)
			}
		}
	}
}

// HealthCheck verifies the YAML file is accessible and contains valid data
func (s *IntegrationService) HealthCheck(ctx context.Context) error {
	return s.yamlProvider.HealthCheck(ctx)
}

// GetStats returns statistics about the YAML file and sync status
func (s *IntegrationService) GetStats(ctx context.Context) map[string]interface{} {
	stats := s.yamlProvider.GetStats(ctx)

	// Add integration-specific stats
	stats["sync_enabled"] = s.config.SyncEnabled
	stats["sync_interval"] = s.config.SyncInterval.String()

	return stats
}

// shouldUpdateNode determines if a node needs to be updated
func (s *IntegrationService) shouldUpdateNode(existing *node.Node, yamlNode YAMLNode) bool {
	// Compare key fields
	if existing.Spec.Role != yamlNode.Role ||
		existing.Spec.SubRole != yamlNode.SubRole ||
		existing.Spec.BootMAC != yamlNode.BootMAC ||
		existing.Status.State != yamlNode.State {
		return true
	}

	// Compare NID
	existingNID := int32(0)
	if existing.Spec.NID != 0 {
		existingNID = existing.Spec.NID
	}
	yamlNID := int32(yamlNode.NID)
	if existingNID != yamlNID {
		return true
	}

	return false
}

// ReloadYAML forces a reload of the YAML file
func (s *IntegrationService) ReloadYAML(ctx context.Context) error {
	return s.yamlProvider.Reload(ctx)
}
