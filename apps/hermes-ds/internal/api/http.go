package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"hermes-ds/internal/app"
	"hermes-messages"
)

type request = messages.Request[json.RawMessage]

type documents interface {
	ListDocuments(ctx context.Context, workspaceID, after string, limit int) ([]messages.DocumentSummary, bool, error)
	GetDocument(ctx context.Context, id string) (messages.Document, bool, error)
}

func New(service *app.LocalService, store documents) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(http.MethodPost+" "+messages.LoginPath, endpoint(messages.Authenticate, nil, func(w http.ResponseWriter, r *http.Request, req request) {
		var input messages.AuthenticateInput
		if json.Unmarshal(req.Payload, &input) != nil || strings.TrimSpace(input.Username) == "" || input.Password == "" {
			fail(w, req, http.StatusBadRequest, "validation", "username and password are required")
			return
		}
		session, err := service.Authenticate(input.Username, input.Password)
		if errors.Is(err, app.ErrUnauthenticated) {
			fail(w, req, http.StatusUnauthorized, "unauthenticated", "invalid credentials")
			return
		}
		if err != nil {
			fail(w, req, http.StatusServiceUnavailable, "unavailable", "could not create session")
			return
		}
		succeed(w, req, session)
	}))
	mux.HandleFunc(http.MethodPost+" "+messages.ListDocumentsPath, endpoint(messages.ListDocuments, service.ValidSession, func(w http.ResponseWriter, r *http.Request, req request) {
		var input messages.ListDocumentsInput
		if json.Unmarshal(req.Payload, &input) != nil || input.Page == nil {
			fail(w, req, http.StatusBadRequest, "validation", "page is required")
			return
		}
		if input.Page.Limit != nil && *input.Page.Limit < 1 {
			fail(w, req, http.StatusBadRequest, "validation", "invalid page")
			return
		}
		var workspaceID string
		if input.WorkspaceID != nil {
			workspaceID = *input.WorkspaceID
			if strings.TrimSpace(workspaceID) == "" {
				fail(w, req, http.StatusBadRequest, "validation", "workspace_id must be non-empty")
				return
			}
		}
		limit := 100
		if input.Page.Limit != nil {
			limit = min(*input.Page.Limit, limit)
		}
		var after string
		if input.Page.Cursor != nil {
			raw, err := base64.RawURLEncoding.DecodeString(*input.Page.Cursor)
			if err != nil || len(raw) == 0 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
				fail(w, req, http.StatusBadRequest, "validation", "invalid cursor")
				return
			}
			after = string(raw)
		}
		items, found, err := store.ListDocuments(r.Context(), workspaceID, after, limit+1)
		if err != nil {
			fail(w, req, http.StatusServiceUnavailable, "unavailable", "could not list documents")
			return
		}
		if !found {
			fail(w, req, http.StatusNotFound, "not_found", "workspace not found")
			return
		}
		page := &messages.DocumentPage{Items: items}
		if len(items) > limit {
			page.Items = items[:limit]
			page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(items[limit-1].ID))
		}
		succeed(w, req, messages.ListDocumentsResult{Page: page})
	}))
	mux.HandleFunc(http.MethodPost+" "+messages.GetDocumentPath, endpoint(messages.GetDocument, service.ValidSession, func(w http.ResponseWriter, r *http.Request, req request) {
		var input messages.GetDocumentInput
		if json.Unmarshal(req.Payload, &input) != nil || strings.TrimSpace(input.DocumentID) == "" {
			fail(w, req, http.StatusBadRequest, "validation", "document_id is required")
			return
		}
		document, found, err := store.GetDocument(r.Context(), input.DocumentID)
		if err != nil {
			fail(w, req, http.StatusServiceUnavailable, "unavailable", "could not read document")
			return
		}
		if !found {
			fail(w, req, http.StatusNotFound, "not_found", "document not found")
			return
		}
		succeed(w, req, messages.GetDocumentResult{Document: &document})
	}))
	return mux
}

func endpoint(operation string, validSession func(string) bool, handle func(http.ResponseWriter, *http.Request, request)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req request
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
		if decoder.Decode(&req) != nil || decoder.Decode(new(any)) != io.EOF ||
			req.SchemaVersion != messages.SchemaVersion || strings.TrimSpace(req.RequestID) == "" ||
			req.Operation != operation || !bytes.HasPrefix(req.Payload, []byte("{")) {
			fail(w, req, http.StatusBadRequest, "validation", "invalid request envelope")
			return
		}
		if validSession != nil {
			headers := r.Header.Values("Authorization")
			if len(headers) != 1 || !authorized(headers[0], validSession) {
				fail(w, req, http.StatusUnauthorized, "unauthenticated", "valid bearer token required")
				return
			}
		}
		handle(w, r, req)
	}
}

func authorized(header string, validSession func(string) bool) bool {
	scheme, token, found := strings.Cut(header, " ")
	return found && strings.EqualFold(scheme, "Bearer") && token != "" && validSession(token)
}

func fail(w http.ResponseWriter, req request, status int, code, message string) {
	req.SchemaVersion = messages.SchemaVersion
	if status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	write(w, status, messages.Response[struct{}]{
		Envelope: req.Envelope,
		Status:   messages.StatusError,
		Error:    &messages.Error{Code: code, Message: message},
	})
}

func succeed[T any](w http.ResponseWriter, req request, payload T) {
	write(w, http.StatusOK, messages.Response[T]{
		Envelope: req.Envelope,
		Status:   messages.StatusSuccess,
		Payload:  &payload,
	})
}

func write[T any](w http.ResponseWriter, status int, result messages.Response[T]) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(result)
}
