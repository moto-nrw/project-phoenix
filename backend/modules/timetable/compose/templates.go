package compose

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// The template, materialization, series conversion, roster and conflict
// services live in internal/application (#4084). The roots build them through
// the constructors and dependency types below.

type (
	ArrivalBaselineProjector             = application.ArrivalBaselineProjector
	ArrivalBaselines                     = application.ArrivalBaselines
	CareBoundReader                      = application.CareBoundReader
	CareOfferingChecks                   = application.CareOfferingChecks
	ConflictArrivalExceptions            = application.ConflictArrivalExceptions
	ConflictCalendarPeriods              = application.ConflictCalendarPeriods
	ConflictDetectionDependencies        = application.ConflictDetectionDependencies
	ConflictExceptions                   = application.ConflictExceptions
	ConflictInstanceStaff                = application.ConflictInstanceStaff
	ConflictInstanceStudents             = application.ConflictInstanceStudents
	ConflictInstances                    = application.ConflictInstances
	ConflictPresence                     = application.ConflictPresence
	ConflictSchedules                    = application.ConflictSchedules
	ConflictSessions                     = application.ConflictSessions
	ConflictShifts                       = application.ConflictShifts
	ConflictStaffDirectory               = application.ConflictStaffDirectory
	InstanceSeriesConversionDependencies = application.InstanceSeriesConversionDependencies
	MaterializationDependencies          = application.MaterializationDependencies
	PlanningTrackAssignments             = application.PlanningTrackAssignments
	PreservedDeviations                  = application.PreservedDeviations
	RosterReconciler                     = application.RosterReconciler
	ScheduleError                        = application.ScheduleError
	SchoolClassRules                     = application.SchoolClassRules
	SeriesDeviations                     = application.SeriesDeviations
	StaffingAnnouncer                    = application.StaffingAnnouncer
	TemplateAdministrationDependencies   = application.TemplateAdministrationDependencies
)

func NewConflictDetection(deps ConflictDetectionDependencies) (timetable.ConflictDetectionCapability, error) {
	return application.NewConflictDetection(deps)
}

func NewInstanceSeriesConversion(deps InstanceSeriesConversionDependencies) (timetable.InstanceSeriesConversion, error) {
	return application.NewInstanceSeriesConversion(deps)
}

func NewMaterialization(deps MaterializationDependencies) (timetable.MaterializationCapability, error) {
	return application.NewMaterialization(deps)
}

func NewTemplateAdministration(deps TemplateAdministrationDependencies) (timetable.TemplateAdministration, error) {
	return application.NewTemplateAdministration(deps)
}

func NewRosterReconciler(
	instanceRepo schedule.ActivityInstanceRepository,
	instanceStudentRepo schedule.InstanceStudentRepository,
	enrollmentRepo activities.StudentEnrollmentRepository,
	logger *slog.Logger,
	now ...func() time.Time,
) *RosterReconciler {
	return application.NewRosterReconciler(instanceRepo, instanceStudentRepo, enrollmentRepo, logger, now...)
}
