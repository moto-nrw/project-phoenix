package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// addMailProblems names the receipt and acceptance mails of the registrations
// that did not reach the parent who registered (#6737), in one read for all of
// them. Without the mail states the review names none.
func (h *HolidayRegistrationReview) addMailProblems(ctx context.Context, organizationID int64, registrations []*enrollment.HolidayRegistrationSummary) error {
	if h.mailStates == nil || len(registrations) == 0 {
		return nil
	}
	keys := make([]string, 0, 2*len(registrations))
	for _, registration := range registrations {
		keys = append(keys, holidayReceiptMailKey(registration.ID), holidayAcceptanceMailKey(registration.ID))
	}
	problems, err := h.mailStates.HolidayMailProblems(ctx, organizationID, keys)
	if err != nil {
		return fmt.Errorf("enrollment: read the mails of the holiday registrations: %w", err)
	}
	for _, registration := range registrations {
		mails := []struct{ kind, key string }{
			{kind: enrollment.MailKindHolidayRegistrationReceived, key: holidayReceiptMailKey(registration.ID)},
			{kind: enrollment.MailKindHolidayRegistrationAccepted, key: holidayAcceptanceMailKey(registration.ID)},
		}
		for _, mail := range mails {
			if state, ok := problems[mail.key]; ok {
				registration.MailProblems = append(registration.MailProblems, enrollment.HolidayRegistrationMailProblem{Kind: mail.kind, State: state})
			}
		}
	}
	return nil
}
