// SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors
//
// SPDX-License-Identifier: MIT

package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rs/zerolog"
)

// TestNewClient_Success verifies client creation with all fields set correctly
func TestNewClient_Success(t *testing.T) {
	logger := DefaultLogger()
	httpClient := &http.Client{}
	baseURL := "http://localhost:8080"

	client, err := NewClient(baseURL, httpClient, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClient() returned nil client")
	}

	if client.baseURL == nil {
		t.Error("baseURL is nil")
	}

	if client.baseURL.String() != baseURL {
		t.Errorf("baseURL = %s, want %s", client.baseURL.String(), baseURL)
	}

	if client.httpClient != httpClient {
		t.Error("httpClient not set correctly")
	}

	// Logger is private field, but we can test it indirectly via behavior
}

// TestNewClient_InvalidURL verifies error handling for invalid URLs
func TestNewClient_InvalidURL(t *testing.T) {
	logger := DefaultLogger()
	// Only URLs that url.Parse() actually rejects will cause errors
	invalidURLs := []string{
		"://invalid",
		"ht!tp://invalid",
	}

	for _, url := range invalidURLs {
		client, err := NewClient(url, nil, logger)
		if err == nil {
			t.Errorf("NewClient(%q) expected error, got nil", url)
		}
		if client != nil {
			t.Errorf("NewClient(%q) expected nil client on error, got %v", url, client)
		}
		if err != nil && !strings.Contains(err.Error(), "invalid base URL") {
			t.Errorf("NewClient(%q) error = %v, want error containing 'invalid base URL'", url, err)
		}
	}
}

// TestNewClient_WithNilHTTPClient verifies default client is used when nil
func TestNewClient_WithNilHTTPClient(t *testing.T) {
	logger := DefaultLogger()
	baseURL := "http://localhost:8080"

	client, err := NewClient(baseURL, nil, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client.httpClient != http.DefaultClient {
		t.Error("httpClient should default to http.DefaultClient when nil")
	}
}

// TestNewClient_WithCustomHTTPClient verifies custom httpClient is preserved
func TestNewClient_WithCustomHTTPClient(t *testing.T) {
	logger := DefaultLogger()
	customClient := &http.Client{
		Transport: &customTransport{},
	}
	baseURL := "http://localhost:8080"

	client, err := NewClient(baseURL, customClient, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	if client.httpClient != customClient {
		t.Error("custom httpClient not preserved")
	}
}

// TestNewClientWithBearerToken_Success verifies bearer token client creation
func TestNewClientWithBearerToken_Success(t *testing.T) {
	logger := DefaultLogger()
	baseURL := "http://localhost:8080"
	token := "test-token-123"

	client, err := NewClientWithBearerToken(baseURL, token, nil, logger)
	if err != nil {
		t.Fatalf("NewClientWithBearerToken() failed: %v", err)
	}

	if client == nil {
		t.Fatal("NewClientWithBearerToken() returned nil client")
	}

	if client.bearerToken != token {
		t.Errorf("bearerToken = %s, want %s", client.bearerToken, token)
	}

	if client.baseURL.String() != baseURL {
		t.Errorf("baseURL = %s, want %s", client.baseURL.String(), baseURL)
	}
}

// TestWithVersion_PreservesLogger is CRITICAL - catches the fabrica bug!
// This test will FAIL until fabrica is fixed to propagate logger field.
func TestWithVersion_PreservesLogger(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	baseURL := "http://localhost:8080"
	client, err := NewClient(baseURL, nil, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Call WithVersion - this should preserve the logger
	versionedClient := client.WithVersion("v1")

	if versionedClient == nil {
		t.Fatal("WithVersion() returned nil client")
	}

	// Create a test server to trigger logger usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Ignoring error since this is a test helper. Encoding errors will cause test to fail via incorrect response.
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}))
	defer server.Close()

	// Update client to use test server
	versionedClient.baseURL, _ = versionedClient.baseURL.Parse(server.URL)

	// Make a request - this will call logger.Debug() if logger is present
	ctx := context.Background()
	err = versionedClient.doRequest(ctx, "GET", "/test", nil, nil)

	// If logger was preserved, we should see debug output
	// If logger was NOT preserved (bug), this might panic or produce no output
	if err != nil {
		t.Logf("Request error (expected for test): %v", err)
	}

	// Note: This test documents the expected behavior.
	// If logger is not preserved, the test may panic or fail differently.
	t.Logf("Logger output length: %d bytes", buf.Len())

	if buf.Len() == 0 {
		t.Error("Logger was not preserved - no debug output captured")
	}
}

