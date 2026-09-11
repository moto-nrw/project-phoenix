package active

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type sessionStartLocker struct {
	db *bun.DB
}

// NewSessionStartLocker creates the PostgreSQL adapter for session-start
// serialization.
func NewSessionStartLocker(db *bun.DB) activeModels.SessionStartLocker {
	return &sessionStartLocker{db: db}
}

func (r *sessionStartLocker) LockSessionStart(ctx context.Context, tenantID, activityID int64) error {
	if tenantID <= 0 || activityID <= 0 {
		return fmt.Errorf("lock session start: tenant and activity IDs must be positive")
	}
	_, err := base.GetDB(ctx, r.db).ExecContext(
		ctx,
		"SELECT pg_advisory_xact_lock(?, ?)",
		tenantID,
		activityID,
	)
	if err != nil {
		return &modelBase.DatabaseError{Op: "lock session start", Err: err}
	}
	return nil
}
