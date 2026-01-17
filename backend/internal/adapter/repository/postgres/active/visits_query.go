package active

import (
	"context"
	"time"

	activeDomain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	modelBase "github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/uptrace/bun"
)

// FindActiveByStudentID finds all active visits for a specific student
func (r *VisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*activeDomain.Visit, error) {
	var visits []*activeDomain.Visit
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
func (r *VisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*activeDomain.Visit, error) {
	var visits []*activeDomain.Visit
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
func (r *VisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*activeDomain.Visit, error) {
	var visits []*activeDomain.Visit
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

// List lists all visits with query options
func (r *VisitRepository) List(ctx context.Context, options *modelBase.QueryOptions) ([]*activeDomain.Visit, error) {
	var visits []*activeDomain.Visit
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
func (r *VisitRepository) FindWithStudent(ctx context.Context, id int64) (*activeDomain.Visit, error) {
	visit := new(activeDomain.Visit)
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
func (r *VisitRepository) FindWithActiveGroup(ctx context.Context, id int64) (*activeDomain.Visit, error) {
	visit := new(activeDomain.Visit)
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

// GetCurrentByStudentID finds the current active visit for a student
func (r *VisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*activeDomain.Visit, error) {
	visit := new(activeDomain.Visit)
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
func (r *VisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*activeDomain.Visit, error) {
	result := make(map[int64]*activeDomain.Visit, len(studentIDs))

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

	var visits []*activeDomain.Visit
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
func (r *VisitRepository) FindActiveVisits(ctx context.Context) ([]*activeDomain.Visit, error) {
	var visits []*activeDomain.Visit
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
func (r *VisitRepository) FindActiveByGroupIDWithDisplayData(ctx context.Context, activeGroupID int64) ([]activeDomain.VisitWithDisplayData, error) {
	var results []activeDomain.VisitWithDisplayData
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
