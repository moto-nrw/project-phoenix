package sse

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
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
type Resource struct {
	hub     *realtime.Hub
	userCtx SubscriptionResolver
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
	userCtx SubscriptionResolver,
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
