package client

import "context"

type SessionStore interface {
	Save(ctx context.Context, session Session) error
	Load(ctx context.Context) (Session, error)
}
