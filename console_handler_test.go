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

var colorRe = regexp.MustCompile(`\033\[\d+m(.+)\033\[0m\s*`)

// uncolorize removes color formatting from a given string.
func uncolorize(t *testing.T, s string) string {
	us := colorRe.FindStringSubmatch(s)
	if us == nil {
		t.Fatalf("failed to remove colorization from string: %s", s)
	}
	return us[1]
}

// parseLogEntryHeader processes a line with indent level 0 as a new log entry,
// extracting the time, level, and message.
func parseLogEntryHeader(t *testing.T, line string) map[string]any {
	entry := make(map[string]any)
	line = strings.TrimSpace(line)
	first, rest, _ := strings.Cut(line, " ")
	cleanFirst := uncolorize(t, first)
	// Try to parse time, if valid then use it.
	if parsedTime, err := time.Parse(timeFormat, cleanFirst); err == nil {
		entry["time"] = parsedTime.Local().Format(time.RFC3339)
		level, msg, _ := strings.Cut(rest, " ")
		entry["level"] = strings.TrimSuffix(uncolorize(t, level), ":")
		entry["msg"] = uncolorize(t, msg)
	} else {
		// Otherwise, first token is the level.
		entry["level"] = strings.TrimSuffix(cleanFirst, ":")
		entry["msg"] = uncolorize(t, rest)
	}
	return entry
}

func TestSlogtest(t *testing.T) {
	var buf bytes.Buffer
	err := slogtest.TestHandler(conslog.NewConsoleHandler(&buf, nil), func() []map[string]any {
		var entries []map[string]any
		var groupKeys []string
		expectedIndent := 2

		lines := bytes.Split(buf.Bytes(), []byte{'\n'})
		for _, lineBytes := range lines {
			if len(lineBytes) == 0 {
				continue // skip empty lines
			}
			line := string(lineBytes)

			// Determine indent count based on spaces.
			currentIndent := len(line) - len(strings.TrimLeft(line, " "))
			if currentIndent < 0 || currentIndent%2 != 0 {
				t.Fatalf("impossible or odd current indent count: %d", currentIndent)
			}
			if expectedIndent < 2 || expectedIndent%2 != 0 {
				t.Fatalf("impossible or odd expected indent count: %d", expectedIndent)
			}

			// New log entry (level 0 indent).
			if currentIndent == 0 {
				entry := parseLogEntryHeader(t, line)
				entries = append(entries, entry)
				expectedIndent = 2 // reset expected indent
				groupKeys = nil    // reset group keys
				continue
			}

			// Process groups and attributes (non 0 level indent).
			entry := entries[len(entries)-1]
			if len(entry) < 1 {
				t.Fatalf("log entry does not have a header: %s", line)
			}
			line = strings.TrimSpace(line)
			line = uncolorize(t, line)

			// Adjust group keys when indent decreases.
			if currentIndent < expectedIndent {
				if len(groupKeys) > 0 {
					groupKeys = groupKeys[:len(groupKeys)-1]
				}
			}

			// Helper to traverse nested groups.
			groupEntry := entry
			for _, key := range groupKeys {
				v, ok := groupEntry[key]
				if !ok {
					t.Fatalf("expected group %q not found", key)
				}
				g, ok := v.(map[string]any)
				if !ok {
					t.Fatalf("value for %q is not map[string]any", key)
				}
				groupEntry = g
			}

			// Check if line represents an attribute or a new group.
			if key, value, ok := strings.Cut(line, ": "); ok {
				// Attribute entry.
				// Remove any quotation marks from value.
				groupEntry[key] = strings.ReplaceAll(value, `"`, "")
			} else {
				// A new group entry.
				trimmedKey := strings.TrimSuffix(line, ":")
				groupKeys = append(groupKeys, trimmedKey)
				// Create a nested map for the group.
				groupEntry[trimmedKey] = make(map[string]any)
				expectedIndent += 2
			}
		}
		return entries
	})
	if err != nil {
		t.Fatal(err)
	}
}
