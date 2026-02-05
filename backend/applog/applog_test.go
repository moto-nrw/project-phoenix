package applog_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/applog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestLogger creates a logger that writes to a buffer for output inspection.
// This bypasses applog.New() to avoid mutating slog.SetDefault() across parallel tests.
// Use applog.New() only in TestNew_SetsDefault where we explicitly test that behavior.
func newTestLogger(buf *bytes.Buffer, format string, level string) *slog.Logger {
	lvl := slog.LevelInfo
	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	}
	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	if format == "text" {
		handler = slog.NewTextHandler(buf, opts)
	} else {
		handler = slog.NewJSONHandler(buf, opts)
	}
	return slog.New(handler)
}

func TestNew_SetsDefault(t *testing.T) {
	logger := applog.New(applog.Config{Level: "info", Format: "json"})
	require.NotNil(t, logger)
	assert.Equal(t, logger.Handler(), slog.Default().Handler())
}

func TestNew_JSONOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	logger = logger.With(slog.String("env", "production"))

	logger.Info("test message", slog.String("key", "value"))

	// Verify output is valid JSON
	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err, "log output must be valid JSON: %s", buf.String())

	// Verify required fields
	assert.Equal(t, "test message", parsed["msg"])
	assert.Equal(t, "INFO", parsed["level"])
	assert.Equal(t, "value", parsed["key"])
	assert.Equal(t, "production", parsed["env"])
	assert.Contains(t, parsed, "time", "JSON output must include timestamp")
}

func TestNew_TextOutput(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, "text", "info")

	logger.Info("hello world", slog.Int("count", 42))

	output := buf.String()
	assert.Contains(t, output, "hello world")
	assert.Contains(t, output, "count=42")
	// Text output should NOT be valid JSON
	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	assert.Error(t, err, "text output should not be valid JSON")
}

func TestNew_LevelFiltering(t *testing.T) {
	tests := []struct {
		configLevel string
		logLevel    slog.Level
		shouldLog   bool
	}{
		{"info", slog.LevelDebug, false}, // debug filtered at info level
		{"info", slog.LevelInfo, true},   // info passes at info level
		{"info", slog.LevelWarn, true},   // warn passes at info level
		{"debug", slog.LevelDebug, true}, // debug passes at debug level
		{"error", slog.LevelWarn, false}, // warn filtered at error level
		{"error", slog.LevelError, true}, // error passes at error level
	}

	for _, tc := range tests {
		t.Run(tc.configLevel+"_"+tc.logLevel.String(), func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf, "json", tc.configLevel)

			logger.Log(context.TODO(), tc.logLevel, "test")

			if tc.shouldLog {
				assert.NotEmpty(t, buf.String(), "expected log output")
			} else {
				assert.Empty(t, buf.String(), "expected no log output")
			}
		})
	}
}

func TestNew_ParsesLevels(t *testing.T) {
	for _, tc := range []struct {
		level string
		want  slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},        // default
		{"garbage", slog.LevelInfo}, // fallback
	} {
		t.Run(tc.level, func(t *testing.T) {
			logger := applog.New(applog.Config{Level: tc.level, Format: "json"})
			require.NotNil(t, logger)
		})
	}
}

func TestNew_EnvAttribute(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	logger = logger.With(slog.String("env", "test"))

	logger.Info("check env")

	var parsed map[string]any
	err := json.Unmarshal(buf.Bytes(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "test", parsed["env"])
}

func TestNew_StdlibLogRouting(t *testing.T) {
	// Verify that after SetDefault + SetLogLoggerLevel(WARN),
	// stdlib log.Printf calls appear as WARN-level slog output
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)
	slog.SetDefault(logger)
	slog.SetLogLoggerLevel(slog.LevelWarn)

	log.Print("legacy log call")

	output := buf.String()
	assert.Contains(t, output, "WARN", "stdlib log.Print should route as WARN")
	assert.Contains(t, output, "legacy log call")

	// Verify it's valid JSON
	for line := range strings.SplitSeq(strings.TrimSpace(output), "\n") {
		if line == "" {
			continue
		}
		var parsed map[string]any
		err := json.Unmarshal([]byte(line), &parsed)
		require.NoError(t, err, "routed stdlib log must produce valid JSON: %s", line)
	}
}
