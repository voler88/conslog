package logging_test

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/voler88/logging"
)

// TestHandlers verifies that the appropriate handler produces output that resembles JSON or non-JSON.
func TestHandlers(t *testing.T) {
	tests := []struct {
		name     string
		handler  string
		wantJSON bool
	}{
		{"JSON", "default", true},
		{"Console", "console", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := logging.New(&buf, tt.handler)

			l.Error("test message", "key", "value")
			out := buf.String()

			if strings.Contains(out, "{") != tt.wantJSON {
				t.Errorf(
					"handler %q: expected JSON output = %v, but got: %s",
					tt.handler,
					tt.wantJSON,
					out,
				)
			}
		})
	}
}

// TestSetLevel tests that the log level filtering works as expected.
// It uses a regular expression to extract the log level from the string output.
func TestSetLevel(t *testing.T) {
	// Regular expression expects output containing a JSON field "level":"".
	levelRe := regexp.MustCompile(`.*"level":"(\w+)".*`)
	tests := []struct {
		name        string
		wantLvl     int
		wantMsgLvls []string
	}{
		{"Error-1", -1, []string{"ERROR"}},
		{"Error", logging.LevelError, []string{"ERROR"}},
		{"Warn", logging.LevelWarn, []string{"ERROR", "WARN"}},
		{"Info", logging.LevelInfo, []string{"ERROR", "WARN", "INFO"}},
		{"Debug", logging.LevelDebug, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
		{"Debug+1", 4, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			// Use the JSON handler to reliably extract levels.
			l := logging.New(&buf, "default")
			if err := l.SetLevel(tt.wantLvl); err != nil {
				t.Fatalf("SetLevel(%d) returned error: %v", tt.wantLvl, err)
			}
			// Log messages at various levels.
			l.Error("test message", "key", "value")
			l.Warn("test message", "key", "value")
			l.Info("test message", "key", "value")
			l.Debug("test message", "key", "value")

			// Split the output into individual lines.
			lines := bytes.Split(buf.Bytes(), []byte{'\n'})

			// Exclude a potential trailing empty line.
			var nonEmptyLines [][]byte
			for _, line := range lines {
				if len(line) > 0 {
					nonEmptyLines = append(nonEmptyLines, line)
				}
			}

			if len(nonEmptyLines) != len(tt.wantMsgLvls) {
				t.Fatalf(
					"expected %d messages, but got %d; output: %s",
					len(tt.wantMsgLvls),
					len(nonEmptyLines),
					buf.String(),
				)
			}

			// Verify that each log message has an expected level.
			for i, lineBytes := range nonEmptyLines {
				s := string(lineBytes)

				gotMsgLevel := levelRe.FindStringSubmatch(s)
				if gotMsgLevel == nil {
					t.Fatalf("failed to extract meassage level: %s", s)
				}

				if !slices.Contains(tt.wantMsgLvls, gotMsgLevel[1]) {
					t.Fatalf(
						"unexpected log level in message %d: expected one of %v, got %q",
						i+1,
						tt.wantMsgLvls,
						gotMsgLevel[1],
					)
				}
			}
		})
	}
}
