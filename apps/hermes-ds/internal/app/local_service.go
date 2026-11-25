package app

import (
	"context"
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

// LocalService manages the local account and sessions and provides an in-memory document.
type LocalService struct {
	salt         []byte
	passwordHash []byte
	document     messages.Document

	mu       sync.Mutex
	sessions map[[sha256.Size]byte]time.Time
}

func NewLocalService(password string) (*LocalService, error) {
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
	return &LocalService{
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

func (s *LocalService) Authenticate(username, password string) (messages.AuthenticateResult, error) {
	hash, err := pbkdf2.Key(sha256.New, password, s.salt, passwordIterations, sha256.Size)
	if err != nil {
		return messages.AuthenticateResult{}, err
	}
	if subtle.ConstantTimeCompare(hash, s.passwordHash) != 1 || !strings.EqualFold(username, "demo") {
		return messages.AuthenticateResult{}, ErrUnauthenticated
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return messages.AuthenticateResult{}, err
	}
	token := hex.EncodeToString(rawToken)

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	for key, expiry := range s.sessions {
		if !now.Before(expiry) {
			delete(s.sessions, key)
		}
	}
	expiresAt := now.Add(sessionTTL)
	s.sessions[sha256.Sum256([]byte(token))] = expiresAt
	return messages.AuthenticateResult{UserID: demoUserID, Token: token, ExpiresAt: expiresAt}, nil
}

func (s *LocalService) ValidSession(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := sha256.Sum256([]byte(token))
	expiry, exists := s.sessions[key]
	if exists && time.Now().Before(expiry) {
		return true
	}
	delete(s.sessions, key)
	return false
}

func (s *LocalService) ListDocuments(_ context.Context, workspaceID, after string, _ int) ([]messages.DocumentSummary, bool, error) {
	if workspaceID != "" && workspaceID != s.document.WorkspaceID {
		return nil, false, nil
	}
	if after != "" {
		return []messages.DocumentSummary{}, true, nil
	}
	return []messages.DocumentSummary{s.document.DocumentSummary}, true, nil
}

func (s *LocalService) GetDocument(_ context.Context, id string) (messages.Document, bool, error) {
	if id != s.document.ID {
		return messages.Document{}, false, nil
	}
	return s.document, true, nil
}
