package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
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

func TestListDocuments(t *testing.T) {
	document := DocumentSummary{ID: "doc-42", WorkspaceID: "ws-9", Title: "План проекта", Version: 7}
	tests := []struct {
		name       string
		httpStatus int
		response   string
		want       DocumentPage
		wantCode   string
		wantError  string
	}{
		{
			name:       "first page with next cursor",
			httpStatus: http.StatusOK,
			response: `"request_id": %q, "operation": "ListDocuments", "status": "success",
				"payload": {"page": {"items": [{"id": "doc-42", "workspace_id": "ws-9", "title": "План проекта", "version": 7}], "next_cursor": "next-page"}}`,
			want: DocumentPage{Items: []DocumentSummary{document}, NextCursor: "next-page"},
		},
		{
			name:       "empty page",
			httpStatus: http.StatusOK,
			response:   `"request_id": %q, "operation": "ListDocuments", "status": "success", "payload": {"page": {"items": []}}`,
			want:       DocumentPage{Items: []DocumentSummary{}},
		},
		{
			name:       "session rejected",
			httpStatus: http.StatusUnauthorized,
			response:   `"request_id": %q, "operation": "ListDocuments", "status": "error", "error": {"code": "unauthenticated"}`,
			wantCode:   "unauthenticated",
		},
		{
			name:       "unexpected operation",
			httpStatus: http.StatusOK,
			response:   `"request_id": %q, "operation": "GetDocument", "status": "success", "payload": {"page": {"items": []}}`,
			wantError:  "invalid document list response envelope",
		},
		{
			name:       "missing items",
			httpStatus: http.StatusOK,
			response:   `"request_id": %q, "operation": "ListDocuments", "status": "success", "payload": {"page": {}}`,
			wantError:  "invalid document list success response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/document/list" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" ||
					r.Header.Get("Content-Type") != "application/json" {
					t.Error("unexpected authorization or content type")
				}
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requestID, ok := request["request_id"].(string)
				if !ok || requestID == "" {
					t.Error("expected a request ID")
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				wantRequest := map[string]any{
					"schema_version": float64(1),
					"request_id":     requestID,
					"operation":      "ListDocuments",
					"payload":        map[string]any{"page": map[string]any{}},
				}
				if !reflect.DeepEqual(request, wantRequest) {
					t.Error("unexpected document list request body")
				}

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.httpStatus)
				if _, err := fmt.Fprintf(w, `{"schema_version":1,`+tt.response+`}`, requestID); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			page, err := NewHTTPClient(server.URL).List(context.Background(), Session{UserID: "user-1", Token: "test-token"})
			switch {
			case tt.wantCode != "":
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Code != tt.wantCode {
					t.Fatalf("expected API error %q, got %v", tt.wantCode, err)
				}
			case tt.wantError != "":
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("expected error %q, got %v", tt.wantError, err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(page, tt.want) {
					t.Errorf("got page %+v, want %+v", page, tt.want)
				}
			}
		})
	}
}

func TestReadDocument(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
	}{
		{"document", `{"document":{"id":"doc-42","workspace_id":"ws-9","title":"План","version":7,"content":"# План\n"}}`},
		{"missing document", `{}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.URL.Path != "/document/read" {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
				}
				if r.Header.Get("Authorization") != "Bearer test-token" || r.Header.Get("Content-Type") != "application/json" {
					t.Error("unexpected authorization or content type")
				}
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Errorf("decode request: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requestID, _ := request["request_id"].(string)
				wantRequest := map[string]any{
					"schema_version": float64(1),
					"request_id":     requestID,
					"operation":      "GetDocument",
					"payload":        map[string]any{"document_id": "doc-42"},
				}
				if requestID == "" || !reflect.DeepEqual(request, wantRequest) {
					t.Errorf("unexpected document read request: %#v", request)
				}
				if _, err := fmt.Fprintf(w, `{"schema_version":1,"request_id":%q,"operation":"GetDocument","status":"success","payload":%s}`, requestID, tc.payload); err != nil {
					t.Errorf("write response: %v", err)
				}
			}))
			defer server.Close()

			document, err := NewHTTPClient(server.URL).Read(context.Background(), Session{Token: "test-token"}, "doc-42")
			if tc.name == "missing document" {
				if err == nil || err.Error() != "invalid document read success response" {
					t.Fatalf("expected invalid success response error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			want := Document{
				Summary: DocumentSummary{ID: "doc-42", WorkspaceID: "ws-9", Title: "План", Version: 7},
				Content: "# План\n",
			}
			if document != want {
				t.Errorf("got document %+v, want %+v", document, want)
			}
		})
	}
}
