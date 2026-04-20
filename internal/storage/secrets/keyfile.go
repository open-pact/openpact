// Package secrets persists Starlark-accessible secrets in the shared
// op_secrets table with AES-256-GCM encryption at rest. The symmetric
// key is stored separately on disk (under secure/data/data_encryption_key)
// so even a compromised DB file without the key yields only ciphertext.
package secrets

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
)

// keyFileName is the filename under secure/data/ where the data
// encryption key lives. Deliberately NOT `jwt_secret` (which has its
// own purpose and rotation story); a separate key means the two can
// rotate independently.
const keyFileName = "data_encryption_key"

// keySize is the AES-256 key length in bytes.
const keySize = 32

// LoadOrCreateKey reads the encryption key from dataDir/data_encryption_key,
// or generates a new random 32-byte key and persists it with 0o600
// perms if absent. Returns a key suitable for aes.NewCipher.
//
// The key lives alongside the other bootstrap secrets (jwt_secret,
// stackllm.db) under secure/data/. Losing it makes every encrypted
// secret unrecoverable — operators back up the entire secure/ dir or
// none of it.
func LoadOrCreateKey(dataDir string) ([]byte, error) {
	path := filepath.Join(dataDir, keyFileName)

	if data, err := os.ReadFile(path); err == nil {
		if len(data) != keySize {
			return nil, fmt.Errorf("secrets: key file %s has wrong length %d (want %d)", path, len(data), keySize)
		}
		return data, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("secrets: read key: %w", err)
	}

	// Generate a fresh key.
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("secrets: create data dir: %w", err)
	}
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("secrets: generate key: %w", err)
	}
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, fmt.Errorf("secrets: write key: %w", err)
	}
	return key, nil
}
