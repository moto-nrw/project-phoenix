package presenceprojectionhttp

import (
	context "context"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassListEntryReader hands over the entries in the class-then-name display
// order the "Klassenliste" export and the class dropdown both rely on. The
// root binds it to the School Membership capability that owns them.
type ClassListEntryReader interface {
	ListClassListEntriesInDisplayOrder(context.Context) ([]peopleModule.StudentClassListEntry, error)
}

// PlannedStudents is the consumer-owned port to the Timetable owner's
// planned-block lookup the day planning reads (#584): which of the children
// have a planned block on a date.
type PlannedStudents interface {
	GetPlannedStudentIDsByDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]int64, error)
}

func (rs *Resource) todayDate() calendar.Date {
	return calendar.DateFromTime(rs.Now())
}
