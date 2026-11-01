package compose

import (
	"io"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// NewPostgresRealtimeHub constructs the process-scoped realtime capability.
func NewPostgresRealtimeHub(_ *bun.DB, logger *slog.Logger, _ bool) (*realtime.Hub, io.Closer, error) {
	return NewRealtimeHub(logger), io.NopCloser(strings.NewReader("")), nil
}
