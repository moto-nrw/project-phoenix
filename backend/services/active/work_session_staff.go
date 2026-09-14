package active

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
)

type WorkSessionStaffName struct {
	FirstName string
	LastName  string
}

type WorkSessionStaff interface {
	StaffScheduleQuery
	StaffNames(context.Context, []int64) (map[int64]WorkSessionStaffName, error)
	// Update retains the existing staff write while schedule commands migrate.
	Update(context.Context, *users.Staff) error
}
