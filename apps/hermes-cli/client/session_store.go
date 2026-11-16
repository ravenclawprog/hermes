package client

import (
	"context"
)

type SessionStore interface {
	Save(ctx context.Context, session Session) error
}
