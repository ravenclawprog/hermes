package cmd

import (
	"crypto/sha256"
	"fmt"
	"hermes-cli/client"
	"os"
	"path/filepath"
)

func newDefaultSessionStore(serverURL string) (*client.FileSessionStore, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("find configuration directory: %w", err)
	}

	serverID := sha256.Sum256([]byte(serverURL))
	filename := fmt.Sprintf("%x.json", serverID)

	path := filepath.Join(configDir, "hermes", "sessions", filename)
	return client.NewFileSessionStore(path), nil
}
