// Package starlark provides a sandboxed Starlark scripting environment.
// Scripts have no filesystem or network access by default - only
// explicitly provided built-in functions are available.
package starlark

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

// Sandbox executes Starlark scripts in a restricted environment
type Sandbox struct {
	mu             sync.Mutex
	maxExecutionMs int64
	enableHTTP     bool
	httpClient     *http.Client
	predeclared    starlark.StringDict
}

// Config configures the sandbox
type Config struct {
	MaxExecutionMs int64 // Maximum script execution time
	DisableHTTP    bool  // Disable HTTP requests (default: false, HTTP enabled)
}

// Result is the result of script execution
type Result struct {
	Value    any           `json:"value,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// New creates a new Starlark sandbox
func New(cfg Config) *Sandbox {
	if cfg.MaxExecutionMs <= 0 {
		cfg.MaxExecutionMs = 30000 // 30 seconds default
	}

	s := &Sandbox{
		maxExecutionMs: cfg.MaxExecutionMs,
		enableHTTP:     !cfg.DisableHTTP, // Enable HTTP by default unless disabled
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		predeclared: make(starlark.StringDict),
	}

	// Add safe built-in functions
	s.addBuiltins()

	return s
}

// addBuiltins adds safe built-in functions to the sandbox
func (s *Sandbox) addBuiltins() {
	// JSON module
	jsonModule := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"encode": starlark.NewBuiltin("json.encode", jsonEncode),
		"decode": starlark.NewBuiltin("json.decode", jsonDecode),
	})
	s.predeclared["json"] = jsonModule

	// Time module (read-only)
	timeModule := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
		"now":   starlark.NewBuiltin("time.now", timeNow),
		"sleep": starlark.NewBuiltin("time.sleep", timeSleep),
	})
	s.predeclared["time"] = timeModule

	// HTTP module (if enabled)
	if s.enableHTTP {
		httpModule := starlarkstruct.FromStringDict(starlarkstruct.Default, starlark.StringDict{
			"get":  starlark.NewBuiltin("http.get", s.httpGet),
			"post": starlark.NewBuiltin("http.post", s.httpPost),
		})
		s.predeclared["http"] = httpModule
	}

	// String utilities
	s.predeclared["format"] = starlark.NewBuiltin("format", formatString)
}

// Execute runs a Starlark script and returns the result
func (s *Sandbox) Execute(ctx context.Context, name, source string) Result {
	start := time.Now()

	// Create a cancellable context with timeout
	timeout := time.Duration(s.maxExecutionMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create thread with cancel checking
	thread := &starlark.Thread{
		Name: name,
		Print: func(_ *starlark.Thread, msg string) {
			// Silently ignore print statements for security
		},
	}

	// Set up cancellation
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("execution timeout or cancelled")
		case <-done:
		}
	}()
	defer close(done)

	// Execute the script
	s.mu.Lock()
	globals, err := starlark.ExecFile(thread, name, source, s.predeclared)
	s.mu.Unlock()

	duration := time.Since(start)

	if err != nil {
		return Result{
			Error:    err.Error(),
			Duration: duration,
		}
	}

	// Look for a "main" function or "result" variable
	var result any

	if mainFn, ok := globals["main"]; ok {
		if fn, ok := mainFn.(*starlark.Function); ok {
			// Call main()
			ret, err := starlark.Call(thread, fn, nil, nil)
			if err != nil {
				return Result{
					Error:    fmt.Sprintf("main() error: %v", err),
					Duration: duration,
				}
			}
			result = starlarkToGo(ret)
		}
	} else if resultVar, ok := globals["result"]; ok {
		result = starlarkToGo(resultVar)
	} else {
		// Return all exported globals
		exported := make(map[string]any)
		for name, val := range globals {
			if name[0] >= 'a' && name[0] <= 'z' {
				continue // Skip private (lowercase) names
			}
			exported[name] = starlarkToGo(val)
		}
		if len(exported) > 0 {
			result = exported
		}
	}

	return Result{
		Value:    result,
		Duration: duration,
	}
}

// ExecuteFunction runs a specific function in a script
func (s *Sandbox) ExecuteFunction(ctx context.Context, name, source, funcName string, args []any) Result {
	start := time.Now()

	timeout := time.Duration(s.maxExecutionMs) * time.Millisecond
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	thread := &starlark.Thread{Name: name}

	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			thread.Cancel("execution timeout or cancelled")
		case <-done:
		}
	}()
	defer close(done)

	// Parse and execute to get globals
	s.mu.Lock()
	globals, err := starlark.ExecFile(thread, name, source, s.predeclared)
	s.mu.Unlock()

	if err != nil {
		return Result{
			Error:    err.Error(),
			Duration: time.Since(start),
		}
	}

	// Find the function
	fnVal, ok := globals[funcName]
	if !ok {
		return Result{
			Error:    fmt.Sprintf("function %q not found", funcName),
			Duration: time.Since(start),
		}
	}

	fn, ok := fnVal.(*starlark.Function)
	if !ok {
		return Result{
			Error:    fmt.Sprintf("%q is not a function", funcName),
			Duration: time.Since(start),
		}
	}

	// Convert args to Starlark values
	starlarkArgs := make(starlark.Tuple, len(args))
	for i, arg := range args {
		starlarkArgs[i] = goToStarlark(arg)
	}

	// Call the function
	ret, err := starlark.Call(thread, fn, starlarkArgs, nil)
	if err != nil {
		return Result{
			Error:    err.Error(),
			Duration: time.Since(start),
		}
	}

	return Result{
		Value:    starlarkToGo(ret),
		Duration: time.Since(start),
	}
}

// AddFunction adds a custom function to the sandbox
func (s *Sandbox) AddFunction(name string, fn func(args []any) (any, error)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	builtin := starlark.NewBuiltin(name, func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		goArgs := make([]any, len(args))
		for i, arg := range args {
			goArgs[i] = starlarkToGo(arg)
		}

		result, err := fn(goArgs)
		if err != nil {
			return starlark.None, err
		}

		return goToStarlark(result), nil
	})

	s.predeclared[name] = builtin
}

// Built-in function implementations

func jsonEncode(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return starlark.None, fmt.Errorf("json.encode: expected 1 argument, got %d", len(args))
	}

	goVal := starlarkToGo(args[0])
	data, err := json.Marshal(goVal)
	if err != nil {
		return starlark.None, err
	}

	return starlark.String(data), nil
}

func jsonDecode(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return starlark.None, fmt.Errorf("json.decode: expected 1 argument, got %d", len(args))
	}

	str, ok := args[0].(starlark.String)
	if !ok {
		return starlark.None, fmt.Errorf("json.decode: expected string, got %s", args[0].Type())
	}

	var goVal any
	if err := json.Unmarshal([]byte(str), &goVal); err != nil {
		return starlark.None, err
	}

	return goToStarlark(goVal), nil
}

func timeNow(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return starlark.String(time.Now().UTC().Format(time.RFC3339)), nil
}

func timeSleep(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) != 1 {
		return starlark.None, fmt.Errorf("time.sleep: expected 1 argument (seconds), got %d", len(args))
	}

	var seconds float64
	switch v := args[0].(type) {
	case starlark.Int:
		i, _ := v.Int64()
		seconds = float64(i)
	case starlark.Float:
		seconds = float64(v)
	default:
		return starlark.None, fmt.Errorf("time.sleep: expected number, got %s", args[0].Type())
	}

	// Cap sleep to 5 seconds for safety
	if seconds > 5 {
		seconds = 5
	}
	if seconds > 0 {
		time.Sleep(time.Duration(seconds * float64(time.Second)))
	}

	return starlark.None, nil
}

func formatString(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if len(args) < 1 {
		return starlark.None, fmt.Errorf("format: expected at least 1 argument")
	}

	format, ok := args[0].(starlark.String)
	if !ok {
		return starlark.None, fmt.Errorf("format: first argument must be string")
	}

	goArgs := make([]any, len(args)-1)
	for i := 1; i < len(args); i++ {
		goArgs[i-1] = starlarkToGo(args[i])
	}

	return starlark.String(fmt.Sprintf(string(format), goArgs...)), nil
}

// httpGet performs an HTTP GET request
// Usage: http.get(url, headers={}) -> {"status": 200, "body": "...", "headers": {...}}
func (s *Sandbox) httpGet(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var headers *starlark.Dict

	if err := starlark.UnpackArgs("http.get", args, kwargs, "url", &urlStr, "headers?", &headers); err != nil {
		return starlark.None, err
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return starlark.None, fmt.Errorf("invalid URL: %w", err)
	}

	// Only allow http and https
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return starlark.None, fmt.Errorf("only http and https URLs are allowed")
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return starlark.None, err
	}

	// Add custom headers
	if headers != nil {
		for _, item := range headers.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				continue
			}
			val, ok := item[1].(starlark.String)
			if !ok {
				continue
			}
			req.Header.Set(string(key), string(val))
		}
	}

	// Set a reasonable user agent
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "OpenPact-Starlark/1.0")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return starlark.None, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read body (limit to 10MB)
	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return starlark.None, fmt.Errorf("failed to read response: %w", err)
	}

	// Build response headers dict
	respHeaders := starlark.NewDict(len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders.SetKey(starlark.String(key), starlark.String(values[0]))
		}
	}

	// Return response as dict
	result := starlark.NewDict(3)
	result.SetKey(starlark.String("status"), starlark.MakeInt(resp.StatusCode))
	result.SetKey(starlark.String("body"), starlark.String(body))
	result.SetKey(starlark.String("headers"), respHeaders)

	return result, nil
}

// httpPost performs an HTTP POST request
// Usage: http.post(url, body="", headers={}, content_type="application/json") -> {...}
func (s *Sandbox) httpPost(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var urlStr string
	var bodyStr string
	var headers *starlark.Dict
	var contentType string = "application/json"

	if err := starlark.UnpackArgs("http.post", args, kwargs,
		"url", &urlStr,
		"body?", &bodyStr,
		"headers?", &headers,
		"content_type?", &contentType); err != nil {
		return starlark.None, err
	}

	// Validate URL
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return starlark.None, fmt.Errorf("invalid URL: %w", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return starlark.None, fmt.Errorf("only http and https URLs are allowed")
	}

	req, err := http.NewRequest("POST", urlStr, strings.NewReader(bodyStr))
	if err != nil {
		return starlark.None, err
	}

	req.Header.Set("Content-Type", contentType)

	// Add custom headers
	if headers != nil {
		for _, item := range headers.Items() {
			key, ok := item[0].(starlark.String)
			if !ok {
				continue
			}
			val, ok := item[1].(starlark.String)
			if !ok {
				continue
			}
			req.Header.Set(string(key), string(val))
		}
	}

	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "OpenPact-Starlark/1.0")
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return starlark.None, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return starlark.None, fmt.Errorf("failed to read response: %w", err)
	}

	respHeaders := starlark.NewDict(len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			respHeaders.SetKey(starlark.String(key), starlark.String(values[0]))
		}
	}

	result := starlark.NewDict(3)
	result.SetKey(starlark.String("status"), starlark.MakeInt(resp.StatusCode))
	result.SetKey(starlark.String("body"), starlark.String(body))
	result.SetKey(starlark.String("headers"), respHeaders)

	return result, nil
}

// Type conversion helpers

func starlarkToGo(v starlark.Value) any {
	switch val := v.(type) {
	case starlark.NoneType:
		return nil
	case starlark.Bool:
		return bool(val)
	case starlark.Int:
		i, _ := val.Int64()
		return i
	case starlark.Float:
		return float64(val)
	case starlark.String:
		return string(val)
	case *starlark.List:
		result := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			result[i] = starlarkToGo(val.Index(i))
		}
		return result
	case starlark.Tuple:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = starlarkToGo(item)
		}
		return result
	case *starlark.Dict:
		result := make(map[string]any)
		for _, item := range val.Items() {
			key := starlarkToGo(item[0])
			if keyStr, ok := key.(string); ok {
				result[keyStr] = starlarkToGo(item[1])
			}
		}
		return result
	default:
		return val.String()
	}
}

func goToStarlark(v any) starlark.Value {
	switch val := v.(type) {
	case nil:
		return starlark.None
	case bool:
		return starlark.Bool(val)
	case int:
		return starlark.MakeInt(val)
	case int64:
		return starlark.MakeInt64(val)
	case float64:
		return starlark.Float(val)
	case string:
		return starlark.String(val)
	case []any:
		list := make([]starlark.Value, len(val))
		for i, item := range val {
			list[i] = goToStarlark(item)
		}
		return starlark.NewList(list)
	case map[string]any:
		dict := starlark.NewDict(len(val))
		for k, v := range val {
			dict.SetKey(starlark.String(k), goToStarlark(v))
		}
		return dict
	default:
		return starlark.String(fmt.Sprintf("%v", val))
	}
}
