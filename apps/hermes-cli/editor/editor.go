package editor

import (
	"os"
	"os/exec"
)

// OpenInSystemEditor create temporary file for editing docs
func OpenInSystemEditor(initialContent string) (string, error) {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vim"
	}

	tmpFile, err := os.CreateTemp("", "hermes-doc-*.txt")
	if err != nil {
		return "", err
	}

	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if initialContent != "" {
		if _, err := tmpFile.WriteString(initialContent); err != nil {
			return "", err
		}
	}

	cmd := exec.Command(editor, tmpFile.Name())

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", err
	}

	updatedContent, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return string(updatedContent), nil
}
