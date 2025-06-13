package conslog_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"
	"testing/slogtest"
	"time"

	"github.com/voler88/conslog"
)

const timeFormat = "[15:04:05.000]"

var colorRe = regexp.MustCompile(`\033\[\d+m(.+?)\033\[0m\s*`)

// uncolorize removes ANSI color codes from the input string.
// Fails the test if color codes are not found as expected.
func uncolorize(t *testing.T, s string) string {
	t.Helper()
	us := colorRe.FindStringSubmatch(s)
	if us == nil {
		t.Fatalf("failed to remove colorization from string: %q", s)
	}
	return us[1]
}

// parseLogEntryHeader parses a top-level log entry line (indent 0)
// extracting time, level, and message fields.
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
		// otherwise, first token is the level
		entry["level"] = strings.TrimSuffix(cleanFirst, ":")
		entry["msg"] = uncolorize(t, rest)
	}
	return entry
}

// parseLogLines parses the entire log output into structured entries
// with nested groups and attributes represented as nested maps.
func parseLogLines(t *testing.T, lines [][]byte) []map[string]any {
	t.Helper()
	var entries []map[string]any
	var groupKeys []string
	expectedIndent := 2

	for _, lineBytes := range lines {
		line := string(lineBytes)
		line = strings.TrimRight(line, "\r\n")
		if len(strings.TrimSpace(line)) == 0 {
			continue // skip empty lines
		}

		currentIndent := len(line) - len(strings.TrimLeft(line, " "))
		if currentIndent%2 != 0 {
			t.Fatalf("indentation must be even, got %d: %q", currentIndent, line)
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

		// remove color codes and trim spaces
		line = strings.TrimSpace(line)
		line = uncolorize(t, line)

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
