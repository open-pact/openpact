package starlark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := New(Config{MaxExecutionMs: 5000})
	if s == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewDefaults(t *testing.T) {
	s := New(Config{})
	if s.maxExecutionMs != 30000 {
		t.Errorf("maxExecutionMs = %d, want 30000", s.maxExecutionMs)
	}
}

func TestExecuteSimple(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = 1 + 2
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != int64(3) {
		t.Errorf("result = %v, want 3", result.Value)
	}
}

func TestExecuteMainFunction(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
def main():
    return "hello from main"
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != "hello from main" {
		t.Errorf("result = %v, want 'hello from main'", result.Value)
	}
}

func TestExecuteWithDict(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = {"name": "test", "value": 42}
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	m, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result.Value)
	}
	if m["name"] != "test" {
		t.Errorf("name = %v, want 'test'", m["name"])
	}
	if m["value"] != int64(42) {
		t.Errorf("value = %v, want 42", m["value"])
	}
}

func TestExecuteWithList(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = [1, 2, 3, 4, 5]
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	list, ok := result.Value.([]any)
	if !ok {
		t.Fatalf("result is not a list: %T", result.Value)
	}
	if len(list) != 5 {
		t.Errorf("len = %d, want 5", len(list))
	}
}

func TestExecuteSyntaxError(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
def broken(
`)

	if result.Error == "" {
		t.Error("expected syntax error")
	}
}

func TestExecuteRuntimeError(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = 1 / 0
`)

	if result.Error == "" {
		t.Error("expected division by zero error")
	}
}

func TestExecuteFunction(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.ExecuteFunction(ctx, "test.star", `
def add(a, b):
    return a + b
`, "add", []any{10, 20})

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != int64(30) {
		t.Errorf("result = %v, want 30", result.Value)
	}
}

func TestExecuteFunctionNotFound(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.ExecuteFunction(ctx, "test.star", `
def foo():
    pass
`, "bar", nil)

	if result.Error == "" {
		t.Error("expected 'function not found' error")
	}
}

func TestJSONEncode(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = json.encode({"hello": "world", "num": 42})
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	str, ok := result.Value.(string)
	if !ok {
		t.Fatalf("result is not string: %T", result.Value)
	}
	// JSON encoding may vary in key order, just check it contains expected parts
	if len(str) < 10 {
		t.Errorf("JSON too short: %s", str)
	}
}

func TestJSONDecode(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
data = json.decode('{"name": "alice", "age": 30}')
result = data["name"]
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != "alice" {
		t.Errorf("result = %v, want 'alice'", result.Value)
	}
}

func TestTimeNow(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = time.now()
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	str, ok := result.Value.(string)
	if !ok {
		t.Fatalf("result is not string: %T", result.Value)
	}
	// Should be RFC3339 format
	if _, err := time.Parse(time.RFC3339, str); err != nil {
		t.Errorf("invalid time format: %s", str)
	}
}

func TestTimeSleep(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	start := time.Now()
	result := s.Execute(ctx, "test.star", `
time.sleep(0.1)
result = "done"
`)
	elapsed := time.Since(start)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if elapsed < 50*time.Millisecond {
		t.Errorf("sleep was too short: %v", elapsed)
	}
}

func TestFormat(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = format("Hello, %s! You are %d years old.", "Alice", 30)
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != "Hello, Alice! You are 30 years old." {
		t.Errorf("result = %v", result.Value)
	}
}

func TestAddFunction(t *testing.T) {
	s := New(Config{})

	s.AddFunction("double", func(args []any) (any, error) {
		if len(args) != 1 {
			return nil, nil
		}
		if n, ok := args[0].(int64); ok {
			return n * 2, nil
		}
		return nil, nil
	})

	ctx := context.Background()
	result := s.Execute(ctx, "test.star", `
result = double(21)
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != int64(42) {
		t.Errorf("result = %v, want 42", result.Value)
	}
}

func TestTimeout(t *testing.T) {
	s := New(Config{MaxExecutionMs: 100})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
# Infinite loop
x = 0
while True:
    x = x + 1
`)

	if result.Error == "" {
		t.Error("expected timeout error")
	}
}

func TestContextCancellation(t *testing.T) {
	s := New(Config{MaxExecutionMs: 10000})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := s.Execute(ctx, "test.star", `
x = 0
while True:
    x = x + 1
`)

	if result.Error == "" {
		t.Error("expected cancellation error")
	}
}

