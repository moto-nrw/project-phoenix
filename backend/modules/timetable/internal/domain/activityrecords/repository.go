package activityrecords

import (
	"time"
)

// CategoryRepository defines operations for managing activity categories

// GroupRepository defines operations for managing activity groups

// GroupTargetRepository manages dynamic target cohorts for timetable templates.

// TemplateStartTime is a (activity_group_id, weekday) → timeframe.start_time
// lookup row returned by ScheduleRepository.FindTemplateStartTimesByGroupIDs.
// Used by the WP-B13 exception-conflict endpoint to resolve the "original"
// start_time for modified exceptions.
//
// Multiple rows may share the same (ActivityGroupID, Weekday) when a template
// has several schedules on the same weekday (e.g. morning + afternoon slots).
// The caller is responsible for disambiguating or flagging ambiguity. Rows
// are returned sorted by (ActivityGroupID ASC, Weekday ASC, StartTime ASC),
// so the earliest slot comes first when multiple exist for a given key.
type TemplateStartTime struct {
	ActivityGroupID int64     `bun:"activity_group_id"`
	Weekday         int       `bun:"weekday"`
	StartTime       time.Time `bun:"start_time"`
}

// ScheduleRepository defines operations for managing activity schedules

// SupervisorPlannedRepository defines operations for managing activity supervisors

type PlannedSupervisionBlocker struct {
	ID           int64
	ActivityID   int64
	ActivityName string
	IsPrimary    bool
}

// StudentEnrollmentRepository defines operations for managing student enrollments

// StudentEnrollmentDate keeps legacy compatibility adapters from importing
// the lower-layer calendar-date package.
