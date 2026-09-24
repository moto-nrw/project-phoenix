package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable/legacy/timetableplanning"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

// TimetablePlanning carries the Timetable owner's planning capabilities the
// timetable HTTP adapter mounts beside the operational day (#3424): the
// template writes, the recurrence gate, the planner's reads, the conflict
// detection, and the retained attendance correction of completed blocks
// (slice S3). services.Factory may not grow a field for each, so they travel
// as one value.
type TimetablePlanning struct {
	Templates             timetable.TemplateAdministration
	RecurrenceLock        timetable.RecurrenceWriteLock
	Data                  timetable.TimetableDataCapability
	ConflictDetection     timetable.ConflictDetectionCapability
	AttendanceCorrections *timetableplanning.AttendanceCorrectionService
}

// timetableMaterializationInputs compose the recurrence engine. CareBounds
// is the per-date care filter (#2487); NonWorkingDays skips holidays and
// closing days (#3594); Broadcaster announces created
// occurrences to the staffing caches (#1844).
type timetableMaterializationInputs struct {
	Rows           repositories.TimetableTemplateRows
	CareBounds     timetableCompose.CareBoundReader
	NonWorkingDays timetable.NonWorkingDayCalendar
	RecurrenceLock timetable.RecurrenceWriteLock
	Broadcaster    realtime.Broadcaster
	DB             *bun.DB
	Logger         *slog.Logger
}

func newTimetableMaterialization(in timetableMaterializationInputs) (timetable.MaterializationCapability, error) {
	return timetableCompose.NewMaterialization(timetableCompose.MaterializationDependencies{
		GroupRepo:      in.Rows.Groups,
		ScheduleRepo:   in.Rows.Schedules,
		EnrollmentRepo: in.Rows.Enrollments,
		SupervisorRepo: in.Rows.Supervisors,
		PeriodRepo:     in.Rows.CalendarPeriods,
		InstanceRepo:   in.Rows.Instances,
		StaffRepo:      in.Rows.InstanceStaff,
		StudentRepo:    in.Rows.Participants,
		ExceptionRepo:  in.Rows.Exceptions,
		TimeframeRepo:  in.Rows.Timeframes,
		CareBounds:     in.CareBounds,
		NonWorkingDays: in.NonWorkingDays,
		RecurrenceLock: in.RecurrenceLock,
		Staffing:       newStaffingAnnouncer(in.Broadcaster),
		DB:             in.DB,
		Logger:         in.Logger,
	})
}

// timetableTemplateInputs compose the template writes. Instances is the
// retained instance lifecycle whose deviation machinery the split preserves
// Vertretungsplan overrides with; CareOfferings and ResyncOfferingRoster are
// Enrollment's.
type timetableTemplateInputs struct {
	Rows                 repositories.TimetableTemplateRows
	PlanningTracks       timetableCompose.PlanningTrackAssignments
	Materialization      timetable.MaterializationCapability
	Instances            timetableplanning.InstanceService
	CareOfferings        enrollment.CareOfferingSeriesValidator
	ResyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error
	RecurrenceLock       timetable.RecurrenceWriteLock
	Broadcaster          realtime.Broadcaster
	DB                   *bun.DB
	Logger               *slog.Logger
	Today                func() timezone.Date
}

func newTimetableTemplates(in timetableTemplateInputs) (timetable.TemplateAdministration, error) {
	if in.CareOfferings == nil || in.Instances == nil {
		return nil, errors.New("timetable templates: care offerings and instance lifecycle are required")
	}
	preserver, err := timetableplanning.NewSeriesDeviationPreserver(in.Instances)
	if err != nil {
		return nil, err
	}
	return timetableCompose.NewTemplateAdministration(timetableCompose.TemplateAdministrationDependencies{
		Groups:               in.Rows.Groups,
		Categories:           in.Rows.Categories,
		Schedules:            in.Rows.Schedules,
		Enrollments:          in.Rows.Enrollments,
		Supervisors:          in.Rows.Supervisors,
		Instances:            in.Rows.Instances,
		InstanceStaff:        in.Rows.InstanceStaff,
		Participants:         in.Rows.Participants,
		Timeframes:           in.Rows.Timeframes,
		PlanningTracks:       in.PlanningTracks,
		EducationGroups:      in.Rows.EducationGroups,
		Materialization:      in.Materialization,
		Deviations:           seriesDeviations{preserver: preserver},
		CareOfferings:        timetableCareOfferingChecks(in.CareOfferings),
		ResyncOfferingRoster: in.ResyncOfferingRoster,
		RecurrenceLock:       in.RecurrenceLock,
		SchoolClasses:        timetableSchoolClassRules(),
		Staffing:             newStaffingAnnouncer(in.Broadcaster),
		Logger:               in.Logger,
		DB:                   in.DB,
		Today:                in.Today,
	})
}

// timetableSchoolClassRules are School Structure's grade range and class
// identity the template writes validate against, served by the retained
// nest beside NormalizeSchoolClass.
func timetableSchoolClassRules() timetableCompose.SchoolClassRules {
	return timetableCompose.SchoolClassRules{
		MinGradeLevel: timetableplanning.MinSchoolGradeLevel,
		MaxGradeLevel: timetableplanning.MaxSchoolGradeLevel,
		Normalize:     timetableplanning.NormalizeSchoolClass,
	}
}

// timetableCareOfferingChecks binds Enrollment's care-offering guards; an
// invalid linked offering is the client-correctable conflict.
func timetableCareOfferingChecks(validator enrollment.CareOfferingSeriesValidator) timetableCompose.CareOfferingChecks {
	return timetableCompose.CareOfferingChecks{
		ValidateSeries:         validator.ValidateTemplateSeries,
		ValidateOfferingSource: validator.ValidateTemplateOfferingSource,
		IsConflict:             enrollment.IsCareOfferingInvalid,
	}
}

// seriesDeviations serves the split's deviation port from the retained
// instance lifecycle.
type seriesDeviations struct {
	preserver *timetableplanning.SeriesDeviationPreserver
}

func (d seriesDeviations) LockDeviationDays(ctx context.Context, tenantID int64, from, to timezone.Date) error {
	return d.preserver.LockDeviationDays(ctx, tenantID, from, to)
}

func (d seriesDeviations) SnapshotDeviations(ctx context.Context, from, to timezone.Date, templateID int64) (timetableCompose.PreservedDeviations, error) {
	snapshot, err := d.preserver.SnapshotDeviations(ctx, from, to, templateID)
	if err != nil {
		return nil, err
	}
	return snapshot, nil
}

// staffingAnnouncer wakes the staffing caches through the realtime hub; the
// event carries no child.
type staffingAnnouncer struct {
	broadcaster realtime.Broadcaster
}

func newStaffingAnnouncer(broadcaster realtime.Broadcaster) timetableCompose.StaffingAnnouncer {
	if broadcaster == nil {
		return nil
	}
	return staffingAnnouncer{broadcaster: broadcaster}
}

func (a staffingAnnouncer) AnnounceStaffingChanged(tenantID int64, source string) error {
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	return a.broadcaster.BroadcastToTenant(tenantID, event)
}
