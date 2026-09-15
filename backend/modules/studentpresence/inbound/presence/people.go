package presence

import (
	"context"
)

type PersonIdentity struct{ ID int64 }

type StaffIdentity struct {
	ID       int64
	TenantID int64
}

type StudentIdentity struct {
	ID      int64
	GroupID *int64
}

type TeacherIdentity struct{ ID int64 }

// People supplies identity and student lookups used by attendance routes.
type People interface {
	FindByAccountID(context.Context, int64) (*PersonIdentity, error)
	GetStaffByPersonID(context.Context, int64) (*StaffIdentity, error)
	GetStudentByPersonID(context.Context, int64) (*StudentIdentity, error)
	GetTeacherByStaffID(context.Context, int64) (*TeacherIdentity, error)
	GetStudentByID(context.Context, int64) (*StudentIdentity, error)
}
