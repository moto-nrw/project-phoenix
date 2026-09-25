package enrollment

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	owner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CareBookingInput is the effective-care data this workflow sends to Care
// Plan; the Enrollment owner defines it for every enrollment flow (#3564).
// Submitted days and notes are written separately through Enrollment.
type CareBookingInput = owner.CareBookingInput

// CareBookingCommands records the effective bookings of a new request child.
type CareBookingCommands = owner.CareBookingCommands

type CareBookingChanges interface {
	ReplaceCareBookings(context.Context, int64, []CareBookingInput) error
	ScheduleCareBookings(context.Context, int64, timezone.Date, []CareBookingInput) error
}

func changeCareBookings(ctx context.Context, commands CareBookingChanges, childID int64, phase *owner.Phase, effectiveFrom *timezone.Date, selections []*RequestChildOffering) error {
	if commands == nil {
		return errors.New("offering adjustment requires Care Plan booking commands")
	}
	start, until := timezone.Date(phase.ServiceStartDate), timezone.Date(phase.ServiceEndDate).AddDays(1)
	bookings := make([]CareBookingInput, 0, len(selections))
	for _, selection := range selections {
		if selection == nil {
			return errors.New("care booking cannot be nil")
		}
		from, end := start, until
		if selection.ValidFrom != nil {
			from = *selection.ValidFrom
		}
		if effectiveFrom != nil {
			from = *effectiveFrom
		}
		if selection.ValidUntil != nil && (effectiveFrom == nil || selection.ValidUntil.Before(until)) {
			end = *selection.ValidUntil
		}
		manual := selection.ManualSelectedDays
		if len(manual) == 0 && len(selection.AutomaticSelectedDays) == 0 {
			manual = selection.SelectedDays
		}
		bookings = append(bookings, CareBookingInput{CareOfferingID: selection.CareOfferingID,
			ManualSelectedDays: manual, AutomaticSelectedDays: selection.AutomaticSelectedDays,
			ValidFrom: &from, ValidUntil: &end})
		selection.ValidFrom, selection.ValidUntil = &from, &end
	}
	if effectiveFrom != nil {
		return commands.ScheduleCareBookings(ctx, childID, *effectiveFrom, bookings)
	}
	return commands.ReplaceCareBookings(ctx, childID, bookings)
}

func (s *requestService) recordOfferingSubmission(ctx context.Context, childID int64, selections []materializedOfferingSelection, from, through owner.Date) error {
	if len(selections) == 0 {
		return nil
	}
	if s.Bookings == nil {
		return errors.New("request submission requires Care Plan booking commands")
	}
	start, until := timezone.Date(from), timezone.Date(through).AddDays(1)
	choices := make([]owner.SubmittedOfferingChoice, 0, len(selections))
	bookings := make([]CareBookingInput, 0, len(selections))
	for _, selection := range selections {
		manual := selection.ManualSelectedDays
		if len(manual) == 0 && len(selection.AutomaticSelectedDays) == 0 {
			manual = selection.SelectedDays
		}
		choices = append(choices, owner.SubmittedOfferingChoice{CareOfferingID: selection.OfferingID, SelectedDays: manual})
		bookings = append(bookings, CareBookingInput{
			CareOfferingID: selection.OfferingID, ManualSelectedDays: manual, AutomaticSelectedDays: selection.AutomaticSelectedDays,
			ValidFrom: &start, ValidUntil: &until,
		})
	}
	if err := s.Children.RecordSubmittedOfferingChoices(ctx, childID, choices); err != nil {
		return fmt.Errorf("record submitted offering choices: %w", err)
	}
	if err := s.Bookings.RecordCareBookings(ctx, childID, bookings); err != nil {
		return fmt.Errorf("record effective care bookings: %w", err)
	}
	return nil
}
