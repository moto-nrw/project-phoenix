// backend/database/repositories/active/visit.go
package active

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActiveVisits            = "active.visits"
	tableExprActiveVisitsAsVisit = `active.visits AS "visit"`
)

// VisitRepository implements active.VisitRepository interface
type VisitRepository struct {
	*base.Repository[*active.Visit]
	db *bun.DB
}

// NewVisitRepository creates a new VisitRepository
func NewVisitRepository(db *bun.DB) active.VisitRepository {
	return &VisitRepository{
		Repository: base.NewRepository[*active.Visit](db, tableActiveVisits, "Visit"),
		db:         db,
	}
}

// FindActiveByStudentID finds all active visits for a specific student
func (r *VisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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

// DeleteExpiredVisits deletes visits older than retention days for a specific student
func (r *VisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -retentionDays)

	result, err := r.db.NewDelete().
		Model((*active.Visit)(nil)).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		ModelTableExpr(tableExprActiveVisitsAsVisit).
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
		StudentID  int64 `bun:"student_id"`
		VisitCount int   `bun:"visit_count"`
	}

	var results []studentVisitCount
	err := r.db.NewSelect().
		TableExpr("active.visits AS v").
		ColumnExpr("v.student_id").
		ColumnExpr("COUNT(*) AS visit_count").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		Group("v.student_id").
		Scan(ctx, &results)

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
	count, err := r.db.NewSelect().
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.privacy_consents AS pc ON pc.student_id = v.student_id").
		Where("v.exit_time IS NOT NULL").
		Where("v.created_at < NOW() - make_interval(days => pc.data_retention_days)").
		Count(ctx)

	if err != nil {
		return 0, &modelBase.DatabaseError{
			Op:  "count expired visits",
			Err: err,
		}
	}

	return int64(count), nil
}

// GetOldestExpiredVisit returns the timestamp of the oldest visit that is past retention
func (r *VisitRepository) GetOldestExpiredVisit(ctx context.Context) (*time.Time, error) {
	var result struct {
		CreatedAt time.Time `bun:"created_at"`
	}

	err := r.db.NewRaw(`
		SELECT MIN(v.created_at) as created_at
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
	`).Scan(ctx, &result)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get oldest expired visit",
			Err: err,
		}
	}

	if result.CreatedAt.IsZero() {
		return nil, nil
	}

	return &result.CreatedAt, nil
}

// GetExpiredVisitsByMonth returns counts of expired visits grouped by month
func (r *VisitRepository) GetExpiredVisitsByMonth(ctx context.Context) (map[string]int64, error) {
	var monthlyStats []struct {
		Month string `bun:"month"`
		Count int64  `bun:"count"`
	}

	err := r.db.NewRaw(`
		SELECT
			TO_CHAR(v.created_at, 'YYYY-MM') as month,
			COUNT(*) as count
		FROM active.visits v
		INNER JOIN users.privacy_consents pc ON pc.student_id = v.student_id
		WHERE v.exit_time IS NOT NULL
			AND v.created_at < NOW() - (pc.data_retention_days || ' days')::INTERVAL
		GROUP BY TO_CHAR(v.created_at, 'YYYY-MM')
		ORDER BY month
	`).Scan(ctx, &monthlyStats)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get expired visits by month",
			Err: err,
		}
	}

	result := make(map[string]int64, len(monthlyStats))
	for _, ms := range monthlyStats {
		result[ms.Month] = ms.Count
	}

	return result, nil
}

// GetCurrentByStudentID finds the current active visit for a student
func (r *VisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*active.Visit, error) {
	visit := new(active.Visit)
	err := r.db.NewSelect().
		Model(visit).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id = ? AND "visit".exit_time IS NULL`, studentID).
		Order(`entry_time DESC`).
		Limit(1).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student ID",
			Err: err,
		}
	}

	return visit, nil
}

// GetCurrentByStudentIDs finds current active visits for multiple students in a single query
func (r *VisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	result := make(map[int64]*active.Visit, len(studentIDs))

	if len(studentIDs) == 0 {
		return result, nil
	}

	uniqueIDs := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]struct{}, len(studentIDs))
	for _, id := range studentIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		uniqueIDs = append(uniqueIDs, id)
	}

	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".student_id IN (?)`, bun.In(uniqueIDs)).
		Where(`"visit".exit_time IS NULL`).
		OrderExpr(`"visit".student_id ASC`).
		OrderExpr(`"visit".entry_time DESC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get current by student IDs",
			Err: err,
		}
	}

	for _, visit := range visits {
		if _, exists := result[visit.StudentID]; !exists {
			result[visit.StudentID] = visit
		}
	}

	return result, nil
}

// FindActiveVisits finds all visits with no exit time (currently active)
func (r *VisitRepository) FindActiveVisits(ctx context.Context) ([]*active.Visit, error) {
	var visits []*active.Visit
	err := r.db.NewSelect().
		Model(&visits).
		ModelTableExpr(tableExprActiveVisitsAsVisit).
		Where(`"visit".exit_time IS NULL`).
		Order(`entry_time ASC`).
		Scan(ctx)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active visits",
			Err: err,
		}
	}

	return visits, nil
}

// FindActiveByGroupIDWithDisplayData finds active visits for a group with student display info
func (r *VisitRepository) FindActiveByGroupIDWithDisplayData(ctx context.Context, activeGroupID int64) ([]active.VisitWithDisplayData, error) {
	var results []active.VisitWithDisplayData
	err := r.db.NewSelect().
		ColumnExpr("v.id AS visit_id").
		ColumnExpr("v.student_id").
		ColumnExpr("v.active_group_id").
		ColumnExpr("v.entry_time").
		ColumnExpr("v.exit_time").
		ColumnExpr("v.created_at").
		ColumnExpr("v.updated_at").
		ColumnExpr("p.first_name").
		ColumnExpr("p.last_name").
		ColumnExpr("COALESCE(s.school_class, '') AS school_class").
		ColumnExpr("COALESCE(g.name, '') AS ogs_group_name").
		TableExpr("active.visits AS v").
		Join("INNER JOIN users.students AS s ON s.id = v.student_id").
		Join("INNER JOIN users.persons AS p ON p.id = s.person_id").
		Join("LEFT JOIN education.groups AS g ON g.id = s.group_id").
		Where("v.active_group_id = ?", activeGroupID).
		Where("v.exit_time IS NULL").
		OrderExpr("v.entry_time DESC").
		Scan(ctx, &results)

	if err != nil {
		return nil, &modelBase.DatabaseError{
			Op:  "find active visits with display data",
			Err: err,
		}
	}

	return results, nil
}
