package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/voler88/conslog"
)

// Level defines a custom log level type based on [slog.Level].
type Level slog.Level

// Log level constants matching slog's predefined levels.
const (
	LevelError = Level(slog.LevelError)
	LevelWarn  = Level(slog.LevelWarn)
	LevelInfo  = Level(slog.LevelInfo)
	LevelDebug = Level(slog.LevelDebug)
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

// Logger wraps [slog.Logger] and holds a pointer to a [slog.LevelVar] for dynamic log level control.
type Logger struct {
	*slog.Logger
	Level *slog.LevelVar
}

// New creates a new [Logger] instance with the specified output writer and handler type.
// If the handler type is invalid, it logs a warning and falls back to JSON handler.
func New(out io.Writer, handler HandlerType) *Logger {
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
		fmt.Fprintf(os.Stderr, "warning: invalid handler type %q, falling back to JSON\n", handler)
		h = slog.NewJSONHandler(out, opts)
	}

	return &Logger{slog.New(h), lvl}
}

// SetLevel sets the log level dynamically using a custom [Level] type.
// It converts the custom [Level] to [slog.Level] before setting.
func (l *Logger) SetLevel(level Level) {
	l.Level.Set(slog.Level(level))
}

// SetLevelByCounter sets the log level based on an integer counter.
// Commonly used with repeated verbosity flags (e.g., -v).
// 0 or less: Error, 1: Warn, 2: Info, 3 or more: Debug.
func (l *Logger) SetLevelByCounter(i int) {
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
func (l *Logger) SetLevelByName(name string) error {
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

// With returns a new [Logger] with additional key-value pairs added to the context.
// It preserves the dynamic log level variable.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		Logger: l.Logger.With(args...),
		Level:  l.Level,
	}
}

// WithGroup returns a new [Logger] that nests subsequent attributes under the given group name.
// It preserves the dynamic log level variable.
func (l *Logger) WithGroup(name string) *Logger {
	return &Logger{
		Logger: l.Logger.WithGroup(name),
		Level:  l.Level,
	}
}
