package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAuthenticateSuccess(t *testing.T) {
	expiresAt := time.Now().UTC().Add(time.Hour).Truncate(time.Second)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/auth/login" {
			t.Error("unexpected method or path")
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}

		var request authenticateRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error("invalid request JSON")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if request.Payload.Username != "alice" ||
			request.Payload.Password != "test-password" {
			t.Error("unexpected credentials")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		err := json.NewEncoder(w).Encode(authenticateResponse{
			SchemaVersion: 1,
			RequestID:     request.RequestID,
			Operation:     request.Operation,
			Status:        "success",
			Payload: &authenticateResult{
				UserID:    "user-1",
				Token:     "test-token",
				ExpiresAt: expiresAt,
			},
		})
		if err != nil {
			t.Error("failed to encode response")
		}
	}))
	defer server.Close()

	authClient := NewHTTPClient(server.URL)
	session, err := authClient.Authenticate(
		context.Background(),
		"alice",
		[]byte("test-password"),
	)
	if err != nil {
		t.Fatalf("Authenticate returned an error: %v", err)
	}

	if session.UserID != "user-1" ||
		session.Token != "test-token" ||
		!session.ExpiresAt.Equal(expiresAt) {
		t.Error("unexpected session")
	}
}

func TestAuthenticateInvalidCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request authenticateRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error("invalid request JSON")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)

		err := json.NewEncoder(w).Encode(authenticateResponse{
			SchemaVersion: 1,
			RequestID:     request.RequestID,
			Operation:     request.Operation,
			Status:        "error",
			Error: &responseError{
				Code:    "unauthenticated",
				Message: "Invalid username or password",
			},
		})
		if err != nil {
			t.Error("failed to encode response")
		}
	}))
	defer server.Close()

	authClient := NewHTTPClient(server.URL)
	session, err := authClient.Authenticate(
		context.Background(),
		"alice",
		[]byte("wrong-password"),
	)

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got: %v", err)
	}
	if apiErr.Code != "unauthenticated" {
		t.Errorf("unexpected error code: %s", apiErr.Code)
	}
	if session != (Session{}) {
		t.Error("expected an empty session")
	}
}
