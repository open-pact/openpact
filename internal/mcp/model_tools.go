package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ModelInfo describes an available model from a provider. It mirrors the
// minimal shape the model_list and model_set_default tools need — the
// orchestrator adapts stackllm's richer profile.ModelInfo to this struct
// when implementing ModelLookup.
type ModelInfo struct {
	ProviderID string
	ModelID    string
	Context    int // Context window in tokens (0 if unknown)
	Output     int // Output limit in tokens (0 if unknown)
}

// ModelLookup provides model listing and default model management.
type ModelLookup interface {
	ListModels() ([]ModelInfo, error)
	GetDefaultModel() (string, string)
	SetDefaultModel(provider, model string) error
}

// RegisterModelTools adds model_list and model_set_default tools to the MCP server.
func RegisterModelTools(s *Server, lookup ModelLookup) {
	s.RegisterTool(modelListTool(lookup))
	s.RegisterTool(modelSetDefaultTool(lookup))
}

func modelListTool(lookup ModelLookup) *Tool {
	return &Tool{
		Name:        "model_list",
		Description: "List all available AI models grouped by provider. Shows the current default model.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			models, err := lookup.ListModels()
			if err != nil {
				return nil, fmt.Errorf("failed to list models: %w", err)
			}

			defaultProvider, defaultModel := lookup.GetDefaultModel()

			// Group by provider
			grouped := make(map[string][]ModelInfo)
			for _, m := range models {
				grouped[m.ProviderID] = append(grouped[m.ProviderID], m)
			}

			// Sort providers for deterministic output
			providerNames := make([]string, 0, len(grouped))
			for p := range grouped {
				providerNames = append(providerNames, p)
			}
			sort.Strings(providerNames)

			var b strings.Builder
			b.WriteString(fmt.Sprintf("Default model: %s/%s\n\n", defaultProvider, defaultModel))

			for _, providerID := range providerNames {
				providerModels := grouped[providerID]
				sort.Slice(providerModels, func(i, j int) bool {
					return providerModels[i].ModelID < providerModels[j].ModelID
				})

				b.WriteString(fmt.Sprintf("## %s\n", providerID))
				for _, m := range providerModels {
					marker := ""
					if m.ProviderID == defaultProvider && m.ModelID == defaultModel {
						marker = " **(default)**"
					}
					meta := ""
					if m.Context > 0 {
						meta = fmt.Sprintf(" (context: %dk)", m.Context/1000)
					}
					b.WriteString(fmt.Sprintf("- %s%s%s\n", m.ModelID, meta, marker))
				}
				b.WriteString("\n")
			}

			return b.String(), nil
		},
	}
}

func modelSetDefaultTool(lookup ModelLookup) *Tool {
	return &Tool{
		Name: "model_set_default",
		Description: "Set the default AI model for new sessions. " +
			"Supports fuzzy matching — you can use a partial model name (e.g. 'opus' or 'sonnet'). " +
			"If provider is omitted, it will be inferred from the match.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"model": map[string]interface{}{
					"type":        "string",
					"description": "Model ID or partial name to match (e.g. 'claude-sonnet-4-20250514' or 'opus')",
				},
				"provider": map[string]interface{}{
					"type":        "string",
					"description": "Provider ID (e.g. 'openai', 'copilot', 'gemini', 'ollama'). Optional — inferred from model match if omitted.",
				},
			},
			"required": []string{"model"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			modelInput, _ := args["model"].(string)
			providerInput, _ := args["provider"].(string)

			if modelInput == "" {
				return nil, fmt.Errorf("model is required")
			}

			models, err := lookup.ListModels()
			if err != nil {
				return nil, fmt.Errorf("failed to list models: %w", err)
			}

			match, err := fuzzyMatchModel(models, modelInput, providerInput)
			if err != nil {
				return nil, err
			}

			if err := lookup.SetDefaultModel(match.ProviderID, match.ModelID); err != nil {
				return nil, fmt.Errorf("failed to set default model: %w", err)
			}

			suffix := ""
			if match.Context > 0 {
				suffix = fmt.Sprintf(" (context: %dk)", match.Context/1000)
			}
			return fmt.Sprintf("Default model set to %s/%s%s",
				match.ProviderID, match.ModelID, suffix), nil
		},
	}
}

// fuzzyMatchModel finds a model by exact match first, then case-insensitive substring.
// Returns an error with suggestions on ambiguous or no match.
func fuzzyMatchModel(models []ModelInfo, modelInput, providerInput string) (*ModelInfo, error) {
	modelLower := strings.ToLower(modelInput)
	providerLower := strings.ToLower(providerInput)

	// Phase 1: exact match
	for _, m := range models {
		if providerInput != "" && !strings.EqualFold(m.ProviderID, providerInput) {
			continue
		}
		if m.ModelID == modelInput {
			return &m, nil
		}
	}

	// Phase 2: case-insensitive substring match
	var matches []ModelInfo
	for _, m := range models {
		if providerInput != "" && !strings.EqualFold(m.ProviderID, providerInput) {
			continue
		}
		if strings.Contains(strings.ToLower(m.ModelID), modelLower) {
			matches = append(matches, m)
		}
		// Also check provider match if no provider filter
		if providerInput == "" && providerLower != "" && strings.Contains(strings.ToLower(m.ProviderID), providerLower) {
			// Already covered by modelLower match above
			continue
		}
	}

	if len(matches) == 1 {
		return &matches[0], nil
	}

	if len(matches) > 1 {
		var suggestions []string
		for _, m := range matches {
			suggestions = append(suggestions, fmt.Sprintf("%s/%s", m.ProviderID, m.ModelID))
		}
		return nil, fmt.Errorf("ambiguous model %q — matches: %s. Please be more specific",
			modelInput, strings.Join(suggestions, ", "))
	}

	// No matches — suggest available models
	var available []string
	for _, m := range models {
		available = append(available, fmt.Sprintf("%s/%s", m.ProviderID, m.ModelID))
	}
	if len(available) > 10 {
		available = available[:10]
		available = append(available, "...")
	}
	return nil, fmt.Errorf("no model matching %q found. Available: %s",
		modelInput, strings.Join(available, ", "))
}
