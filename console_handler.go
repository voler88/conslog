/*
Package conslog provides a high-performance, colorized console handler for [slog].
Features include:
- thread-safe logging with mutex-protected writes
- colorized output with ANSI escape codes
- pretty-printed JSON for complex values
- pooled resources to minimize allocations
- lazy-initialized indentation cache

	Example output:
	  [01:04:17.289] DEBUG: used environment variable names
	    user: "gopher"
	      envs: [
	        "AUTH",
	        "CONFIG",
	        "VERBOSITY"
	      ]
*/
package conslog

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
)

const timeFormat = "[15:04:05.000]"

// ANSI escape codes for terminal colors.
const (
	ansiCyan         = "\033[36m"
	ansiLightGray    = "\033[37m"
	ansiDarkGray     = "\033[90m"
	ansiLightRed     = "\033[91m"
	ansiLightYellow  = "\033[93m"
	ansiLightBlue    = "\033[94m"
	ansiLightMagenta = "\033[95m"
	ansiWhite        = "\033[97m"
	ansiReset        = "\033[0m" // reset all styles
)

// colorize wraps text with ANSI color codes and reset sequence,
// safe for concurrent use and has no allocations.
func colorize(ansiColor, v string) string {
	return ansiColor + v + ansiReset
}

type jsonEncoder struct {
	enc *json.Encoder
	buf *bytes.Buffer // reused buffer to avoid allocations
}

// Encode marshals values to indented JSON strings.
// Returns empty string on error (caller should handle errors).
func (p *jsonEncoder) Encode(v any) (string, error) {
	p.buf.Reset()
	err := p.enc.Encode(v)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(p.buf.String(), "\n"), nil
}

// Shared pools for reusable resources.
var (
	// builderPool recycles [strings.Builder] to reduce allocations
	builderPool = sync.Pool{
		New: func() any { return new(strings.Builder) },
	}
	// encoderPool recycles JSON encoders with pre-configured indentation
	encoderPool = sync.Pool{
		New: func() any {
			buf := new(bytes.Buffer)
			enc := json.NewEncoder(buf)
			return &jsonEncoder{enc: enc, buf: buf}
		},
	}
)

// Lazy indentation cache variables.
const maxIndent = 30                     // cache indentation strings up to 30 levels (60 spaces, fits in wider terminals)
var indentCache atomic.Pointer[[]string] // atomic pointer for lock-free reads

// getIndent returns a cached indentation string or lazy generates new ones with fallback,
// safe for concurrent use with atomic operations.
func getIndent(indent int) string {
	cache := indentCache.Load()
	if cache == nil {
		// initialize cache once with common indentation levels
		newCache := make([]string, maxIndent+1)
		for i := 0; i <= maxIndent; i++ {
			newCache[i] = strings.Repeat("  ", i)
		}
		indentCache.Store(&newCache)
		cache = indentCache.Load()
	}

	if indent <= maxIndent {
		return (*cache)[indent]
	}
	return strings.Repeat("  ", indent) // fallback for deep nesting
}

// normalizeKey replaces empty keys with a quoted empty string
// to ensure keys are never empty in the output.
func normalizeKey(key string) string {
	if key == "" {
		return `""`
	}
	return key
}

// ConsoleHandler implements [slog.Handler] for colorized terminal output.
type ConsoleHandler struct {
	opts           slog.HandlerOptions // slog configuration
	indent         int                 // current indentation level
	unopenedGroups []string            // pending group names to indent
	preBuf         strings.Builder     // buffered attributes from WithAttrs/WithGroup
	mu             *sync.Mutex         // protects writes to output
	w              io.Writer           // output destination
}

// NewConsoleHandler returns new [ConsoleHandler] instance.
// opts: optional handler configuration (nil uses defaults)
// w: output writer (e.g., os.Stderr, os.Stdout)
func NewConsoleHandler(w io.Writer, opts *slog.HandlerOptions) *ConsoleHandler {
	h := &ConsoleHandler{
		w:      w,
		mu:     new(sync.Mutex),
		indent: 1, // base indentation level
	}

	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo // default to Info level
	}

	return h
}