// TestWithBearerToken_PreservesLogger is CRITICAL - catches the fabrica bug!
// This test will FAIL until fabrica is fixed to propagate logger field.
func TestWithBearerToken_PreservesLogger(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	baseURL := "http://localhost:8080"
	client, err := NewClient(baseURL, nil, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Call WithBearerToken - this should preserve the logger
	tokenClient := client.WithBearerToken("test-token")

	if tokenClient == nil {
		t.Fatal("WithBearerToken() returned nil client")
	}

	// Create a test server to trigger logger usage
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Ignoring error since this is a test helper. Encoding errors will cause test to fail via incorrect response.
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}))
	defer server.Close()

	// Update client to use test server
	tokenClient.baseURL, _ = tokenClient.baseURL.Parse(server.URL)

	// Make a request - this will call logger.Debug() if logger is present
	ctx := context.Background()
	err = tokenClient.doRequest(ctx, "GET", "/test", nil, nil)

	if err != nil {
		t.Logf("Request error (expected for test): %v", err)
	}

	t.Logf("Logger output length: %d bytes", buf.Len())

	if buf.Len() == 0 {
		t.Error("Logger was not preserved - no debug output captured")
	}
}

// TestWithVersion_PreservesAllFields verifies all fields are copied
func TestWithVersion_PreservesAllFields(t *testing.T) {
	logger := DefaultLogger()
	customClient := &http.Client{}
	baseURL := "http://localhost:8080"
	token := "original-token"

	// Create client with all fields set
	client, err := NewClient(baseURL, customClient, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client = client.WithBearerToken(token)

	// Call WithVersion
	version := "v2"
	versionedClient := client.WithVersion(version)

	if versionedClient == nil {
		t.Fatal("WithVersion() returned nil")
	}

	// Verify all fields preserved
	if versionedClient.baseURL.String() != baseURL {
		t.Errorf("baseURL not preserved: got %s, want %s", versionedClient.baseURL.String(), baseURL)
	}

	if versionedClient.httpClient != customClient {
		t.Error("httpClient not preserved")
	}

	if versionedClient.version != version {
		t.Errorf("version = %s, want %s", versionedClient.version, version)
	}

	if versionedClient.bearerToken != token {
		t.Errorf("bearerToken not preserved: got %s, want %s", versionedClient.bearerToken, token)
	}

	// Note: Can't directly test logger field (private), but other tests cover it
}

// TestWithBearerToken_PreservesAllFields verifies all fields are copied
func TestWithBearerToken_PreservesAllFields(t *testing.T) {
	logger := DefaultLogger()
	customClient := &http.Client{}
	baseURL := "http://localhost:8080"
	version := "v1"

	// Create client with all fields set
	client, err := NewClient(baseURL, customClient, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}
	client = client.WithVersion(version)

	// Call WithBearerToken
	token := "new-token"
	tokenClient := client.WithBearerToken(token)

	if tokenClient == nil {
		t.Fatal("WithBearerToken() returned nil")
	}

	// Verify all fields preserved
	if tokenClient.baseURL.String() != baseURL {
		t.Errorf("baseURL not preserved: got %s, want %s", tokenClient.baseURL.String(), baseURL)
	}

	if tokenClient.httpClient != customClient {
		t.Error("httpClient not preserved")
	}

	if tokenClient.version != version {
		t.Errorf("version not preserved: got %s, want %s", tokenClient.version, version)
	}

	if tokenClient.bearerToken != token {
		t.Errorf("bearerToken = %s, want %s", tokenClient.bearerToken, token)
	}
}

// TestChainedWithMethods verifies chaining preserves all fields
func TestChainedWithMethods(t *testing.T) {
	logger := DefaultLogger()
	baseURL := "http://localhost:8080"

	client, err := NewClient(baseURL, nil, logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Chain multiple With* methods
	version := "v2"
	token := "chained-token"
	finalClient := client.WithVersion(version).WithBearerToken(token)

	if finalClient == nil {
		t.Fatal("Chained methods returned nil")
	}

	if finalClient.version != version {
		t.Errorf("version = %s, want %s", finalClient.version, version)
	}

	if finalClient.bearerToken != token {
		t.Errorf("bearerToken = %s, want %s", finalClient.bearerToken, token)
	}

	if finalClient.baseURL.String() != baseURL {
		t.Errorf("baseURL = %s, want %s", finalClient.baseURL.String(), baseURL)
	}
}

// TestDefaultLogger verifies default logger creation
func TestDefaultLogger(t *testing.T) {
	logger := DefaultLogger()

	// Verify logger is not nil (zerolog doesn't have nil, but we can test it works)
	// Write a test log message to verify it doesn't panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DefaultLogger() panic: %v", r)
		}
	}()

	logger.Warn().Msg("test message")
	logger.Debug().Msg("debug message")
}

// TestNewLogger_ValidLevels tests all valid log levels
func TestNewLogger_ValidLevels(t *testing.T) {
	tests := []struct {
		level LogLevel
		name  string
	}{
		{LogLevelInfo, "info"},
		{LogLevelWarning, "warning"},
		{LogLevelDebug, "debug"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.level)
			if err != nil {
				t.Errorf("NewLogger(%s) unexpected error: %v", tt.level, err)
			}

			// Verify logger works
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Logger panic: %v", r)
				}
			}()

			logger.Info().Msg("test")
		})
	}
}

