// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// HSM client for Hardware State Manager integration
package hsm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// HSMComponent represents a component from HSM
type HSMComponent struct {
	ID              string            `json:"ID"`
	Type            string            `json:"Type"`
	State           string            `json:"State"`
	Flag            string            `json:"Flag"`
	Enabled         bool              `json:"Enabled"`
	Role            string            `json:"Role"`
	SubRole         string            `json:"SubRole"`
	NID             int32             `json:"NID,omitempty"`
	NetType         string            `json:"NetType,omitempty"`
	Arch            string            `json:"Arch,omitempty"`
	Class           string            `json:"Class,omitempty"`
	ExtraProperties map[string]string `json:"ExtraProperties,omitempty"`
	LastUpdateTime  string            `json:"LastUpdateTime,omitempty"`
}

// HSMResponse represents the response from HSM components endpoint
type HSMResponse struct {
	Components []HSMComponent `json:"Components"`
}

// HSMEthernetInterface represents network interface information from HSM
type HSMEthernetInterface struct {
	MACAddress  string `json:"MACAddress"`
	IPAddress   string `json:"IPAddress,omitempty"`
	ComponentID string `json:"ComponentID"`
	Description string `json:"Description,omitempty"`
	Type        string `json:"Type,omitempty"`
	LastUpdate  string `json:"LastUpdate,omitempty"`
}

// HSMEthernetResponse represents the response from HSM ethernet interfaces endpoint
type HSMEthernetResponse struct {
	EthernetInterfaces []HSMEthernetInterface `json:"EthernetInterfaces"`
}

// HSMConfig holds configuration for HSM client
type HSMConfig struct {
	BaseURL              string        `json:"baseURL"`
	Timeout              time.Duration `json:"timeout"`
	RetryAttempts        int           `json:"retryAttempts"`
	RetryDelay           time.Duration `json:"retryDelay"`
	CacheExpiry          time.Duration `json:"cacheExpiry"`
	AuthToken            string        `json:"authToken,omitempty"`
	EnableCircuitBreaker bool          `json:"enableCircuitBreaker"`
}

// DefaultHSMConfig returns a default HSM configuration
func DefaultHSMConfig() HSMConfig {
	return HSMConfig{
		BaseURL:              "http://localhost:27779",
		Timeout:              30 * time.Second,
		RetryAttempts:        3,
		RetryDelay:           1 * time.Second,
		CacheExpiry:          5 * time.Minute,
		EnableCircuitBreaker: true,
	}
}

// HSMClient provides an interface to Hardware State Manager
type HSMClient struct {
	config     HSMConfig
	httpClient *http.Client
	logger     *log.Logger
	cache      *HSMCache
	mu         sync.RWMutex
}

// HSMCache provides caching for HSM responses to reduce load
type HSMCache struct {
	components         map[string]*CacheEntry
	ethernetInterfaces map[string]*CacheEntry
	mu                 sync.RWMutex
	expiry             time.Duration
}

// CacheEntry represents a cached item with expiration
type CacheEntry struct {
	Data      interface{}
	ExpiresAt time.Time
}

// NewHSMCache creates a new HSM cache
func NewHSMCache(expiry time.Duration) *HSMCache {
	return &HSMCache{
		components:         make(map[string]*CacheEntry),
		ethernetInterfaces: make(map[string]*CacheEntry),
		expiry:             expiry,
	}
}

// Get retrieves an item from cache if not expired
func (c *HSMCache) Get(key string, cacheMap map[string]*CacheEntry) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := cacheMap[key]
	if !exists || time.Now().After(entry.ExpiresAt) {
		return nil, false
	}

	return entry.Data, true
}

// Set stores an item in cache with expiration
func (c *HSMCache) Set(key string, data interface{}, cacheMap map[string]*CacheEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cacheMap[key] = &CacheEntry{
		Data:      data,
		ExpiresAt: time.Now().Add(c.expiry),
	}
}

// NewHSMClient creates a new HSM client
func NewHSMClient(config HSMConfig, logger *log.Logger) *HSMClient {
	if logger == nil {
		logger = log.New(log.Writer(), "hsm: ", log.LstdFlags)
	}

	httpClient := &http.Client{
		Timeout: config.Timeout,
		Transport: &http.Transport{
			MaxIdleConns:        10,
			MaxIdleConnsPerHost: 2,
			IdleConnTimeout:     30 * time.Second,
		},
	}

	cache := NewHSMCache(config.CacheExpiry)

	return &HSMClient{
		config:     config,
		httpClient: httpClient,
		logger:     logger,
		cache:      cache,
	}
}

