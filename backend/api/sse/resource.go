package sse

import (
	"cmp"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/uptrace/bun"
)

// Resource defines the SSE resource with dependencies
type Resource struct {
	hub     *realtime.Hub
	userCtx usercontext.UserContextService
	db      *bun.DB
	logger  *slog.Logger
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.logger, slog.Default())
}

// NewResource creates a new SSE resource
func NewResource(
	hub *realtime.Hub,
	userCtx usercontext.UserContextService,
	db *bun.DB,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		hub:     hub,
		userCtx: userCtx,
		db:      db,
		logger:  logger,
	}
}
