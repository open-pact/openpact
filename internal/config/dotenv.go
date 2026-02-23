package config

import (
	"bufio"
	"os"
	"strings"
)

// LoadDotEnv reads a .env file from the current working directory and sets
// environment variables for any keys not already present in the environment.
// This gives real environment variables precedence over .env file values.
// If the file does not exist, this is a no-op.
func LoadDotEnv() error {
	return LoadDotEnvFile(".env")
}

// LoadDotEnvFile reads the specified file and sets environment variables for
// any keys not already present in the environment.
func LoadDotEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := parseDotEnvLine(line)
		if !ok {
			continue
		}

		// Only set if not already in environment
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}

	return scanner.Err()
}

// parseDotEnvLine parses a single KEY=VALUE line.
// It handles double-quoted, single-quoted, and unquoted values.
func parseDotEnvLine(line string) (key, value string, ok bool) {
	// Split on first '='
	idx := strings.IndexByte(line, '=')
	if idx < 0 {
		return "", "", false
	}

	key = strings.TrimSpace(line[:idx])
	if key == "" {
		return "", "", false
	}

	// Skip "export " prefix
	key = strings.TrimPrefix(key, "export ")

	value = strings.TrimSpace(line[idx+1:])

	// Handle quoted values
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') ||
			(value[0] == '\'' && value[len(value)-1] == '\'') {
			value = value[1 : len(value)-1]
			return key, value, true
		}
	}

	// For unquoted values, strip inline comments
	if i := strings.IndexByte(value, '#'); i >= 0 {
		value = strings.TrimSpace(value[:i])
	}

	return key, value, true
}
