package logging

import (
	"io"
	"log/slog"
)

const (
	LevelError = iota
	LevelWarn
	LevelInfo
	LevelDebug
)

// Logger embeds [slog.Logger] and holds a pointer to the level variable.
type Logger struct {
	*slog.Logger
	Level *slog.LevelVar
}

// New creates a new [Logger]. The [slog.JSONHandler] is used by default, but for "console"
// a [ConsoleHandler] is used to output pretty and colorful logs.
func New(out io.Writer, handler string) *Logger {
	l := &Logger{Level: new(slog.LevelVar)}
	l.Level.Set(slog.LevelError)
	opts := &slog.HandlerOptions{Level: l.Level}
	switch handler {
	case "console":
		l.Logger = slog.New(NewConsoleHandler(out, opts))
	default:
		l.Logger = slog.New(slog.NewJSONHandler(out, opts))
	}
	return l
}

// SetLevel sets [Logger] verbosity level by counter.
func (l *Logger) SetLevel(i int) error {
	var s string
	switch {
	case i <= LevelError:
		s = "ERROR"
	case i == LevelWarn:
		s = "WARN"
	case i == LevelInfo:
		s = "INFO"
	case i >= LevelDebug:
		s = "DEBUG"
	}
	return l.Level.UnmarshalText([]byte(s))
}

// With returns a [Logger] with the provided key-value pairs added to the context.
// These values will be included in all subsequent log entries.
func (l *Logger) With(args ...any) *Logger {
	l.Logger = l.Logger.With(args...)
	return l
}

// WithGroup returns a [Logger] that creates a named group for the log attributes.
// All key-value pairs added after will be nested under the group name.
func (l *Logger) WithGroup(name string) *Logger {
	l.Logger = l.Logger.WithGroup(name)
	return l
}
