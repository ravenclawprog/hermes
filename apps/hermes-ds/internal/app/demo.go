package app

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"

	"hermes-messages"
)

var ErrUnauthenticated = errors.New("authentication required or invalid credentials")

const (
	demoUserID         = "demo-user"
	passwordIterations = 600_000
	sessionTTL         = time.Hour
)

// Demo holds one local account and its personal workspace.
type Demo struct {
	salt         []byte
	passwordHash []byte
	document     messages.Document

	mu       sync.Mutex
	sessions map[[sha256.Size]byte]time.Time
}

func NewDemo(password string) (*Demo, error) {
	if password == "" {
		return nil, errors.New("set HERMES_DS_DEMO_PASSWORD")
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	hash, err := pbkdf2.Key(sha256.New, password, salt, passwordIterations, sha256.Size)
	if err != nil {
		return nil, err
	}
	return &Demo{
		salt: salt, passwordHash: hash,
		sessions: make(map[[sha256.Size]byte]time.Time),
		document: messages.Document{
			DocumentSummary: messages.DocumentSummary{
				ID: "example", WorkspaceID: "demo-workspace", Title: "Welcome to Hermes", Version: 1,
			},
			Content: "# Welcome to Hermes\n\nThis is a sample document.\n",
		},
	}, nil
}

func (d *Demo) Authenticate(username, password string) (messages.AuthenticateResult, error) {
	hash, err := pbkdf2.Key(sha256.New, password, d.salt, passwordIterations, sha256.Size)
	if err != nil {
		return messages.AuthenticateResult{}, err
	}
	if subtle.ConstantTimeCompare(hash, d.passwordHash) != 1 || !strings.EqualFold(username, "demo") {
		return messages.AuthenticateResult{}, ErrUnauthenticated
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return messages.AuthenticateResult{}, err
	}
	token := hex.EncodeToString(rawToken)

	d.mu.Lock()
	defer d.mu.Unlock()
	now := time.Now().UTC()
	for key, expiry := range d.sessions {
		if !now.Before(expiry) {
			delete(d.sessions, key)
		}
	}
	expiresAt := now.Add(sessionTTL)
	d.sessions[sha256.Sum256([]byte(token))] = expiresAt
	return messages.AuthenticateResult{UserID: demoUserID, Token: token, ExpiresAt: expiresAt}, nil
}

func (d *Demo) ValidSession(token string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	key := sha256.Sum256([]byte(token))
	expiry, exists := d.sessions[key]
	if exists && time.Now().Before(expiry) {
		return true
	}
	delete(d.sessions, key)
	return false
}

func (d *Demo) ListDocuments(workspaceID string) ([]messages.DocumentSummary, bool) {
	if workspaceID != "" && workspaceID != d.document.WorkspaceID {
		return nil, false
	}
	return []messages.DocumentSummary{d.document.DocumentSummary}, true
}

func (d *Demo) GetDocument(id string) (messages.Document, bool) {
	if id != d.document.ID {
		return messages.Document{}, false
	}
	return d.document, true
}
