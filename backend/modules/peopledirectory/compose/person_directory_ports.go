package compose

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The ports below are what the retained person directory (PersonDirectory)
// reaches the rows through. They left services/users with it (#3753); the
// composition root binds every one of them.

// PersonWriter is the owner's person write path (#3349); the composition root
// binds database/repositories.NewPersonDirectory. Each call reflects the
// stored row back onto the caller's struct, so a handler that renders the
// person it just wrote renders what was actually persisted.
type PersonWriter interface {
	CreatePerson(ctx context.Context, person *userModels.Person) error
	UpdatePerson(ctx context.Context, person *userModels.Person) error
	// DeletePerson soft-deletes the row; the person keeps existing for every
	// record that references them.
	DeletePerson(ctx context.Context, id int64) error
}

// RFIDCards is the card existence check and registration the tag paths need.
// Identity & Access implements it; the composition root binds
// identityaccess.RFIDCards.
type RFIDCards interface {
	LookupRFIDCard(context.Context, string) (id string, active, found bool, err error)
	RegisterRFIDCard(context.Context, string) error
}

// StudentLocker is the owner's locked child read (#3349); the composition
// root binds database/repositories.NewStudentDirectory.
type StudentLocker interface {
	// LockStudent re-reads one owned row and holds its lock for the caller's
	// transaction. A child that is not there is userModels.ErrStudentRowMissing.
	LockStudent(ctx context.Context, id int64) (*userModels.Student, error)
}

// StaffDirectory serves the staff and teacher lookups and the two staff writes
// the person directory hands on (#3752). The rows belong to School Membership
// and Workforce; the composition root binds services.NewStaffDirectory.
//
// Every read returns the repository result and error verbatim, without
// wrapping or sentinel mapping: callers (IoT device flows, PyrePortal) branch
// on sql.ErrNoRows and render err.Error() into response bodies.
type StaffDirectory interface {
	GetStaffByID(ctx context.Context, id int64) (*userModels.Staff, error)
	GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error)
	GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error)
	ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error)
	ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error)
	GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error)
	GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error)
	GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error)
	GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error)
	ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error)

	// CreateStaffWithTeacher creates a staff record and, when requested, a
	// teacher record in one tenant transaction. A failed teacher creation is
	// deliberately non-fatal (teacherCreationFailed=true, teacher=nil): the
	// staff row still persists, mirroring the historical api/staff behaviour.
	CreateStaffWithTeacher(ctx context.Context, input CreateStaffInput) (staff *userModels.Staff, teacher *userModels.Teacher, teacherCreationFailed bool, err error)
	// UpdateStaffWithTeacher applies the already-mutated directory fields to a
	// freshly locked staff row, reloads it with person data, and applies the
	// requested teacher-record change. Teacher-record failures are non-fatal
	// and reported through the returned TeacherAction.
	UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, TeacherAction, error)
}

// CreateStaffInput carries the payload for CreateStaffWithTeacher.
type CreateStaffInput struct {
	PersonID       int64
	StaffNotes     string
	IsTeacher      bool
	Specialization string
	Role           string
	Qualifications string

	// ActorPermissions is the calling account's permission set, needed because
	// the create route may end up editing: a person that already carries a
	// staff record adopts it instead of getting a second one, and that write
	// belongs to staff:manage, not users:create (#2906). Decided inside the same
	// transaction that finds the record, so the check cannot be raced.
	//
	// Empty means "no permissions" and therefore no adoption. There is no
	// system-caller shortcut here: the HTTP handler is the only caller.
	ActorPermissions []string
}

// TeacherAction describes the teacher-record outcome of UpdateStaffWithTeacher.
type TeacherAction int

const (
	// TeacherActionNone — not a teacher request and no existing teacher record.
	TeacherActionNone TeacherAction = iota
	// TeacherActionExisting — not a teacher request, existing record untouched.
	TeacherActionExisting
	// TeacherActionUpdated — existing teacher record updated.
	TeacherActionUpdated
	// TeacherActionUpdateFailed — teacher update failed; staff update persisted.
	TeacherActionUpdateFailed
	// TeacherActionCreated — new teacher record created.
	TeacherActionCreated
	// TeacherActionCreateFailed — teacher creation failed; staff update persisted.
	TeacherActionCreateFailed
)
