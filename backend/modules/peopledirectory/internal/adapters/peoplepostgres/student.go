// backend/database/repositories/users/student.go
package users

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	ports "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// studentRepository implements the StudentRepository interface.
type studentRepository struct {
	runtime  Runtime
	students StudentDirectoryRows
}

// newRawStudentRepository creates a new StudentRepository
func newRawStudentRepository(runtime Runtime, students StudentDirectoryRows) rawStudentRepository {
	return &studentRepository{runtime: requireRuntime(runtime), students: requireStudentRows(students)}
}

// List retains the legacy equality-filter contract without a generic
// repository that can accidentally fall back to the rollback view.
func (r *studentRepository) List(ctx context.Context, filters map[string]any) ([]*users.Student, error) {
	rows := make([]*users.Student, 0)
	db := r.runtime.DB(ctx)
	query := r.students.StudentModelRows(db, r.runtime.TenantID(ctx), &rows)
	for field, value := range filters {
		if value != nil {
			query = query.Where("? = ?", bun.Ident(field), value)
		}
	}
	if err := query.Scan(ctx); err != nil {
		return nil, &ports.DatabaseError{Op: "list", Err: err}
	}
	return rows, nil
}

func (r *studentRepository) CountWithOptions(ctx context.Context, options *ports.QueryOptions) (int, error) {
	db := r.runtime.DB(ctx)
	query := studentRows(ctx, r.runtime, r.students, db).Column("student.id")
	if options != nil && options.Filter != nil {
		options.Filter.WithTableAlias("student")
		query = applyFilter(query, options.Filter)
	}
	count, err := query.Count(ctx)
	if err != nil {
		return 0, &ports.DatabaseError{Op: "count with options", Err: err}
	}
	return int(count), nil
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

func (r *studentRepository) Create(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *studentRepository) Update(context.Context, *users.Student) error {
	return errStudentWritesMoved
}

func (r *studentRepository) Delete(context.Context, any) error {
	return errStudentWritesMoved
}

func (r *studentRepository) VerifyCompanionStrandingBatch(context.Context) error {
	return errStudentWritesMoved
}

// The rosters are the owner's: the child and the identity it renders under are
// both its rows, so it answers them in one statement. These stay only as the
// interface the retained StudentRepository declares.
func (r *studentRepository) FindByTeacherIDWithGroups(context.Context, int64) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByTeacherStaffIDsWithGroups(context.Context, []int64) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindAllWithGroups(context.Context) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindOverlappingWithGroups(
	context.Context, calendar.Date, calendar.Date, calendar.Date,
) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindOverlappingWithGroupsOnDate(
	context.Context, string, time.Time,
) ([]*users.StudentWithGroupInfo, error) {
	return nil, errStudentWritesMoved
}

// The scoped roster reads that replaced the generic filter surface are the
// owner's: which children a lookup counts is its decision, not a filter the
// caller assembles.
func (r *studentRepository) ListByGroupIDsIncludingAlumni(context.Context, []int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) ListClassRoster(context.Context, string) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) CountEnrolled(context.Context) (int, error) {
	return 0, errStudentWritesMoved
}

// The locked reads and the per-tenant class-writes gate are the owner's too:
// it takes the gate before the row, which is what keeps the acquisition order
// gate-then-rows for every writer.
func (r *studentRepository) FindByIDForUpdate(context.Context, int64) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) LockStudentClassWritesShared(context.Context) error {
	return errStudentWritesMoved
}

// The lifecycle status and the care window are the owner's too (#3349). These
// stay only as the interface the retained StudentRepository declares; the
// composition root routes them to People Directory.
func (r *studentRepository) UpdateStatus(context.Context, int64, users.StudentStatus) error {
	return errStudentWritesMoved
}

func (r *studentRepository) TransitionStatus(
	context.Context, int64, users.StudentStatus, users.StudentStatus,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *studentRepository) FindCareBoundsByIDs(context.Context, []int64) (map[int64]calendar.Date, error) {
	return nil, errStudentWritesMoved
}

// These reads moved to People Directory in #3349. They stay on this type
// because the retained StudentRepository interface still declares them, and the
// composition root routes them to the owner (database/repositories.NewStudentReads).
func (r *studentRepository) FindByID(context.Context, any) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByPersonID(context.Context, int64) (*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindReadScopeByIDs(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByGroupID(context.Context, int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByGroupIDs(context.Context, []int64) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) ExistsEnrolledByNameAndBirthday(
	context.Context, int64, string, string, calendar.Date,
) (bool, error) {
	return false, errStudentWritesMoved
}

func (r *studentRepository) FindEnrolledStudentIDByNameAndBirthday(
	context.Context, int64, string, string, calendar.Date,
) (*int64, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) ListSchoolClasses(context.Context) ([]string, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) CountByGroupIDs(context.Context, []int64) (map[int64]int, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindPendingDueForActivation(context.Context, calendar.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindActiveDueForDeactivation(context.Context, calendar.Date) ([]*users.Student, error) {
	return nil, errStudentWritesMoved
}

func (r *studentRepository) FindByIDsForUpdate(context.Context, []int64) (map[int64]*users.Student, error) {
	return nil, errStudentWritesMoved
}
