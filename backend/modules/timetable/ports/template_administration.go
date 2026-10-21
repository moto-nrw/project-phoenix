package ports

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TemplateAdministrationDependencies wires timetable.TemplateAdministration:
// the retained template rows, the owner's materialization and recurrence
// gate, and the ports the composition root binds — the instance lifecycle's
// deviation preservation, Care Plan's care-offering checks, Enrollment's
// roster resync (late-bound: the decision service is built after this one),
// School
// Structure's education groups and class rules, and the realtime staffing
// announcement. Staffing, Logger and Today are optional.
type TemplateAdministrationDependencies struct {
	Groups               GroupRows
	Categories           timetable.CategoryQuery
	Schedules            timetable.ScheduleCapability
	Enrollments          ActivityEnrollmentRecords
	Supervisors          PlannedSupervisorRecords
	Instances            ActivityInstanceRecords
	InstanceStaff        InstanceStaffRecords
	Participants         InstanceStudentRecords
	Timeframes           timetable.TimeframeCapability
	PlanningTracks       PlanningTrackAssignments
	EducationGroups      EducationGroupDirectory
	Materialization      timetable.MaterializationCapability
	Deviations           SeriesDeviations
	CareOfferings        CareOfferingChecks
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	SchoolClasses        SchoolClassRules
	Staffing             StaffingAnnouncer
	Logger               *slog.Logger
	Transactions         TenantTransactions
	Today                func() calendar.Date
}
