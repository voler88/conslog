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
	logger := logging.New(&buf, logging.JSON)
	// Set verbosity level to DEBUG so that all log messages are processed.
	logger.SetLevel(logging.LevelDebug)

	for b.Loop() {
		logger.Debug("benchmark logging", "key", "value")
		// Reset the buffer if needed so that the buffer doesn't grow indefinitely.
		buf.Reset()
	}
}

// BenchmarkLoggingText benchmarks logging performance using the Text handler.
func BenchmarkLoggingText(b *testing.B) {
	var buf bytes.Buffer
	// Create a logger with the default JSON handler.
	logger := logging.New(&buf, logging.Text)
	// Set verbosity level to DEBUG so that all log messages are processed.
	logger.SetLevel(logging.LevelDebug)

	for b.Loop() {
		logger.Debug("benchmark logging", "key", "value")
		// Reset the buffer if needed so that the buffer doesn't grow indefinitely.
		buf.Reset()
	}
}

// BenchmarkLoggingConsole benchmarks logging performance using the Console handler.
func BenchmarkLoggingConsole(b *testing.B) {
	var buf bytes.Buffer
	// Create a logger with the console handler.
	logger := logging.New(&buf, logging.Console)
	logger.SetLevel(logging.LevelDebug)

	for b.Loop() {
		logger.Debug("benchmark logging", "key", "value")
		buf.Reset()
	}
}
