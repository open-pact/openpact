package auth

import (
	"testing"
)

func TestCheckAuth_UnknownEngine(t *testing.T) {
	status := CheckAuth("unknown-engine")
	if status.Authenticated {
		t.Error("expected not authenticated for unknown engine")
	}
	if status.Error != "unknown engine type" {
		t.Errorf("expected 'unknown engine type' error, got %q", status.Error)
	}
}

func TestCheckOpenCodeAuth_EnvVar(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test123")

	status := CheckOpenCodeAuth()
	if !status.Authenticated {
		t.Error("expected authenticated with ANTHROPIC_API_KEY")
	}
	if status.Method != "env" {
		t.Errorf("expected method 'env', got %q", status.Method)
	}
}

func TestCheckOpenCodeAuth_OpenAIKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "sk-openai-test")
	t.Setenv("GOOGLE_API_KEY", "")
	t.Setenv("AZURE_OPENAI_API_KEY", "")

	status := CheckOpenCodeAuth()
	if !status.Authenticated {
		t.Error("expected authenticated with OPENAI_API_KEY")
	}
	if status.Method != "env" {
		t.Errorf("expected method 'env', got %q", status.Method)
	}
}
