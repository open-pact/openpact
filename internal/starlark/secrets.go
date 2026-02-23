package starlark

import (
	"regexp"
	"strings"
	"sync"

	"go.starlark.net/starlark"
)

// SecretProvider provides secrets to scripts without exposing them to the AI
type SecretProvider struct {
	mu      sync.RWMutex
	secrets map[string]string // name -> value
}

// NewSecretProvider creates a new secret provider
func NewSecretProvider() *SecretProvider {
	return &SecretProvider{
		secrets: make(map[string]string),
	}
}

// Set adds or updates a secret
func (sp *SecretProvider) Set(name, value string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.secrets[name] = value
}

// Get retrieves a secret by name
func (sp *SecretProvider) Get(name string) (string, bool) {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	v, ok := sp.secrets[name]
	return v, ok
}

// Names returns all secret names (not values)
func (sp *SecretProvider) Names() []string {
	sp.mu.RLock()
	defer sp.mu.RUnlock()
	names := make([]string, 0, len(sp.secrets))
	for k := range sp.secrets {
		names = append(names, k)
	}
	return names
}

// Replace atomically swaps the entire secrets map.
func (sp *SecretProvider) Replace(secrets map[string]string) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.secrets = make(map[string]string, len(secrets))
	for k, v := range secrets {
		sp.secrets[k] = v
	}
}

// InjectSecrets injects secrets into the sandbox as a read-only module
// Scripts access via: secrets.get("API_KEY")
func (s *Sandbox) InjectSecrets(provider *SecretProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create a secrets module with a get function
	getSecret := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var name string
		if err := starlark.UnpackArgs("secrets.get", args, kwargs, "name", &name); err != nil {
			return starlark.None, err
		}

		value, ok := provider.Get(name)
		if !ok {
			return starlark.None, nil
		}
		return starlark.String(value), nil
	}

	// List available secret names (not values)
	listSecrets := func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		names := provider.Names()
		list := make([]starlark.Value, len(names))
		for i, name := range names {
			list[i] = starlark.String(name)
		}
		return starlark.NewList(list), nil
	}

	secretsModule := &starlark.Builtin{}
	_ = secretsModule // placeholder

	// Build module
	s.predeclared["secrets"] = starlarkBuiltinModule("secrets", starlark.StringDict{
		"get":  starlark.NewBuiltin("secrets.get", getSecret),
		"list": starlark.NewBuiltin("secrets.list", listSecrets),
	})
}

// starlarkBuiltinModule creates a simple module from a dict of functions
func starlarkBuiltinModule(name string, funcs starlark.StringDict) *secretsModuleValue {
	return &secretsModuleValue{name: name, funcs: funcs}
}

// secretsModuleValue implements starlark.Value for our secrets module
type secretsModuleValue struct {
	name  string
	funcs starlark.StringDict
}

func (m *secretsModuleValue) String() string        { return m.name }
func (m *secretsModuleValue) Type() string          { return "module" }
func (m *secretsModuleValue) Freeze()               {}
func (m *secretsModuleValue) Truth() starlark.Bool  { return true }
func (m *secretsModuleValue) Hash() (uint32, error) { return 0, nil }

func (m *secretsModuleValue) Attr(name string) (starlark.Value, error) {
	if v, ok := m.funcs[name]; ok {
		return v, nil
	}
	return nil, nil
}

func (m *secretsModuleValue) AttrNames() []string {
	names := make([]string, 0, len(m.funcs))
	for k := range m.funcs {
		names = append(names, k)
	}
	return names
}

// SanitizeResult removes any secret values from the result before returning to AI
func SanitizeResult(result Result, provider *SecretProvider) Result {
	if provider == nil {
		return result
	}

	// Sanitize error message
	if result.Error != "" {
		result.Error = sanitizeString(result.Error, provider)
	}

	// Sanitize value
	if result.Value != nil {
		result.Value = sanitizeValue(result.Value, provider)
	}

	return result
}

// sanitizeString replaces secret values with "[REDACTED]"
func sanitizeString(s string, provider *SecretProvider) string {
	for _, name := range provider.Names() {
		value, ok := provider.Get(name)
		if !ok || value == "" {
			continue
		}
		// Only redact if the value is long enough to be meaningful
		if len(value) >= 8 {
			s = strings.ReplaceAll(s, value, "[REDACTED:"+name+"]")
		}
	}
	return s
}

// sanitizeValue recursively sanitizes any secret values in the result
func sanitizeValue(v any, provider *SecretProvider) any {
	switch val := v.(type) {
	case string:
		return sanitizeString(val, provider)
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = sanitizeValue(item, provider)
		}
		return result
	case map[string]any:
		result := make(map[string]any, len(val))
		for k, v := range val {
			result[k] = sanitizeValue(v, provider)
		}
		return result
	default:
		return v
	}
}

// ExtractRequiredSecrets parses script metadata to find required secrets
// Format: # @secrets: API_KEY, OTHER_SECRET
func ExtractRequiredSecrets(source string) []string {
	re := regexp.MustCompile(`(?m)^#\s*@secrets?:\s*(.+)$`)
	matches := re.FindStringSubmatch(source)
	if len(matches) < 2 {
		return nil
	}

	parts := strings.Split(matches[1], ",")
	secrets := make([]string, 0, len(parts))
	for _, p := range parts {
		name := strings.TrimSpace(p)
		if name != "" {
			secrets = append(secrets, name)
		}
	}
	return secrets
}
