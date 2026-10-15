package compose

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The roots configure the process logger from its level setting; the text
// flag and the environment do not change which levels pass.
func TestProcessLoggerHonoursTheConfiguredLevel(t *testing.T) {
	t.Parallel()

	for _, textLogging := range []bool{false, true} {
		logger := newProcessLogger("warn", textLogging, "test")
		assert.False(t, logger.Enabled(context.Background(), slog.LevelInfo))
		assert.True(t, logger.Enabled(context.Background(), slog.LevelWarn))
	}
	assert.True(t, newProcessLogger("debug", false, "test").Enabled(context.Background(), slog.LevelDebug))
}
