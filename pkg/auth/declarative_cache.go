// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"sync"
)

type declarativeCachedKey struct {
	public *rsa.PublicKey
	alg    string
	use    string
	ops    []string
}

// The cache holds one bounded JWKS snapshot; every use first fetches a fresh response.
type declarativeKeyCache struct {
	mu    sync.Mutex
	hash  [sha256.Size]byte
	keys  map[string]declarativeCachedKey
	slots chan struct{}
}

func (c *declarativeKeyCache) load(data []byte) (map[string]declarativeCachedKey, error) {
	hash := sha256.Sum256(data)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.keys != nil && c.hash == hash {
		return c.keys, nil
	}
	var set struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if !uniqueJSONObject(data, false) || json.Unmarshal(data, &set) != nil || len(set.Keys) == 0 {
		return nil, ErrDeclarativeUnavailable
	}
	keys := make(map[string]declarativeCachedKey)
	for _, raw := range set.Keys {
		var key declarativeJWK
		if !uniqueJSONObject(raw, false) || json.Unmarshal(raw, &key) != nil || key.private() {
			return nil, ErrDeclarativeUnavailable
		}
		if key.Kty != "RSA" {
			continue
		}
		if key.Kid == "" || len(key.Kid) > 256 {
			return nil, ErrDeclarativeUnavailable
		}
		if _, exists := keys[key.Kid]; exists {
			return nil, ErrDeclarativeUnavailable
		}
		public, err := key.publicKey()
		if err != nil {
			return nil, ErrDeclarativeUnavailable
		}
		keys[key.Kid] = declarativeCachedKey{public: public, alg: key.Alg, use: key.Use, ops: key.KeyOps}
	}
	if len(keys) == 0 {
		return nil, ErrDeclarativeUnavailable
	}
	c.hash, c.keys = hash, keys
	return keys, nil
}
