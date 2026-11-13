package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

var _ enrollment.HolidayRegistrationMailReview = (*HolidayRegistrationReview)(nil)

// HolidayRegistrationMailProblems names the registrations of the period with a
// mail that did not reach the parents, in one read of the registrations with
// the keys of their closing day notices and one of the mails. It records no
// read: it names no personal data.
func (h *HolidayRegistrationReview) HolidayRegistrationMailProblems(ctx context.Context, reviewer enrollment.HolidayReviewer, periodID int64) ([]enrollment.HolidayRegistrationMails, error) {
	if err := holidayReviewKey(reviewer, periodID, 1); err != nil {
		return nil, err
	}
	if _, err := h.period(ctx, reviewer.OrganizationID, periodID, false); err != nil {
		return nil, err
	}
	ids, byRegistration, err := h.registrationMails(ctx, reviewer.OrganizationID, periodID, 0)
	if err != nil {
		return nil, err
	}
	mails := []enrollment.HolidayRegistrationMails{}
	for _, id := range ids {
		if problems := holidayMailProblems(byRegistration[id]); len(problems) > 0 {
			mails = append(mails, enrollment.HolidayRegistrationMails{RegistrationID: id, MailProblems: problems})
		}
	}
	return mails, nil
}

// addMails names every mail of the registration with its outcome and those
// that did not reach the parents (#6737, #6976). Without the mail states the
// detail names none.
func (h *HolidayRegistrationReview) addMails(ctx context.Context, organizationID, periodID int64, detail *enrollment.HolidayRegistrationDetail) error {
	_, byRegistration, err := h.registrationMails(ctx, organizationID, periodID, detail.ID)
	if err != nil {
		return err
	}
	mails := byRegistration[detail.ID]
	detail.MailProblems = holidayMailProblems(mails)
	for _, mail := range mails {
		detail.Mails = append(detail.Mails, enrollment.RequestMailOutcome{Kind: mail.Kind, Outcome: mail.Outcome})
	}
	return nil
}

// registrationMails reads the registrations of the period, or the one
// registration, and every mail to their parents, by registration: one read
// of the registrations and one of the mails, flat in their number. A
// family's closing day notice relates to one of its registrations; its key
// assigns it to every registration it names. Without the mail states it
// reads nothing.
func (h *HolidayRegistrationReview) registrationMails(ctx context.Context, organizationID, periodID, registrationID int64) ([]int64, map[int64][]enrollment.HolidayMailState, error) {
	if h.mailStates == nil {
		return nil, nil, nil
	}
	ids, closureKeys, err := h.store.ReadHolidayRegistrationMailKeys(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment: read holiday registration IDs: %w", err)
	}
	if len(ids) == 0 {
		return ids, nil, nil
	}
	keys, byKey := closureMailKeys(ids, closureKeys)
	states, err := h.mailStates.HolidayMails(ctx, organizationID, ids, keys)
	if err != nil {
		return nil, nil, fmt.Errorf("enrollment: read the mails of the holiday registrations: %w", err)
	}
	return ids, mailsByRegistration(ids, byKey, states), nil
}

// closureMailKeys lists each key of a closing day notice once, with the
// registrations it names.
func closureMailKeys(ids []int64, closureKeys map[int64][]string) ([]string, map[string][]int64) {
	byKey := map[string][]int64{}
	keys := []string{}
	for _, id := range ids {
		for _, key := range closureKeys[id] {
			if _, seen := byKey[key]; !seen {
				keys = append(keys, key)
			}
			byKey[key] = append(byKey[key], id)
		}
	}
	return keys, byKey
}

// mailsByRegistration assigns a closing day notice to every registration its
// key names and any other mail to the registration it relates to; a mail of
// a registration outside ids stays out.
func mailsByRegistration(ids []int64, byKey map[string][]int64, states []enrollment.HolidayMailState) map[int64][]enrollment.HolidayMailState {
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	byRegistration := map[int64][]enrollment.HolidayMailState{}
	for _, state := range states {
		registrations, named := byKey[state.IdempotencyKey]
		if !named && wanted[state.RegistrationID] {
			registrations = []int64{state.RegistrationID}
		}
		for _, id := range registrations {
			byRegistration[id] = append(byRegistration[id], state)
		}
	}
	return byRegistration
}

// holidayMailProblems names the mails that did not reach the parents.
func holidayMailProblems(mails []enrollment.HolidayMailState) []enrollment.HolidayRegistrationMailProblem {
	var problems []enrollment.HolidayRegistrationMailProblem
	for _, mail := range mails {
		if mail.Problem != "" {
			problems = append(problems, enrollment.HolidayRegistrationMailProblem{Kind: mail.Kind, State: mail.Problem})
		}
	}
	return problems
}
