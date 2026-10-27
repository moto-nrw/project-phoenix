package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OfferingWeekdayBookings lists, per active offering fitting the period,
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
	return collectOfferingWeekdayBookings(byID, children, from), nil
}

type offeringStudentKey struct {
	offeringID int64
	studentID  int64
}

func collectOfferingWeekdayBookings(
	offerings map[int64]*careplan.CareOffering,
	children []ports.ApprovedOfferingChild,
	from calendar.Date,
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
		key := offeringStudentKey{offeringID: offering.ID, studentID: child.StudentID}
		if position, ok := index[key]; ok {
			merged := append(result[position].Weekdays, weekdays...)
			slices.Sort(merged)
			result[position].Weekdays = slices.Compact(merged)
			continue
		}
		index[key] = len(result)
		result = append(result, careplan.OfferingWeekdayBooking{
			OfferingID:    offering.ID,
			OfferingName:  offering.Name,
			StudentID:     child.StudentID,
			SchoolClass:   child.SchoolClass,
			GradeLevel:    child.GradeLevel,
			Weekdays:      weekdays,
			CountsAsCare:  offering.CountsAsCare,
			IncludesLunch: offering.IncludesLunch,
		})
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
