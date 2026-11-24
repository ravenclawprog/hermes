// Package messages defines the HTTP contract shared by hermes-cli and hermes-ds.
package messages

import "time"

const SchemaVersion = 1

const (
	Authenticate  = "Authenticate"
	ListDocuments = "ListDocuments"
	GetDocument   = "GetDocument"

	LoginPath         = "/auth/login"
	ListDocumentsPath = "/document/list"
	GetDocumentPath   = "/document/read"
)

const (
	StatusSuccess = "success"
	StatusError   = "error"
)

type Envelope struct {
	SchemaVersion int    `json:"schema_version"`
	RequestID     string `json:"request_id"`
	Operation     string `json:"operation"`
}

type Request[T any] struct {
	Envelope
	Payload T `json:"payload"`
}

type Response[T any] struct {
	Envelope
	Status  string `json:"status"`
	Payload *T     `json:"payload,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AuthenticateInput struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type AuthenticateResult struct {
	UserID    string    `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type ListDocumentsInput struct {
	WorkspaceID *string    `json:"workspace_id,omitempty"`
	Page        *PageInput `json:"page"`
}

type PageInput struct {
	Cursor *string `json:"cursor,omitempty"`
	Limit  *int    `json:"limit,omitempty"`
}

type ListDocumentsResult struct {
	Page *DocumentPage `json:"page"`
}

type DocumentPage struct {
	Items      []DocumentSummary `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type DocumentSummary struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Version     int64  `json:"version"`
}

type Document struct {
	DocumentSummary
	Content string `json:"content"`
}

type GetDocumentInput struct {
	DocumentID string `json:"document_id"`
}

type GetDocumentResult struct {
	Document *Document `json:"document"`
}
