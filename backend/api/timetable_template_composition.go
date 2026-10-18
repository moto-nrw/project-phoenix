package api

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// TimetableMaterializationDependencies binds the recurrence engine to the
// realtime hub over the template rows.
func TimetableMaterializationDependencies(in TimetableMaterializationInputs) timetablePorts.MaterializationDependencies {
	return timetablePorts.MaterializationDependencies{
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
		Logger:         in.Logger,
	}
}

// TimetableTemplateDependencies binds the template writes to Care Plan's
// catalog guards, School Structure's class rules and the realtime hub.
func TimetableTemplateDependencies(in TimetableTemplateInputs) (timetablePorts.TemplateAdministrationDependencies, error) {
	if in.CareOfferings == nil || in.Deviations == nil {
		return timetablePorts.TemplateAdministrationDependencies{}, errors.New("timetable templates: care offerings and instance lifecycle are required")
	}
	return timetablePorts.TemplateAdministrationDependencies{
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
		Deviations:           in.Deviations,
		CareOfferings:        timetableCareOfferingChecks(in.CareOfferings),
		ResyncOfferingRoster: in.ResyncOfferingRoster,
		RecurrenceLock:       in.RecurrenceLock,
		SchoolClasses:        timetableSchoolClassRules(),
		Staffing:             newStaffingAnnouncer(in.Broadcaster),
		Logger:               in.Logger,
		Today:                in.Today,
	}, nil
}

// timetableSchoolClassRules are School Structure's grade range and class
// identity the template writes validate against.
func timetableSchoolClassRules() timetablePorts.SchoolClassRules {
	return timetablePorts.SchoolClassRules{
		MinGradeLevel: schoolStructure.MinGradeLevel,
		MaxGradeLevel: schoolStructure.MaxGradeLevel,
		Normalize:     schoolStructure.NormalizeClass,
	}
}

// timetableCareOfferingChecks binds the Care Plan catalog's guards; an
// invalid linked offering is the client-correctable conflict.
func timetableCareOfferingChecks(guards careplan.CareOfferingGuards) timetablePorts.CareOfferingChecks {
	return timetablePorts.CareOfferingChecks{
		ValidateSeries:         guards.ValidateTemplateSeries,
		ValidateOfferingSource: guards.ValidateTemplateOfferingSource,
		IsConflict:             IsCareOfferingConfigInvalid,
	}
}

// IsCareOfferingConfigInvalid reports whether err is a care-offering
// compatibility refusal rather than an infrastructure failure.
func IsCareOfferingConfigInvalid(err error) bool {
	return errors.Is(err, careplan.ErrCareOfferingConfigInvalid)
}

// staffingAnnouncer wakes the staffing caches through the realtime hub; the
// event carries no child.
type staffingAnnouncer struct {
	broadcaster realtime.Broadcaster
}

func newStaffingAnnouncer(broadcaster realtime.Broadcaster) timetablePorts.StaffingAnnouncer {
	if broadcaster == nil {
		return nil
	}
	return staffingAnnouncer{broadcaster: broadcaster}
}

func (a staffingAnnouncer) AnnounceStaffingChanged(tenantID int64, source string) error {
	event := realtime.NewEvent(realtime.EventStaffingDeviationChanged, "", realtime.EventData{Source: &source})
	return a.broadcaster.BroadcastToTenant(tenantID, event)
}
