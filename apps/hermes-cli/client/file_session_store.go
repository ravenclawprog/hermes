package client

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileSessionStore struct {
	path string
}

func NewFileSessionStore(path string) *FileSessionStore {
	return &FileSessionStore{
		path: path,
	}
}

func (s *FileSessionStore) Save(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("encode session: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}

	file, err := os.CreateTemp(dir, ".session-*")
	if err != nil {
		return fmt.Errorf("create temporary session file: %w", err)
	}
	defer os.Remove(file.Name())

	if _, err := file.Write(data); err != nil {
		file.Close()
		return fmt.Errorf("write session: %w", err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close session file: %w", err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := os.Rename(file.Name(), s.path); err != nil {
		return fmt.Errorf("replace session file: %w", err)
	}

	return nil
}

func (s *FileSessionStore) Load(ctx context.Context) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return Session{}, fmt.Errorf("read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode session file: %w", err)
	}

	if session.UserID == "" || session.Token == "" || session.ExpiresAt.IsZero() {
		return Session{}, fmt.Errorf("incomplete saved session")
	}
	if !session.ExpiresAt.After(time.Now()) {
		return Session{}, fmt.Errorf("saved session has expired")
	}

	return session, nil
}
