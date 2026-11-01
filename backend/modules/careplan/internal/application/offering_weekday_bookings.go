package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingWeekdayBookings lists, per active offering overlapping the period,
// each approved child with the weekdays its bookings plan on or after from
// (#5103). The Regeltermin hint compares them with a hand-kept roster. The
// weekdays follow the resync's rule (effectiveOfferingDaysForEnrollment): a
// link's own days, else the fixed offering's days; a link the rule cannot
// resolve is left out, as the resync would leave it out.
func (m *BookingMaterialization) OfferingWeekdayBookings(ctx context.Context, calendarPeriodID *int64, from calendar.Date) ([]careplan.OfferingWeekdayBooking, error) {
	enabled, err := m.careOfferingsEnabled(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering weekday bookings: resolve care offerings setting: %w", err)
	}
	if !enabled {
		return []careplan.OfferingWeekdayBooking{}, nil
	}
	offerings, err := m.catalog.listTenantRecords(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering weekday bookings: list offerings: %w", err)
	}
	phases, _, err := m.offeringSourcePhases(ctx, calendarPeriodID)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]*careplan.CareOffering, len(offerings))
	ids := make([]int64, 0, len(offerings))
	for i := range offerings {
		if _, ok := phases[offerings[i].PhaseID]; !offerings[i].IsActive || !ok {
			continue
		}
		byID[offerings[i].ID] = &offerings[i]
		ids = append(ids, offerings[i].ID)
	}
	if len(ids) == 0 {
		return []careplan.OfferingWeekdayBooking{}, nil
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, ids, from)
	if err != nil {
		return nil, fmt.Errorf("offering weekday bookings: list approved children: %w", err)
	}
	return collectOfferingWeekdayBookings(byID, phases, children, from, calendarPeriodID), nil
}

type offeringStudentKey struct {
	offeringID int64
	studentID  int64
}

func collectOfferingWeekdayBookings(
	offerings map[int64]*careplan.CareOffering,
	phases map[int64]careplan.OfferingPhase,
	children []ports.ApprovedOfferingChild,
	from calendar.Date,
	calendarPeriodID *int64,
) []careplan.OfferingWeekdayBooking {
	result := make([]careplan.OfferingWeekdayBooking, 0, len(children))
	index := make(map[offeringStudentKey]int, len(children))
	for _, child := range children {
		offering := offerings[child.Link.CareOfferingID]
		if offering == nil || (child.Link.ValidUntil != nil && !child.Link.ValidUntil.After(from)) {
			continue
		}
		link := child.Link
		days, err := effectiveOfferingDaysForEnrollment(offering, &link)
		if err != nil {
			continue
		}
		weekdays := offeringWeekdays(days)
		phase, ok := phases[offering.PhaseID]
		if !ok {
			continue
		}
		// Use the materialized roster's service and link bounds, including
		// the phase's last service day but never dates outside the phase.
		validFrom, validUntil, ok := sourcedRosterWindow(&link, rosterWindow{phase: phase, effectiveFrom: from})
		if !ok {
			continue
		}
		window := careplan.OfferingBookingWindow{
			ValidFrom: &validFrom, ValidUntil: &validUntil, Weekdays: weekdays,
			Ended: link.ValidUntil != nil && *link.ValidUntil == validUntil,
		}
		key := offeringStudentKey{offeringID: offering.ID, studentID: child.StudentID}
		if position, ok := index[key]; ok {
			merged := append(result[position].Weekdays, weekdays...)
			slices.Sort(merged)
			result[position].Weekdays = slices.Compact(merged)
			result[position].Windows = append(result[position].Windows, window)
			continue
		}
		index[key] = len(result)
		result = append(result, careplan.OfferingWeekdayBooking{
			OfferingID:    offering.ID,
			OfferingName:  offering.Name,
			PhaseID:       offering.PhaseID,
			HolidayPhase:  phase.Holiday,
			StudentID:     child.StudentID,
			SchoolClass:   child.SchoolClass,
			GradeLevel:    child.GradeLevel,
			Weekdays:      weekdays,
			CountsAsCare:  offering.CountsAsCare,
			IncludesLunch: offering.IncludesLunch,
			// The offering's decision for the period travels with each
			// booking like its catalog flags, so the gap check needs no read
			// of its own.
			TemplateGapAckClasses: periodTemplateGapAck(offering, calendarPeriodID),
			ClassDayApplies:       phase.SchoolYear && !offering.IncludesHolidayCare,
			PickupTimes:           offeringPickupWeekdays(offering.PickupTimes),
			Windows:               []careplan.OfferingBookingWindow{window},
		})
	}
	return result
}

// periodTemplateGapAck is the offering's decision for the requested
// planning period; without a period there is none.
func periodTemplateGapAck(offering *careplan.CareOffering, calendarPeriodID *int64) []string {
	if calendarPeriodID == nil {
		return nil
	}
	return offering.TemplateGapAcks[*calendarPeriodID]
}

// offeringPickupWeekdays keys the offering's pickup times by ISO weekday;
// an unknown day key or an empty time is left out.
func offeringPickupWeekdays(times map[string]string) map[int]string {
	result := make(map[int]string, len(times))
	for day, hhmm := range times {
		weekday, ok := offeringDayWeekday(day)
		if value := strings.TrimSpace(hhmm); ok && value != "" {
			result[weekday] = value
		}
	}
	return result
}

// offeringWeekdays maps stored day abbreviations to sorted ISO weekdays; an
// empty list books every weekday, like the resync's allWeekdays draft.
func offeringWeekdays(days []string) []int {
	if len(days) == 0 {
		return []int{1, 2, 3, 4, 5, 6, 7}
	}
	weekdays := make([]int, 0, len(days))
	for _, day := range days {
		if weekday, ok := offeringDayWeekday(day); ok {
			weekdays = append(weekdays, weekday)
		}
	}
	slices.Sort(weekdays)
	return slices.Compact(weekdays)
}
