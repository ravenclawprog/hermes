package cmd

import (
	"bytes"
	"context"
	"errors"
	"hermes-cli/client"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type authClientFunc func(context.Context, string, []byte) (client.Session, error)

func (f authClientFunc) Authenticate(
	ctx context.Context,
	username string,
	password []byte,
) (client.Session, error) {
	return f(ctx, username, password)
}

type sessionStoreFunc func(context.Context, client.Session) error

func (f sessionStoreFunc) Save(
	ctx context.Context,
	session client.Session,
) error {
	return f(ctx, session)
}

func (sessionStoreFunc) Load(context.Context) (client.Session, error) {
	return client.Session{}, errors.New("unexpected session load in auth test")
}

func TestAuthCmdRequiresUsername(t *testing.T) {
	readCalled := false

	command := newAuthCmd(nil, nil, func() ([]byte, error) {
		readCalled = true
		return nil, nil
	})

	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{})

	err := command.Execute()
	if err == nil {
		t.Fatal("expected missing username error")
	}
	if !strings.Contains(err.Error(), "received 0") {
		t.Errorf("unexpected error: %v", err)
	}
	if readCalled {
		t.Error("password must not be requested without username")
	}
}

func TestAuthCmdSuccessfulAuth(t *testing.T) {
	called := false

	authClient := authClientFunc(func(
		ctx context.Context,
		username string,
		password []byte,
	) (client.Session, error) {
		called = true

		if username != "alice" || string(password) != "test-password" {
			t.Error("unexpected credentials")
		}

		return client.Session{
			UserID: "user-1",
			Token:  "test-token",
		}, nil
	})
	var savedSession client.Session
	saved := false

	store := sessionStoreFunc(func(
		ctx context.Context,
		session client.Session,
	) error {
		saved = true
		savedSession = session
		return nil
	})
	command := newAuthCmd(authClient, store, func() ([]byte, error) {
		return []byte("test-password"), nil
	})

	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SetArgs([]string{"alice"})

	if err := command.Execute(); err != nil {
		t.Fatalf("execute command: %v", err)
	}
	if !called {
		t.Error("authentication client was not called")
	}
	if stdout.String() != "Signed in as user-1\n" {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
	if stderr.String() != "Password: \n" {
		t.Errorf("unexpected stderr: %q", stderr.String())
	}
	if !saved {
		t.Error("session was not saved")
	}
	if savedSession.UserID != "user-1" {
		t.Error("unexpected saved session")
	}
	if savedSession.Token != "test-token" {
		t.Error("unexpected saved token")
	}
}

func TestAuthCmdFailedAuth(t *testing.T) {
	rejected := &client.APIError{Code: "unauthenticated"}
	authClient := authClientFunc(func(context.Context, string, []byte) (client.Session, error) {
		return client.Session{}, rejected
	})
	store := sessionStoreFunc(func(context.Context, client.Session) error {
		t.Fatal("must not save a session after failed authentication")
		return nil
	})
	command := newAuthCmd(authClient, store, func() ([]byte, error) {
		return []byte("test-password"), nil
	})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetErr(&output)
	command.SetArgs([]string{"alice"})

	if err := command.Execute(); !errors.Is(err, rejected) {
		t.Fatalf("expected authentication error, got %v", err)
	}
}

type documentClientFunc func(context.Context, client.Session) (client.DocumentPage, error)

func (f documentClientFunc) List(ctx context.Context, session client.Session) (client.DocumentPage, error) {
	return f(ctx, session)
}

func (documentClientFunc) Read(context.Context, client.Session, string) (client.Document, error) {
	return client.Document{}, errors.New("unexpected Read call")
}

func TestDocListCmd(t *testing.T) {
	store := client.NewFileSessionStore(filepath.Join(t.TempDir(), "session.json"))
	session := client.Session{UserID: "user-1", Token: "test-token", ExpiresAt: time.Now().Add(time.Hour)}
	if err := store.Save(context.Background(), session); err != nil {
		t.Fatal(err)
	}

	documents := documentClientFunc(func(_ context.Context, loaded client.Session) (client.DocumentPage, error) {
		if loaded.UserID != session.UserID || loaded.Token != session.Token {
			t.Fatal("unexpected session")
		}
		return client.DocumentPage{Items: []client.DocumentSummary{
			{ID: "doc-42", WorkspaceID: "ws-9", Title: "План проекта", Version: 7},
		}}, nil
	})
	command := newDocCmd(documents, store)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"list"})

	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	if want := "doc-42\tws-9\tПлан проекта\t7\n"; output.String() != want {
		t.Errorf("want output %q, got %q", want, output.String())
	}
}

func TestDocListCmdMissingSession(t *testing.T) {
	store := client.NewFileSessionStore(filepath.Join(t.TempDir(), "session.json"))
	documents := documentClientFunc(func(context.Context, client.Session) (client.DocumentPage, error) {
		t.Fatal("list must not run without a saved session")
		return client.DocumentPage{}, nil
	})

	command := newDocCmd(documents, store)
	var stdout, stderr bytes.Buffer
	command.SetOut(&stdout)
	command.SetErr(&stderr)
	command.SilenceUsage = true
	command.SetArgs([]string{"list"})

	if err := command.Execute(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing session error, got %v", err)
	}
	if stdout.Len() != 0 {
		t.Errorf("unexpected stdout: %q", stdout.String())
	}
}
