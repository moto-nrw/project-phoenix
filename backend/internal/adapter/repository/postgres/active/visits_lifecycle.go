package active

import (
	"context"
	"fmt"
	"time"

	activeDomain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

// Create creates a new visit, overriding base Create to handle validation
func (r *VisitRepository) Create(ctx context.Context, visit *activeDomain.Visit) error {
	if visit == nil {
		return fmt.Errorf("visit cannot be nil")
	}

	// Validate visit
	if err := visit.Validate(); err != nil {
		return err
	}

	// Use the base Create method
	return r.Repository.Create(ctx, visit)
}

// EndVisit marks a visit as ended at the current time
func (r *VisitRepository) EndVisit(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Table(tableActiveVisits).
		Set(`exit_time = ?`, time.Now()).
		Where(`id = ? AND exit_time IS NULL`, id).
		Exec(ctx)

	if err != nil {
		return &modelBase.DatabaseError{
			Op:  "end visit",
			Err: err,
		}
	}

	return nil
}

// TransferVisitsFromRecentSessions transfers active visits from recent ended sessions on the same device to a new session
func (r *VisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	// Transfer active visits from recent sessions (ended within last hour) on the same device
	result, err := r.db.NewUpdate().
		Table(tableActiveVisits).
		Set("active_group_id = ?", newActiveGroupID).
		Where(`active_group_id IN (
			SELECT id FROM active.groups
			WHERE device_id = ?
			AND end_time IS NOT NULL
			AND end_time > NOW() - INTERVAL '1 hour'
		) AND exit_time IS NULL`, deviceID).
		Exec(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "transfer visits from recent sessions",
			Err: err,
		}
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get affected rows from visit transfer",
			Err: err,
		}
	}

	return int(rowsAffected), nil
}
