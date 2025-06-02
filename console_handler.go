package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"
)

const (
	timeFormat = "[15:04:05.000]"

	cyan         = 36
	lightGray    = 37
	darkGray     = 90
	lightRed     = 91
	lightYellow  = 93
	lightBlue    = 94
	lightMagenta = 95
	white        = 97
)

// colorize returns the input string v wrapped in ANSI escape codes corresponding to the given colorCode.
func colorize(colorCode int, v string) string {
	return fmt.Sprintf("\033[%sm%s\033[0m", strconv.Itoa(colorCode), v)
}

/*
ConsoleHandler is a [slog.Handler] that writes colorized records to an io.Writer for console output.
String atrributes and groups prints with an appropriate indentation level, its content prints as pretty JSON.

	Example output:
	[01:04:17.289] DEBUG: used environment variable names
	  user: "gopher"
	    envs: [
	      "AUTH",
	      "CONFIG",
	      "VERBOSITY"
	    ]
*/
type ConsoleHandler struct {
	opts           slog.HandlerOptions
	indent         int
	unopenedGroups []string
	preBuf         strings.Builder
	buf            strings.Builder
	mu             *sync.Mutex
	w              io.Writer
}

// NewConsoleHandler returns new [ConsoleHandler] instance.
func NewConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *ConsoleHandler {
	h := &ConsoleHandler{w: w, mu: &sync.Mutex{}, indent: 1}

	if opts != nil {
		h.opts = *opts
	}

	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}

	return h
}

// Enabled reports whether the handler handles records at the given level.
// The handler ignores records whose level is lower.
func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle formats its argument [slog.Record] with different color per level and time format.
// The attributes following the message and their values are pretty formatted like
// JSON objects on next lines.
func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	h.buf = strings.Builder{}

	// Format the time attribute if available.
	timeAttr := slog.Attr{
		Key:   slog.TimeKey,
		Value: slog.StringValue(r.Time.Format(timeFormat)),
	}
	if !timeAttr.Equal(slog.Attr{}) && !r.Time.IsZero() {
		h.buf.WriteString(colorize(lightGray, timeAttr.Value.String()))
		h.buf.WriteString(" ")
	}

	// Format the level attribute with different colors by log level.
	levelAttr := slog.Attr{
		Key:   slog.LevelKey,
		Value: slog.AnyValue(r.Level),
	}
	if !levelAttr.Equal(slog.Attr{}) {
		level := levelAttr.Value.String() + ":"
		if r.Level <= slog.LevelDebug {
			level = colorize(lightGray, level)
		} else if r.Level <= slog.LevelInfo {
			level = colorize(cyan, level)
		} else if r.Level < slog.LevelWarn {
			level = colorize(lightBlue, level)
		} else if r.Level < slog.LevelError {
			level = colorize(lightYellow, level)
		} else if r.Level <= slog.LevelError+1 {
			level = colorize(lightRed, level)
		} else if r.Level > slog.LevelError+1 {
			level = colorize(lightMagenta, level)
		}
		h.buf.WriteString(level)
		h.buf.WriteString(" ")
	}

	// Format and write the log message in white.
	msgAttr := slog.Attr{
		Key:   slog.MessageKey,
		Value: slog.StringValue(r.Message),
	}
	if !msgAttr.Equal(slog.Attr{}) {
		h.buf.WriteString(colorize(white, msgAttr.Value.String()))
		h.buf.WriteString(" ")
	}

	// End header line, and then print pre-buffer (groups and attributes added via WithAttrs/WithGroup).
	h.buf.WriteString("\n")
	h.buf.WriteString(h.preBuf.String())

	// Append each record attribute formatted with appropriate indentation.
	if r.NumAttrs() > 0 {
		h.appendUnopenedGroups(&h.buf, h.indent)
		r.Attrs(func(a slog.Attr) bool {
			h.appendAttr(&h.buf, a, h.indent+len(h.unopenedGroups))
			return true
		})
	}

	// Lock and write the formatted log entry to the output writer.
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, h.buf.String())
	return err
}

// WithAttrs returns a new [ConsoleHandler] with the given attributes appended
// to any pre‑existing attributes.
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}

	h2 := *h

	h2.preBuf = strings.Builder{}
	h2.preBuf.WriteString(h.preBuf.String())

	h2.appendUnopenedGroups(&h2.preBuf, h2.indent)
	h2.indent += len(h2.unopenedGroups)
	h2.unopenedGroups = nil

	for _, a := range attrs {
		h2.appendAttr(&h2.preBuf, a, h2.indent)
	}

	return &h2
}

// WithGroup returns a new [ConsoleHandler] with the given group appended to the receiver's existing groups.
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	h2 := *h

	h2.unopenedGroups = make([]string, len(h.unopenedGroups)+1)
	copy(h2.unopenedGroups, h.unopenedGroups)
	h2.unopenedGroups[len(h2.unopenedGroups)-1] = name

	return &h2
}

// appendUnopenedGroups formats groups using colorize and indentation. Each group on a new line.
func (h *ConsoleHandler) appendUnopenedGroups(buf *strings.Builder, indent int) {
	for _, g := range h.unopenedGroups {
		fmt.Fprintf(buf, "%*s%s\n", indent*2, "", colorize(darkGray, fmt.Sprintf("%s:", g)))
		indent++
	}
}

// appendAttr formats attributes using colorize and indentation and
// their non-string values as JSON objects. Each key-value pair on a new line.
func (h *ConsoleHandler) appendAttr(buf *strings.Builder, a slog.Attr, indent int) {
	a.Value = a.Value.Resolve()
	prefix := fmt.Sprintf("%*s", indent*2, "")

	// Ignore empty Attrs.
	if a.Equal(slog.Attr{}) {
		return
	}

	switch a.Value.Kind() {
	case slog.KindGroup:
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return
		}
		if a.Key != "" {
			buf.WriteString(prefix)
			fmt.Fprintf(buf, "%s\n", colorize(darkGray, fmt.Sprintf("%s:", a.Key)))
			indent++
		}
		for _, ga := range attrs {
			h.appendAttr(buf, ga, indent)
		}

	case slog.KindString:
		buf.WriteString(prefix)
		if a.Key == "" {
			a.Key = `""`
		}
		fmt.Fprintf(buf, "%s\n", colorize(darkGray, fmt.Sprintf("%s: %q", a.Key, a.Value.String())))

	default:
		buf.WriteString(prefix)
		if a.Key == "" {
			a.Key = `""`
		}
		data, err := json.MarshalIndent(a.Value.Any(), prefix, "  ")
		if err != nil {
			panic(fmt.Sprintf("log marshaling error: %s", err.Error()))
		}
		fmt.Fprintf(buf, "%s\n", colorize(darkGray, fmt.Sprintf("%s: %s", a.Key, data)))
	}
}
