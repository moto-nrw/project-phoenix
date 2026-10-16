package sse

import (
	"context"
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
	// SetSchoolAccess; the school handler refuses to stream without it.
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
