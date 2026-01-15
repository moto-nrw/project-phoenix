package sse

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/realtime"
	"github.com/moto-nrw/project-phoenix/internal/core/service/active"
	"github.com/moto-nrw/project-phoenix/internal/core/service/usercontext"
	"github.com/moto-nrw/project-phoenix/internal/core/service/users"
)

// Resource defines the SSE resource with dependencies
type Resource struct {
	hub       *realtime.Hub
	activeSvc active.Service
	personSvc users.PersonService
	userCtx   usercontext.UserContextService
}

// NewResource creates a new SSE resource
func NewResource(
	hub *realtime.Hub,
	activeSvc active.Service,
	personSvc users.PersonService,
	userCtx usercontext.UserContextService,
) *Resource {
	return &Resource{
		hub:       hub,
		activeSvc: activeSvc,
		personSvc: personSvc,
		userCtx:   userCtx,
	}
}
