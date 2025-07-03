package logging

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/voler88/conslog"
)

// Level is an alias for [slog.Level], representing log severity levels.
type Level = slog.Level

// Log level constants matching slog's predefined levels.
const (
	LevelError = slog.LevelError
	LevelWarn  = slog.LevelWarn
	LevelInfo  = slog.LevelInfo
	LevelDebug = slog.LevelDebug
)

// HandlerType represents the type of log output handler.
type HandlerType string

// Supported handler types for log output formatting.
const (
	Console HandlerType = "console" // custom console handler with pretty output
	JSON    HandlerType = "json"    // JSON formatted output
	Text    HandlerType = "text"    // plain text output using slog's standard text handler
)

// String implements [fmt.Stringer] for [HandlerType].
func (h HandlerType) String() string {
	return string(h)
}

// IsValid checks if the [HandlerType] is one of the supported types.
func (h HandlerType) IsValid() bool {
	switch h {
	case Console, Text, JSON:
		return true
	}
	return false
}

// Logger interface for logging with dynamic level control.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)

	With(args ...any) Logger
	WithGroup(name string) Logger

	SetLevel(level Level)
	SetLevelByCounter(i int)
	SetLevelByName(name string) error
}

// logger uses [slog.Logger] as the underlying logger and holds a pointer to a
// [slog.LevelVar] for dynamic log level control.
type logger struct {
	logger *slog.Logger
	level  *slog.LevelVar
}

// NewLogger creates a [Logger] with the specified output writer and handler type.
// If the handler type is invalid, it logs a warning and falls back to JSON handler.
func NewLogger(out io.Writer, handler HandlerType) Logger {
	lvl := new(slog.LevelVar)
	opts := &slog.HandlerOptions{Level: lvl}

	var h slog.Handler
	switch handler {
	case Console:
		h = conslog.NewConsoleHandler(out, opts)
	case Text:
		h = slog.NewTextHandler(out, opts)
	case JSON:
		h = slog.NewJSONHandler(out, opts)
	default:
		fmt.Fprintf(out, "warning: invalid handler type %q, falling back to JSON\n", handler)
		h = slog.NewJSONHandler(out, opts)
	}

	return &logger{slog.New(h), lvl}
}

// Debug logs a message at Debug level with optional key-value pairs.
func (l *logger) Debug(msg string, args ...any) {
	l.logger.Debug(msg, args...)
}

// Info logs a message at Info level with optional key-value pairs.
func (l *logger) Info(msg string, args ...any) {
	l.logger.Info(msg, args...)
}

// Warn logs a message at Warn level with optional key-value pairs.
func (l *logger) Warn(msg string, args ...any) {
	l.logger.Warn(msg, args...)
}

// Error logs a message at Error level with optional key-value pairs.
func (l *logger) Error(msg string, args ...any) {
	l.logger.Error(msg, args...)
}

// SetLevel sets the log level dynamically.
func (l *logger) SetLevel(level Level) {
	l.level.Set(level)
}

// SetLevelByCounter sets the log level based on an integer counter.
// Commonly used with repeated verbosity flags (e.g., -v).
// 0 or less: Error, 1: Warn, 2: Info, 3 or more: Debug.
func (l *logger) SetLevelByCounter(i int) {
	var lvl Level
	switch {
	case i >= 3:
		lvl = LevelDebug
	case i == 2:
		lvl = LevelInfo
	case i == 1:
		lvl = LevelWarn
	default:
		lvl = LevelError
	}
	l.SetLevel(lvl)
}

// SetLevelByName sets the log level by parsing a string name (case-insensitive).
// Valid names: "error", "warn", "info", "debug".
// Returns an error if the name is invalid.
func (l *logger) SetLevelByName(name string) error {
	switch strings.ToLower(name) {
	case "error":
		l.SetLevel(LevelError)
	case "warn", "warning":
		l.SetLevel(LevelWarn)
	case "info":
		l.SetLevel(LevelInfo)
	case "debug":
		l.SetLevel(LevelDebug)
	default:
		return fmt.Errorf(
			"invalid log level name %q: must be one of error, warn, info, debug",
			name,
		)
	}
	return nil
}

// With returns a [Logger] with additional key-value pairs added to the context.
// It preserves the dynamic log level variable.
func (l *logger) With(args ...any) Logger {
	return &logger{l.logger.With(args...), l.level}
}

// WithGroup returns a [Logger] that nests subsequent attributes under the given group name.
// It preserves the dynamic log level variable.
func (l *logger) WithGroup(name string) Logger {
	return &logger{l.logger.WithGroup(name), l.level}
}
