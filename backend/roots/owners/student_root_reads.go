package owners

import (
	"context"
	"time"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// StudentRosterRow is one child of a roster read, in the values the roots
// that may not name the retained models consume.
type StudentRosterRow struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// StudentRootReads are the retained student reads the serving root consumes
// outside the service graph (#2743): the meal plan's participants and the
// Worker's activation tick.
type StudentRootReads struct {
	// OverlappingOnDate lists the children enrolled on the ISO date, as the
	// meal-plan boundary reads them; now decides the immediate activation.
	OverlappingOnDate func(ctx context.Context, date string, now time.Time) ([]StudentRosterRow, error)
	// PendingDueForActivation and ActiveDueForDeactivation list the children
	// whose care starts or ends by asOf.
	PendingDueForActivation  func(ctx context.Context, asOf calendar.Date) ([]int64, error)
	ActiveDueForDeactivation func(ctx context.Context, asOf calendar.Date) ([]int64, error)
	// TransitionStatus moves a child's status if it is still expected.
	TransitionStatus func(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// studentRootRows are the retained student reads the root's roster and
// activation reads take.
type studentRootRows interface {
	FindOverlappingWithGroupsOnDate(ctx context.Context, date string, now time.Time) ([]*peopleValues.StudentRosterEntry, error)
	FindPendingDueForActivation(ctx context.Context, asOf calendar.Date) ([]*peopleValues.StudentState, error)
	FindActiveDueForDeactivation(ctx context.Context, asOf calendar.Date) ([]*peopleValues.StudentState, error)
	TransitionStatus(ctx context.Context, studentID int64, expected, next string) (bool, error)
}

// NewStudentRootReads binds the root's student reads to the retained student
// repository.
func NewStudentRootReads(students studentRootRows) StudentRootReads {
	return StudentRootReads{
		OverlappingOnDate: func(ctx context.Context, date string, now time.Time) ([]StudentRosterRow, error) {
			rows, err := students.FindOverlappingWithGroupsOnDate(ctx, date, now)
			if err != nil {
				return nil, err
			}
			result := make([]StudentRosterRow, 0, len(rows))
			for _, row := range rows {
				if row == nil || row.Record == nil || row.Record.Person == nil {
					continue
				}
				result = append(result, StudentRosterRow{
					StudentID: row.Record.ID, FirstName: row.Record.Person.FirstName, LastName: row.Record.Person.LastName, SchoolClass: row.Record.SchoolClass,
				})
			}
			return result, nil
		},
		PendingDueForActivation: func(ctx context.Context, asOf calendar.Date) ([]int64, error) {
			return studentIDsOf(students.FindPendingDueForActivation(ctx, asOf))
		},
		ActiveDueForDeactivation: func(ctx context.Context, asOf calendar.Date) ([]int64, error) {
			return studentIDsOf(students.FindActiveDueForDeactivation(ctx, asOf))
		},
		TransitionStatus: func(ctx context.Context, studentID int64, expected, next string) (bool, error) {
			return students.TransitionStatus(ctx, studentID, string(expected), string(next))
		},
	}
}

func studentIDsOf(rows []*peopleValues.StudentState, err error) ([]int64, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			ids = append(ids, row.ID)
		}
	}
	return ids, err
}
