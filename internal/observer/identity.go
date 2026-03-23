package observer

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/yevgetman/fry/internal/config"
	"github.com/yevgetman/fry/templates"
)

// EnsureIdentity ensures the identity doc exists. If missing, copies the seed
// template. Returns the content.
func EnsureIdentity(projectDir string) (string, error) {
	identityPath := filepath.Join(projectDir, config.ObserverIdentityFile)

	content, err := ReadIdentity(projectDir)
	if err != nil {
		return "", err
	}
	if content != "" {
		return content, nil
	}

	// Identity does not exist — seed from template
	seed, err := identitySeed()
	if err != nil {
		return "", fmt.Errorf("ensure identity: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(identityPath), 0o755); err != nil {
		return "", fmt.Errorf("ensure identity: create dir: %w", err)
	}
	if err := os.WriteFile(identityPath, []byte(seed), 0o644); err != nil {
		return "", fmt.Errorf("ensure identity: write seed: %w", err)
	}

	return seed, nil
}

// ReadIdentity reads the identity doc. Returns "", nil if missing.
func ReadIdentity(projectDir string) (string, error) {
	identityPath := filepath.Join(projectDir, config.ObserverIdentityFile)
	data, err := os.ReadFile(identityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("read identity: %w", err)
	}
	return string(data), nil
}

// WriteIdentity writes the identity doc.
func WriteIdentity(projectDir string, content string) error {
	identityPath := filepath.Join(projectDir, config.ObserverIdentityFile)
	if err := os.MkdirAll(filepath.Dir(identityPath), 0o755); err != nil {
		return fmt.Errorf("write identity: create dir: %w", err)
	}
	if err := os.WriteFile(identityPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write identity: %w", err)
	}
	return nil
}

// identitySeed reads the embedded seed from templates.TemplateFS.
func identitySeed() (string, error) {
	data, err := fs.ReadFile(templates.TemplateFS, config.ObserverIdentitySeedFile)
	if err != nil {
		return "", fmt.Errorf("read identity seed: %w", err)
	}
	return string(data), nil
}
