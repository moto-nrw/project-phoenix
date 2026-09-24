package enrollment

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
)

// The rules below read a request child's offering links in the enrollment
// rows the retained intake, decision and offering-change services still
// speak. Care Plan applies its own booking rules to its own records (#3560);
// these move with the services that call them.

// currentOfferingSelectionDate returns the day a child's persisted offering
// links have to be read at to yield the booking that is in force right now.
// Since a dated change splits those links into intervals, reading them without
// a date returns the whole history and reading them at the service start
// returns the superseded booking - both of which a caller asking for "the
// current selection" would misread as a live one. The window clamp keeps the
// answer meaningful outside the service period too: before it starts that is
// the initial selection, after it ended the last one.
func currentOfferingSelectionDate(phase *enrollmentOwner.Phase) timezone.Date {
	return offeringSelectionDateOn(phase, timezone.TodayDate())
}

func offeringSelectionDateOn(phase *enrollmentOwner.Phase, today timezone.Date) timezone.Date {
	if phase == nil {
		return today
	}
	if today.Before(timezone.Date(phase.ServiceStartDate)) {
		return timezone.Date(phase.ServiceStartDate)
	}
	if today.After(timezone.Date(phase.ServiceEndDate)) {
		return timezone.Date(phase.ServiceEndDate)
	}
	return today
}

func offeringIDsFromLinks(links []*RequestChildOffering) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}

func requestChildOfferingLinksHaveCareDays(
	links []*RequestChildOffering,
	offerings map[int64]*enrollmentModels.CareOffering,
) bool {
	for _, link := range links {
		if link == nil {
			continue
		}
		offering := offerings[link.CareOfferingID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		hasCareDays := len(link.SelectedDays) > 0
		if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			hasCareDays = len(offering.AvailableDays) > 0
		}
		if hasCareDays {
			return true
		}
	}
	return false
}

// grandfatheredOfferingsFromLinks classifies the bookings a child already
// holds for the Bestandsschutz exemption (#2186): an availability rule
// tightened after the fact does not revoke what is already on file for THIS
// child, even when the grade rule now excludes it. Newly added blocked
// offerings are not on file and are still rejected with
// ErrCareOfferingUnavailable.
//
// The split follows the DAYS on each link, not the link as a whole: manual
// days are what the admin ticked and can untick, automatic days are derived
// from a trigger and never appear in a payload at all. A link can carry both,
// so the two buckets deliberately OVERLAP (#2186 review) — unticking such a
// booking withdraws only its manual half, and its automatic half must still be
// re-derived while the trigger stays selected. Classifying a mixed link as
// manual only dropped it out of the auto-materialization catalog on that very
// save and deleted its automatic days.
func grandfatheredOfferingsFromLinks(links []*RequestChildOffering) GrandfatheredOfferings {
	grandfathered := GrandfatheredOfferings{
		Manual:    make(map[int64]bool, len(links)),
		Automatic: make(map[int64]bool, len(links)),
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		if len(link.AutomaticSelectedDays) > 0 {
			grandfathered.Automatic[link.CareOfferingID] = true
		}
		// Legacy links carry neither breakdown, only SelectedDays. Nothing
		// derived those, so they are manual by construction.
		if len(link.ManualSelectedDays) > 0 || len(link.AutomaticSelectedDays) == 0 {
			grandfathered.Manual[link.CareOfferingID] = true
		}
	}
	return grandfathered
}

// SchoolClassGradeLevel derives the numeric Jahrgang from the free-text
// school class ("3a" -> 3), the grade Care Plan's Jahrgang filters of
// offering-sourced Regeltermine match on (#2137). Classes without a
// supported grade number ("Bienen") yield nil — a set grade filter then
// never matches.
func SchoolClassGradeLevel(schoolClass string) *int16 {
	prefix := schoolclass.GradePrefix(schoolClass)
	if prefix == "" {
		return nil
	}
	parsed, err := strconv.Atoi(prefix)
	if err != nil || parsed < schoolclass.MinGradeLevel || parsed > schoolclass.MaxGradeLevel {
		return nil
	}
	grade := int16(parsed)
	return &grade
}

// nativeOfferingCatalog describes enrollment offering rows in the terms of
// Enrollment's selection engine.
func nativeOfferingCatalog(catalog map[int64]*enrollmentModels.CareOffering) map[int64]*selection.Offering {
	result := make(map[int64]*selection.Offering, len(catalog))
	for id, offering := range catalog {
		if offering == nil {
			continue
		}
		native := &selection.Offering{ID: offering.ID, SortOrder: offering.SortOrder, DaysOfWeekMode: offering.DaysOfWeekMode, AvailableDays: offering.AvailableDays, CountsAsCare: offering.CountsAsCare, IncludesLunch: offering.IncludesLunch, IsRequired: offering.IsRequired, SelectionGroup: offering.SelectionGroup, SelectionRule: offering.SelectionRule, AutoAddTriggerOfferingIDs: offering.AutoAddTriggerOfferingIDs, AutoAddGradeLevels: offering.AutoAddGradeLevels}
		if rule := offering.AvailabilityRule; rule != nil {
			native.AvailabilityRule = &selection.AvailabilityRule{Match: rule.Match, Conditions: make([]selection.AvailabilityCondition, len(rule.Conditions))}
			for i, condition := range rule.Conditions {
				native.AvailabilityRule.Conditions[i] = selection.AvailabilityCondition{Source: condition.Source, Operator: condition.Operator, Value: append([]int(nil), condition.Value...)}
			}
		}
		result[id] = native
	}
	return result
}
