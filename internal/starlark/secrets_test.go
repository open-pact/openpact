package starlark

import (
	"context"
	"testing"
)

func TestSecretProvider(t *testing.T) {
	sp := NewSecretProvider()

	sp.Set("API_KEY", "secret123")
	sp.Set("OTHER", "value456")

	val, ok := sp.Get("API_KEY")
	if !ok || val != "secret123" {
		t.Errorf("Get(API_KEY) = %q, %v", val, ok)
	}

	_, ok = sp.Get("NONEXISTENT")
	if ok {
		t.Error("Get(NONEXISTENT) should return false")
	}

	names := sp.Names()
	if len(names) != 2 {
		t.Errorf("Names() len = %d, want 2", len(names))
	}
}

func TestInjectSecrets(t *testing.T) {
	s := New(Config{})
	sp := NewSecretProvider()
	sp.Set("WEATHER_API_KEY", "abc123xyz")

	s.InjectSecrets(sp)

	ctx := context.Background()
	result := s.Execute(ctx, "test.star", `
key = secrets.get("WEATHER_API_KEY")
result = key != None
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != true {
		t.Error("expected secrets.get to return the key")
	}
}

func TestSecretsGetMissing(t *testing.T) {
	s := New(Config{})
	sp := NewSecretProvider()
	s.InjectSecrets(sp)

	ctx := context.Background()
	result := s.Execute(ctx, "test.star", `
result = secrets.get("NONEXISTENT") == None
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != true {
		t.Error("expected None for missing secret")
	}
}

func TestSecretsList(t *testing.T) {
	s := New(Config{})
	sp := NewSecretProvider()
	sp.Set("KEY1", "val1")
	sp.Set("KEY2", "val2")
	s.InjectSecrets(sp)

	ctx := context.Background()
	result := s.Execute(ctx, "test.star", `
names = secrets.list()
result = len(names)
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != int64(2) {
		t.Errorf("len = %v, want 2", result.Value)
	}
}

func TestSanitizeResultString(t *testing.T) {
	sp := NewSecretProvider()
	sp.Set("API_KEY", "supersecretkey123")

	result := Result{
		Value: "The API key is supersecretkey123 and it works",
	}

	sanitized := SanitizeResult(result, sp)

	str := sanitized.Value.(string)
	if str == result.Value.(string) {
		t.Error("secret was not redacted")
	}
	if !contains(str, "[REDACTED:API_KEY]") {
		t.Errorf("expected redaction marker, got: %s", str)
	}
}

func TestSanitizeResultMap(t *testing.T) {
	sp := NewSecretProvider()
	sp.Set("TOKEN", "mytoken12345")

	result := Result{
		Value: map[string]any{
			"message": "Token is mytoken12345",
			"nested": map[string]any{
				"deep": "Also mytoken12345 here",
			},
		},
	}

	sanitized := SanitizeResult(result, sp)

	m := sanitized.Value.(map[string]any)
	msg := m["message"].(string)
	if contains(msg, "mytoken12345") {
		t.Error("secret in message was not redacted")
	}

	nested := m["nested"].(map[string]any)
	deep := nested["deep"].(string)
	if contains(deep, "mytoken12345") {
		t.Error("secret in nested was not redacted")
	}
}

func TestSanitizeResultList(t *testing.T) {
	sp := NewSecretProvider()
	sp.Set("SECRET", "topsecret99")

	result := Result{
		Value: []any{
			"First topsecret99",
			"Second topsecret99",
		},
	}

	sanitized := SanitizeResult(result, sp)

	list := sanitized.Value.([]any)
	for _, item := range list {
		if contains(item.(string), "topsecret99") {
			t.Error("secret in list was not redacted")
		}
	}
}

func TestSanitizeResultError(t *testing.T) {
	sp := NewSecretProvider()
	sp.Set("PASSWORD", "hunter2secret")

	result := Result{
		Error: "Failed to connect with password hunter2secret",
	}

	sanitized := SanitizeResult(result, sp)

	if contains(sanitized.Error, "hunter2secret") {
		t.Error("secret in error was not redacted")
	}
}

func TestSanitizeShortSecrets(t *testing.T) {
	sp := NewSecretProvider()
	sp.Set("SHORT", "abc") // Too short to redact (could cause false positives)

	result := Result{
		Value: "The value abc appears here",
	}

	sanitized := SanitizeResult(result, sp)

	// Short secrets should NOT be redacted (too risky for false positives)
	if sanitized.Value != result.Value {
		t.Error("short secret should not be redacted")
	}
}

func TestExtractRequiredSecrets(t *testing.T) {
	tests := []struct {
		source   string
		expected []string
	}{
		{
			source:   "# @secrets: API_KEY\nresult = 1",
			expected: []string{"API_KEY"},
		},
		{
			source:   "# @secrets: KEY1, KEY2, KEY3\nresult = 1",
			expected: []string{"KEY1", "KEY2", "KEY3"},
		},
		{
			source:   "# @secret: SINGLE\nresult = 1",
			expected: []string{"SINGLE"},
		},
		{
			source:   "result = 1",
			expected: nil,
		},
		{
			source:   "# @description: test\n# @secrets: WEATHER_API\n\nresult = 1",
			expected: []string{"WEATHER_API"},
		},
	}

	for _, tt := range tests {
		got := ExtractRequiredSecrets(tt.source)
		if len(got) != len(tt.expected) {
			t.Errorf("ExtractRequiredSecrets(%q) = %v, want %v", tt.source[:20], got, tt.expected)
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("ExtractRequiredSecrets[%d] = %q, want %q", i, v, tt.expected[i])
			}
		}
	}
}

func TestSecretsInHTTPRequest(t *testing.T) {
	// This tests the real use case: using a secret in an HTTP request
	// and ensuring the response doesn't leak the secret

	s := New(Config{})
	sp := NewSecretProvider()
	sp.Set("API_KEY", "supersecretapikey123")
	s.InjectSecrets(sp)

	ctx := context.Background()

	// Script that uses the secret (we can't test actual HTTP here,
	// but we can test the secret is available and sanitization works)
	result := s.Execute(ctx, "test.star", `
api_key = secrets.get("API_KEY")
# In real use: http.get("https://api.example.com?key=" + api_key)
result = {
    "url": "https://api.example.com?key=" + api_key,
    "key_length": len(api_key)
}
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	// Now sanitize before returning to AI
	sanitized := SanitizeResult(result, sp)

	m := sanitized.Value.(map[string]any)
	url := m["url"].(string)

	// The URL should have the secret redacted
	if contains(url, "supersecretapikey123") {
		t.Error("secret in URL was not redacted")
	}
	if !contains(url, "[REDACTED:API_KEY]") {
		t.Errorf("expected redaction marker in URL: %s", url)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
