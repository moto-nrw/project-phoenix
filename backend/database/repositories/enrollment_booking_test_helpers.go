package repositories

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// EnrollmentBookingFixture writes test arrangements through the two storage owners.
// Production workflows use the owner commands directly.
type EnrollmentBookingFixture struct {
	EnrollmentBookingProjection
	bookings     *careplan.OfferingBookings
	withinTenant func(context.Context, func(context.Context) error) error
}

func NewEnrollmentBookingFixture(withinTenant func(context.Context, func(context.Context) error) error) EnrollmentBookingFixture {
	return EnrollmentBookingFixture{NewEnrollmentBookingProjection(enrollmentCompose.New()), carePlanCompose.NewOfferingBookings(), withinTenant}
}

// These retained composition methods translate the old fixture-facing command
// shape into owner commands. They never access the compatibility storage.
func (a EnrollmentBookingFixture) InsertRequestChildOffering(ctx context.Context, selection *enrollment.RequestChildOffering) error {
	if selection == nil || selection.RequestChildID <= 0 {
		return fmt.Errorf("request_child_id is required")
	}
	return a.withinTenant(ctx, func(ctx context.Context) error {
		bookings, err := a.bookingCommandValues(ctx, selection.RequestChildID, nil, []*enrollment.RequestChildOffering{selection})
		if err != nil {
			return err
		}
		choices, err := a.SubmittedOfferingChoices(ctx, []int64{selection.RequestChildID})
		if err != nil {
			return err
		}
		found := false
		for _, choice := range choices {
			if choice.CareOfferingID == selection.CareOfferingID {
				found = true
				break
			}
		}
		if !found {
			err = a.RecordSubmittedOfferingChoices(ctx, selection.RequestChildID, []enrollment.SubmittedOfferingChoice{{CareOfferingID: selection.CareOfferingID, SelectedDays: bookings[0].ManualSelectedDays, Notes: selection.Notes}})
			if err != nil {
				return err
			}
		}
		if err := a.bookings.RecordCareOfferingBookings(ctx, selection.RequestChildID, bookings); err != nil {
			return err
		}
		return a.refreshBookingCommandRows(ctx, selection.RequestChildID, []*enrollment.RequestChildOffering{selection})
	})
}

func (a EnrollmentBookingFixture) ReplaceRequestChildOfferings(ctx context.Context, childID int64, rows []*enrollment.RequestChildOffering) error {
	return a.withinTenant(ctx, func(ctx context.Context) error {
		bookings, err := a.bookingCommandValues(ctx, childID, nil, rows)
		if err != nil {
			return err
		}
		if err := a.bookings.ReplaceCareOfferingBookings(ctx, childID, bookings); err != nil {
			return fmt.Errorf("failed to insert replacement request child offerings: %w", err)
		}
		return a.refreshBookingCommandRows(ctx, childID, rows)
	})
}

func (a EnrollmentBookingFixture) ScheduleRequestChildOfferings(ctx context.Context, childID int64, from enrollment.Date, rows []*enrollment.RequestChildOffering) error {
	if from.IsZero() {
		return fmt.Errorf("effective_from is required")
	}
	return a.withinTenant(ctx, func(ctx context.Context) error {
		bookings, err := a.bookingCommandValues(ctx, childID, &from, rows)
		if err != nil {
			return err
		}
		if err := a.bookings.ScheduleCareOfferingBookings(ctx, childID, careplan.Date(from), bookings); err != nil {
			return err
		}
		return a.refreshBookingCommandRows(ctx, childID, rows)
	})
}

func (a EnrollmentBookingFixture) bookingCommandValues(ctx context.Context, childID int64, switchDate *enrollment.Date, selections []*enrollment.RequestChildOffering) ([]careplan.CareOfferingBooking, error) {
	if childID <= 0 {
		return nil, fmt.Errorf("request_child_id is required")
	}
	child, err := a.ChildByID(ctx, childID)
	if err != nil {
		return nil, err
	}
	request, err := a.RequestByID(ctx, child.RequestID, false)
	if err != nil {
		return nil, err
	}
	phase, err := a.Phase(ctx, request.PhaseID)
	if err != nil {
		return nil, err
	}
	end, err := time.Parse(time.DateOnly, string(phase.ServiceEndDate))
	if err != nil {
		return nil, err
	}
	exclusive := enrollment.Date(end.AddDate(0, 0, 1).Format(time.DateOnly))
	values := make([]careplan.CareOfferingBooking, 0, len(selections))
	for _, selection := range selections {
		if selection == nil {
			return nil, fmt.Errorf("request child offering row cannot be nil")
		}
		if selection.TenantID != 0 && selection.TenantID != child.TenantID {
			return nil, fmt.Errorf("request child offering tenant mismatch")
		}
		from, until := selection.ValidFrom, selection.ValidUntil
		if from == nil {
			from = &phase.ServiceStartDate
		}
		if switchDate != nil {
			from = switchDate
		}
		if until == nil || (switchDate != nil && exclusive.Before(*until)) {
			until = &exclusive
		}
		manual := selection.ManualSelectedDays
		if len(manual) == 0 && len(selection.AutomaticSelectedDays) == 0 {
			manual = selection.SelectedDays
		}
		values = append(values, careplan.CareOfferingBooking{RequestChildID: childID, CareOfferingID: selection.CareOfferingID,
			ManualSelectedDays: manual, AutomaticSelectedDays: selection.AutomaticSelectedDays, ValidFrom: carePlanBookingDate(from), ValidUntil: carePlanBookingDate(until)})
		selection.RequestChildID, selection.ValidFrom, selection.ValidUntil = childID, from, until
	}
	return values, nil
}

func (a EnrollmentBookingFixture) refreshBookingCommandRows(ctx context.Context, childID int64, inputs []*enrollment.RequestChildOffering) error {
	if len(inputs) == 0 {
		return nil
	}
	rows, err := a.RequestChildOfferingHistory(ctx, childID)
	if err != nil {
		return err
	}
	for _, input := range inputs {
		found := false
		for _, row := range rows {
			if row.CareOfferingID == input.CareOfferingID && equalOfferingDate(row.ValidFrom, input.ValidFrom) && equalOfferingDate(row.ValidUntil, input.ValidUntil) {
				*input = *row
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("recorded care booking was not returned by its owner")
		}
	}
	return nil
}

func equalOfferingDate(a, b *enrollment.Date) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
