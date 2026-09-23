package repositories

import (
	"context"
	"log/slog"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// The cutover of activity execution and attendance to Student Presence (#2762)
// leaves the two owners asking each other one question each: Timetable asks
// which of its planned rows already carry an execution or observed presence,
// Student Presence asks which participants the plan lists. Both questions go
// through consumer-owned ports; these are their bindings.

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
	query carePlanCompose.ExceptionQueries
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
	values, err := pickupExceptionDirectory(d).ListPickupExceptions(ctx, timetableCompose.PickupExceptionFilter(filter))
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
	values, err := pickupExceptionDirectory(d).ListStudentStatusDays(ctx, timetableCompose.StudentStatusDayFilter(filter))
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

// StudentLocker locks one child's row for the care-day locks.
type StudentLocker interface {
	LockStudent(context.Context, int64) error
}

// NewPresenceRulesDependencies binds the attendance rules of Student Presence
// to the given Timetable owner and to the Care Plan queries and locks of the
// database.
func NewPresenceRulesDependencies(db *bun.DB, capability timetable.InstanceStudentQuery, students StudentLocker, observe func(carePlanCompose.Observation)) (PresenceRulesDependencies, error) {
	queries, err := carePlanCompose.NewExceptionQueries(db, observe)
	if err != nil {
		return PresenceRulesDependencies{}, err
	}
	locks, err := carePlanCompose.NewDayLocks(db, students.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		return PresenceRulesDependencies{}, err
	}
	return PresenceRulesDependencies{
		Roster:   NewTimetablePlannedRoster(capability),
		CarePlan: presenceCarePlanDirectory{query: queries},
		CareDays: locks,
	}, nil
}

// NewStudentPresenceWithRules composes Student Presence with its attendance
// rules bound to the given Timetable owner.
func NewStudentPresenceWithRules(db *bun.DB, rules PresenceRulesDependencies, observe func(presenceCompose.Observation)) (*studentpresence.Module, error) {
	return presenceCompose.New(presenceCompose.Dependencies{
		DB: db, Observe: observe, Roster: rules.Roster, CarePlan: rules.CarePlan, CareDays: rules.CareDays,
	})
}

// NewStudentPresence composes the Student Presence owner with its attendance
// rules bound to an unobserved Timetable owner over the same database. The
// serving roots and the retained services compose their presence module
// through it so every attendance rule sees the same planned roster.
func NewStudentPresence(db *bun.DB, observe func(presenceCompose.Observation)) (*studentpresence.Module, error) {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	rooms, err := NewFacilities(db)
	if err != nil {
		return nil, err
	}
	capability, err := NewTimetable(db, students, rooms)
	if err != nil {
		return nil, err
	}
	rules, err := NewPresenceRulesDependencies(db, capability, students, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	return NewStudentPresenceWithRules(db, rules, observe)
}

func presenceObserver() func(presenceCompose.Observation) {
	return func(observation presenceCompose.Observation) {
		if observation.Err != nil {
			slog.Default().Warn("presence operation failed",
				"operation", observation.Operation,
				"error", observation.Err,
			)
		}
	}
}

// newStudentPresence composes the bootstrap Student Presence owner behind the
// legacy repository adapters.
func newStudentPresence(db *bun.DB) *studentpresence.Module {
	module, err := NewStudentPresence(db, presenceObserver())
	if err != nil {
		panic(err)
	}
	return module
}

// NewPresenceFacts composes the Student Presence queries Timetable's session
// facts read. They need no roster, which is what keeps the two owners from
// composing each other forever.
func NewPresenceFacts(db *bun.DB) timetable.SessionFacts {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: presenceObserver()})
	if err != nil {
		panic(err)
	}
	return NewPresenceSessionFacts(module)
}

// NewPresenceSessionFacts serves Timetable's SessionFacts port from the Student
// Presence owner.
func NewPresenceSessionFacts(presence studentpresence.Query) timetable.SessionFacts {
	return timetableCompose.NewPresenceSessionFacts(presence)
}
