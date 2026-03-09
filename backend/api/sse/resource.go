package sse

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

// Resource defines the SSE resource with dependencies
type Resource struct {
	hub       *realtime.Hub
	activeSvc active.Service
	personSvc users.PersonService
	userCtx   usercontext.UserContextService
	logger    *slog.Logger
	db        *bun.DB
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource) getLogger() *slog.Logger {
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}

// NewResource creates a new SSE resource
func NewResource(
	hub *realtime.Hub,
	activeSvc active.Service,
	personSvc users.PersonService,
	userCtx usercontext.UserContextService,
	logger *slog.Logger,
	db *bun.DB,
) *Resource {
	return &Resource{
		hub:       hub,
		activeSvc: activeSvc,
		personSvc: personSvc,
		userCtx:   userCtx,
		logger:    logger,
		db:        db,
	}
}
