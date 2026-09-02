package absence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/careplanning"
)

// BindCareStudentLock installs the People Directory's student row lock
// behind the excused-request writers (#2662) for test graphs. Production binds it once
// through the timetable services; test graphs that wire this package by
// hand bind it here, because they may import neither the owner nor the
// care-planning package themselves.
func BindCareStudentLock(lock func(ctx context.Context, studentID int64) error, notFound error) func(context.Context, int64) error {
	return careplanning.BindStudentLock(lock, notFound)
}
