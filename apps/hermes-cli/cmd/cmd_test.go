package cmd

import (
	"bytes"
	"context"
	"hermes-cli/client"
	"strings"
	"testing"
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

// wrapping stdout by buffer
func executeCommand(args ...string) (string, error) {
	buf := new(bytes.Buffer)

	// getting root command
	root := rootCmd
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs(args)

	// execution
	err := root.Execute()
	return buf.String(), err
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
	_, err := executeCommand("auth")
	if err == nil {
		t.Error("want: error, but got:nothing (successfull auth without auth)")
	}
}

func TestDocListCmdSuccessfull(t *testing.T) {
	output, err := executeCommand("doc", "list")
	if err != nil {
		t.Fatalf("want:no error; but got: %v", err)
	}

	expected := "Doc list: [...]\n"
	if output != expected {
		t.Errorf("want output: %q, but got: %q", expected, output)
	}
}
