package students

import (
	context "context"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ActiveEnrollments is the consumer-owned port to Timetable & Activities
// (#3356) behind the export's "angemeldet" column: the names of the activity
// groups each child is actively enrolled in on a date. A child without an
// active enrollment is absent from the map.
type ActiveEnrollments interface {
	ActiveEnrollmentGroups(ctx context.Context, studentIDs []int64, onDate calendar.Date) (map[int64][]peopleModule.StudentActivityGroup, error)
}
