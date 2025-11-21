// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/openchami/tokensmith/pkg/token"
)

// TestingConfig returns a configuration optimized for testing
// TestingConfig returns an auth configuration for testing with locally generated tokens
func TestingConfig(publicKeyPEM string) Config {
	return Config{
		Enabled:            true,
		NonEnforcing:       false,
		ValidateExpiration: true,
		ValidateIssuer:     true,
		ValidateAudience:   true,
		RequiredClaims:     []string{"sub", "iss", "aud"},
		RequiredScopes:     []string{},   // No required scopes by default for testing
		JWTPublicKey:       publicKeyPEM, // Use the PEM-encoded public key
	}
}

// NonEnforcingConfig returns a config that logs auth errors but doesn't block requests
func NonEnforcingConfig() Config {
	config := DefaultConfig()
	config.AllowEmptyToken = true      // Allow requests without tokens
	config.NonEnforcing = true         // Log errors but don't block
	config.ValidateExpiration = false  // Don't check expiration
	config.ValidateIssuer = false      // Don't check issuer
	config.ValidateAudience = false    // Don't check audience
	config.RequiredClaims = []string{} // Don't require specific claims
	return config
}

// TestKeyPair represents an RSA key pair for testing
type TestKeyPair struct {
	PrivateKey   *rsa.PrivateKey
	PublicKey    *rsa.PublicKey
	PublicKeyPEM string
}

// GenerateTestKeyPair creates an RSA key pair for testing JWT tokens
func GenerateTestKeyPair() (*TestKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Convert public key to PEM format
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyDER,
	})

	return &TestKeyPair{
		PrivateKey:   privateKey,
		PublicKey:    &privateKey.PublicKey,
		PublicKeyPEM: string(publicKeyPEM),
	}, nil
}

// CreateTestToken creates a JWT token for testing purposes
func CreateTestToken(keyPair *TestKeyPair, claims *token.TSClaims) (string, error) {
	if claims == nil {
		// Create default test claims with all required NIST fields
		now := time.Now()
		claims = &token.TSClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Issuer:    "test-issuer",
				Subject:   "test-user",
				Audience:  []string{"boot-service"},
				ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
				NotBefore: jwt.NewNumericDate(now),
				IssuedAt:  jwt.NewNumericDate(now),
			},
			Scope:       []string{"read", "write"}, // Use simple scopes
			ClusterID:   "test-cluster",
			OpenCHAMIID: "test-openchami",
			AuthLevel:   "IAL2",
			AuthFactors: 2,
			AuthMethods: []string{"password", "mfa"},
			SessionID:   "test-session-123",
			SessionExp:  now.Add(1 * time.Hour).Unix(),
			AuthEvents:  []string{"login"},
		}
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(keyPair.PrivateKey)
}

// CreateTestTokenWithScopes creates a test token with specific scopes
func CreateTestTokenWithScopes(keyPair *TestKeyPair, scopes []string) (string, error) {
	now := time.Now()
	claims := &token.TSClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-issuer",
			Subject:   "test-user",
			Audience:  []string{"boot-service"},
			ExpiresAt: jwt.NewNumericDate(now.Add(1 * time.Hour)),
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Scope:       scopes,
		ClusterID:   "test-cluster",
		OpenCHAMIID: "test-openchami",
		AuthLevel:   "IAL2",
		AuthFactors: 2,
		AuthMethods: []string{"password", "mfa"},
		SessionID:   "test-session-123",
		SessionExp:  now.Add(1 * time.Hour).Unix(),
		AuthEvents:  []string{"login"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(keyPair.PrivateKey)
}

// CreateServiceToken creates a service-to-service test token
func CreateServiceToken(keyPair *TestKeyPair, serviceID, targetService string, scopes []string) (string, error) {
	now := time.Now()
	claims := &token.TSClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "test-tokensmith",
			Subject:   serviceID,
			Audience:  []string{targetService},
			ExpiresAt: jwt.NewNumericDate(now.Add(5 * time.Minute)), // Short-lived for services
			NotBefore: jwt.NewNumericDate(now),
			IssuedAt:  jwt.NewNumericDate(now),
		},
		Scope:       scopes,
		ClusterID:   "test-cluster",
		OpenCHAMIID: "test-openchami",
		// Required NIST claims for service tokens
		AuthLevel:   "IAL2",
		AuthFactors: 2,
		AuthMethods: []string{"service", "certificate"},
		SessionID:   "service-session-123",
		SessionExp:  now.Add(5 * time.Minute).Unix(),
		AuthEvents:  []string{"service_auth"},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(keyPair.PrivateKey)
}

// CreateStaticKeyConfig creates a test config with a static public key
func CreateStaticKeyConfig(publicKeyPEM string) Config {
	config := TestingConfig(publicKeyPEM)
	config.JWTIssuer = "test-issuer"
	config.JWTAudience = "boot-service"
	return config
}
