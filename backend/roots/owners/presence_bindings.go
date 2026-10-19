package owners

import (
	"context"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// TimetablePlannedRoster serves Student Presence's PlannedRoster port from
// the Timetable owner.
type TimetablePlannedRoster struct {
	timetable interface {
		ListPlannedInstanceStudents(context.Context, timetable.InstanceStudentFilter) ([]timetable.PlannedInstanceStudent, error)
	}
}

func NewTimetablePlannedRoster(capability timetable.InstanceStudentQuery) studentpresence.PlannedRoster {
	if capability == nil {
		panic("timetable planned roster: timetable is required")
	}
	return TimetablePlannedRoster{timetable: capability}
}

func (r TimetablePlannedRoster) ListPlannedParticipants(ctx context.Context, filter studentpresence.PlannedRosterFilter) ([]studentpresence.PlannedParticipant, error) {
	query := timetable.InstanceStudentFilter{IDs: filter.IDs, InstanceIDs: filter.InstanceIDs, StudentIDs: filter.StudentIDs, ExcludeCancelled: filter.ExcludeCancelled}
	if filter.Date != "" {
		date := filter.Date
		query.Date = &date
	}
	if filter.FromClock != "" {
		clock := filter.FromClock
		query.FromClock = &clock
	}
	rows, err := r.timetable.ListPlannedInstanceStudents(ctx, query)
	if err != nil {
		return nil, err
	}
	result := make([]studentpresence.PlannedParticipant, 0, len(rows))
	for _, row := range rows {
		result = append(result, studentpresence.PlannedParticipant(row))
	}
	return result, nil
}

// presenceCarePlanDirectory serves Student Presence's care-plan port from the
// Care Plan exception queries.
type presenceCarePlanDirectory struct {
	query carePlanPorts.ExceptionQueries
}

func (d presenceCarePlanDirectory) FindPickupException(ctx context.Context, id int64) (*studentpresence.SessionPickupException, error) {
	value, err := pickupExceptionDirectory(d).FindPickupException(ctx, id)
	if value == nil || err != nil {
		return nil, err
	}
	result := studentpresence.SessionPickupException(*value)
	return &result, nil
}

func (d presenceCarePlanDirectory) ListPickupExceptions(ctx context.Context, filter studentpresence.SessionPickupExceptionFilter) ([]studentpresence.SessionPickupException, error) {
	values, err := pickupExceptionDirectory(d).ListPickupExceptions(ctx, timetablePorts.PickupExceptionFilter(filter))
	result := make([]studentpresence.SessionPickupException, 0, len(values))
	for _, value := range values {
		result = append(result, studentpresence.SessionPickupException(value))
	}
	return result, err
}

func (d presenceCarePlanDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*studentpresence.SessionStudentStatusDay, error) {
	value, err := pickupExceptionDirectory(d).FindStudentStatusDay(ctx, id, activeOnly)
	if value == nil || err != nil {
		return nil, err
	}
	result := studentpresence.SessionStudentStatusDay(*value)
	return &result, nil
}

func (d presenceCarePlanDirectory) ListStudentStatusDays(ctx context.Context, filter studentpresence.SessionStudentStatusDayFilter) ([]studentpresence.SessionStudentStatusDay, error) {
	values, err := pickupExceptionDirectory(d).ListStudentStatusDays(ctx, timetablePorts.StudentStatusDayFilter(filter))
	result := make([]studentpresence.SessionStudentStatusDay, 0, len(values))
	for _, value := range values {
		result = append(result, studentpresence.SessionStudentStatusDay(value))
	}
	return result, err
}

// PresenceRulesDependencies are the ports Student Presence's attendance rules
// need, bound for one graph.
type PresenceRulesDependencies struct {
	Roster   studentpresence.PlannedRoster
	CarePlan studentpresence.SessionCarePlanDirectory
	CareDays studentpresence.SessionCareDayLocker
}

// NewPresenceRulesDependencies binds the attendance rules of Student Presence
// to the given Timetable owner and to the built Care Plan queries and locks.
func NewPresenceRulesDependencies(capability timetable.InstanceStudentQuery, queries carePlanPorts.ExceptionQueries, locks carePlanCompose.DayLocks) PresenceRulesDependencies {
	return PresenceRulesDependencies{
		Roster:   NewTimetablePlannedRoster(capability),
		CarePlan: presenceCarePlanDirectory{query: queries},
		CareDays: locks,
	}
}