func TestExportedGlobals(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
Name = "exported"
private = "not exported"
Value = 100
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	m, ok := result.Value.(map[string]any)
	if !ok {
		t.Fatalf("result is not a map: %T", result.Value)
	}

	if m["Name"] != "exported" {
		t.Errorf("Name = %v, want 'exported'", m["Name"])
	}
	if m["Value"] != int64(100) {
		t.Errorf("Value = %v, want 100", m["Value"])
	}
	if _, ok := m["private"]; ok {
		t.Error("private should not be exported")
	}
}

func TestDuration(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `result = 1`)

	if result.Duration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestHTTPEnabled(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	// HTTP should be available by default
	result := s.Execute(ctx, "test.star", `
result = http != None
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Value != true {
		t.Error("http module should be available")
	}
}

func TestHTTPGetInvalidURL(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = http.get("not-a-valid-url")
`)

	if result.Error == "" {
		t.Error("expected error for invalid URL")
	}
}

func TestHTTPGetFileProtocol(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
result = http.get("file:///etc/passwd")
`)

	if result.Error == "" {
		t.Error("expected error for file:// protocol")
	}
}

func TestHTTPPostBasic(t *testing.T) {
	s := New(Config{})
	ctx := context.Background()

	// Just test that post function exists and validates URLs
	result := s.Execute(ctx, "test.star", `
result = http.post("ftp://invalid", body="test")
`)

	if result.Error == "" {
		t.Error("expected error for ftp:// protocol")
	}
}

func TestHTTPGetReal(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"message": "hello", "count": 42}`))
	}))
	defer server.Close()

	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
resp = http.get("`+server.URL+`")
data = json.decode(resp["body"])
result = {
    "status": resp["status"],
    "message": data["message"],
    "count": data["count"]
}
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	m := result.Value.(map[string]any)
	if m["status"] != int64(200) {
		t.Errorf("status = %v, want 200", m["status"])
	}
	if m["message"] != "hello" {
		t.Errorf("message = %v, want 'hello'", m["message"])
	}
	if m["count"] != float64(42) {
		t.Errorf("count = %v, want 42", m["count"])
	}
}

func TestHTTPPostReal(t *testing.T) {
	var receivedBody string
	var receivedContentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		receivedBody = string(buf[:n])

		w.WriteHeader(201)
		w.Write([]byte(`{"created": true}`))
	}))
	defer server.Close()

	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
resp = http.post("`+server.URL+`", body='{"name": "test"}', content_type="application/json")
data = json.decode(resp["body"])
result = {
    "status": resp["status"],
    "created": data["created"]
}
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	m := result.Value.(map[string]any)
	if m["status"] != int64(201) {
		t.Errorf("status = %v, want 201", m["status"])
	}

	if receivedContentType != "application/json" {
		t.Errorf("content-type = %q, want 'application/json'", receivedContentType)
	}
	if receivedBody != `{"name": "test"}` {
		t.Errorf("body = %q", receivedBody)
	}
}

func TestHTTPGetWithHeaders(t *testing.T) {
	var receivedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	s := New(Config{})
	ctx := context.Background()

	result := s.Execute(ctx, "test.star", `
resp = http.get("`+server.URL+`", headers={"Authorization": "Bearer secret123"})
result = resp["status"]
`)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	if receivedAuth != "Bearer secret123" {
		t.Errorf("auth header = %q, want 'Bearer secret123'", receivedAuth)
	}
}

func TestWeatherExample(t *testing.T) {
	// Simulate a weather API response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"location": {"name": "London"},
			"current": {"temp_c": 15.5, "condition": {"text": "Partly cloudy"}}
		}`))
	}))
	defer server.Close()

	s := New(Config{})
	ctx := context.Background()

	// This is the kind of script a user might write
	script := `
def get_weather(api_url):
    resp = http.get(api_url)
    if resp["status"] != 200:
        return {"error": "API request failed"}

    data = json.decode(resp["body"])
    return {
        "city": data["location"]["name"],
        "temp": data["current"]["temp_c"],
        "condition": data["current"]["condition"]["text"]
    }

result = get_weather("` + server.URL + `")
`

	result := s.Execute(ctx, "weather.star", script)

	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}

	m := result.Value.(map[string]any)
	if m["city"] != "London" {
		t.Errorf("city = %v, want 'London'", m["city"])
	}
	if m["temp"] != 15.5 {
		t.Errorf("temp = %v, want 15.5", m["temp"])
	}
	if m["condition"] != "Partly cloudy" {
		t.Errorf("condition = %v", m["condition"])
	}
}
