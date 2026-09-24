package timetable

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// templateRosterMaintenanceResponse is the Regeltermin indicator (#3140):
// whether children who are approved later reach the roster and the already
// created future occurrences without staff action.
type templateRosterMaintenanceResponse struct {
	// Mode is "automatic", "partial" or "manual".
	Mode string `json:"mode"`
	// Offerings feed the roster effectively.
	Offerings []templateRosterMaintenanceOfferingResponse `json:"offerings"`
	// GradeLevels / SchoolClasses restrict the source offerings.
	GradeLevels   []int    `json:"grade_levels,omitempty"`
	SchoolClasses []string `json:"school_classes,omitempty"`
	// InactiveOfferings are switched-off source offerings; they stop the
	// source rule until the offering is active again or removed.
	InactiveOfferings []templateRosterMaintenanceOfferingResponse `json:"inactive_offerings,omitempty"`
	// InvalidOfferings are active sources that resync cannot use for this
	// Regeltermin.
	InvalidOfferings []templateRosterMaintenanceOfferingResponse `json:"invalid_offerings,omitempty"`
	// DynamicTargets: a Klasse, Jahrgang or Gruppe target exists. Children who
	// join it later are not added to occurrences that already exist.
	DynamicTargets bool `json:"dynamic_targets"`
	// CareOfferingsDisabled: offerings are configured, but the school has
	// switched Betreuungsangebote off.
	CareOfferingsDisabled bool `json:"care_offerings_disabled"`
}

type templateRosterMaintenanceOfferingResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// attachRosterMaintenance fills RosterMaintenance on every template with a
// constant number of reads. A failed read leaves the field empty instead of
// failing the Regeltermin list: the indicator is information, not a gate.
func (rs *Resource) attachRosterMaintenance(
	ctx context.Context,
	templates []templateResponse,
	requestedPeriodID *int64,
) {
	if rs.OfferingSourceOptions == nil || len(templates) == 0 || requestedPeriodID == nil {
		return
	}
	queries := make([]enrollmentSvc.TemplateRosterFeedQuery, 0, len(templates))
	for _, template := range templates {
		queries = append(queries, enrollmentSvc.TemplateRosterFeedQuery{
			TemplateID:            template.ID,
			CalendarPeriodID:      rosterMaintenanceCalendarPeriodID(template, requestedPeriodID),
			SourceCareOfferingIDs: template.SourceCareOfferingIDs,
		})
	}
	feeds, err := rs.OfferingSourceOptions.TemplateRosterMaintenanceFeeds(ctx, queries)
	if err != nil {
		rs.getLogger().Warn("template list: roster maintenance read failed, omitting indicator",
			slog.String("error", err.Error()),
		)
		return
	}
	for i := range templates {
		derived := enrollmentSvc.DeriveTemplateRosterMaintenance(enrollmentSvc.TemplateRosterMaintenanceInput{
			SourceCareOfferingIDs: templates[i].SourceCareOfferingIDs,
			SourceGradeLevels:     templates[i].SourceGradeLevels,
			SourceSchoolClasses:   templates[i].SourceSchoolClasses,
			HasDynamicTargets:     templateHasDynamicTargets(templates[i]),
			Feeds:                 feeds[templates[i].ID],
		})
		templates[i].RosterMaintenance = rosterMaintenanceResponse(derived)
	}
}

// rosterMaintenanceCalendarPeriodID mirrors the schedule-first period
// resolution used for materialization for a period-scoped read.
func rosterMaintenanceCalendarPeriodID(template templateResponse, requestedPeriodID *int64) *int64 {
	if requestedPeriodID != nil {
		for _, schedule := range template.Schedules {
			if schedule.CalendarPeriodID != nil && *schedule.CalendarPeriodID == *requestedPeriodID {
				return schedule.CalendarPeriodID
			}
		}
	}
	if len(template.Schedules) > 0 && template.Schedules[0].CalendarPeriodID != nil {
		return template.Schedules[0].CalendarPeriodID
	}
	if template.CalendarPeriodID != nil {
		return template.CalendarPeriodID
	}
	return requestedPeriodID
}

func templateHasDynamicTargets(template templateResponse) bool {
	if len(template.Targets) > 0 {
		return true
	}
	switch template.TargetGroupType {
	case timetable.TargetGroupTypeGrade, timetable.TargetGroupTypeSchoolClass, timetable.TargetGroupTypeEducationGroup:
		return true
	}
	return false
}

func rosterMaintenanceResponse(derived enrollmentSvc.TemplateRosterMaintenance) *templateRosterMaintenanceResponse {
	return &templateRosterMaintenanceResponse{
		Mode:                  string(derived.Mode),
		Offerings:             rosterMaintenanceOfferings(derived.Offerings),
		GradeLevels:           derived.GradeLevels,
		SchoolClasses:         derived.SchoolClasses,
		InactiveOfferings:     rosterMaintenanceOfferings(derived.InactiveOfferings),
		InvalidOfferings:      rosterMaintenanceOfferings(derived.InvalidOfferings),
		DynamicTargets:        derived.DynamicTargetsManual,
		CareOfferingsDisabled: derived.CareOfferingsDisabled,
	}
}

func rosterMaintenanceOfferings(offerings []enrollmentSvc.RosterMaintenanceOffering) []templateRosterMaintenanceOfferingResponse {
	result := make([]templateRosterMaintenanceOfferingResponse, 0, len(offerings))
	for _, offering := range offerings {
		result = append(result, templateRosterMaintenanceOfferingResponse{ID: offering.ID, Name: offering.Name})
	}
	return result
}
