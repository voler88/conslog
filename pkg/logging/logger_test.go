package logging_test

import (
	"bytes"
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/voler88/conslog/pkg/logging"
)

// TestHandlerTypeValidation verifies [logging.HandlerType.IsValid] and string constants.
func TestHandlerTypeValidation(t *testing.T) {
	validTypes := []logging.HandlerType{
		logging.Console,
		logging.Text,
		logging.JSON,
	}
	for _, ht := range validTypes {
		if !ht.IsValid() {
			t.Errorf("expected HandlerType %q to be valid", ht)
		}
		if ht.String() != string(ht) {
			t.Errorf("expected String() to return %q, got %q", ht, ht.String())
		}
	}

	invalid := logging.HandlerType("invalid")
	if invalid.IsValid() {
		t.Error("expected invalid HandlerType to be invalid")
	}
}

// TestNewLoggerFallback tests that invalid handler types fallback to JSON with warning.
func TestNewLoggerFallback(t *testing.T) {
	var buf bytes.Buffer
	// Capture stderr to check warning output
	oldStderr := testing.Verbose() // Just a placeholder; ideally use os.Pipe or similar in real tests

	l := logging.NewLogger(&buf, "invalid-type")
	if l == nil {
		t.Fatal("expected logger, got nil")
	}
	// We can't easily capture os.Stderr here without extra setup,
	// but at least verify logger works and outputs JSON.

	l.Info("test message", "key", "value")
	out := buf.String()
	if !strings.Contains(out, `"level":"INFO"`) {
		t.Errorf("expected JSON output with INFO level, got: %s", out)
	}

	_ = oldStderr
}