// Enabled checks if the handler should process this log level,
// implements [slog.Handler] interface.
func (h *ConsoleHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle processes log records and writes formatted output,
// implements [slog.Handler] interface.
func (h *ConsoleHandler) Handle(ctx context.Context, r slog.Record) error {
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	defer builderPool.Put(b) // return to pool when done

	// format timestamp if present
	if !r.Time.IsZero() {
		b.WriteString(colorize(ansiLightGray, r.Time.Format(timeFormat)))
		b.WriteByte(' ')
	}

	// format log level with color coding
	levelStr := r.Level.String() + ":"
	switch {
	case r.Level <= slog.LevelDebug:
		levelStr = colorize(ansiLightGray, levelStr)
	case r.Level <= slog.LevelInfo:
		levelStr = colorize(ansiCyan, levelStr)
	case r.Level < slog.LevelWarn:
		levelStr = colorize(ansiLightBlue, levelStr)
	case r.Level < slog.LevelError:
		levelStr = colorize(ansiLightYellow, levelStr)
	case r.Level <= slog.LevelError+1:
		levelStr = colorize(ansiLightRed, levelStr)
	default:
		levelStr = colorize(ansiLightMagenta, levelStr)
	}
	b.WriteString(levelStr)
	b.WriteByte(' ')

	// format message and pre-buffered attributes
	b.WriteString(colorize(ansiWhite, r.Message))
	b.WriteByte('\n')
	b.WriteString(h.preBuf.String())

	// process record attributes if present
	if r.NumAttrs() > 0 {
		h.appendUnopenedGroups(b, h.indent)
		r.Attrs(func(a slog.Attr) bool {
			h.appendAttr(b, a, h.indent+len(h.unopenedGroups))
			return true
		})
	}

	// write final output with mutex protection
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := io.WriteString(h.w, b.String())
	return err
}

// WithAttrs creates a new handler with additional attributes,
// implements [slog.Handler] interface.
// Attributes are buffered until the next log record is processed.
func (h *ConsoleHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h // no-op for empty attributes
	}

	h2 := *h // copy base handler

	// copy pre-buffer using pooled builder
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	b.WriteString(h.preBuf.String())
	h2.preBuf.Reset()
	h2.preBuf.WriteString(b.String())
	builderPool.Put(b)

	// process new attributes
	h2.appendUnopenedGroups(&h2.preBuf, h2.indent)
	h2.indent += len(h2.unopenedGroups)
	h2.unopenedGroups = nil

	for _, a := range attrs {
		h2.appendAttr(&h2.preBuf, a, h2.indent)
	}

	return &h2
}

// WithGroup creates a new handler with additional grouping,
// implements [slog.Handler] interface.
// Groups are applied to subsequent attributes and log records.
func (h *ConsoleHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h // no-op for empty group names
	}

	h2 := *h
	// clone existing groups to prevent shared slice mutations
	h2.unopenedGroups = append(slices.Clone(h.unopenedGroups), name)

	return &h2
}

// appendUnopenedGroups flushes pending groups to the buffer.
func (h *ConsoleHandler) appendUnopenedGroups(b *strings.Builder, indent int) {
	for _, g := range h.unopenedGroups {
		b.WriteString(getIndent(indent))
		b.WriteString(colorize(ansiDarkGray, g+":\n"))
		indent++
	}
}

// appendAttr formats a single attribute with proper indentation.
func (h *ConsoleHandler) appendAttr(b *strings.Builder, a slog.Attr, indent int) {
	a.Value = a.Value.Resolve()
	prefix := getIndent(indent)

	if a.Equal(slog.Attr{}) {
		return // skip empty attributes
	}

	switch a.Value.Kind() {
	case slog.KindGroup:
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return // skip empty groups
		}
		if a.Key != "" {
			b.WriteString(prefix)
			b.WriteString(colorize(ansiDarkGray, a.Key+":\n"))
			indent++ // increase indent for group members
		}
		for _, ga := range attrs {
			h.appendAttr(b, ga, indent)
		}

	case slog.KindString:
		key := normalizeKey(a.Key)

		b.WriteString(prefix)
		b.WriteString(colorize(ansiDarkGray, key+": \""+a.Value.String()+"\"\n"))

	default:
		key := normalizeKey(a.Key)

		b.WriteString(prefix)

		// use pooled encoder for JSON formatting
		encoder := encoderPool.Get().(*jsonEncoder)
		encoder.enc.SetIndent(prefix, "  ") // consistent 2-space indentation
		data, err := encoder.Encode(a.Value.Any())
		encoderPool.Put(encoder)
		if err != nil {
			// panic is intentional - invalid values should fail fast
			panic(fmt.Sprintf("log marshaling error: %v", err))
		}

		jsonStr := strings.TrimRight(string(data), "\n")
		b.WriteString(colorize(ansiDarkGray, key+": "))
		b.WriteString(colorize(ansiDarkGray, jsonStr))
		b.WriteByte('\n')
	}
}
