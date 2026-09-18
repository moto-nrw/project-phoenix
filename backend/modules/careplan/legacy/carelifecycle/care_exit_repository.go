package carelifecycle

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careexitview"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// CareExitReasonDirectory is the Care Plan owner behind the reason rows.
type CareExitReasonDirectory interface {
	FindCareExits(context.Context, []int64) (map[int64]*userModels.CareExit, error)
	UpsertCareExit(context.Context, *userModels.CareExit) error
	DeleteCareExits(context.Context, []int64) error
}

// CareExitRepository preserves the legacy archive contract. Care Plan owns the
// reason rows; this adapter combines them with People Directory rows whose care
// interval has run out (#2487).
type CareExitRepository struct {
	db       *bun.DB
	carePlan func() CareExitReasonDirectory
}

// NewCareExitRepository builds the repository. The owner arrives as a resolver
// because the composition root builds the Care Plan capability after the
// repository factory; a post-construction setter would be mutable wiring.
func NewCareExitRepository(db *bun.DB, carePlan func() CareExitReasonDirectory) userModels.CareExitRepository {
	if carePlan == nil {
		panic("care exit repository: care plan resolver is required")
	}
	return &CareExitRepository{db: db, carePlan: carePlan}
}

// reasons resolves the owner, or reports the configuration error a graph
// reaching this repository without a composed Care Plan would otherwise hide.
func (r *CareExitRepository) reasons() (CareExitReasonDirectory, error) {
	directory := r.carePlan()
	if directory == nil {
		return nil, errors.New("care exit repository: care plan capability is not bound")
	}
	return directory, nil
}

func (r *CareExitRepository) FindByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*userModels.CareExit, error) {
	carePlan, err := r.reasons()
	if err != nil {
		return nil, err
	}
	values, err := carePlan.FindCareExits(ctx, studentIDs)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find care exits by student ids", Err: err}
	}
	return values, nil
}

func (r *CareExitRepository) Upsert(ctx context.Context, exit *userModels.CareExit) error {
	if err := exit.Validate(); err != nil {
		return err
	}
	carePlan, err := r.reasons()
	if err != nil {
		return err
	}
	if err := carePlan.UpsertCareExit(ctx, exit); err != nil {
		return &modelBase.DatabaseError{Op: "upsert care exit", Err: base.TranslateNotFound(err)}
	}
	return nil
}

func (r *CareExitRepository) DeleteByStudentIDs(ctx context.Context, studentIDs []int64) error {
	carePlan, err := r.reasons()
	if err != nil {
		return err
	}
	if err := carePlan.DeleteCareExits(ctx, studentIDs); err != nil {
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
	values, total, err := careexitview.ListEndedCare(ctx, base.GetDB(ctx, r.db), tenant.FromContext(ctx), asOf,
		careexitview.EndedCareFilter{
			Search: filter.Search, SchoolClasses: filter.SchoolClasses,
			Page: filter.Page, PageSize: filter.PageSize,
		})
	if err != nil {
		return nil, 0, &modelBase.DatabaseError{Op: "list ended care", Err: base.TranslateNotFound(err)}
	}
	rows := make([]*userModels.EndedCare, 0, len(values))
	studentIDs := make([]int64, 0, len(values))
	for _, value := range values {
		rows = append(rows, &userModels.EndedCare{
			StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName,
			SchoolClass: value.SchoolClass, LastCareDay: value.LastCareDay,
		})
		studentIDs = append(studentIDs, value.StudentID)
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
