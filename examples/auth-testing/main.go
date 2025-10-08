// Copyright © 2025 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

// Example demonstrating how to test the boot service with authentication
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/openchami/boot-service/pkg/auth"
)

func main() {
	// Generate test keys
	keyPair, err := auth.GenerateTestKeyPair()
	if err != nil {
		log.Fatal("Failed to generate test keys:", err)
	}

	// Create test tokens
	userToken, err := auth.CreateTestTokenWithScopes(keyPair, []string{"boot:read", "boot:write"})
	if err != nil {
		log.Fatal("Failed to create user token:", err)
	}

	serviceToken, err := auth.CreateServiceToken(keyPair, "test-client", "boot-service", []string{"service:boot"})
	if err != nil {
		log.Fatal("Failed to create service token:", err)
	}

	readOnlyToken, err := auth.CreateTestTokenWithScopes(keyPair, []string{"boot:read"})
	if err != nil {
		log.Fatal("Failed to create read-only token:", err)
	}

	// Print tokens for testing
	fmt.Println("=== Authentication Test Tokens ===")
	fmt.Printf("User Token (read+write): %s\n\n", userToken)
	fmt.Printf("Service Token: %s\n\n", serviceToken)
	fmt.Printf("Read-Only Token: %s\n\n", readOnlyToken)

	// Setup different auth configurations for testing
	fmt.Println("=== Starting Test Server on :8080 ===")

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// 1. Development mode - no authentication
	r.Route("/dev", func(r chi.Router) {
		devConfig := auth.DevConfig()
		devAuth := devConfig.CreateMiddleware(nil)
		r.Use(devAuth)

		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("OK - No auth required"))
		})
	})

	// 2. Non-enforcing mode - logs auth errors but allows requests
	r.Route("/non-enforcing", func(r chi.Router) {
		config := auth.NonEnforcingConfig()
		config.JWTPublicKey = keyPair.PublicKeyPEM
		nonEnforcingAuth := config.CreateMiddleware(nil)
		r.Use(nonEnforcingAuth)

		r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetClaimsFromRequest(r)
			if err != nil {
				w.Write([]byte("OK - No valid token, but non-enforcing mode allows request"))
				return
			}
			w.Write([]byte(fmt.Sprintf("OK - Authenticated as %s with scopes %v", claims.Subject, claims.Scope)))
		})
	})

	// 3. Enforcing mode with static key
	r.Route("/enforcing", func(r chi.Router) {
		config := auth.CreateStaticKeyConfig(keyPair.PublicKeyPEM)
		config.NonEnforcing = false // Strict enforcement
		enforcingAuth := config.CreateMiddleware(nil)
		r.Use(enforcingAuth)

		r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
			claims, err := auth.GetClaimsFromRequest(r)
			if err != nil {
				http.Error(w, "Failed to get claims", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(fmt.Sprintf("Protected resource accessed by %s", claims.Subject)))
		})

		// Scope-protected routes
		r.Group(func(r chi.Router) {
			r.Use(auth.CreateScopeMiddleware("boot:read"))
			r.Get("/read", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Read operation successful"))
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(auth.CreateScopeMiddleware("boot:write"))
			r.Post("/write", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Write operation successful"))
			})
		})

		// Service-to-service routes
		r.Group(func(r chi.Router) {
			r.Use(auth.CreateServiceTokenMiddleware("boot-service"))
			r.Get("/internal", func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("Internal service endpoint"))
			})
		})
	})

	// Print test commands
	fmt.Println("\n=== Test Commands ===")
	fmt.Println("# No authentication required:")
	fmt.Println("curl http://localhost:8080/dev/health")
	fmt.Println()

	fmt.Println("# Non-enforcing mode (works with or without token):")
	fmt.Println("curl http://localhost:8080/non-enforcing/test")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/non-enforcing/test\n", userToken[:50]+"...")
	fmt.Println()

	fmt.Println("# Enforcing mode (requires valid token):")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/enforcing/protected\n", userToken[:50]+"...")
	fmt.Println()

	fmt.Println("# Scope-based authorization:")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/enforcing/read\n", readOnlyToken[:50]+"...")
	fmt.Printf("curl -X POST -H \"Authorization: Bearer %s\" http://localhost:8080/enforcing/write\n", userToken[:50]+"...")
	fmt.Printf("curl -X POST -H \"Authorization: Bearer %s\" http://localhost:8080/enforcing/write\n", readOnlyToken[:50]+"...  # Should fail")
	fmt.Println()

	fmt.Println("# Service token:")
	fmt.Printf("curl -H \"Authorization: Bearer %s\" http://localhost:8080/enforcing/internal\n", serviceToken[:50]+"...")
	fmt.Println()

	fmt.Println("# Invalid token (should fail):")
	fmt.Println("curl -H \"Authorization: Bearer invalid.token.here\" http://localhost:8080/enforcing/protected")
	fmt.Println()

	// Start server
	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}
