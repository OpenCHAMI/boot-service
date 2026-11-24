// SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package bootscript

import (
	"crypto/sha256"
	"encoding/json"
	"sync"

	v1 "github.com/openchami/boot-service/apis/boot.openchami.io/v1"
	"github.com/openchami/fabrica/pkg/revision"
)

// This cache only avoids rendering. Every lookup follows a fresh node lookup
// and an atomic assignment/revision resolution; it cannot bypass either check.
type declarativeCache struct {
	mu      sync.Mutex
	scripts map[[32]byte]string
	hits    uint64
}

func declarativeCacheKey(node v1.NodeSpec, assignment v1.BootAssignment, content v1.BootConfigSpec, rev revision.Metadata, profile string) [32]byte {
	// These concrete data-only types cannot fail JSON marshaling. Include the
	// complete detached inputs so future template fields also invalidate hits.
	data, _ := json.Marshal(struct {
		Node              v1.NodeSpec
		Assignment        v1.BootAssignment
		Content           v1.BootConfigSpec
		Revision          revision.Metadata
		Profile, Template string
	}{node, assignment, content, rev, profile, DefaultIPXETemplate})
	return sha256.Sum256(data)
}

func (c *declarativeCache) get(key [32]byte) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	script, found := c.scripts[key]
	if found {
		c.hits++
	}
	return script, found
}

func (c *declarativeCache) put(key [32]byte, script string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.scripts == nil || len(c.scripts) >= 1024 {
		c.scripts = make(map[[32]byte]string)
	}
	c.scripts[key] = script
}

// CacheHits counts actual declarative render-cache hits for diagnostics.
func (c *DeclarativeController) CacheHits() uint64 {
	c.cache.mu.Lock()
	defer c.cache.mu.Unlock()
	return c.cache.hits
}
