package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The acceptance mail of a confirmed holiday registration (#6672).

var errHolidayAcceptanceMailRequired = errors.New("enrollment: confirming a holiday registration needs the mail outbox and the Träger directory")

// holidayAcceptanceMailKey is the idempotency key of the acceptance mail:
// one per registration, however often the confirmation is retried.
func holidayAcceptanceMailKey(registrationID int64) string {
	return fmt.Sprintf("holiday-registration-accepted:%d", registrationID)
}

// enqueueAcceptance puts the acceptance mail of the confirmed registration
// on the outbox in the transaction of the confirmation: to the parent who
// registered, with each child's weeks and time models and the text of the
// period.
func (h *HolidayRegistrationReview) enqueueAcceptance(ctx context.Context, organizationID int64, period enrollment.HolidayPeriod, registrationID int64) error {
	if h.placement.mail == nil || h.placement.traeger == nil {
		return errHolidayAcceptanceMailRequired
	}
	read, _, err := h.store.ReadHolidayReview(ctx, organizationID, period.ID, registrationID)
	if err != nil {
		return fmt.Errorf("enrollment: read holiday registration: %w", err)
	}
	if len(read) == 0 {
		return enrollment.ErrHolidayRegistrationEntryNotFound
	}
	registration := read[0]
	traeger, err := h.placement.traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return fmt.Errorf("enrollment: read the Träger of the holiday acceptance mail: %w", err)
	}
	payload, err := json.Marshal(enrollment.HolidayRegistrationAcceptedMail{
		Recipient: registration.GuardianEmail, GuardianName: registration.GuardianFirstName + " " + registration.GuardianLastName,
		TraegerName: traeger.Name, PeriodName: period.Name, Binding: period.Binding,
		Children: holidayAcceptanceChildren(registration, holidayWeekLines(traeger, period)), TraegerText: period.AcceptanceMailText,
		OrganizationID: organizationID,
	})
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday acceptance mail: %w", err)
	}
	if err := h.placement.mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: organizationID, Kind: enrollment.MailKindHolidayRegistrationAccepted,
		Recipient: registration.GuardianEmail, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: registrationID,
		IdempotencyKey: holidayAcceptanceMailKey(registrationID),
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday acceptance mail: %w", err)
	}
	return nil
}

// holidayAcceptanceChildren lists each child with its weeks: those with a
// place apart from those on the waiting list and those the Büro still
// decides on.
func holidayAcceptanceChildren(registration enrollment.HolidayRegistrationDetail, line func(weekID, timeModelID int64, days []calendar.Date) string) []enrollment.HolidayRegistrationMailChild {
	children := make([]enrollment.HolidayRegistrationMailChild, 0, len(registration.Children))
	for _, child := range registration.Children {
		mailChild := enrollment.HolidayRegistrationMailChild{
			Name: child.FirstName + " " + child.LastName, Guest: child.HomeSchoolID == nil, Weeks: []string{},
		}
		for _, booking := range child.Bookings {
			week := line(booking.WeekID, booking.TimeModelID, booking.Days)
			switch booking.Placement {
			case enrollment.HolidayPlacementWaitlisted:
				mailChild.WaitlistWeeks = append(mailChild.WaitlistWeeks, week)
			case enrollment.HolidayPlacementPending:
				mailChild.PendingWeeks = append(mailChild.PendingWeeks, week)
			default:
				mailChild.Weeks = append(mailChild.Weeks, week)
			}
		}
		children = append(children, mailChild)
	}
	return children
}
