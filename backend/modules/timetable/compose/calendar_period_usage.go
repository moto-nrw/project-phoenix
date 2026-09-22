package compose

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// CalendarPeriodReferences mirrors the Timetable owner's per-period reference
// counts. The composition root adapts the owner's type; this package must not
// import the Timetable public package (architecture policy).
type CalendarPeriodReferences struct {
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// CalendarPeriodUsageRepository counts how many planning objects reference a
// calendar period. The periods themselves belong to the School Calendar
// owner (#2666); the referencing rows belong to Enrollment (phases) and
// Timetable (every other planning table), so this repository only composes
// the two owners' counts and touches no table itself (#3124). A period
// without references is simply absent from the result.
type CalendarPeriodUsageRepository struct {
	enrollment      EnrollmentPhaseQueries
	countReferences func(context.Context) (map[int64]CalendarPeriodReferences, error)
}

type EnrollmentPhaseQueries interface {
	PhaseCountsByCalendarPeriod(context.Context) (map[int64]int, error)
}

// NewCalendarPeriodUsageRepository creates a new CalendarPeriodUsageRepository.
func NewCalendarPeriodUsageRepository(enrollment EnrollmentPhaseQueries, countReferences func(context.Context) (map[int64]CalendarPeriodReferences, error)) *CalendarPeriodUsageRepository {
	if enrollment == nil {
		panic("calendar period usage: enrollment queries are required")
	}
	if countReferences == nil {
		panic("calendar period usage: timetable reference query is required")
	}
	return &CalendarPeriodUsageRepository{enrollment: enrollment, countReferences: countReferences}
}

// UsageCounts returns, per calendar period of the current tenant, how many
// rows reference it through nullable calendar_period_id FKs. Periods without
// references are omitted from the map. Each owner answers with one statement:
// Enrollment for its phases, Timetable for its five planning tables.
func (r *CalendarPeriodUsageRepository) UsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	usage, err := r.Usage(ctx)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]schedule.CalendarPeriodUsage, len(usage))
	for id, entry := range usage {
		result[id] = schedule.CalendarPeriodUsage(entry)
	}
	return result, nil
}

// CalendarPeriodUsage counts, per calendar period, the rows that reference it
// through nullable calendar_period_id FKs: Enrollment's phases and the five
// Timetable planning tables.
type CalendarPeriodUsage struct {
	EnrollmentPhases   int
	ActivityGroups     int
	Schedules          int
	StudentEnrollments int
	Supervisors        int
	ActivityInstances  int
}

// Usage is UsageCounts in the composition's own shape.
func (r *CalendarPeriodUsageRepository) Usage(ctx context.Context) (map[int64]CalendarPeriodUsage, error) {
	phaseCounts, err := r.enrollment.PhaseCountsByCalendarPeriod(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "usage counts", Err: err}
	}
	references, err := r.countReferences(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "usage counts", Err: err}
	}

	usage := make(map[int64]CalendarPeriodUsage, len(phaseCounts)+len(references))
	for id, count := range phaseCounts {
		usage[id] = CalendarPeriodUsage{EnrollmentPhases: count}
	}
	for id, refs := range references {
		entry := usage[id]
		entry.ActivityGroups = refs.ActivityGroups
		entry.Schedules = refs.Schedules
		entry.StudentEnrollments = refs.StudentEnrollments
		entry.Supervisors = refs.Supervisors
		entry.ActivityInstances = refs.ActivityInstances
		usage[id] = entry
	}
	return usage, nil
}
