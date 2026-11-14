package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

type mailStatesDouble struct {
	mails []enrollment.HolidayMailState
	err   error
	calls int
	ids   []int64
	keys  []string
	org   int64
}

func (d *mailStatesDouble) HolidayMails(_ context.Context, organizationID int64, registrationIDs []int64, keys []string) ([]enrollment.HolidayMailState, error) {
	d.calls++
	d.org, d.ids, d.keys = organizationID, registrationIDs, keys
	return d.mails, d.err
}

// mailKeysStore answers the registrations of a period with the keys of their
// closing day notices.
type mailKeysStore struct {
	HolidayReviewStore
	ids         []int64
	closureKeys map[int64][]string
	calls       int
}

func (s *mailKeysStore) ReadHolidayRegistrationMailKeys(_ context.Context, _, _, registrationID int64) ([]int64, map[int64][]string, error) {
	s.calls++
	if registrationID != 0 {
		return []int64{registrationID}, map[int64][]string{registrationID: s.closureKeys[registrationID]}, nil
	}
	return s.ids, s.closureKeys, nil
}

// #6737, #6976: the Träger sees which of the five mails of a registration
// did not reach the parents, for all registrations of the list in one read
// of the registrations and one of the mails. A family's closing day notice
// relates to one registration and counts for every registration it names.
func TestRegistrationMailsAssignsEveryMailInOneRead(t *testing.T) {
	t.Parallel()
	store := &mailKeysStore{ids: []int64{1, 2, 3}, closureKeys: map[int64][]string{1: {"holiday-closure-op:a:f"}, 2: {"holiday-closure-op:a:f"}}}
	states := &mailStatesDouble{mails: []enrollment.HolidayMailState{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, RegistrationID: 1, IdempotencyKey: "holiday-registration-received:1", Outcome: "send_failed", Problem: enrollment.HolidayMailFailed},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, RegistrationID: 1, IdempotencyKey: "holiday-registration-accepted:1", Outcome: "blocked", Problem: enrollment.HolidayMailBlocked},
		{Kind: enrollment.MailKindHolidayPlaceGranted, RegistrationID: 3, IdempotencyKey: "holiday-place-granted:3:9", Outcome: "unknown", Problem: enrollment.HolidayMailUnknown},
		{Kind: enrollment.MailKindHolidayParentAccountOffer, RegistrationID: 3, IdempotencyKey: "holiday-parent-account-offer:3", Outcome: "delivered"},
		{Kind: enrollment.MailKindHolidayClosureNotice, RegistrationID: 1, IdempotencyKey: "holiday-closure-op:a:f", Outcome: "undeliverable", Problem: enrollment.HolidayMailFailed},
		{Kind: enrollment.MailKindHolidayRegistrationReceived, RegistrationID: 99, IdempotencyKey: "holiday-registration-received:99", Outcome: "send_failed", Problem: enrollment.HolidayMailFailed},
	}}
	review := &HolidayRegistrationReview{store: store, mailStates: states}

	ids, byRegistration, err := review.registrationMails(context.Background(), 7, 5, 0)

	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, ids)
	assert.Equal(t, 1, store.calls, "one read of the registrations")
	assert.Equal(t, 1, states.calls, "one read of the mails for the whole list")
	assert.Equal(t, int64(7), states.org)
	assert.Equal(t, []int64{1, 2, 3}, states.ids)
	assert.Equal(t, []string{"holiday-closure-op:a:f"}, states.keys, "each closing day notice once")
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, State: enrollment.HolidayMailFailed},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, State: enrollment.HolidayMailBlocked},
		{Kind: enrollment.MailKindHolidayClosureNotice, State: enrollment.HolidayMailFailed},
	}, holidayMailProblems(byRegistration[1]))
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayClosureNotice, State: enrollment.HolidayMailFailed},
	}, holidayMailProblems(byRegistration[2]), "the family's notice names the second registration too")
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayPlaceGranted, State: enrollment.HolidayMailUnknown},
	}, holidayMailProblems(byRegistration[3]), "a delivered mail is no problem")
	assert.Len(t, byRegistration[3], 2, "every mail counts, also the delivered one")
	assert.NotContains(t, byRegistration, int64(99), "a mail of another registration stays out")
}

// #6976: the detail names every mail with its outcome, the delivered too.
func TestAddMailsNamesEveryMailOfTheRegistration(t *testing.T) {
	t.Parallel()
	states := &mailStatesDouble{mails: []enrollment.HolidayMailState{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, RegistrationID: 4, Outcome: "delivered"},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, RegistrationID: 4, Outcome: "blocked", Problem: enrollment.HolidayMailBlocked},
	}}
	review := &HolidayRegistrationReview{store: &mailKeysStore{}, mailStates: states}
	detail := enrollment.HolidayRegistrationDetail{HolidayRegistrationSummary: enrollment.HolidayRegistrationSummary{ID: 4}}

	require.NoError(t, review.addMails(context.Background(), 7, 5, &detail))

	assert.Equal(t, []enrollment.RequestMailOutcome{
		{Kind: enrollment.MailKindHolidayRegistrationReceived, Outcome: "delivered"},
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, Outcome: "blocked"},
	}, detail.Mails)
	assert.Equal(t, []enrollment.HolidayRegistrationMailProblem{
		{Kind: enrollment.MailKindHolidayRegistrationAccepted, State: enrollment.HolidayMailBlocked},
	}, detail.MailProblems)
	assert.Equal(t, []int64{4}, states.ids)
}

func TestRegistrationMailsReportsAFailedRead(t *testing.T) {
	t.Parallel()
	review := &HolidayRegistrationReview{store: &mailKeysStore{ids: []int64{1}}, mailStates: &mailStatesDouble{err: errors.New("database gone")}}

	_, _, err := review.registrationMails(context.Background(), 7, 5, 0)

	require.ErrorContains(t, err, "database gone")
}

func TestRegistrationMailsWithoutMailStatesReadsNothing(t *testing.T) {
	t.Parallel()
	store := &mailKeysStore{ids: []int64{1}}
	detail := enrollment.HolidayRegistrationDetail{HolidayRegistrationSummary: enrollment.HolidayRegistrationSummary{ID: 1}}

	require.NoError(t, (&HolidayRegistrationReview{store: store}).addMails(context.Background(), 7, 5, &detail))
	assert.Empty(t, detail.MailProblems)
	assert.Empty(t, detail.Mails)
	assert.Zero(t, store.calls, "without the mail states nothing is read")
}