// GetComponents retrieves all components from HSM
func (c *HSMClient) GetComponents(ctx context.Context) ([]HSMComponent, error) {
	// Check cache first
	if data, found := c.cache.Get("all_components", c.cache.components); found {
		c.logger.Printf("HSM components cache hit")
		return data.([]HSMComponent), nil
	}

	url := fmt.Sprintf("%s/hsm/v2/State/Components", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HSM request: %w", err)
	}

	// Add authentication if provided
	if c.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call HSM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HSM returned status %d", resp.StatusCode)
	}

	var hsmResp HSMResponse
	if err := json.NewDecoder(resp.Body).Decode(&hsmResp); err != nil {
		return nil, fmt.Errorf("failed to decode HSM response: %w", err)
	}

	// Cache the result
	c.cache.Set("all_components", hsmResp.Components, c.cache.components)

	c.logger.Printf("Retrieved %d components from HSM", len(hsmResp.Components))
	return hsmResp.Components, nil
}

// GetComponent retrieves a specific component by ID from HSM
func (c *HSMClient) GetComponent(ctx context.Context, componentID string) (*HSMComponent, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("component_%s", componentID)
	if data, found := c.cache.Get(cacheKey, c.cache.components); found {
		c.logger.Printf("HSM component cache hit for %s", componentID)
		return data.(*HSMComponent), nil
	}

	url := fmt.Sprintf("%s/hsm/v2/State/Components/%s", c.config.BaseURL, componentID)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HSM request: %w", err)
	}

	// Add authentication if provided
	if c.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call HSM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("component %s not found in HSM", componentID)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HSM returned status %d for component %s", resp.StatusCode, componentID)
	}

	var component HSMComponent
	if err := json.NewDecoder(resp.Body).Decode(&component); err != nil {
		return nil, fmt.Errorf("failed to decode HSM response: %w", err)
	}

	// Cache the result
	c.cache.Set(cacheKey, &component, c.cache.components)

	c.logger.Printf("Retrieved component %s from HSM", componentID)
	return &component, nil
}

// GetEthernetInterfaces retrieves all ethernet interfaces from HSM
func (c *HSMClient) GetEthernetInterfaces(ctx context.Context) ([]HSMEthernetInterface, error) {
	// Check cache first
	if data, found := c.cache.Get("all_ethernet", c.cache.ethernetInterfaces); found {
		c.logger.Printf("HSM ethernet interfaces cache hit")
		return data.([]HSMEthernetInterface), nil
	}

	url := fmt.Sprintf("%s/hsm/v2/Inventory/EthernetInterfaces", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HSM request: %w", err)
	}

	// Add authentication if provided
	if c.config.AuthToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.config.AuthToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call HSM: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HSM returned status %d", resp.StatusCode)
	}

	var hsmResp HSMEthernetResponse
	if err := json.NewDecoder(resp.Body).Decode(&hsmResp); err != nil {
		return nil, fmt.Errorf("failed to decode HSM response: %w", err)
	}

	// Cache the result
	c.cache.Set("all_ethernet", hsmResp.EthernetInterfaces, c.cache.ethernetInterfaces)

	c.logger.Printf("Retrieved %d ethernet interfaces from HSM", len(hsmResp.EthernetInterfaces))
	return hsmResp.EthernetInterfaces, nil
}

// GetComponentByMAC finds a component by its MAC address
func (c *HSMClient) GetComponentByMAC(ctx context.Context, macAddress string) (*HSMComponent, error) {
	// Get ethernet interfaces to find the component ID
	interfaces, err := c.GetEthernetInterfaces(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get ethernet interfaces: %w", err)
	}

	// Find the component ID for this MAC address
	var componentID string
	for _, iface := range interfaces {
		if iface.MACAddress == macAddress {
			componentID = iface.ComponentID
			break
		}
	}

	if componentID == "" {
		return nil, fmt.Errorf("no component found for MAC address %s", macAddress)
	}

	// Get the component details
	return c.GetComponent(ctx, componentID)
}

// Health checks if HSM is reachable and responding
func (c *HSMClient) Health(ctx context.Context) error {
	url := fmt.Sprintf("%s/hsm/v2/service/ready", c.config.BaseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HSM health request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HSM health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HSM health check returned status %d", resp.StatusCode)
	}

	return nil
}

// ClearCache clears all cached HSM data
func (c *HSMClient) ClearCache() {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()

	c.cache.components = make(map[string]*CacheEntry)
	c.cache.ethernetInterfaces = make(map[string]*CacheEntry)

	c.logger.Printf("HSM cache cleared")
}
