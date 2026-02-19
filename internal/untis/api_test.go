package untis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{
			name:    "valid URL",
			baseURL: "https://test.webuntis.com",
			wantErr: false,
		},
		{
			name:    "empty URL",
			baseURL: "",
			wantErr: true,
		},
		{
			name:    "URL with trailing slash",
			baseURL: "https://test.webuntis.com/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewClient("school", "user", "pass", tt.baseURL)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && client == nil {
				t.Error("NewClient() returned nil client")
			}
		})
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	client, err := NewClient(
		"school",
		"user",
		"pass",
		"https://test.webuntis.com",
		WithMaxRetries(5),
		WithRetryDelay(2*time.Second),
		WithRequestTimeout(60*time.Second),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if client.maxRetries != 5 {
		t.Errorf("maxRetries = %d, want 5", client.maxRetries)
	}
	if client.retryDelay != 2*time.Second {
		t.Errorf("retryDelay = %v, want 2s", client.retryDelay)
	}
	if client.requestTimeout != 60*time.Second {
		t.Errorf("requestTimeout = %v, want 60s", client.requestTimeout)
	}
}

func TestClient_doREST_RetryLimit(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient(
		"school",
		"user",
		"pass",
		server.URL,
		WithMaxRetries(2),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.token = "test-token"
	client.claims = &JWTClaims{TenantID: "test"}

	var result interface{}
	err = client.doREST(context.Background(), "GET", "/test", nil, &result, 0)

	if err == nil {
		t.Error("doREST() expected error, got nil")
	}

	if callCount > client.maxRetries+1 {
		t.Errorf("doREST() called %d times, expected at most %d", callCount, client.maxRetries+1)
	}
}

func TestClient_doREST_ServerError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(
		"school",
		"user",
		"pass",
		server.URL,
		WithMaxRetries(2),
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.token = "test-token"
	client.claims = &JWTClaims{TenantID: "test", Exp: time.Now().Add(1 * time.Hour).Unix()}

	var result interface{}
	err = client.doREST(context.Background(), "GET", "/test", nil, &result, 0)

	if err == nil {
		t.Error("doREST() expected error, got nil")
	}
}

func TestClient_doREST_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"test": "value"}`))
	}))
	defer server.Close()

	client, err := NewClient("school", "user", "pass", server.URL)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	client.token = "test-token"
	client.claims = &JWTClaims{TenantID: "test", Exp: time.Now().Add(1 * time.Hour).Unix()}

	var result map[string]string
	err = client.doREST(context.Background(), "GET", "/test", nil, &result, 0)

	if err != nil {
		t.Errorf("doREST() error = %v", err)
	}

	if result["test"] != "value" {
		t.Errorf("result = %v, want {test: value}", result)
	}
}

func TestClient_EnsureToken(t *testing.T) {
	client, err := NewClient("school", "user", "pass", "https://test.webuntis.com")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	if err := client.EnsureToken(context.Background()); err == nil {
		t.Error("EnsureToken() should return error when no token is set and authentication fails")
	}

	client.token = "valid-token"
	client.claims = &JWTClaims{Exp: time.Now().Add(1 * time.Hour).Unix()}

	if err := client.EnsureToken(context.Background()); err != nil {
		t.Errorf("EnsureToken() error = %v for valid token", err)
	}
}

func TestParseJWTClaims(t *testing.T) {
	tests := []struct {
		name    string
		token   string
		wantErr bool
	}{
		{
			name:    "invalid token format",
			token:   "invalid",
			wantErr: true,
		},
		{
			name:    "empty token",
			token:   "",
			wantErr: true,
		},
		{
			name:    "valid JWT format",
			token:   "header.eyJ0ZW5hbnRfaWQiOiIxMjMifQ.signature",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseJWTClaims(tt.token)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseJWTClaims() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "timeout error",
			err:      context.DeadlineExceeded,
			expected: false,
		},
		{
			name:     "connection refused",
			err:      &testError{msg: "connection refused"},
			expected: true,
		},
		{
			name:     "temporary error",
			err:      &testError{msg: "temporary failure"},
			expected: true,
		},
		{
			name:     "EOF error",
			err:      &testError{msg: "unexpected EOF"},
			expected: true,
		},
		{
			name:     "other error",
			err:      &testError{msg: "some other error"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.expected {
				t.Errorf("isRetryableError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
