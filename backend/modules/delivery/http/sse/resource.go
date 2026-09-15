package sse

import (
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
	// schoolAccess re-checks an open school-portal stream (#2208). Wired via
	// SetSchoolAccess; the school handler refuses to stream without it.
	schoolAccess SchoolAccessChecker
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	return loggerOrDefault(rs.logger)
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
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
