// Package logging provides structured logging for OpenPact.
// Supports log levels, JSON output, and context-aware logging.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Level represents a logging level
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLevel parses a string into a Level
func ParseLevel(s string) Level {
	switch s {
	case "debug", "DEBUG":
		return LevelDebug
	case "info", "INFO":
		return LevelInfo
	case "warn", "WARN", "warning", "WARNING":
		return LevelWarn
	case "error", "ERROR":
		return LevelError
	default:
		return LevelInfo
	}
}

// Entry represents a single log entry
type Entry struct {
	Time    time.Time         `json:"time"`
	Level   string            `json:"level"`
	Message string            `json:"message"`
	Fields  map[string]any    `json:"fields,omitempty"`
}

// Logger is a structured logger
type Logger struct {
	mu     sync.Mutex
	level  Level
	output io.Writer
	json   bool
	fields map[string]any
}

// Config configures a Logger
type Config struct {
	Level      Level
	Output     io.Writer
	JSONFormat bool
}

// New creates a new Logger with the given config
func New(cfg Config) *Logger {
	if cfg.Output == nil {
		cfg.Output = os.Stdout
	}
	return &Logger{
		level:  cfg.Level,
		output: cfg.Output,
		json:   cfg.JSONFormat,
		fields: make(map[string]any),
	}
}

// Default creates a logger with default settings
func Default() *Logger {
	return New(Config{
		Level:  LevelInfo,
		Output: os.Stdout,
	})
}

// WithField returns a new logger with the given field added
func (l *Logger) WithField(key string, value any) *Logger {
	newLogger := &Logger{
		level:  l.level,
		output: l.output,
		json:   l.json,
		fields: make(map[string]any),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields returns a new logger with the given fields added
func (l *Logger) WithFields(fields map[string]any) *Logger {
	newLogger := &Logger{
		level:  l.level,
		output: l.output,
		json:   l.json,
		fields: make(map[string]any),
	}
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}
	for k, v := range fields {
		newLogger.fields[k] = v
	}
	return newLogger
}

// log writes a log entry
func (l *Logger) log(level Level, msg string, args ...any) {
	if level < l.level {
		return
	}

	entry := Entry{
		Time:    time.Now().UTC(),
		Level:   level.String(),
		Message: fmt.Sprintf(msg, args...),
	}

	if len(l.fields) > 0 {
		entry.Fields = l.fields
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.json {
		data, _ := json.Marshal(entry)
		fmt.Fprintln(l.output, string(data))
	} else {
		// Human-readable format
		fieldsStr := ""
		if len(entry.Fields) > 0 {
			for k, v := range entry.Fields {
				fieldsStr += fmt.Sprintf(" %s=%v", k, v)
			}
		}
		fmt.Fprintf(l.output, "%s [%s] %s%s\n",
			entry.Time.Format("2006-01-02T15:04:05Z"),
			entry.Level,
			entry.Message,
			fieldsStr,
		)
	}
}

// Debug logs at debug level
func (l *Logger) Debug(msg string, args ...any) {
	l.log(LevelDebug, msg, args...)
}

// Info logs at info level
func (l *Logger) Info(msg string, args ...any) {
	l.log(LevelInfo, msg, args...)
}

// Warn logs at warn level
func (l *Logger) Warn(msg string, args ...any) {
	l.log(LevelWarn, msg, args...)
}

// Error logs at error level
func (l *Logger) Error(msg string, args ...any) {
	l.log(LevelError, msg, args...)
}

// SetLevel changes the logging level
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetOutput changes the output writer
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.output = w
}

// SetJSONFormat enables or disables JSON output
func (l *Logger) SetJSONFormat(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.json = enabled
}
