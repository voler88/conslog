package logging_test

import (
	"bytes"
	"testing"

	"github.com/voler88/logging"
)

// BenchmarkLoggingJSON benchmarks logging performance using the JSON handler.
func BenchmarkLoggingJSON(b *testing.B) {
	var buf bytes.Buffer
	// Create a logger with the default JSON handler.
	logger := logging.New(&buf, "default")
	// Set verbosity level to DEBUG so that all log messages are processed.
	if err := logger.SetLevel(logging.LevelDebug); err != nil {
		b.Fatalf("failed to set log level: %v", err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		logger.Debug("benchmark logging", "key", "value")
		// Reset the buffer if needed so that the buffer doesn't grow indefinitely.
		buf.Reset()
	}
}

// BenchmarkLoggingConsole benchmarks logging performance using the Console handler.
func BenchmarkLoggingConsole(b *testing.B) {
	var buf bytes.Buffer
	// Create a logger with the console handler.
	logger := logging.New(&buf, "console")
	if err := logger.SetLevel(logging.LevelDebug); err != nil {
		b.Fatalf("failed to set log level: %v", err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		logger.Debug("benchmark logging", "key", "value")
		buf.Reset()
	}
}
