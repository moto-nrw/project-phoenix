package sse

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the SSE resource with dependencies
type Resource struct {
	hub       *realtime.Hub
	activeSvc active.Service
	personSvc users.PersonService
	userCtx   usercontext.UserContextService
	logger    *slog.Logger
}

// NewResource creates a new SSE resource
func NewResource(
	hub *realtime.Hub,
	activeSvc active.Service,
	personSvc users.PersonService,
	userCtx usercontext.UserContextService,
	logger *slog.Logger,
) *Resource {
	return &Resource{
		hub:       hub,
		activeSvc: activeSvc,
		personSvc: personSvc,
		userCtx:   userCtx,
		logger:    logger,
	}
}
