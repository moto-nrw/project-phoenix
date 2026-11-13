package ports

import (
	"context"
	"log/slog"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// TemplatePeriods reads the planning periods a series edit re-plans its
// planned occurrences through (#6838).
type TemplatePeriods interface {
	FindByID(context.Context, int64) (*schedulerecords.CalendarPeriod, error)
	FindActiveByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)
}

// TemplateAdministrationDependencies wires timetable.TemplateAdministration:
// the retained template rows, the owner's materialization and recurrence
// gate, and the ports the composition root binds — the instance lifecycle's
// deviation preservation, Care Plan's care-offering checks, Enrollment's
// roster resync (late-bound: the decision service is built after this one),
// School
// Structure's education groups and class rules, and the realtime staffing
// announcement. Staffing, Logger and Today are optional.
type TemplateAdministrationDependencies struct {
	Groups      GroupRows
	Categories  timetable.CategoryQuery
	Schedules   timetable.ScheduleCapability
	Enrollments timetable.StudentEnrollmentCapability
	Supervisors timetable.PlannedSupervisorCapability
	// Directories name the enrolled children with their alumni state and
	// the planned supervisors' persons.
	Directories     RepositoryDirectories
	Instances       InstanceRows
	InstanceStaff   timetable.InstanceStaffCapability
	Participants    ParticipantRows
	Timeframes      timetable.TimeframeCapability
	PlanningTracks  PlanningTrackAssignments
	EducationGroups EducationGroupDirectory
	Materialization timetable.MaterializationCapability
	Deviations      SeriesDeviations
	// Periods bound the re-plan an update requests (#6838).
	Periods              TemplatePeriods
	CareOfferings        CareOfferingChecks
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	SickReports          SickReportReader // nil: a series roster change marks nobody sick (#6292)
	SchoolClasses        SchoolClassRules
	Staffing             StaffingAnnouncer
	Logger               *slog.Logger
	Transactions         TenantTransactions
	Today                func() calendar.Date
}
