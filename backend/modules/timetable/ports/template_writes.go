package ports

import (
	"context"
	"fmt"
)

// ScheduleError wraps a failed step of the template writes, the roster
// maintenance or the materialization with the operation that failed.
// errors.Is and errors.As see the cause.
type ScheduleError struct {
	Op  string // Operation that failed
	Err error  // Original error
}

func (e *ScheduleError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("schedule error during %s", e.Op)
	}
	return fmt.Sprintf("schedule error during %s: %v", e.Op, e.Err)
}

func (e *ScheduleError) Unwrap() error {
	return e.Err
}

// StaffingAnnouncer wakes the tenant's planner and "Heute geplant" caches
// after writes that bypass the instance lifecycle (#1844). The event carries
// no payload; the composition root binds the realtime broadcaster.
type StaffingAnnouncer interface {
	AnnounceStaffingChanged(tenantID int64, source string) error
}

// EducationGroupDirectory is School Structure's existence check of an
// education group in the caller's tenant.
type EducationGroupDirectory interface {
	Exists(ctx context.Context, id int64) (bool, error)
}

// SchoolClassRules are School Structure's supported grade range and the
// identity of a free-text school class, bound at the root. The template
// writes validate grade targets and filters against them and compare class
// filters by their normalized name.
type SchoolClassRules struct {
	MinGradeLevel int
	MaxGradeLevel int
	Normalize     func(string) string
}

// CareOfferingChecks are Care Plan's guards of template mutations, bound at
// the root. ValidateSeries rejects a mutation that makes a linked care
// offering impossible to materialize; ValidateOfferingSource resolves the
// offering-source references before the template row carrying them is
// written (#2147); IsConflict tells the client-correctable verdict of
// ValidateSeries from an operational failure.
type CareOfferingChecks struct {
	ValidateSeries         func(ctx context.Context, templateID int64) error
	ValidateOfferingSource func(ctx context.Context, offeringIDs, storedOfferingIDs []int64, calendarPeriodID *int64) error
	IsConflict             func(error) bool
}

// PlanningTrackAssignments is the slice of the Timetable owner's
// planning-track administration (timetable.PlanningTrackAdministration) the
// template writes check a track assignment against.
type PlanningTrackAssignments interface {
	ValidatePlanningTrackAssignment(ctx context.Context, id, allowedArchivedID *int64) error
}
