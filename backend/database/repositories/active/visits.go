// backend/database/repositories/active/visit.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// VisitRepository implements active.VisitRepository interface
type VisitRepository struct {
	*base.Repository[*active.Visit]
	db *bun.DB
}

// NewVisitRepository creates a new VisitRepository
func NewVisitRepository(db *bun.DB) active.VisitRepository {
	return &VisitRepository{
		Repository: base.NewRepository[*active.Visit](db, "active.visits", "Visit"),
		db:         db,
	}
}

// FindActiveByStudentID finds all active visits for a specific student
func (r *VisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active by student ID",
			Err: err,
		}
	}

	return visits, nil
}

// FindByActiveGroupID finds all visits for a specific active group
func (r *VisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".active_group_id = ?`, activeGroupID).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by active group ID",
			Err: err,
		}
	}

	return visits, nil
}

// FindByTimeRange finds all visits active during a specific time range
func (r *VisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".entry_time <= ? AND ("visit".exit_time IS NULL OR "visit".exit_time >= ?)`, end, start).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find by time range",
			Err: err,
		}
	}

	return visits, nil
}

// EndVisit marks a visit as ended at the current time
func (r *VisitRepository) EndVisit(ctx context.Context, id int64) error {
	_, err := r.db.NewUpdate().
		Table("active.visits").
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

// Create overrides base Create to handle validation
func (r *VisitRepository) Create(ctx context.Context, visit *active.Visit) error {
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

// List overrides the base List method to accept the new QueryOptions type
func (r *VisitRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*active.Visit, error) {
	var visits []*active.Visit
	query := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(`active.visits AS "visit"`)

	// Apply query options
	if options != nil {
		query = options.ApplyToQuery(query)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "list",
			Err: err,
		}
	}

	return visits, nil
}

// FindWithStudent retrieves visits with student details
func (r *VisitRepository) FindWithStudent(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	err := r.db.NewSelect().
		Model(visit).
		ModelTableExpr(`active.visits AS "visit"`).
		Relation("Student").
		Where(`"visit".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with student",
			Err: err,
		}
	}

	return visit, nil
}

// FindWithActiveGroup retrieves visits with active group details
func (r *VisitRepository) FindWithActiveGroup(ctx context.Context, id int64) (*active.Visit, error) {
	visit := new(active.Visit)
	err := r.db.NewSelect().
		Model(visit).
		ModelTableExpr(`active.visits AS "visit"`).
		Relation("ActiveGroup").
		Where(`"visit".id = ?`, id).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find with active group",
			Err: err,
		}
	}

	return visit, nil
}

// TransferVisitsFromRecentSessions transfers active visits from recent ended sessions on the same device to a new session
func (r *VisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	// Transfer active visits from recent sessions (ended within last hour) on the same device
	result, err := r.db.NewUpdate().
		Table("active.visits").
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

// DeleteExpiredVisits deletes visits older than retention days for a specific student
func (r *VisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)
	
	result, err := r.db.NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, cutoffDate).
		Where(`"visit".exit_time IS NOT NULL`). // Only delete completed visits
		Exec(ctx)
	
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete expired visits",
			Err: err,
		}
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}
	
	return rowsAffected, nil
}

// DeleteVisitsBeforeDate deletes visits created before a specific date for a student
func (r *VisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	result, err := r.db.NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(`active.visits AS "visit"`).
		Where(`"visit".student_id = ?`, studentID).
		Where(`"visit".created_at < ?`, beforeDate).
		Where(`"visit".exit_time IS NOT NULL`). // Only delete completed visits
		Exec(ctx)
	
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "delete visits before date",
			Err: err,
		}
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "get rows affected",
			Err: err,
		}
	}
	
	return rowsAffected, nil
}

// GetVisitRetentionStats gets statistics about visits that are candidates for deletion
func (r *VisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	type studentVisitCount struct {
		StudentID   int64 `bun:"student_id"`
		VisitCount  int   `bun:"visit_count"`
	}
	
	var results []studentVisitCount
	err := r.db.NewRaw(`
		SELECT 
			v.student_id,
			COUNT(*) as visit_count
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
		GROUP BY v.student_id
	`).Scan(ctx, &results)
	
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get visit retention stats",
			Err: err,
		}
	}
	
	// Convert to map
	stats := make(map[int64]int)
	for _, result := range results {
		stats[result.StudentID] = result.VisitCount
	}
	
	return stats, nil
}

// CountExpiredVisits counts visits that are older than retention period for all students
func (r *VisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.NewRaw(`
		SELECT COUNT(*)
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
	`).Scan(ctx, &count)
	
	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count expired visits",
			Err: err,
		}
	}
	
	return count, nil
}
