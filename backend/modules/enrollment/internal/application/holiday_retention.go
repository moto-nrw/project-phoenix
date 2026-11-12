package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Retention of the holiday registrations (#6691, ADR 0118 addendum): a
// registration keeps the days its privacy text names, from the setting of
// the Träger at the submission. The health information goes
// HolidayHealthRetentionDays after the period whatever the registration
// keeps.

// errHolidayRetentionRequired reports a registration built without the
// retention of the Träger.
var errHolidayRetentionRequired = errors.New("enrollment: the holiday registration needs the Träger's retention")

// retentionDays reads the retention of the Träger and refuses a value
// outside its bounds: the text would promise what the store refuses.
func (h *HolidayRegistrations) retentionDays(ctx context.Context, organizationID int64) (int, error) {
	days, err := h.deps.Retention.HolidayRegistrationRetentionDays(ctx, organizationID)
	if err != nil {
		return 0, fmt.Errorf("enrollment: read holiday registration retention: %w", err)
	}
	if days < enrollment.HolidayRetentionDaysMin || days > enrollment.HolidayRetentionDaysMax {
		return 0, fmt.Errorf("enrollment: holiday registration retention of %d days is out of bounds", days)
	}
	return days, nil
}

// checkAgreedRetention refuses a registration whose privacy text names
// another retention than the Träger keeps now: the Träger changed it while
// the parent filled in the form, so the parent is told the form changed
// instead of agreeing to a text nobody showed.
func (h *HolidayRegistrations) checkAgreedRetention(ctx context.Context, organizationID int64, input enrollment.HolidayRegistrationInput) error {
	days, err := h.retentionDays(ctx, organizationID)
	if err != nil {
		return err
	}
	if input.ConsentVersion == enrollment.HolidayConsentVersionCurrent && input.RetentionDays != days {
		return enrollment.InvalidInput(enrollment.CodeHolidayRegistrationFormChanged, "retention_days", enrollment.ErrHolidayRegistrationFormChanged)
	}
	return nil
}

// holidayDeletionDays are the days the retention deletes a registration of
// a period and its health information on: the day after the last kept day.
// Both are nil for a period without weeks, which never ends.
func holidayDeletionDays(period enrollment.HolidayPeriod, retentionDays int) (registration, health *calendar.Date) {
	if len(period.Weeks) == 0 {
		return nil, nil
	}
	last := period.Weeks[0].EndsOn
	for _, week := range period.Weeks[1:] {
		if week.EndsOn.After(last) {
			last = week.EndsOn
		}
	}
	registrationDay, healthDay := last.AddDays(retentionDays+1), last.AddDays(enrollment.HolidayHealthRetentionDays+1)
	return &registrationDay, &healthDay
}
