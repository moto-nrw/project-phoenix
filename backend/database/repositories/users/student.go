// backend/database/repositories/users/student.go
package users

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableUsersStudents              = "users.students"
	tableExprUsersStudentsAsStudent = "users.students AS student"
)

// StudentRepository implements users.StudentRepository interface
type StudentRepository struct {
	*base.Repository[*users.Student]
	db *bun.DB
	// teacherGroupIDs resolves education.group_teacher through composition. This
	// Postgres adapter stays independent of the School Membership owner and can
	// resolve several teachers without one owner call per teacher.
	teacherGroupIDs      func(context.Context, int64) ([]int64, error)
	teacherStaffGroupIDs func(context.Context, []int64) ([]int64, error)
}

// NewStudentRepository creates a new StudentRepository
func NewStudentRepository(db *bun.DB) users.StudentRepository {
	repo := base.NewRepository[*users.Student](db, tableUsersStudents, "Student")
	repo.TenantScoped = true
	return &StudentRepository{
		Repository: repo,
		db:         db,
	}
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

func (r *StudentRepository) FindByIDForUpdateNoWait(context.Context, int64) (*users.Student, error) {
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

func (r *StudentRepository) SetEnrolledUntilByIDs(context.Context, []int64, *timezone.Date) (int64, error) {
	return 0, errStudentWritesMoved
}

func (r *StudentRepository) SetEnrollmentWindowByID(
	context.Context, int64, timezone.Date, users.StudentStatus,
) error {
	return errStudentWritesMoved
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

func (r *StudentRepository) FindBySchoolClass(context.Context, string) ([]*users.Student, error) {
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

func (r *StudentRepository) ListIDs(context.Context) ([]int64, error) {
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
