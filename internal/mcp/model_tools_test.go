package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/open-pact/openpact/internal/engine"
)

// mockModelLookup implements ModelLookup for testing.
type mockModelLookup struct {
	models          []engine.ModelInfo
	defaultProvider string
	defaultModel    string
	setProvider     string
	setModel        string
	setErr          error
}

func (m *mockModelLookup) ListModels() ([]engine.ModelInfo, error) {
	return m.models, nil
}

func (m *mockModelLookup) GetDefaultModel() (string, string) {
	return m.defaultProvider, m.defaultModel
}

func (m *mockModelLookup) SetDefaultModel(provider, model string) error {
	m.setProvider = provider
	m.setModel = model
	return m.setErr
}

func testModels() []engine.ModelInfo {
	return []engine.ModelInfo{
		{ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514", Context: 200000, Output: 16000},
		{ProviderID: "anthropic", ModelID: "claude-opus-4-20250514", Context: 200000, Output: 32000},
		{ProviderID: "anthropic", ModelID: "claude-haiku-3-5-20241022", Context: 200000, Output: 8192},
		{ProviderID: "openai", ModelID: "gpt-4o", Context: 128000, Output: 16384},
	}
}

func TestModelListTool(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelListTool(lookup)

	if tool.Name != "model_list" {
		t.Errorf("expected name 'model_list', got '%s'", tool.Name)
	}

	result, err := tool.Handler(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := result.(string)
	if !strings.Contains(output, "anthropic") {
		t.Error("expected output to contain 'anthropic'")
	}
	if !strings.Contains(output, "claude-sonnet-4-20250514") {
		t.Error("expected output to contain 'claude-sonnet-4-20250514'")
	}
	if !strings.Contains(output, "**(default)**") {
		t.Error("expected output to mark the default model")
	}
	if !strings.Contains(output, "openai") {
		t.Error("expected output to contain 'openai'")
	}
}

func TestModelSetDefaultExactMatch(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "claude-opus-4-20250514",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setProvider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", lookup.setProvider)
	}
	if lookup.setModel != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "claude-opus-4-20250514") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultFuzzyMatch(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "opus",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setProvider != "anthropic" {
		t.Errorf("expected provider 'anthropic', got '%s'", lookup.setProvider)
	}
	if lookup.setModel != "claude-opus-4-20250514" {
		t.Errorf("expected model 'claude-opus-4-20250514', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "opus") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultFuzzyWithProvider(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model":    "haiku",
		"provider": "anthropic",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setModel != "claude-haiku-3-5-20241022" {
		t.Errorf("expected model 'claude-haiku-3-5-20241022', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "haiku") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultAmbiguous(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelSetDefaultTool(lookup)

	// "claude" matches multiple anthropic models
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "claude",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous match")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

func TestModelSetDefaultNoMatch(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "anthropic",
		defaultModel:    "claude-sonnet-4-20250514",
	}

	tool := modelSetDefaultTool(lookup)

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "nonexistent-model",
	})
	if err == nil {
		t.Fatal("expected error for no match")
	}
	if !strings.Contains(err.Error(), "no model matching") {
		t.Errorf("expected 'no model matching' in error, got: %v", err)
	}
}

func TestModelSetDefaultMissingModel(t *testing.T) {
	lookup := &mockModelLookup{
		models: testModels(),
	}

	tool := modelSetDefaultTool(lookup)

	_, err := tool.Handler(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Errorf("expected 'model is required' in error, got: %v", err)
	}
}

func TestModelSetDefaultSetError(t *testing.T) {
	lookup := &mockModelLookup{
		models: testModels(),
		setErr: fmt.Errorf("disk full"),
	}

	tool := modelSetDefaultTool(lookup)

	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "gpt-4o",
	})
	if err == nil {
		t.Fatal("expected error when set fails")
	}
	if !strings.Contains(err.Error(), "disk full") {
		t.Errorf("expected 'disk full' in error, got: %v", err)
	}
}

func TestRegisterModelTools(t *testing.T) {
	s := NewServer(nil, nil)
	lookup := &mockModelLookup{
		models: testModels(),
	}

	RegisterModelTools(s, lookup)

	tools := s.ListTools()
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}

	names := map[string]bool{}
	for _, tool := range tools {
		names[tool.Name] = true
	}
	if !names["model_list"] {
		t.Error("expected 'model_list' tool to be registered")
	}
	if !names["model_set_default"] {
		t.Error("expected 'model_set_default' tool to be registered")
	}
}
