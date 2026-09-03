package users

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// CareExitRepository preserves the legacy archive contract. Care Plan owns the
// reason rows; this adapter combines them with People Directory rows whose care
// interval has run out (#2487).
type CareExitRepository struct {
	db       *bun.DB
	carePlan interface {
		FindCareExits(context.Context, []int64) (map[int64]*userModels.CareExit, error)
		UpsertCareExit(context.Context, *userModels.CareExit) error
		DeleteCareExits(context.Context, []int64) error
	}
}

// NewCareExitRepository builds the repository.
func NewCareExitRepository(db *bun.DB) userModels.CareExitRepository {
	return &CareExitRepository{
		db: db,
	}
}

func (r *CareExitRepository) BindCarePlan(capability interface {
	FindCareExits(context.Context, []int64) (map[int64]*userModels.CareExit, error)
	UpsertCareExit(context.Context, *userModels.CareExit) error
	DeleteCareExits(context.Context, []int64) error
}) {
	if capability == nil {
		panic("care exit repository: care plan capability is required")
	}
	r.carePlan = capability
}

func (r *CareExitRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*userModels.CareExit, error) {
	if r.carePlan == nil {
		return nil, errors.New("care exit repository: care plan capability is not bound")
	}
	values, err := r.carePlan.FindCareExits(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find care exits by student ids", Err: err}
	}
	return values, nil
}

func (r *CareExitRepository) Upsert(ctx context.Context, exit *userModels.CareExit) error {
	if err := exit.Validate(); err != nil {
		return err
	}
	if r.carePlan == nil {
		return errors.New("care exit repository: care plan capability is not bound")
	}
	err := r.carePlan.UpsertCareExit(ctx, exit)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert care exit", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *CareExitRepository) DeleteByStudentIDs(ctx context.Context, studentIDs []int64) error {
	if r.carePlan == nil {
		return errors.New("care exit repository: care plan capability is not bound")
	}
	if err := r.carePlan.DeleteCareExits(ctx, studentIDs); err != nil {
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
	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	exits, err := r.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, row := range rows {
		exit := exits[row.StudentID]
		if exit == nil {
			continue
		}
		row.Reason, row.ReasonNote, row.RecordedBy = &exit.Reason, exit.ReasonNote, exit.RecordedBy
		recordedAt := timezone.DateFromTime(exit.CreatedAt)
		row.RecordedAt = &recordedAt
	}
	return rows, total, nil
}
