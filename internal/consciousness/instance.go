package consciousness

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// InstanceID returns a stable, anonymized identifier for this machine.
// It hashes the hostname with SHA-256 and returns the first 16 hex characters.
// Falls back to a hash of the user's home directory, then "unknown".
func InstanceID() string {
	if hostname, err := os.Hostname(); err == nil && hostname != "" {
		h := sha256.Sum256([]byte(hostname))
		return fmt.Sprintf("%x", h[:8])
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		h := sha256.Sum256([]byte(home))
		return fmt.Sprintf("%x", h[:8])
	}
	return "unknown"
}
