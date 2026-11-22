package client

import (
	"context"
	"time"
)

type Session struct {
	UserID    string
	Token     string
	ExpiresAt time.Time
}

type DocumentSummary struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspace_id"`
	Title       string `json:"title"`
	Version     int64  `json:"version"`
}

type DocumentPage struct {
	Items      []DocumentSummary `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}

type AuthClient interface {
	Authenticate(
		ctx context.Context,
		username string,
		password []byte,
	) (Session, error)
}

type DocumentClient interface {
	List(
		ctx context.Context,
		session Session,
	) (DocumentPage, error)
}
