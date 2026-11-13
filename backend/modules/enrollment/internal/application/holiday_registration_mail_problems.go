package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

var _ enrollment.HolidayRegistrationMailReview = (*HolidayRegistrationReview)(nil)

// HolidayRegistrationMailProblems names the registrations of the period whose
// receipt or acceptance mail did not reach the parents, in one read of the IDs
// and one of the mails. It records no read: it names no personal data.
func (h *HolidayRegistrationReview) HolidayRegistrationMailProblems(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) ([]enrollment.HolidayRegistrationMails, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return nil, err
	}
	if _, err := h.period(ctx, reviewer.OrganizationID, periodID, false); err != nil {
		return nil, err
	}
	ids, err := h.store.ReadHolidayRegistrationIDs(ctx, reviewer.OrganizationID, periodID)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read holiday registration IDs: %w", err)
	}
	registrations := make([]enrollment.HolidayRegistrationSummary, len(ids))
	summaries := make([]*enrollment.HolidayRegistrationSummary, len(ids))
	for i, id := range ids {
		registrations[i].ID = id
		summaries[i] = &registrations[i]
	}
	if err := h.addMailProblems(ctx, reviewer.OrganizationID, summaries); err != nil {
		return nil, err
	}
	mails := []enrollment.HolidayRegistrationMails{}
	for _, registration := range registrations {
		if len(registration.MailProblems) > 0 {
			mails = append(mails, enrollment.HolidayRegistrationMails{RegistrationID: registration.ID, MailProblems: registration.MailProblems})
		}
	}
	return mails, nil
}

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
