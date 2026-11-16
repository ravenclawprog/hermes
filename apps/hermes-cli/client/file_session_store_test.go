package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileSessionStoreSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions", "session.json")
	store := NewFileSessionStore(path)

	expected := Session{
		UserID:    "user-1",
		Token:     "test-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Second),
	}

	if err := store.Save(context.Background(), expected); err != nil {
		t.Fatalf("save session: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read session file: %v", err)
	}

	var actual Session
	if err := json.Unmarshal(data, &actual); err != nil {
		t.Fatalf("decode session: %v", err)
	}

	if actual.UserID != expected.UserID ||
		actual.Token != expected.Token ||
		!actual.ExpiresAt.Equal(expected.ExpiresAt) {
		t.Error("saved session differs from expected")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat session file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %04o", info.Mode().Perm())
	}
}

func TestFileSessionStoreLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	store := NewFileSessionStore(path)

	expected := Session{
		UserID:    "user-1",
		Token:     "test-token",
		ExpiresAt: time.Now().UTC().Add(time.Hour).Truncate(time.Second),
	}

	if err := store.Save(context.Background(), expected); err != nil {
		t.Fatalf("save session: %v", err)
	}

	actual, err := store.Load(context.Background())
	if err != nil {
		t.Fatalf("load session: %v", err)
	}

	if actual.UserID != expected.UserID ||
		actual.Token != expected.Token ||
		!actual.ExpiresAt.Equal(expected.ExpiresAt) {
		t.Error("loaded session differs from expected")
	}
}

func TestFileSessionStoreLoadMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.json")
	store := NewFileSessionStore(path)

	session, err := store.Load(context.Background())

	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing file error, got: %v", err)
	}
	if session != (Session{}) {
		t.Error("expected an empty session")
	}
}
