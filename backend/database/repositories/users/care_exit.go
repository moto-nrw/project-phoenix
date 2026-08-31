package users

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprCareExits = `users.student_care_exits AS "care_exit"`

// CareExitRepository owns the reason rows behind "Betreuung beenden" and the
// archive read model that joins them onto the children whose care interval has
// run out (#2487).
type CareExitRepository struct {
	*base.Repository[*userModels.CareExit]
	db *bun.DB
}

// NewCareExitRepository builds the repository.
func NewCareExitRepository(db *bun.DB) userModels.CareExitRepository {
	return &CareExitRepository{
		Repository: base.NewRepository[*userModels.CareExit](db, "users.student_care_exits", "care_exit"),
		db:         db,
	}
}

func (r *CareExitRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*userModels.CareExit, error) {
	result := make(map[int64]*userModels.CareExit, len(studentIDs))
	if len(studentIDs) == 0 {
		return result, nil
	}
	var rows []*userModels.CareExit
	query := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(tableExprCareExits).
		Where(`"care_exit".student_id IN (?)`, bun.List(studentIDs))
	query = base.WithTenantFilter(ctx, query, "care_exit")
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "find care exits by student ids", Err: base.TranslateNotFound(err)}
	}
	for _, row := range rows {
		result[row.StudentID] = row
	}
	return result, nil
}

func (r *CareExitRepository) Upsert(ctx context.Context, exit *userModels.CareExit) error {
	if err := exit.Validate(); err != nil {
		return err
	}
	base.EnsureTenantID(ctx, exit)
	_, err := base.GetDB(ctx, r.db).NewInsert().
		Model(exit).
		ModelTableExpr(tableExprCareExits).
		On("CONFLICT (tenant_id, student_id) DO UPDATE").
		Set("reason = EXCLUDED.reason").
		Set("reason_note = EXCLUDED.reason_note").
		Set("recorded_by = EXCLUDED.recorded_by").
		Set("withdrawal_completion_id = EXCLUDED.withdrawal_completion_id").
		Set("updated_at = NOW()").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert care exit", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *CareExitRepository) DeleteByStudentIDs(ctx context.Context, studentIDs []int64) error {
	if len(studentIDs) == 0 {
		return nil
	}
	query := base.GetDB(ctx, r.db).NewDelete().
		Model((*userModels.CareExit)(nil)).
		ModelTableExpr(tableExprCareExits).
		Where(`"care_exit".student_id IN (?)`, bun.List(studentIDs))
	query = base.WithTenantFilter(ctx, query, "care_exit")
	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "delete care exits", Err: base.TranslateNotFound(err)}
	}
	return nil
}

// ListEnded is the archive view. It reads the STUDENTS whose enrollment
// interval has run out rather than the reason rows, on purpose: the acceptance
// criteria require the view to hold every regularly ended care, including the
// ones that ended because an enrollment phase ran out and therefore never got
// a manually recorded reason.
func (r *CareExitRepository) ListEnded(
	ctx context.Context,
	asOf timezone.Date,
	filter userModels.CareExitListFilter,
) ([]*userModels.EndedCare, int, error) {
	build := func() *bun.SelectQuery {
		query := base.GetDB(ctx, r.db).NewSelect().
			TableExpr(`users.students AS "student"`).
			Join(`JOIN users.persons AS "person" ON "person".id = "student".person_id`).
			Join(`LEFT JOIN users.student_care_exits AS "care_exit" ON "care_exit".student_id = "student".id AND "care_exit".tenant_id = "student".tenant_id`).
			Where(`"student".enrolled_until IS NOT NULL`).
			Where(`"student".enrolled_until < ?`, asOf).
			Where(`"student".status <> ?`, string(userModels.StudentStatusAlumnus))
		query = base.WithTenantFilter(ctx, query, "student")
		if search := strings.TrimSpace(filter.Search); search != "" {
			pattern := "%" + strings.ToLower(search) + "%"
			query = query.Where(
				`(LOWER("person".first_name) LIKE ? OR LOWER("person".last_name) LIKE ? OR LOWER("student".school_class) LIKE ?)`,
				pattern, pattern, pattern,
			)
		}
		if len(filter.SchoolClasses) > 0 {
			query = query.Where(`"student".school_class IN (?)`, bun.List(filter.SchoolClasses))
		}
		return query
	}

	total, err := build().Count(ctx)
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "count ended care", Err: base.TranslateNotFound(err)}
	}

	var rows []*userModels.EndedCare
	query := build().
		ColumnExpr(`"student".id AS student_id`).
		ColumnExpr(`"person".first_name AS first_name`).
		ColumnExpr(`"person".last_name AS last_name`).
		ColumnExpr(`"student".school_class AS school_class`).
		ColumnExpr(`"student".enrolled_until AS last_care_day`).
		ColumnExpr(`"care_exit".reason AS reason`).
		ColumnExpr(`"care_exit".reason_note AS reason_note`).
		ColumnExpr(`"care_exit".recorded_by AS recorded_by`).
		// created_at is a TIMESTAMPTZ: casting it straight to DATE would use
		// the session timezone, so an exit recorded at 00:30 Berlin would be
		// archived under the previous day on a UTC session.
		ColumnExpr(`("care_exit".created_at AT TIME ZONE 'Europe/Berlin')::date AS recorded_at`).
		OrderExpr(`"student".enrolled_until DESC, "person".last_name ASC, "person".first_name ASC, "student".id ASC`)

	if filter.PageSize > 0 {
		query = query.Limit(filter.PageSize)
		if filter.Page > 1 {
			query = query.Offset((filter.Page - 1) * filter.PageSize)
		}
	}
	if err := query.Scan(ctx, &rows); err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "list ended care", Err: base.TranslateNotFound(err)}
	}
	return rows, total, nil
}
