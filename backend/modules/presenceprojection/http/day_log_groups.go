package presenceprojectionhttp

import (
	context "context"
	errors "errors"
	"fmt"
	strconv "strconv"

	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// GroupDayLog is one group's live day log as the weekly protocol records it
// on the evening of the day (#4769): the verdict counters, every child of
// the group and the children who were missing, with the day log's label.
type GroupDayLog struct {
	GroupID      int64
	StudentIDs   []int64
	Present      int
	Sick         int
	ClassTrip    int
	Excused      int
	Absent       int
	MissingChild []GroupDayLogChild
}

// GroupDayLogChild is a child the day log reports as missing.
type GroupDayLogChild struct {
	StudentID int64
	FirstName string
	LastName  string
	Status    string
	Label     string
}

// ErrGroupDayLogNotToday refuses a day other than today: the student record
// keeps only the current group, so the day log is a live-day view.
var ErrGroupDayLogNotToday = errors.New("the group day log answers today only")

// GroupDayLogs answers today's day log of the given groups without a
// request: the weekly protocol's evening run reads it in the school's
// transaction. It writes no data access entry, because no person reads the
// log; the protocol's views record their own.
func (rs *Resource) GroupDayLogs(ctx context.Context, day calendar.Date, groupIDs []int64) ([]GroupDayLog, error) {
	clock := rs.dayLogClock()
	if !clock.isToday(day) {
		return nil, fmt.Errorf("%w: asked for %s on %s", ErrGroupDayLogNotToday, day, clock.today)
	}
	if len(groupIDs) == 0 {
		return []GroupDayLog{}, nil
	}
	// The log needs only the group ids; their names stay with the protocol.
	groups := make([]*SchoolGroup, 0, len(groupIDs))
	for _, id := range groupIDs {
		groups = append(groups, &SchoolGroup{ID: id})
	}
	data, err := rs.loadDayLogData(ctx, groups, day, clock)
	if err != nil {
		return nil, err
	}
	resp := buildDayLogResponse(day, groups, data)
	result := make([]GroupDayLog, 0, len(resp.Groups))
	for _, group := range resp.Groups {
		entry, err := groupDayLogOf(group)
		if err != nil {
			return nil, err
		}
		result = append(result, entry)
	}
	return result, nil
}

// SchoolDayLog answers today's day log of the whole school as one log
// without a group (GroupID 0): every child whose care covers the day, with a
// group or without one, each once. The weekly protocol of a school in open
// care records it (#5401); like GroupDayLogs it reads without a request.
func (rs *Resource) SchoolDayLog(ctx context.Context, day calendar.Date) (GroupDayLog, error) {
	clock := rs.dayLogClock()
	if !clock.isToday(day) {
		return GroupDayLog{}, fmt.Errorf("%w: asked for %s on %s", ErrGroupDayLogNotToday, day, clock.today)
	}
	records, err := rs.Persons.ListSchoolStudentsForDay(ctx, day, clock.today)
	if err != nil {
		return GroupDayLog{}, err
	}
	wholeSchool := func(*peopleModule.StudentState) (int64, bool) { return 0, true }
	data, err := rs.dayLogDataOf(ctx, records, day, clock, wholeSchool)
	if err != nil {
		return GroupDayLog{}, err
	}
	resp := buildDayLogResponse(day, []*SchoolGroup{{ID: 0}}, data)
	return groupDayLogOf(resp.Groups[0])
}

func groupDayLogOf(group dayLogGroup) (GroupDayLog, error) {
	groupID, err := strconv.ParseInt(group.GroupID, 10, 64)
	if err != nil {
		return GroupDayLog{}, fmt.Errorf("day log group id %q: %w", group.GroupID, err)
	}
	entry := GroupDayLog{
		GroupID: groupID, Present: group.Counters.Present, Sick: group.Counters.Sick,
		ClassTrip: group.Counters.ClassTrip, Excused: group.Counters.Excused, Absent: group.Counters.Absent,
		StudentIDs: make([]int64, 0, len(group.Students)), MissingChild: []GroupDayLogChild{},
	}
	for _, student := range group.Students {
		studentID, err := strconv.ParseInt(student.StudentID, 10, 64)
		if err != nil {
			return GroupDayLog{}, fmt.Errorf("day log student id %q: %w", student.StudentID, err)
		}
		entry.StudentIDs = append(entry.StudentIDs, studentID)
		switch student.Status {
		case dayLogStatusSick, dayLogStatusExcused, dayLogStatusClassTrip, dayLogStatusAbsent:
			entry.MissingChild = append(entry.MissingChild, GroupDayLogChild{
				StudentID: studentID, FirstName: student.FirstName, LastName: student.LastName,
				Status: student.Status, Label: student.Label,
			})
		}
	}
	return entry, nil
}
