package client

import (
	"context"
	"time"

	"hermes-messages"
)

type Session struct {
	UserID    string
	Token     string
	ExpiresAt time.Time
}

type DocumentSummary = messages.DocumentSummary

type Document struct {
	Summary DocumentSummary `json:"summary"`
	Content string          `json:"content"`
}

type DocumentPage = messages.DocumentPage

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
	Read(
		ctx context.Context,
		session Session,
		documentID string,
	) (Document, error)
}
