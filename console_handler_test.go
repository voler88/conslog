package conslog_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/voler88/conslog"
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

	maxIndent = 30
)

// ansi returns the ANSI escape sequence for a given color code.
func ansi(code int) string {
	return "\033[" + strconv.Itoa(code) + "m"
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func uncolorize(_ *testing.T, s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

// parseLogEntryHeader parses a log entry header (time, level, message).
func parseLogEntryHeader(t *testing.T, line string) map[string]any {
	t.Helper()
	entry := make(map[string]any)
	line = strings.TrimSpace(line)
	first, rest, _ := strings.Cut(line, " ")
	cleanFirst := uncolorize(t, first)

	// try to parse time; if valid, first token is time
	if parsedTime, err := time.Parse(timeFormat, cleanFirst); err == nil {
		entry["time"] = parsedTime.Local().Format(time.RFC3339)
		level, msg, _ := strings.Cut(rest, " ")
		entry["level"] = strings.TrimSuffix(uncolorize(t, level), ":")
		entry["msg"] = uncolorize(t, msg)
	} else {
		entry["level"] = strings.TrimSuffix(cleanFirst, ":")
		entry["msg"] = uncolorize(t, rest)
	}
	return entry
}

// parseLogLines parses all log lines into structured entries.
func parseLogLines(t *testing.T, lines [][]byte) []map[string]any {
	t.Helper()
	var entries []map[string]any
	var groupKeys []string
	var expectedIndent int

	for _, lineBytes := range lines {
		line := string(lineBytes)
		line = strings.TrimRight(uncolorize(t, line), "\r\n")
		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if currentIndent%2 != 0 {
			t.Fatalf("indentation must be even, got %d: %q", currentIndent, line)
		}
		line = strings.TrimSpace(line)
		if len(line) == 0 {
			continue // skip empty lines
		}

		if currentIndent == 0 {
			// new top-level log entry
			entry := parseLogEntryHeader(t, line)
			entries = append(entries, entry)
			expectedIndent = 2
			groupKeys = nil
			continue
		}

		// non-zero indent: groups or attributes
		entry := entries[len(entries)-1]
		if len(entry) == 0 {
			t.Fatalf("log entry missing header for line: %q", line)
		}

		// adjust group keys stack if indent decreased
		for currentIndent < expectedIndent && len(groupKeys) > 0 {
			groupKeys = groupKeys[:len(groupKeys)-1]
			expectedIndent -= 2
		}

		// traverse nested groups to find current map context
		groupEntry := entry
		for _, key := range groupKeys {
			val, ok := groupEntry[key]
			if !ok {
				t.Fatalf("expected group %q not found in entry", key)
			}
			nestedMap, ok := val.(map[string]any)
			if !ok {
				t.Fatalf("value for group %q is not map[string]any", key)
			}
			groupEntry = nestedMap
		}

		// check if line is an attribute (key: value) or a new group (key:)
		if key, value, ok := strings.Cut(line, ": "); ok {
			// attribute line
			groupEntry[key] = strings.Trim(value, `"`)
		} else {
			// new group line
			trimmedKey := strings.TrimSuffix(line, ":")
			groupKeys = append(groupKeys, trimmedKey)
			groupEntry[trimmedKey] = make(map[string]any)
			expectedIndent += 2
		}
	}
	return entries
}

// TestSlogtest verifies compatibility with slogtest.TestHandler.
func TestSlogtest(t *testing.T) {
	var buf bytes.Buffer
	err := slogtest.TestHandler(conslog.NewConsoleHandler(&buf, nil), func() []map[string]any {
		lines := bytes.Split(buf.Bytes(), []byte{'\n'})
		return parseLogLines(t, lines)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestNewWithOptions verifies that handler options are respected.
func TestNewWithOptions(t *testing.T) {
	opts := &slog.HandlerOptions{Level: slog.LevelDebug}
	h := conslog.NewConsoleHandler(new(bytes.Buffer), opts)
	pc, _, _, _ := runtime.Caller(0)
	r := slog.NewRecord(time.Now(), slog.LevelDebug, "test", pc)
	if !h.Enabled(context.Background(), r.Level) {
		t.Error("handler should be enabled for debug level")
	}
}

// TestHandleLevelsAndColors checks color output and level handling for all log levels.
func TestHandleLevelsAndColors(t *testing.T) {
	var buf bytes.Buffer
	h := conslog.NewConsoleHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})

	testCases := []struct {
		level      slog.Level
		colorCodes []string
		message    string
	}{
		{
			level:      slog.LevelDebug,
			colorCodes: []string{ansi(lightGray), ansi(darkGray)},
			message:    "debug message",
		},
		{
			level:      slog.LevelInfo,
			colorCodes: []string{ansi(cyan)},
			message:    "info message",
		},
		{
			level:      slog.Level(1),
			colorCodes: []string{ansi(lightBlue)},
			message:    "lightBlue message",
		},
		{
			level:      slog.LevelWarn,
			colorCodes: []string{ansi(lightYellow)},
			message:    "warn message",
		},
		{
			level:      slog.LevelError,
			colorCodes: []string{ansi(lightRed)},
			message:    "error message",
		},
		{
			level:      slog.LevelError + 1,
			colorCodes: []string{ansi(lightRed)},
			message:    "error+1 message",
		},
		{
			level:      slog.LevelError + 2,
			colorCodes: []string{ansi(lightMagenta)},
			message:    "lightMagenta message",
		},
	}

	for _, tc := range testCases {
		buf.Reset()
		pc, _, _, _ := runtime.Caller(0)
		r := slog.NewRecord(time.Now(), tc.level, tc.message, pc)
		r.AddAttrs(slog.String("key", "value"))

		if !h.Enabled(context.Background(), tc.level) {
			t.Logf("Skipping level %v because handler is not enabled for it", tc.level)
			continue
		}

		if err := h.Handle(context.Background(), r); err != nil {
			t.Errorf("Handle failed for level %v: %v", tc.level, err)
			continue
		}

		output := buf.String()
		if !strings.Contains(output, tc.message) {
			t.Errorf("missing message for level %v: %q", tc.level, output)
		}
		if !strings.Contains(output, "key") {
			t.Errorf("missing attribute for level %v: %q", tc.level, output)
		}
		found := false
		for _, code := range tc.colorCodes {
			if strings.Contains(output, code) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(
				"missing expected color code(s) %v for level %v: %q",
				tc.colorCodes,
				tc.level,
				output,
			)
		}
	}
}

// TestAttributeHandling covers attribute and group handling, including empty key and empty group.
func TestAttributeHandling(t *testing.T) {
	tests := []struct {
		name      string
		withAttrs []slog.Attr
		withGroup string
		addAttrs  []slog.Attr
		expect    []string
		notExpect []string
	}{
		{
			name:      "StringAttribute",
			withAttrs: []slog.Attr{slog.String("key", "value")},
			expect:    []string{`key: "value"`},
		},
		{
			name:      "GroupAttribute",
			withGroup: "group",
			withAttrs: []slog.Attr{slog.Int("key", 42)},
			expect:    []string{"group:", "key: 42"},
		},
		{
			name:     "EmptyKeyString",
			addAttrs: []slog.Attr{slog.String("", "value")},
			expect:   []string{`"": "value"`},
		},
		{
			name:     "EmptyKeyInt",
			addAttrs: []slog.Attr{slog.Int("", 123)},
			expect:   []string{`"": 123`},
		},
		{
			name:      "EmptyGroup",
			withAttrs: []slog.Attr{slog.Group("emptyGroup")},
			notExpect: []string{"emptyGroup"},
		},
		{
			name:      "NestedGroupsIndentation",
			withGroup: "level1",
			withAttrs: []slog.Attr{slog.String("key1", "val1")},
			expect:    []string{"level1:", "  key1: \"val1\""},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			var h slog.Handler = conslog.NewConsoleHandler(&buf, nil)

			if tc.withGroup != "" {
				h = h.WithGroup(tc.withGroup)
			}
			if len(tc.withAttrs) > 0 {
				h = h.WithAttrs(tc.withAttrs)
			}

			pc, _, _, _ := runtime.Caller(0)
			r := slog.NewRecord(time.Now(), slog.LevelInfo, "test", pc)
			if len(tc.addAttrs) > 0 {
				r.AddAttrs(tc.addAttrs...)
			}

			if err := h.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle failed: %v", err)
			}

			output := buf.String()
			plain := uncolorize(t, output)

			for _, want := range tc.expect {
				if !strings.Contains(plain, want) {
					t.Errorf("expected output to contain %q, got %q", want, plain)
				}
			}
			for _, notWant := range tc.notExpect {
				if strings.Contains(plain, notWant) {
					t.Errorf("expected output NOT to contain %q, got %q", notWant, plain)
				}
			}
		})
	}

	// additional subtest to cover indentation with deep nesting beyond maxIndent
	t.Run("IndentationCoverage", func(t *testing.T) {
		var buf bytes.Buffer
		h := conslog.NewConsoleHandler(&buf, nil)

		// test indentation from 0 up to maxIndent + 5 to cover cache and fallback
		for indentLevel := 0; indentLevel <= maxIndent+5; indentLevel++ {
			h2 := h
			for i := range indentLevel {
				h2 = h2.WithGroup(fmt.Sprintf("g%d", i)).(*conslog.ConsoleHandler)
			}

			pc, _, _, _ := runtime.Caller(0)
			r := slog.NewRecord(time.Now(), slog.LevelInfo, "indent test", pc)
			r.AddAttrs(slog.String("key", "value"))

			buf.Reset()
			if err := h2.Handle(context.Background(), r); err != nil {
				t.Fatalf("Handle failed: %v", err)
			}

			plain := uncolorize(t, buf.String())
			expectedIndent := strings.Repeat(" ", indentLevel*2) + `key: "value"`
			if !strings.Contains(plain, expectedIndent) {
				t.Errorf("indentation test failed for indentLevel=%d; expected %q in output:\n%s",
					indentLevel, expectedIndent, plain)
			}
		}
	})
}

// TestEdgeCases covers edge behaviors for WithAttrs and WithGroup.
func TestEdgeCases(t *testing.T) {
	t.Run("NilWithAttrsReturnsSameHandler", func(t *testing.T) {
		h := conslog.NewConsoleHandler(nil, nil)
		if h.WithAttrs(nil) != h {
			t.Error("WithAttrs(nil) should return same handler")
		}
	})

	t.Run("WithGroupEmptyNameReturnsSameHandler", func(t *testing.T) {
		h := conslog.NewConsoleHandler(nil, nil)
		h2 := h.WithGroup("")
		if h2 != h {
			t.Error("WithGroup(\"\") should return the same handler instance")
		}
	})

	t.Run("WithGroupChainingReturnsNewHandler", func(t *testing.T) {
		h := conslog.NewConsoleHandler(nil, nil)
		h2 := h.WithGroup("group1").WithGroup("group2")
		if h2 == h {
			t.Error("WithGroup chaining should return new handler")
		}
	})
}

// TestPanicOnInvalidType checks that a panic occurs for unmarshalable attribute types.
func TestPanicOnInvalidType(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid type")
		}
	}()
	var buf bytes.Buffer
	h := conslog.NewConsoleHandler(&buf, nil)
	pc, _, _, _ := runtime.Caller(0)
	r := slog.NewRecord(time.Now(), slog.LevelInfo, "test", pc)
	r.AddAttrs(slog.Any("bad", make(chan int))) // channel cannot be marshaled to JSON
	err := h.Handle(context.Background(), r)
	if err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
}
