package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockModelLookup implements ModelLookup for testing.
type mockModelLookup struct {
	models          []ModelInfo
	defaultProvider string
	defaultModel    string
	setProvider     string
	setModel        string
	setErr          error
}

func (m *mockModelLookup) ListModels() ([]ModelInfo, error) {
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

func testModels() []ModelInfo {
	return []ModelInfo{
		{ProviderID: "openai", ModelID: "gpt-4o", Context: 128000, Output: 16384},
		{ProviderID: "openai", ModelID: "gpt-4o-mini", Context: 128000, Output: 16384},
		{ProviderID: "gemini", ModelID: "gemini-2.0-flash", Context: 1000000, Output: 8192},
		{ProviderID: "copilot", ModelID: "gpt-5-codex", Context: 272000, Output: 100000},
		{ProviderID: "copilot", ModelID: "gpt-5.1-codex", Context: 272000, Output: 100000},
	}
}

func TestModelListTool(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
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
	if !strings.Contains(output, "openai") {
		t.Error("expected output to contain 'openai'")
	}
	if !strings.Contains(output, "gpt-4o") {
		t.Error("expected output to contain 'gpt-4o'")
	}
	if !strings.Contains(output, "**(default)**") {
		t.Error("expected output to mark the default model")
	}
	if !strings.Contains(output, "gemini") {
		t.Error("expected output to contain 'gemini'")
	}
}

func TestModelSetDefaultExactMatch(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "gpt-4o-mini",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setProvider != "openai" {
		t.Errorf("expected provider 'openai', got '%s'", lookup.setProvider)
	}
	if lookup.setModel != "gpt-4o-mini" {
		t.Errorf("expected model 'gpt-4o-mini', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "gpt-4o-mini") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultFuzzyMatch(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "gemini-2.0",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setProvider != "gemini" {
		t.Errorf("expected provider 'gemini', got '%s'", lookup.setProvider)
	}
	if lookup.setModel != "gemini-2.0-flash" {
		t.Errorf("expected model 'gemini-2.0-flash', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "gemini-2.0") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultFuzzyWithProvider(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
	}

	tool := modelSetDefaultTool(lookup)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"model":    "5.1",
		"provider": "copilot",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if lookup.setModel != "gpt-5.1-codex" {
		t.Errorf("expected model 'gpt-5.1-codex', got '%s'", lookup.setModel)
	}
	if !strings.Contains(result.(string), "5.1") {
		t.Errorf("expected success message, got: %v", result)
	}
}

func TestModelSetDefaultAmbiguous(t *testing.T) {
	lookup := &mockModelLookup{
		models:          testModels(),
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
	}

	tool := modelSetDefaultTool(lookup)

	// "gpt" matches multiple openai + copilot models
	_, err := tool.Handler(context.Background(), map[string]interface{}{
		"model": "gpt",
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
		defaultProvider: "openai",
		defaultModel:    "gpt-4o",
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
		"model": "gemini-2.0-flash",
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
