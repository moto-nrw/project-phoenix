package presenceprojectionhttp

import (
	context "context"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// SchoolGroup is an OGS group (Gruppe) as these routes show it. RoomName is
// empty when the group has no room or the room was not loaded with it.
type SchoolGroup struct {
	ID       int64
	Name     string
	RoomID   *int64
	RoomName string
}

// SchoolGroups is the consumer-owned port to School Structure (#3356): the
// groups the student list, detail, day log and absence overview name, and
// the group teachers the detail lists as supervisors. The root binds it to
// the School Structure group service.
type SchoolGroups interface {
	GetGroup(ctx context.Context, id int64) (*SchoolGroup, error)
	// GetGroupsByIDs answers the groups found; a missing id is absent.
	GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*SchoolGroup, error)
	ListGroups(ctx context.Context) ([]*SchoolGroup, error)
	// GetGroupTeachers answers the teachers of a group as the detail lists
	// them; a teacher without a staff member or person is left out.
	GetGroupTeachers(ctx context.Context, groupID int64) ([]GroupTeacher, error)
}

// GroupTeacher is a teacher of a group as a supervisor contact names them.
// Email is empty when the teacher has no account.
type GroupTeacher struct {
	ID        int64
	FirstName string
	LastName  string
	Email     string
}

// ActiveEnrollments is the consumer-owned port to Timetable & Activities
// (#3356) behind the export's "angemeldet" column: the names of the activity
// groups each child is actively enrolled in on a date. A child without an
// active enrollment is absent from the map.
type ActiveEnrollments interface {
	ActiveEnrollmentGroups(ctx context.Context, studentIDs []int64, onDate calendar.Date) (map[int64][]peopleModule.StudentActivityGroup, error)
}
