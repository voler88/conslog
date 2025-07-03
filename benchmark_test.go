package conslog_test

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/voler88/conslog/pkg/logging"
)

// prepareLargeMap creates a map with 100 key-value pairs for benchmarking.
func prepareLargeMap() map[string]any {
	largeMap := make(map[string]any, 100)
	for i := range 100 {
		largeMap[fmt.Sprintf("key%d", i)] = fmt.Sprintf("value%d", i)
	}
	return largeMap
}

// logEntryFields returns a slice of key-value pairs including a large map.
func logEntryFields(largeMap map[string]any) []any {
	return []any{
		"key1", "value1",
		"key2", "value2",
		"key3", 123,
		"key4", true,
		"largeMap", largeMap,
	}
}

// runBenchmark runs the benchmark for given logger and fields.
func runBenchmark(b *testing.B, newLogger func(io.Writer) logging.Logger, fields []any) {
	var buf bytes.Buffer
	logger := newLogger(&buf)
	logger.SetLevel(logging.LevelDebug)

	for b.Loop() {
		logger.Debug("benchmark logging", fields...)
		buf.Reset()
	}
}

func BenchmarkLoggingJSON(b *testing.B) {
	largeMap := prepareLargeMap()
	fields := logEntryFields(largeMap)

	b.Run("LargeMap", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.JSON)
		}, fields)
	})

	b.Run("SingleKeyValue", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.JSON)
		}, []any{"key", "value"})
	})
}

func BenchmarkLoggingText(b *testing.B) {
	largeMap := prepareLargeMap()
	fields := logEntryFields(largeMap)

	b.Run("LargeMap", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.Text)
		}, fields)
	})

	b.Run("SingleKeyValue", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.Text)
		}, []any{"key", "value"})
	})
}

func BenchmarkLoggingConsole(b *testing.B) {
	largeMap := prepareLargeMap()
	fields := logEntryFields(largeMap)

	b.Run("LargeMap", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.Console)
		}, fields)
	})

	b.Run("SingleKeyValue", func(b *testing.B) {
		runBenchmark(b, func(w io.Writer) logging.Logger {
			return logging.NewLogger(w, logging.Console)
		}, []any{"key", "value"})
	})
}