// TestNewLogger_InvalidLevel verifies error handling for invalid log level
func TestNewLogger_InvalidLevel(t *testing.T) {
	invalidLevel := LogLevel("invalid")
	logger, err := NewLogger(invalidLevel)

	if err == nil {
		t.Error("NewLogger(invalid) expected error, got nil")
	}

	if err != nil && !strings.Contains(err.Error(), "invalid log level") {
		t.Errorf("Error message = %v, want to contain 'invalid log level'", err)
	}

	// Should return default logger even on error
	if logger.GetLevel() != zerolog.WarnLevel {
		t.Error("Expected default logger on error")
	}
}

// TestLogLevel_String verifies String() method
func TestLogLevel_String(t *testing.T) {
	tests := []struct {
		level LogLevel
		want  string
	}{
		{LogLevelInfo, "info"},
		{LogLevelWarning, "warning"},
		{LogLevelDebug, "debug"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("LogLevel.String() = %s, want %s", got, tt.want)
		}
	}
}

// TestLogLevel_Set verifies Set() method with valid and invalid values
func TestLogLevel_Set(t *testing.T) {
	// Test valid values
	validTests := []struct {
		input string
		want  LogLevel
	}{
		{"info", LogLevelInfo},
		{"warning", LogLevelWarning},
		{"debug", LogLevelDebug},
	}

	for _, tt := range validTests {
		var ll LogLevel
		err := ll.Set(tt.input)
		if err != nil {
			t.Errorf("LogLevel.Set(%s) unexpected error: %v", tt.input, err)
		}
		if ll != tt.want {
			t.Errorf("LogLevel.Set(%s) = %s, want %s", tt.input, ll, tt.want)
		}
	}

	// Test invalid value
	var ll LogLevel
	err := ll.Set("invalid")
	if err == nil {
		t.Error("LogLevel.Set(invalid) expected error, got nil")
	}
	if err != nil && !strings.Contains(err.Error(), "must be one of") {
		t.Errorf("Error message = %v, want to contain 'must be one of'", err)
	}
}

// TestLogLevel_Type verifies Type() method
func TestLogLevel_Type(t *testing.T) {
	var ll LogLevel
	if got := ll.Type(); got != "LogLevel" {
		t.Errorf("LogLevel.Type() = %s, want LogLevel", got)
	}
}

// TestClient_LoggerCalledDuringRequest verifies logger is used during requests
func TestClient_LoggerCalledDuringRequest(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Ignoring error since this is a test helper. Encoding errors will cause test to fail via incorrect response.
		json.NewEncoder(w).Encode(map[string]string{"message": "success"}) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	client, err := NewClient(server.URL, server.Client(), logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Make a request
	ctx := context.Background()
	var result map[string]string
	err = client.doRequest(ctx, "GET", "/test", nil, &result)

	if err != nil {
		t.Fatalf("doRequest() failed: %v", err)
	}

	// Verify logger was called (debug output captured)
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected debug output from logger, got none")
	}

	// Verify output contains expected debug information
	if !strings.Contains(output, "GET") {
		t.Error("Logger output should contain HTTP method")
	}

	if !strings.Contains(output, "/test") {
		t.Error("Logger output should contain endpoint path")
	}

	t.Logf("Logger output:\n%s", output)
}

// TestClient_LoggerCalledOnError verifies logger error handling
func TestClient_LoggerCalledOnError(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	// Create test server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		// Ignoring error since this is a test helper. Encoding errors will cause test to fail via incorrect response.
		json.NewEncoder(w).Encode(map[string]string{"error": "internal error"}) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	client, err := NewClient(server.URL, server.Client(), logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Make a request that will fail
	ctx := context.Background()
	err = client.doRequest(ctx, "GET", "/error", nil, nil)

	if err == nil {
		t.Error("Expected error from failed request")
	}

	// Verify logger captured debug info even for error response
	output := buf.String()
	if len(output) == 0 {
		t.Error("Expected debug output from logger even on error")
	}

	t.Logf("Logger output on error:\n%s", output)
}

// TestClient_RequestWithBody verifies logger handles request body correctly
func TestClient_RequestWithBody(t *testing.T) {
	// Create a buffer to capture logger output
	var buf bytes.Buffer
	logger := zerolog.New(&buf).Level(zerolog.DebugLevel)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Read and echo back the request body
		body, _ := io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Ignoring error since this is a test helper. Write errors will cause test to fail via incorrect response.
		w.Write(body) //nolint:errcheck
	}))
	defer server.Close()

	// Create client
	client, err := NewClient(server.URL, server.Client(), logger)
	if err != nil {
		t.Fatalf("NewClient() failed: %v", err)
	}

	// Make a request with body
	ctx := context.Background()
	requestBody := map[string]string{"key": "value"}
	var result map[string]string
	err = client.doRequest(ctx, "POST", "/test", requestBody, &result)

	if err != nil {
		t.Fatalf("doRequest() failed: %v", err)
	}

	// Verify logger captured request body
	output := buf.String()
	if !strings.Contains(output, "Request body") {
		t.Error("Logger should log request body")
	}

	if !strings.Contains(output, "key") {
		t.Error("Logger should include request body content")
	}

	t.Logf("Logger output with body:\n%s", output)
}

// customTransport is a test helper for custom HTTP client
type customTransport struct{}

func (t *customTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return http.DefaultTransport.RoundTrip(req)
}
