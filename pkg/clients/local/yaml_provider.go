// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Local YAML node provider for development and standalone deployments
package local

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// YAMLNodeProvider provides node information from a local YAML file
type YAMLNodeProvider struct {
	filePath     string
	nodes        map[string]YAMLNode
	lastModified time.Time
	mutex        sync.RWMutex
	logger       *log.Logger
	autoReload   bool
}

// YAMLNode represents a node configuration in the YAML file
type YAMLNode struct {
	ID                 string              `yaml:"id"`
	XName              string              `yaml:"xname"`
	Type               string              `yaml:"type"`
	Role               string              `yaml:"role"`
	SubRole            string              `yaml:"subrole,omitempty"`
	State              string              `yaml:"state"`
	Enabled            bool                `yaml:"enabled"`
	NID                int                 `yaml:"nid,omitempty"`
	BootMAC            string              `yaml:"boot_mac,omitempty"`
	EthernetInterfaces []EthernetInterface `yaml:"ethernet_interfaces,omitempty"`
	Metadata           map[string]string   `yaml:"metadata,omitempty"`
}

// EthernetInterface represents a network interface
type EthernetInterface struct {
	MACAddress  string `yaml:"mac_address"`
	IPAddress   string `yaml:"ip_address,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// YAMLNodesFile represents the structure of the YAML file
type YAMLNodesFile struct {
	Version string     `yaml:"version"`
	Nodes   []YAMLNode `yaml:"nodes"`
}

// NewYAMLNodeProvider creates a new YAML node provider
func NewYAMLNodeProvider(filePath string, autoReload bool, logger *log.Logger) (*YAMLNodeProvider, error) {
	provider := &YAMLNodeProvider{
		filePath:   filePath,
		nodes:      make(map[string]YAMLNode),
		logger:     logger,
		autoReload: autoReload,
	}

	// Load initial data
	err := provider.loadNodes()
	if err != nil {
		return nil, fmt.Errorf("failed to load initial nodes from %s: %w", filePath, err)
	}

	provider.logger.Printf("YAML node provider initialized with %d nodes from %s", len(provider.nodes), filePath)
	return provider, nil
}

// loadNodes loads node data from the YAML file
func (p *YAMLNodeProvider) loadNodes() error {
	// Check if file needs reloading
	if p.autoReload {
		// For simplicity, we'll reload on every call when autoReload is true
		// In production, you might want to check file modification time
	}

	// Read YAML file
	data, err := ioutil.ReadFile(p.filePath)
	if err != nil {
		return fmt.Errorf("reading YAML file: %w", err)
	}

	// Parse YAML
	var yamlFile YAMLNodesFile
	err = yaml.Unmarshal(data, &yamlFile)
	if err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	// Build lookup maps
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.nodes = make(map[string]YAMLNode)

	for _, node := range yamlFile.Nodes {
		// Index by ID, XName, and MAC addresses for flexible lookup
		if node.ID != "" {
			p.nodes[node.ID] = node
		}
		if node.XName != "" {
			p.nodes[node.XName] = node
		}
		if node.BootMAC != "" {
			p.nodes[strings.ToLower(node.BootMAC)] = node
		}

		// Index by all ethernet interface MAC addresses
		for _, iface := range node.EthernetInterfaces {
			if iface.MACAddress != "" {
				p.nodes[strings.ToLower(iface.MACAddress)] = node
			}
		}

		// Index by NID if present
		if node.NID > 0 {
			p.nodes[fmt.Sprintf("%d", node.NID)] = node
		}
	}

	p.lastModified = time.Now()
	p.logger.Printf("Loaded %d nodes from YAML file, indexed %d entries", len(yamlFile.Nodes), len(p.nodes))
	return nil
}

// GetNodeByIdentifier retrieves a node by any identifier (ID, XName, MAC, NID)
func (p *YAMLNodeProvider) GetNodeByIdentifier(ctx context.Context, identifier string) (*YAMLNode, error) {
	// Reload if auto-reload is enabled
	if p.autoReload {
		if err := p.loadNodes(); err != nil {
			p.logger.Printf("Warning: failed to reload YAML file: %v", err)
			// Continue with cached data
		}
	}

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Try direct lookup
	if node, found := p.nodes[identifier]; found {
		return &node, nil
	}

	// Try lowercase MAC address lookup
	if node, found := p.nodes[strings.ToLower(identifier)]; found {
		return &node, nil
	}

	return nil, fmt.Errorf("node not found for identifier: %s", identifier)
}

// GetAllNodes returns all nodes from the YAML file
func (p *YAMLNodeProvider) GetAllNodes(ctx context.Context) ([]YAMLNode, error) {
	// Reload if auto-reload is enabled
	if p.autoReload {
		if err := p.loadNodes(); err != nil {
			p.logger.Printf("Warning: failed to reload YAML file: %v", err)
		}
	}

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Build unique list of nodes (since we have multiple index entries per node)
	seen := make(map[string]bool)
	var nodes []YAMLNode

	for _, node := range p.nodes {
		if !seen[node.XName] {
			nodes = append(nodes, node)
			seen[node.XName] = true
		}
	}

	return nodes, nil
}

// GetNodesByRole returns nodes filtered by role
func (p *YAMLNodeProvider) GetNodesByRole(ctx context.Context, role string) ([]YAMLNode, error) {
	allNodes, err := p.GetAllNodes(ctx)
	if err != nil {
		return nil, err
	}

	var filteredNodes []YAMLNode
	for _, node := range allNodes {
		if strings.EqualFold(node.Role, role) {
			filteredNodes = append(filteredNodes, node)
		}
	}

	return filteredNodes, nil
}

// HealthCheck verifies the YAML file is readable and contains valid data
func (p *YAMLNodeProvider) HealthCheck(ctx context.Context) error {
	// Try to reload data
	err := p.loadNodes()
	if err != nil {
		return fmt.Errorf("YAML health check failed: %w", err)
	}

	p.mutex.RLock()
	nodeCount := len(p.nodes)
	p.mutex.RUnlock()

	if nodeCount == 0 {
		return fmt.Errorf("YAML file contains no nodes")
	}

	return nil
}

// GetStats returns statistics about the loaded nodes
func (p *YAMLNodeProvider) GetStats(ctx context.Context) map[string]interface{} {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	// Count unique nodes and roles
	uniqueNodes := make(map[string]bool)
	roles := make(map[string]int)

	for _, node := range p.nodes {
		if !uniqueNodes[node.XName] {
			uniqueNodes[node.XName] = true
			roles[node.Role]++
		}
	}

	stats := map[string]interface{}{
		"yaml_file":     p.filePath,
		"last_loaded":   p.lastModified,
		"auto_reload":   p.autoReload,
		"total_nodes":   len(uniqueNodes),
		"total_indexes": len(p.nodes),
		"roles":         roles,
		"yaml_healthy":  true,
	}

	return stats
}

// Reload forces a reload of the YAML file
func (p *YAMLNodeProvider) Reload(ctx context.Context) error {
	return p.loadNodes()
}
