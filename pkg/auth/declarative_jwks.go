// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"slices"
	"time"
)

const declarativeJWKSMaxBytes = 1 << 20

type declarativeJWK struct {
	Kty    string          `json:"kty"`
	Kid    string          `json:"kid"`
	Alg    string          `json:"alg"`
	Use    string          `json:"use"`
	KeyOps []string        `json:"key_ops"`
	N      string          `json:"n"`
	E      string          `json:"e"`
	D      json.RawMessage `json:"d"`
	P      json.RawMessage `json:"p"`
	Q      json.RawMessage `json:"q"`
	DP     json.RawMessage `json:"dp"`
	DQ     json.RawMessage `json:"dq"`
	QI     json.RawMessage `json:"qi"`
	Oth    json.RawMessage `json:"oth"`
	K      json.RawMessage `json:"k"`
}

func (v *DeclarativeVerifier) lookupKey(ctx context.Context, kid, alg string) (*rsa.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	select {
	case v.cache.slots <- struct{}{}:
		defer func() { <-v.cache.slots }()
	case <-ctx.Done():
		return nil, ErrDeclarativeUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return nil, ErrDeclarativeUnavailable
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return nil, ErrDeclarativeUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, ErrDeclarativeUnavailable
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, declarativeJWKSMaxBytes+1))
	if err != nil || len(data) > declarativeJWKSMaxBytes {
		return nil, ErrDeclarativeUnavailable
	}
	keys, err := v.cache.load(data)
	if err != nil {
		return nil, ErrDeclarativeUnavailable
	}
	key, ok := keys[kid]
	if !ok || (key.alg != "" && key.alg != alg) || (key.use != "" && key.use != "sig") || (key.ops != nil && !slices.Contains(key.ops, "verify")) {
		return nil, ErrDeclarativeInvalid
	}
	return key.public, nil
}

func (k declarativeJWK) private() bool {
	return k.D != nil || k.P != nil || k.Q != nil || k.DP != nil || k.DQ != nil || k.QI != nil || k.Oth != nil || k.K != nil
}

func (k declarativeJWK) publicKey() (*rsa.PublicKey, error) {
	if k.Alg != "" && !slices.Contains(declarativeAlgorithms, k.Alg) {
		return nil, ErrDeclarativeUnavailable
	}
	n, err := base64.RawURLEncoding.Strict().DecodeString(k.N)
	if err != nil || len(n) == 0 || n[0] == 0 {
		return nil, ErrDeclarativeUnavailable
	}
	e, err := base64.RawURLEncoding.Strict().DecodeString(k.E)
	if err != nil || len(e) == 0 || len(e) > 4 || e[0] == 0 {
		return nil, ErrDeclarativeUnavailable
	}
	modulus := new(big.Int).SetBytes(n)
	exponent := new(big.Int).SetBytes(e).Int64()
	if modulus.BitLen() < 2048 || modulus.BitLen() > 8192 || exponent < 3 || exponent > 1<<31-1 || exponent%2 == 0 || modulus.Bit(0) == 0 {
		return nil, ErrDeclarativeUnavailable
	}
	return &rsa.PublicKey{N: modulus, E: int(exponent)}, nil
}
