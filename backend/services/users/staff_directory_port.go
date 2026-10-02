package users

import (
	"context"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// The staff and teacher half of the person service left in #3752. The rows
// belong to School Membership and Workforce, so the retained repositories and
// the staff write orchestration are bound by the composition root
// (services.NewStaffDirectory) and this package only names the contract its
// callers use.
//
// Every read returns the repository result and error verbatim, without
// wrapping or sentinel mapping: callers (IoT device flows, PyrePortal) branch
// on sql.ErrNoRows and render err.Error() into response bodies.

// StaffDirectory serves the staff and teacher lookups and the two staff writes
// of the person service.
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
	// teacher record in one tenant transaction.
	CreateStaffWithTeacher(ctx context.Context, input CreateStaffInput) (staff *userModels.Staff, teacher *userModels.Teacher, teacherCreationFailed bool, err error)
	// UpdateStaffWithTeacher applies the mutable directory fields to a freshly
	// locked staff row and the requested teacher-record change.
	UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, TeacherAction, error)
}
