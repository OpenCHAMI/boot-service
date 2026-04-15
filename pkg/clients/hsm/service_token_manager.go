// Copyright © 2026 OpenCHAMI a Series of LF Projects, LLC
//
// SPDX-License-Identifier: MIT

package hsm

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/openchami/tokensmith/pkg/tokenservice"
)

// TokenExchangeConfig controls how HSM service tokens are obtained from TokenSmith.
type TokenExchangeConfig struct {
	TokenSmithURL  string
	BootstrapToken string
	TargetService  string
	// Scopes is retained as an operator hint for diagnostics only.
	// Current TokenSmith ServiceClient does not send scopes during exchange;
	// issued scopes come from bootstrap-token policy on the server.
	Scopes                  []string
	RequestTimeout          time.Duration
	RefreshBefore           time.Duration
	BootstrapMaxAttempts    int
	BootstrapInitialBackoff time.Duration
	BootstrapMaxBackoff     time.Duration
}

// DefaultTokenExchangeConfig returns safe defaults for TokenSmith service-token exchange.
func DefaultTokenExchangeConfig() TokenExchangeConfig {
	return TokenExchangeConfig{
		RequestTimeout:          10 * time.Second,
		RefreshBefore:           2 * time.Minute,
		BootstrapMaxAttempts:    5,
		BootstrapInitialBackoff: 1 * time.Second,
		BootstrapMaxBackoff:     15 * time.Second,
	}
}

// ServiceTokenManager exchanges bootstrap tokens for short-lived service tokens and refreshes on demand.
type ServiceTokenManager struct {
	config TokenExchangeConfig
	client *tokenservice.ServiceClient
	logger *log.Logger
}

// NewServiceTokenManager creates a token manager for HSM service authentication.
func NewServiceTokenManager(config TokenExchangeConfig, logger *log.Logger) *ServiceTokenManager {
	if logger == nil {
		logger = log.New(log.Writer(), "hsm-tokens: ", log.LstdFlags)
	}

	if config.RequestTimeout <= 0 {
		config.RequestTimeout = DefaultTokenExchangeConfig().RequestTimeout
	}
	if config.RefreshBefore <= 0 {
		config.RefreshBefore = DefaultTokenExchangeConfig().RefreshBefore
	}
	if config.BootstrapMaxAttempts <= 0 {
		config.BootstrapMaxAttempts = DefaultTokenExchangeConfig().BootstrapMaxAttempts
	}
	if config.BootstrapInitialBackoff <= 0 {
		config.BootstrapInitialBackoff = DefaultTokenExchangeConfig().BootstrapInitialBackoff
	}
	if config.BootstrapMaxBackoff <= 0 {
		config.BootstrapMaxBackoff = DefaultTokenExchangeConfig().BootstrapMaxBackoff
	}
	if strings.TrimSpace(config.TargetService) == "" {
		config.TargetService = "hsm"
	}

	client := tokenservice.NewServiceClientWithOptions(
		strings.TrimSpace(config.TokenSmithURL),
		"boot-service",
		"boot-service",
		"",
		"",
		tokenservice.WithHTTPClient(&http.Client{Timeout: config.RequestTimeout}),
		tokenservice.WithBootstrapToken(config.BootstrapToken),
		tokenservice.WithTargetService(strings.TrimSpace(config.TargetService)),
		tokenservice.WithRefreshBefore(config.RefreshBefore),
		tokenservice.WithBootstrapMaxAttempts(config.BootstrapMaxAttempts),
		tokenservice.WithBootstrapInitialBackoff(config.BootstrapInitialBackoff),
		tokenservice.WithBootstrapMaxBackoff(config.BootstrapMaxBackoff),
	)

	return &ServiceTokenManager{
		config: config,
		client: client,
		logger: logger,
	}
}

// Initialize fetches the first service token. Call this during startup to fail closed.
func (m *ServiceTokenManager) Initialize(ctx context.Context) error {
	endpoint := m.serviceTokenEndpoint()
	bootstrapPresent := strings.TrimSpace(m.config.BootstrapToken) != ""

	m.logger.Printf("initializing HSM service token exchange: endpoint=%s target=%s scope_hint=%s bootstrap_token_present=%v max_attempts=%d",
		endpoint,
		strings.TrimSpace(m.config.TargetService),
		strings.Join(sortedScopes(m.config.Scopes), ","),
		bootstrapPresent,
		m.config.BootstrapMaxAttempts,
	)

	err := m.client.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap token exchange failed: endpoint=%s: %w", endpoint, err)
	}

	stats := m.client.Stats()
	if stats.RefreshFailures > 0 {
		m.logger.Printf("HSM service token initialized after %d attempts", stats.RefreshFailures+1)
	}

	return nil
}

// GetToken returns a valid bearer token, refreshing if close to expiry.
func (m *ServiceTokenManager) GetToken(ctx context.Context) (string, error) {
	if err := m.client.RefreshTokenIfNeeded(ctx); err != nil {
		return "", err
	}

	token := m.client.GetServiceToken()
	if token == nil || strings.TrimSpace(token.Token) == "" {
		return "", fmt.Errorf("service token unavailable")
	}

	return token.Token, nil
}

// StartAutoRefresh keeps tokens warm in the background. It exits when ctx is canceled.
func (m *ServiceTokenManager) StartAutoRefresh(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			refreshCtx, cancel := context.WithTimeout(ctx, m.config.RequestTimeout)
			err := m.client.RefreshTokenIfNeeded(refreshCtx)
			cancel()
			if err != nil {
				stats := m.Stats()
				m.logger.Printf("warning: failed to refresh HSM service token: %v (success=%v failure=%v)",
					err,
					stats["refresh_success_count"],
					stats["refresh_failure_count"],
				)
			}
		}
	}
}

// Stats returns token refresh counters and latest refresh/error state for diagnostics.
func (m *ServiceTokenManager) Stats() map[string]interface{} {
	clientStats := m.client.Stats()

	stats := map[string]interface{}{
		"refresh_success_count": clientStats.RefreshSuccesses,
		"refresh_failure_count": clientStats.RefreshFailures,
		"last_error":            clientStats.LastError,
		"last_refresh_at":       "",
		"last_success_at":       "",
	}
	if !clientStats.LastRefresh.IsZero() {
		stats["last_refresh_at"] = clientStats.LastRefresh.UTC().Format(time.RFC3339)
	}
	if !clientStats.LastSuccess.IsZero() {
		stats["last_success_at"] = clientStats.LastSuccess.UTC().Format(time.RFC3339)
	}

	return stats
}

// GetServiceToken exposes the current shared client token for callers that want direct access.
func (m *ServiceTokenManager) GetServiceToken() *tokenservice.ServiceToken {
	return m.client.GetServiceToken()
}

// RefreshTokenIfNeeded refreshes the shared service token if it is missing or near expiry.
func (m *ServiceTokenManager) RefreshTokenIfNeeded(ctx context.Context) error {
	return m.client.RefreshTokenIfNeeded(ctx)
}

func (m *ServiceTokenManager) serviceTokenEndpoint() string {
	return strings.TrimRight(m.config.TokenSmithURL, "/") + "/oauth/token"
}

func sortedScopes(scopes []string) []string {
	filtered := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			filtered = append(filtered, scope)
		}
	}
	sort.Strings(filtered)
	return filtered
}
