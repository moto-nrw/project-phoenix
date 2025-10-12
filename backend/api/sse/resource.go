package sse

import (
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// Resource defines the SSE resource with dependencies
type Resource struct {
	hub       *realtime.Hub
	activeSvc active.Service
}

// NewResource creates a new SSE resource
func NewResource(hub *realtime.Hub, activeSvc active.Service) *Resource {
	return &Resource{
		hub:       hub,
		activeSvc: activeSvc,
	}
}
