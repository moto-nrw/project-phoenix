package sse

import (
	"context"
	"errors"
	"log/slog"
)

// SubscriptionResolver resolves the caller's live-update topics through the
// Identity & Access caller context: the staff member (0 for effective admins
// without a staff record), the room session topics, the educational-group
// topics and their deduplicated union. A caller it rejects fails with an
// error carrying the HTTP status (SetupRejection).
type SubscriptionResolver interface {
	ResolveSSETopics(ctx context.Context) (staffID int64, activeGroupIDs, eduTopics, allTopics []string, err error)
}

// SetupRejection is a rejected subscription: 401 for an account without a
// person, 403 for a caller who is neither staff nor an effective admin.
type SetupRejection interface {
	error
	SetupStatus() int
	SetupMessage() string
}

// Resource defines the SSE resource with dependencies
type Resource[E Event, C Client[E]] struct {
	hub     Hub[C]
	userCtx SubscriptionResolver
	runtime Runtime[E, C]
	logger  *slog.Logger
	// schoolAccess re-checks an open school-portal stream (#2208). Wired via
	// NewResourceWithSchoolAccess; the school handler refuses to stream
	// without it: a long-lived connection that nobody re-authorizes is
	// exactly what this guard exists to prevent.
	schoolAccess SchoolAccessChecker
}

// getLogger returns a nil-safe logger, falling back to slog.Default() if logger is nil
func (rs *Resource[E, C]) getLogger() *slog.Logger {
	return loggerOrDefault(rs.logger)
}

func loggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return slog.Default()
	}
	return logger
}

// NewResource creates a new SSE resource
func NewResource[E Event, C Client[E]](
	hub Hub[C],
	userCtx SubscriptionResolver,
	runtime Runtime[E, C],
	logger *slog.Logger,
) *Resource[E, C] {
	return &Resource[E, C]{
		hub:     hub,
		userCtx: userCtx,
		runtime: runtime,
		logger:  logger,
	}
}

// NewResourceWithSchoolAccess is NewResource with the school stream's access
// re-check at construction (ADR 0059). A missing hub, subscription resolver
// or access checker is a construction error.
func NewResourceWithSchoolAccess[E Event, C Client[E]](
	hub Hub[C],
	userCtx SubscriptionResolver,
	runtime Runtime[E, C],
	schoolAccess SchoolAccessChecker,
	logger *slog.Logger,
) (*Resource[E, C], error) {
	if hub == nil || userCtx == nil || schoolAccess == nil {
		return nil, errors.New("sse resource: hub, subscription resolver and school access are required")
	}
	resource := NewResource(hub, userCtx, runtime, logger)
	resource.schoolAccess = schoolAccess
	return resource, nil
}
