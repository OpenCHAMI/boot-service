// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package bootscript

import (
	"sync"
	"time"
)

// CacheEntry represents a cached boot script
type CacheEntry struct {
	Script      string
	GeneratedAt time.Time
	ExpiresAt   time.Time
	NodeID      string
	ConfigID    string
}

// ScriptCache manages caching of generated boot scripts
type ScriptCache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

// NewScriptCache creates a new script cache with the specified TTL
func NewScriptCache(ttl time.Duration) *ScriptCache {
	cache := &ScriptCache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}

	// Start cleanup routine
	go cache.cleanup()

	return cache
}

// Get retrieves a cached script if it exists and is not expired
func (c *ScriptCache) Get(cacheKey string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[cacheKey]
	if !exists {
		return "", false
	}

	// Check if entry has expired
	if time.Now().After(entry.ExpiresAt) {
		// Entry expired, remove it
		delete(c.entries, cacheKey)
		return "", false
	}

	return entry.Script, true
}

// Set stores a script in the cache
func (c *ScriptCache) Set(cacheKey, script, nodeID, configID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	entry := &CacheEntry{
		Script:      script,
		GeneratedAt: now,
		ExpiresAt:   now.Add(c.ttl),
		NodeID:      nodeID,
		ConfigID:    configID,
	}

	c.entries[cacheKey] = entry
}

// Invalidate removes a specific entry from the cache
func (c *ScriptCache) Invalidate(cacheKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.entries, cacheKey)
}

// InvalidateByNodeID removes all cache entries for a specific node
func (c *ScriptCache) InvalidateByNodeID(nodeID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.NodeID == nodeID {
			delete(c.entries, key)
		}
	}
}

// InvalidateByConfigID removes all cache entries using a specific configuration
func (c *ScriptCache) InvalidateByConfigID(configID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key, entry := range c.entries {
		if entry.ConfigID == configID {
			delete(c.entries, key)
		}
	}
}

// Clear removes all entries from the cache
func (c *ScriptCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
}

// Stats returns cache statistics
func (c *ScriptCache) Stats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	expired := 0

	for _, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			expired++
		}
	}

	return CacheStats{
		TotalEntries:   len(c.entries),
		ExpiredEntries: expired,
		ValidEntries:   len(c.entries) - expired,
	}
}

// CacheStats provides cache performance metrics
type CacheStats struct {
	TotalEntries   int
	ExpiredEntries int
	ValidEntries   int
}

// cleanup periodically removes expired entries
func (c *ScriptCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2) // Clean up twice per TTL period
	defer ticker.Stop()

	for range ticker.C {
		c.cleanupExpired()
	}
}

// cleanupExpired removes expired entries from the cache
func (c *ScriptCache) cleanupExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, entry := range c.entries {
		if now.After(entry.ExpiresAt) {
			delete(c.entries, key)
		}
	}
}

// generateCacheKey creates a cache key from node identifier and configuration
func (c *BootScriptController) generateCacheKey(identifier string, configName string) string {
	if configName == "" {
		configName = "default"
	}
	return identifier + ":" + configName
}