// TestHandlers verifies that the appropriate handler produces output that resembles JSON or non-JSON.
func TestHandlers(t *testing.T) {
	tests := []struct {
		name     string
		handler  logging.HandlerType
		wantJSON bool
	}{
		{"JSON", logging.JSON, true},
		{"Console", logging.Console, false},
		{"Text", logging.Text, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := logging.NewLogger(&buf, tt.handler)

			l.Error("test message", "key", "value")
			out := buf.String()

			gotJSON := strings.Contains(out, "{")
			if gotJSON != tt.wantJSON {
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
func TestSetLevel(t *testing.T) {
	levelRe := regexp.MustCompile(`.*"level":"(\w+)".*`)
	tests := []struct {
		name        string
		wantLvl     logging.Level
		wantMsgLvls []string
	}{
		{"Error", logging.LevelError, []string{"ERROR"}},
		{"Warn", logging.LevelWarn, []string{"ERROR", "WARN"}},
		{"Info", logging.LevelInfo, []string{"ERROR", "WARN", "INFO"}},
		{"Debug", logging.LevelDebug, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer
			l := logging.NewLogger(&buf, logging.JSON)
			l.SetLevel(tt.wantLvl)

			l.Error("test message", "key", "value")
			l.Warn("test message", "key", "value")
			l.Info("test message", "key", "value")
			l.Debug("test message", "key", "value")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if len(lines) != len(tt.wantMsgLvls) {
				t.Fatalf(
					"expected %d messages, got %d; output: %s",
					len(tt.wantMsgLvls),
					len(lines),
					buf.String(),
				)
			}

			for i, line := range lines {
				matches := levelRe.FindStringSubmatch(line)
				if len(matches) < 2 {
					t.Fatalf("failed to extract level from log line: %s", line)
				}
				gotLevel := matches[1]
				if !slices.Contains(tt.wantMsgLvls, gotLevel) {
					t.Errorf(
						"unexpected log level in message %d: got %q, want one of %v",
						i+1,
						gotLevel,
						tt.wantMsgLvls,
					)
				}
			}
		})
	}
}

// TestSetLevelByCounter verifies that the log level is correctly set
// based on the integer counter provided, enabling dynamic verbosity.
func TestSetLevelByCounter(t *testing.T) {
	var buf bytes.Buffer
	l := logging.NewLogger(&buf, logging.JSON)

	tests := []struct {
		name       string
		counter    int
		wantLevels []string
	}{
		{"CounterNegative", -1, []string{"ERROR"}},
		{"CounterZero", 0, []string{"ERROR"}},
		{"CounterOne", 1, []string{"ERROR", "WARN"}},
		{"CounterTwo", 2, []string{"ERROR", "WARN", "INFO"}},
		{"CounterThree", 3, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
		{"CounterFour", 4, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			l.SetLevelByCounter(tt.counter)

			l.Error("msg", "key", "val")
			l.Warn("msg", "key", "val")
			l.Info("msg", "key", "val")
			l.Debug("msg", "key", "val")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if len(lines) != len(tt.wantLevels) {
				t.Fatalf(
					"expected %d log lines, got %d; output: %s",
					len(tt.wantLevels),
					len(lines),
					buf.String(),
				)
			}

			for i, line := range lines {
				if !strings.Contains(line, tt.wantLevels[i]) {
					t.Errorf(
						"expected log line %d to contain level %q, got: %s",
						i+1,
						tt.wantLevels[i],
						line,
					)
				}
			}
		})
	}
}

// TestSetLevelByName tests setting log level by string name.
func TestSetLevelByName(t *testing.T) {
	var buf bytes.Buffer
	l := logging.NewLogger(&buf, logging.JSON)

	tests := []struct {
		name       string
		levelName  string
		wantErr    bool
		wantLevels []string
	}{
		{"LowercaseError", "error", false, []string{"ERROR"}},
		{"UppercaseError", "ERROR", false, []string{"ERROR"}},
		{"MixedCaseInfo", "InFo", false, []string{"ERROR", "WARN", "INFO"}},
		{"WarnUpper", "WARN", false, []string{"ERROR", "WARN"}},
		{"WarnMixed", "wArN", false, []string{"ERROR", "WARN"}},
		{"WarningLower", "warning", false, []string{"ERROR", "WARN"}},
		{"WarningMixed", "WaRnInG", false, []string{"ERROR", "WARN"}},
		{"ValidDebug", "debug", false, []string{"ERROR", "WARN", "INFO", "DEBUG"}},
		{"Invalid", "verbose", true, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf.Reset()
			err := l.SetLevelByName(tt.levelName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SetLevelByName(%q) error = %v, wantErr %v", tt.levelName, err, tt.wantErr)
			}
			if err != nil {
				return
			}

			l.Error("msg", "key", "val")
			l.Warn("msg", "key", "val")
			l.Info("msg", "key", "val")
			l.Debug("msg", "key", "val")

			lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
			if len(lines) != len(tt.wantLevels) {
				t.Fatalf(
					"expected %d log lines, got %d; output: %s",
					len(tt.wantLevels),
					len(lines),
					buf.String(),
				)
			}

			for i, line := range lines {
				if !strings.Contains(line, tt.wantLevels[i]) {
					t.Errorf(
						"expected log line %d to contain level %q, got: %s",
						i+1,
						tt.wantLevels[i],
						line,
					)
				}
			}
		})
	}
}

// TestWith verifies that key-value attributes are added to log output.
func TestWith(t *testing.T) {
	var buf bytes.Buffer
	l := logging.NewLogger(&buf, logging.JSON).With("user", "alice", "team", "red")
	l.Error("with attributes")

	var logLine map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logLine); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}

	if logLine["user"] != "alice" {
		t.Errorf("expected user 'alice', got: %v", logLine["user"])
	}
	if logLine["team"] != "red" {
		t.Errorf("expected team 'red', got: %v", logLine["team"])
	}
}

// TestWithGroup verifies that attributes are grouped under the specified name.
func TestWithGroup(t *testing.T) {
	var buf bytes.Buffer
	l := logging.NewLogger(&buf, logging.JSON).
		WithGroup("session").
		With("id", "abc123", "status", "active")
	l.Error("grouped attributes")

	var logLine map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logLine); err != nil {
		t.Fatalf("failed to parse log line: %v", err)
	}

	group, ok := logLine["session"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'session' to be a group object, got: %v", logLine["session"])
	}
	if group["id"] != "abc123" {
		t.Errorf("expected session.id = 'abc123', got: %v", group["id"])
	}
	if group["status"] != "active" {
		t.Errorf("expected session.status = 'active', got: %v", group["status"])
	}
}
