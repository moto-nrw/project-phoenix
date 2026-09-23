// backend/database/repositories/users/student.go
package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// StudentRepository implements users.StudentRepository interface
type StudentRepository struct {
	db *bun.DB
	// teacherGroupIDs resolves education.group_teacher through composition. This
	// Postgres adapter stays independent of the School Membership owner and can
	// resolve several teachers without one owner call per teacher.
	teacherGroupIDs      func(context.Context, int64) ([]int64, error)
	teacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

// NewStudentRepository creates a new StudentRepository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	return &StudentRepository{
		db: db,
	}
}

// List retains the legacy equality-filter contract without a generic
// repository that can accidentally fall back to the rollback view.
func (r *StudentRepository) List(ctx context.Context, filters map[string]any) ([]*users.Student, error) {
	rows := make([]*users.Student, 0)
	db := base.GetDB(ctx, r.db)
	query := studentdirectoryview.ModelQuery(db, tenant.FromContext(ctx), &rows)
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list", Err: err}
	}
	return rows, nil
}

func (r *StudentRepository) CountWithOptions(ctx context.Context, options *modelBase.QueryOptions) (int, error) {
	db := base.GetDB(ctx, r.db)
	query := studentdirectoryview.Query(db, tenant.FromContext(ctx)).Column("student.id")
	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias("student")
		query = base.ApplyFilter(query, options.Filter)
	}
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count with options", Err: err}
	}
	return count, nil
}

func (r *StudentRepository) BindTeacherGroupIDs(query func(context.Context, int64) ([]int64, error)) {
	if query == nil {
		panic("student repository: school membership is required")
	}
	r.teacherGroupIDs = query
}

func (r *StudentRepository) BindTeacherStaffGroupIDs(query func(context.Context, []int64) ([]int64, error)) {
	if query == nil {
		panic("student repository: school membership is required")
	}
	r.teacherStaffGroupIDs = query
}

// The child's own lifecycle moved to People Directory in #3349: the gates, the
// row lock order, the departure-plan resolution, the companion reconcile and
// the stranding refusal all live in modules/peopledirectory now. The four write
// entry points stay on this type because the retained StudentRepository
// interface still declares them, and the composition root routes them to the
// owner (database/repositories.bindStudentWrites).
//
// Reaching one of them here means a graph was built without the owner behind
// it. That is a configuration error, and saying so is better than writing the
// row through a path that no longer enforces any of the above.
var errStudentWritesMoved = errors.New(
	"student writes moved to People Directory: bind the directory before writing a child")

func (r *StudentRepository) Create(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) Update(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) Delete(context.Context, any) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) VerifyCompanionStrandingBatch(context.Context) error {
	return errStudentWritesMoved
}

// The rosters are the owner's: the child and the identity it renders under are
// both its rows, so it answers them in one statement. These stay only as the
// interface the retained StudentRepository declares.
func (r *StudentRepository) FindByTeacherIDWithGroups(context.Context, int64) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByTeacherStaffIDsWithGroups(context.Context, []int64) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindAllWithGroups(context.Context) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindOverlappingWithGroups(
	context.Context, timezone.Date, timezone.Date, timezone.Date,
) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindOverlappingWithGroupsOnDate(
	context.Context, string, time.Time,
) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

// The scoped roster reads that replaced the generic filter surface are the
// owner's: which children a lookup counts is its decision, not a filter the
// caller assembles.
func (r *StudentRepository) ListByGroupIDsIncludingAlumni(context.Context, []int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ListClassRoster(context.Context, string) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) CountEnrolled(context.Context) (int, error) {
	return 0, errStudentWritesMoved
}

// The locked reads and the per-tenant class-writes gate are the owner's too:
// it takes the gate before the row, which is what keeps the acquisition order
// gate-then-rows for every writer.
func (r *StudentRepository) FindByIDForUpdate(context.Context, int64) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) LockStudentClassWritesShared(context.Context) error {
	return errStudentWritesMoved
}

// The lifecycle status and the care window are the owner's too (#3349). These
// stay only as the interface the retained StudentRepository declares; the
// composition root routes them to People Directory.
func (r *StudentRepository) UpdateStatus(context.Context, int64, users.StudentStatus) error {
	return errStudentWritesMoved
}

func (r *StudentRepository) TransitionStatus(
	context.Context, int64, users.StudentStatus, users.StudentStatus,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *StudentRepository) FindCareBoundsByIDs(context.Context, []int64) (map[int64]timezone.Date, error) {
	return nil, errStudentWritesMoved
}

// These reads moved to People Directory in #3349. They stay on this type
// because the retained StudentRepository interface still declares them, and the
// composition root routes them to the owner (database/repositories.NewStudentReads).
func (r *StudentRepository) FindByID(context.Context, any) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByPersonID(context.Context, int64) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindReadScopeByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGroupID(context.Context, int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByGroupIDs(context.Context, []int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ExistsEnrolledByNameAndBirthday(
	context.Context, int64, string, string, timezone.Date,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *StudentRepository) FindEnrolledStudentIDByNameAndBirthday(
	context.Context, int64, string, string, timezone.Date,
) (*int64, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) ListSchoolClasses(context.Context) ([]string, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) CountByGroupIDs(context.Context, []int64) (map[int64]int, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindPendingDueForActivation(context.Context, timezone.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindActiveDueForDeactivation(context.Context, timezone.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *StudentRepository) FindByIDsForUpdate(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}
