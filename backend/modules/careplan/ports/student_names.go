package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// StudentName is the display projection Care Plan needs for companion links.
// Composition translates the People Directory result at its boundary.
type StudentName struct {
	StudentID int64
	FirstName string
	LastName  string
}

// StudentNameFinder reads the display names of children.
type StudentNameFinder interface {
	ListStudentNamesByID(context.Context, []int64) ([]StudentName, error)
}

// ExceptionQueries can be constructed before the Timetable status-slot
// commands that Care Plan mutations require.
type ExceptionQueries interface {
	FindPickupException(context.Context, int64, bool) (careplan.PickupException, error)
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	FindStudentStatusDay(context.Context, int64, bool) (careplan.StudentStatusDay, error)
	ListStudentStatusDays(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error)
}
