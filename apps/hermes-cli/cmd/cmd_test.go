package cmd

import (
	"bytes"
	"testing"
)

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

func TestAuthCmdSuccessfullAuth(t *testing.T) {

	output, err := executeCommand("auth", "microice")
	if err != nil {
		t.Fatalf("want: no error; but got: %v", err)
	}

	expected := "Successful authorization for user: microice\n"
	if output != expected {
		t.Errorf("want output: %q, but got: %q", expected, output)
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
