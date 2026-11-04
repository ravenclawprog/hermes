package client

import "time"

// --- gRPC / HTTP DTO ---

type DocumentResponse struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Version   int64     `json:"version"` // for optimistic block
	UpdatedAt time.Time `json:"updated_at"`
}

type UpdateDocumentRequest struct {
	ID             string `json:"id"`
	Content        string `json:"content"`
	CurrentVersion int64  `json:"current_version"`
}

// --- WebSocket Messages ---

type EventType string

const (
	EventDocUpdated    EventType = "DOC_UPDATED"
	EventDocCommented  EventType = "DOC_COMMENTED"
	EventUserMentioned EventType = "USER_MENTIONED"
)

// WSFrame — base message  that store another
type WSFrame struct {
	Type      EventType   `json:"type"`      // Event type (DOC_UPDATED, etc.)
	Timestamp time.Time   `json:"timestamp"` // Generation time
	Payload   interface{} `json:"payload"`   // Message body
}

type DocUpdatedPayload struct {
	DocumentID string `json:"document_id"`
	Actor      string `json:"actor"`   // Updated (username)
	Version    int64  `json:"version"` // New version of doc
	Summary    string `json:"summary"` // overview of changes
}

type UserMentionedPayload struct {
	DocumentID     string `json:"document_id"`
	Author         string `json:"author"`          // who mentioned
	CommentSnippet string `json:"comment_snippet"` // commentary part
}
