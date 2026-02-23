package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotEnvFile(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	content := `# Comment line
DOTENV_TEST_A=hello
DOTENV_TEST_B="quoted value"
DOTENV_TEST_C='single quoted'
DOTENV_TEST_D=with inline comment # this is a comment
DOTENV_TEST_E=

export DOTENV_TEST_F=exported

`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Clean up env after test
	keys := []string{
		"DOTENV_TEST_A", "DOTENV_TEST_B", "DOTENV_TEST_C",
		"DOTENV_TEST_D", "DOTENV_TEST_E", "DOTENV_TEST_F",
	}
	for _, k := range keys {
		os.Unsetenv(k)
	}
	defer func() {
		for _, k := range keys {
			os.Unsetenv(k)
		}
	}()

	if err := LoadDotEnvFile(envFile); err != nil {
		t.Fatalf("LoadDotEnvFile failed: %v", err)
	}

	tests := []struct {
		key  string
		want string
	}{
		{"DOTENV_TEST_A", "hello"},
		{"DOTENV_TEST_B", "quoted value"},
		{"DOTENV_TEST_C", "single quoted"},
		{"DOTENV_TEST_D", "with inline comment"},
		{"DOTENV_TEST_E", ""},
		{"DOTENV_TEST_F", "exported"},
	}

	for _, tt := range tests {
		got := os.Getenv(tt.key)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestLoadDotEnvDoesNotOverrideExisting(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	content := `DOTENV_TEST_EXISTING=from-file
`
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Set the env var before loading
	os.Setenv("DOTENV_TEST_EXISTING", "from-env")
	defer os.Unsetenv("DOTENV_TEST_EXISTING")

	if err := LoadDotEnvFile(envFile); err != nil {
		t.Fatalf("LoadDotEnvFile failed: %v", err)
	}

	got := os.Getenv("DOTENV_TEST_EXISTING")
	if got != "from-env" {
		t.Errorf("expected env var to keep 'from-env', got %q", got)
	}
}

func TestLoadDotEnvMissingFile(t *testing.T) {
	err := LoadDotEnvFile("/nonexistent/.env")
	if err != nil {
		t.Errorf("expected no error for missing .env file, got: %v", err)
	}
}

func TestParseDotEnvLine(t *testing.T) {
	tests := []struct {
		line    string
		wantKey string
		wantVal string
		wantOK  bool
	}{
		{"KEY=value", "KEY", "value", true},
		{"KEY=\"quoted\"", "KEY", "quoted", true},
		{"KEY='single'", "KEY", "single", true},
		{"KEY=value # comment", "KEY", "value", true},
		{"export KEY=value", "KEY", "value", true},
		{"KEY=", "KEY", "", true},
		{"=value", "", "", false},
		{"no-equals", "", "", false},
		{"KEY=has=equals", "KEY", "has=equals", true},
	}

	for _, tt := range tests {
		key, val, ok := parseDotEnvLine(tt.line)
		if ok != tt.wantOK {
			t.Errorf("parseDotEnvLine(%q): ok=%v, want %v", tt.line, ok, tt.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if key != tt.wantKey {
			t.Errorf("parseDotEnvLine(%q): key=%q, want %q", tt.line, key, tt.wantKey)
		}
		if val != tt.wantVal {
			t.Errorf("parseDotEnvLine(%q): val=%q, want %q", tt.line, val, tt.wantVal)
		}
	}
}
