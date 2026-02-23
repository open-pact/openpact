package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestLevelString(t *testing.T) {
	tests := []struct {
		level Level
		want  string
	}{
		{LevelDebug, "DEBUG"},
		{LevelInfo, "INFO"},
		{LevelWarn, "WARN"},
		{LevelError, "ERROR"},
	}

	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("Level(%d).String() = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input string
		want  Level
	}{
		{"debug", LevelDebug},
		{"DEBUG", LevelDebug},
		{"info", LevelInfo},
		{"INFO", LevelInfo},
		{"warn", LevelWarn},
		{"WARN", LevelWarn},
		{"warning", LevelWarn},
		{"error", LevelError},
		{"ERROR", LevelError},
		{"unknown", LevelInfo}, // default
	}

	for _, tt := range tests {
		if got := ParseLevel(tt.input); got != tt.want {
			t.Errorf("ParseLevel(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelWarn,
		Output: &buf,
	})

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	if strings.Contains(output, "debug message") {
		t.Error("debug message should not be logged at WARN level")
	}
	if strings.Contains(output, "info message") {
		t.Error("info message should not be logged at WARN level")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("warn message should be logged at WARN level")
	}
	if !strings.Contains(output, "error message") {
		t.Error("error message should be logged at WARN level")
	}
}

func TestLoggerJSONFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	logger.Info("test message")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON log entry: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("entry.Level = %q, want %q", entry.Level, "INFO")
	}
	if entry.Message != "test message" {
		t.Errorf("entry.Message = %q, want %q", entry.Message, "test message")
	}
}

func TestLoggerWithField(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	contextLogger := logger.WithField("component", "test")
	contextLogger.Info("with field")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Fields["component"] != "test" {
		t.Errorf("fields[component] = %v, want %q", entry.Fields["component"], "test")
	}
}

func TestLoggerWithFields(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:      LevelInfo,
		Output:     &buf,
		JSONFormat: true,
	})

	contextLogger := logger.WithFields(map[string]any{
		"component": "test",
		"version":   "1.0",
	})
	contextLogger.Info("with fields")

	var entry Entry
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Fields["component"] != "test" {
		t.Errorf("fields[component] = %v, want %q", entry.Fields["component"], "test")
	}
	if entry.Fields["version"] != "1.0" {
		t.Errorf("fields[version] = %v, want %q", entry.Fields["version"], "1.0")
	}
}

func TestLoggerFormatting(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelInfo,
		Output: &buf,
	})

	logger.Info("user %s logged in from %s", "alice", "192.168.1.1")

	output := buf.String()
	if !strings.Contains(output, "user alice logged in from 192.168.1.1") {
		t.Errorf("expected formatted message, got: %s", output)
	}
}

func TestLoggerHumanReadableFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelInfo,
		Output: &buf,
	})

	logger.WithField("user", "bob").Info("action performed")

	output := buf.String()
	if !strings.Contains(output, "[INFO]") {
		t.Errorf("expected [INFO] in output, got: %s", output)
	}
	if !strings.Contains(output, "action performed") {
		t.Errorf("expected message in output, got: %s", output)
	}
	if !strings.Contains(output, "user=bob") {
		t.Errorf("expected field in output, got: %s", output)
	}
}

func TestDefault(t *testing.T) {
	logger := Default()
	if logger == nil {
		t.Error("Default() returned nil")
	}
}

func TestSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := New(Config{
		Level:  LevelError,
		Output: &buf,
	})

	logger.Info("should not appear")
	if buf.Len() > 0 {
		t.Error("info should not be logged at ERROR level")
	}

	logger.SetLevel(LevelInfo)
	logger.Info("should appear")
	if buf.Len() == 0 {
		t.Error("info should be logged after SetLevel(LevelInfo)")
	}
}
