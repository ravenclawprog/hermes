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

type AuthClient interface {
	Authenticate(
		ctx context.Context,
		username string,
		password []byte,
	) (Session, error)
}
