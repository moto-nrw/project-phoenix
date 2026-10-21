package api

import (
	"context"
	"errors"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	schoolStructureOwner "github.com/moto-nrw/project-phoenix/modules/schoolstructure"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// TimetableTemplateDependencies binds the template writes to Care Plan's
// catalog guards, School Structure's class rules and the realtime hub over
// the template rows. deviations is the instance lifecycle's deviation
// machinery the split preserves Vertretungsplan overrides with;
// careOfferings and resyncOfferingRoster are Enrollment's. The composition
// root adds its database handle to the dependencies they bind.
func TimetableTemplateDependencies(
	rows timetableCompose.TemplateRows, planningTracks timetablePorts.PlanningTrackAssignments, materialization timetable.MaterializationCapability,
	deviations timetablePorts.SeriesDeviations, careOfferings careplan.CareOfferingGuards,
	resyncOfferingRoster func(context.Context, timetable.OfferingRosterResyncInput) error, recurrenceLock timetable.RecurrenceWriteLock,
	broadcaster realtime.Broadcaster, logger *slog.Logger, today func() timezone.Date,
) (timetablePorts.TemplateAdministrationDependencies, error) {
	if careOfferings == nil || deviations == nil {
		return timetablePorts.TemplateAdministrationDependencies{}, errors.New("timetable templates: care offerings and instance lifecycle are required")
	}
	return timetablePorts.TemplateAdministrationDependencies{
		Groups:               rows.Groups,
		Categories:           rows.Categories,
		Schedules:            rows.Schedules,
		Enrollments:          rows.Enrollments,
		Supervisors:          rows.Supervisors,
		Directories:          rows.Directories,
		Instances:            rows.Instances,
		InstanceStaff:        rows.InstanceStaff,
		Participants:         rows.Participants,
		Timeframes:           rows.Timeframes,
		PlanningTracks:       planningTracks,
		EducationGroups:      rows.EducationGroups,
		Materialization:      materialization,
		Deviations:           deviations,
		CareOfferings:        timetableCareOfferingChecks(careOfferings),
		ResyncOfferingRoster: resyncOfferingRoster,
		RecurrenceLock:       recurrenceLock,
		SchoolClasses:        timetableSchoolClassRules(),
		Staffing:             owners.NewStaffingAnnouncer(broadcaster),
		Logger:               logger,
		Today:                today,
	}, nil
}

// timetableSchoolClassRules are School Structure's grade range and class
// identity the template writes validate against.
func timetableSchoolClassRules() timetablePorts.SchoolClassRules {
	return timetablePorts.SchoolClassRules{
		MinGradeLevel: schoolStructureOwner.MinGradeLevel,
		MaxGradeLevel: schoolStructureOwner.MaxGradeLevel,
		Normalize:     schoolStructureOwner.NormalizeClass,
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
