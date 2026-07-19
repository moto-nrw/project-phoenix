// backend/database/repositories/users/student_companion.go
package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentCompanionRepository implements users.StudentCompanionRepository.
//
// Every method has to look at BOTH endpoint columns, because an edge is stored
// once in normalized low/high order (see migration 1.15.208) — "the companions
// of child X" is therefore an OR over student_low_id and student_high_id, never
// a lookup on a single column.
type StudentCompanionRepository struct {
	*base.Repository[*users.StudentCompanion]
	db *bun.DB
}

// NewStudentCompanionRepository creates a new StudentCompanionRepository
func NewStudentCompanionRepository(db *bun.DB) users.StudentCompanionRepository {
	repo := base.NewRepository[*users.StudentCompanion](db, "users.student_companions", "StudentCompanion")
	repo.TenantScoped = true
	return &StudentCompanionRepository{
		Repository: repo,
		db:         db,
	}
}

// ListForStudent returns every edge touching the given student, across all
// weekdays.
func (r *StudentCompanionRepository) ListForStudent(ctx context.Context, studentID int64) ([]*users.StudentCompanion, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student ID is required")
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID).
		Order("weekday ASC")

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student companions", Err: err}
	}
	return edges, nil
}

// ListLinksForStudent returns the child's companions folded per companion, with
// the companion's name joined in so the detail view can render them without a
// second round trip.
func (r *StudentCompanionRepository) ListLinksForStudent(ctx context.Context, studentID int64) ([]users.CompanionLink, error) {
	edges, err := r.ListForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}

	links := users.CompanionLinksFromEdges(studentID, edges)
	if len(links) == 0 {
		return links, nil
	}

	ids := make([]int64, 0, len(links))
	for _, link := range links {
		ids = append(ids, link.CompanionStudentID)
	}

	type nameRow struct {
		StudentID int64  `bun:"student_id"`
		FirstName string `bun:"first_name"`
		LastName  string `bun:"last_name"`
	}
	var rows []nameRow
	nameQuery := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		Join(`INNER JOIN users.persons AS "person" ON "person".id = "student".person_id`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`)

	nameQuery = base.WithTenantFilter(ctx, nameQuery, "student")

	if err := nameQuery.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "load companion names", Err: err}
	}

	names := make(map[int64]nameRow, len(rows))
	for _, row := range rows {
		names[row.StudentID] = row
	}
	for i := range links {
		if row, ok := names[links[i].CompanionStudentID]; ok {
			links[i].FirstName = row.FirstName
			links[i].LastName = row.LastName
		}
	}
	return links, nil
}

// ReplaceForStudent makes the given edges the child's complete companion set:
// every existing edge touching the student is removed first, then the new set is
// inserted.
//
// Replace rather than diff on purpose — the child detail view submits the whole
// "läuft mit" list, and a delete+insert of at most a handful of rows keeps the
// symmetry invariant trivially correct (removing Tom from Lina's card must
// remove Lina from Tom's card, which is the same row).
//
// Callers run inside the request's tenant transaction (withTx), so the delete
// and the insert commit or roll back together.
func (r *StudentCompanionRepository) ReplaceForStudent(ctx context.Context, studentID int64, edges []*users.StudentCompanion) error {
	if studentID <= 0 {
		return fmt.Errorf("student ID is required")
	}

	db := base.GetDB(ctx, r.db)
	tenantID := tenant.FromContext(ctx)

	deleteQuery := db.NewDelete().
		Model((*users.StudentCompanion)(nil)).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`("student_companion".student_low_id = ? OR "student_companion".student_high_id = ?)`, studentID, studentID)
	if tenantID > 0 {
		deleteQuery = deleteQuery.Where(`"student_companion".tenant_id = ?`, tenantID)
	}
	if _, err := deleteQuery.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "clear student companions", Err: err}
	}

	if len(edges) == 0 {
		return nil
	}

	for _, edge := range edges {
		if edge == nil {
			return fmt.Errorf("companion edge cannot be nil")
		}
		if edge.TenantID == 0 && tenantID > 0 {
			edge.TenantID = tenantID
		}
	}

	if _, err := db.NewInsert().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "insert student companions", Err: err}
	}
	return nil
}

// CompanionIDsForWeekday returns, per requested student, the ids of the children
// they walk home with on the given weekday. Students without a link are absent
// from the map.
//
// This is the bulk read behind the Kindersuche grouping: one query for the whole
// page instead of one per child.
func (r *StudentCompanionRepository) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	result := make(map[int64][]int64)
	if len(studentIDs) == 0 {
		return result, nil
	}
	if _, ok := users.CompanionWeekdayKeys[weekday]; !ok {
		return nil, fmt.Errorf("%w: got %d", users.ErrCompanionInvalidWeekday, weekday)
	}

	var edges []*users.StudentCompanion
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&edges).
		ModelTableExpr(`users.student_companions AS "student_companion"`).
		Where(`"student_companion".weekday = ?`, weekday).
		Where(`("student_companion".student_low_id IN (?) OR "student_companion".student_high_id IN (?))`,
			bun.List(studentIDs), bun.List(studentIDs))

	query = base.WithTenantFilter(ctx, query, "student_companion")

	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list companions for weekday", Err: err}
	}

	requested := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		requested[id] = true
	}

	// Both directions are recorded so the reader never has to know which
	// endpoint a child sits on. An edge whose far end is outside the requested
	// set still gets recorded for the near end — the caller decides what to do
	// with a companion that is not on the current page.
	for _, edge := range edges {
		if requested[edge.StudentLowID] {
			result[edge.StudentLowID] = append(result[edge.StudentLowID], edge.StudentHighID)
		}
		if requested[edge.StudentHighID] {
			result[edge.StudentHighID] = append(result[edge.StudentHighID], edge.StudentLowID)
		}
	}
	return result, nil
}
