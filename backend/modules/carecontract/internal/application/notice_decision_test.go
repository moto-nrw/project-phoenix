package application

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The decided end of an extraordinary termination (#4682, decision of
// 07.11.2026) on the real store: the Träger-Büro sets the last day, or it
// does not accept the reason and the declaration holds as an ordinary
// termination to the next notice date from its receipt (§ 140 BGB). Either
// way the parents get one notice in Textform.

// decisionCount counts the trail entries of the decided end.
func (f noticeFixture) decisionCount(t *testing.T) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ?
		AND action IN ('set_end', 'reject_reason')`, f.contract).Scan(context.Background(), &count))
	return count
}

// endMails are the notices of a decided end the post received.
func (f noticeFixture) endMails() []carecontract.NoticeMail {
	f.post.mu.Lock()
	defer f.post.mu.Unlock()
	var mails []carecontract.NoticeMail
	for _, mail := range f.post.mails {
		if mail.Decision != "" {
			mails = append(mails, mail)
		}
	}
	return mails
}

// Invariant: a reason the Träger-Büro does not accept turns the declaration
// into an ordinary termination to the next notice date counted from its
// receipt: received on 10.11.2026 with one month to the month end, it ends
// on 31.12.2026. VersionsInForce answers up to that day and nothing after,
// so the monthly contribution (#4684) stops there. The parents get the new
// end in Textform once; a second refusal or an end set afterwards finds no
// open end. Red without the cut of the contract (VersionsInForce answers
// January) and without the guard in openEnd and the store (second mail).
func TestUnacceptedReasonHoldsAsOrdinaryTermination(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Grund abgelehnt", noticeToday)
	_, err := f.parentDeclare(extraordinary)
	require.NoError(t, err)
	end := calendar.NewDate(2026, 12, 31)

	options, err := f.notices.NoticeOptions(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	require.NoError(t, err)
	assert.Equal(t, &end, options.ReinterpretedEndsOn, "the office sees the end before it refuses the reason")

	_, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, calendar.NewDate(2026, 11, 30))
	var invalid *carecontract.ValidationError
	require.ErrorAs(t, err, &invalid, "an end other than the one shown is refused")
	assert.Equal(t, "changed", invalid.Fields["expected_ends_on"])
	assert.Empty(t, f.endMails())

	options, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, end)
	require.NoError(t, err)
	require.NotNil(t, options.Notice)
	assert.Equal(t, carecontract.StatusTerminated, options.Status)
	assert.Equal(t, &end, options.Notice.EndsOn)
	assert.Equal(t, carecontract.EndDecisionReinterpreted, options.Notice.EndDecision)
	assert.NotNil(t, options.Notice.EndDecidedAt)
	assert.Nil(t, options.ReinterpretedEndsOn, "the end is decided")
	want := "Grund nicht anerkannt für Lina Muster, eingegangen am 10.11.2026 um 11:00 Uhr. Ganztag im Quartier e.V.\n\nLetzter Betreuungstag: 31.12.2026"
	assert.Equal(t, want, options.Notice.EndNoticeText, "the template without {Vertragsende} gets the last day appended")
	assert.Equal(t, &end, f.state(t).endsOn)

	mails := f.endMails()
	require.Len(t, mails, 1, "one notice to the parents")
	assert.Equal(t, carecontract.EndDecisionReinterpreted, mails[0].Decision)
	assert.Equal(t, "erika@example.org", mails[0].Recipient, "to the address the confirmation went to")
	assert.Equal(t, want, mails[0].ConfirmationText)
	assert.Regexp(t, `^care-contract-notice-end:\d+$`, mails[0].IdempotencyKey, "keyed to the declaration")
	payload, err := json.Marshal(mails[0])
	require.NoError(t, err)
	rendered, err := RenderNoticeMail(t.Context(), payload)
	require.NoError(t, err)
	assert.Equal(t, "Ihre Kündigung gilt als ordentliche Kündigung", rendered.Subject)
	assert.Equal(t, rendered.Subject, rendered.Heading)

	periods := f.versionsInForce(t, calendar.NewDate(2026, 12, 1), calendar.NewDate(2027, 1, 31))
	require.Len(t, periods, 1)
	assert.Equal(t, end, periods[0].Until, "the contract holds up to the new end")
	assert.Empty(t, f.versionsInForce(t, calendar.NewDate(2027, 1, 1), calendar.NewDate(2027, 1, 31)), "and promises nothing after it")

	_, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "the reason is refused once")
	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, calendar.NewDate(2027, 1, 31))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "a refused reason leaves no end to set")
	assert.Len(t, f.endMails(), 1, "a repetition sends no second notice")
	assert.Equal(t, 1, f.decisionCount(t))
	assert.Equal(t, &end, f.state(t).endsOn)
}

// Invariant: the reason is refused only on an open extraordinary
// termination. An ordinary termination has its end, an active contract no
// declaration.
func TestReasonIsRefusedOnlyOnAnOpenExtraordinaryTermination(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Grund nur außerordentlich", noticeToday)
	_, err := f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an active contract has no termination")
	_, err = f.parentDeclare(terminate)
	require.NoError(t, err)
	_, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an ordinary termination has no reason to refuse")
	assert.Empty(t, f.endMails())
	assert.Zero(t, f.decisionCount(t))
}

// Invariant: an end the Träger-Büro sets reaches the parents in Textform
// with the last day, once; the repetition finds no open end and sends
// nothing.
func TestSetEndTellsTheParentsTheLastDayOnce(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Ende Mitteilung", noticeToday)
	_, err := f.parentDeclare(extraordinary)
	require.NoError(t, err)
	end := calendar.NewDate(2026, 11, 30)
	options, err := f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
	require.NoError(t, err)
	assert.Equal(t, carecontract.EndDecisionSet, options.Notice.EndDecision)
	assert.Equal(t, "Ende für Lina Muster: letzter Tag 30.11.2026. Ganztag im Quartier e.V.\n\nEingang Ihrer Erklärung: 10.11.2026 um 11:00 Uhr",
		options.Notice.EndNoticeText)
	mails := f.endMails()
	require.Len(t, mails, 1)
	assert.Equal(t, carecontract.EndDecisionSet, mails[0].Decision)
	assert.Equal(t, options.Notice.EndNoticeText, mails[0].ConfirmationText)
	payload, err := json.Marshal(mails[0])
	require.NoError(t, err)
	rendered, err := RenderNoticeMail(t.Context(), payload)
	require.NoError(t, err)
	assert.Equal(t, "Ende Ihres Betreuungsvertrags", rendered.Subject)

	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen)
	_, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "a set end leaves no reason to refuse")
	assert.Len(t, f.endMails(), 1, "a repetition sends no second notice")
	assert.Equal(t, 1, f.decisionCount(t))
}

// Invariant: refusing the reason and setting the end exclude each other.
// Two real Träger transactions decide at once: the second waits for the
// contract's row lock, then finds the end decided. One decision, one trail
// entry, one notice. Red without the row lock in decideEnd together with the
// store's guard on the open end (both win, two notices); each guard alone is
// pinned by TestNoticeEndIsSetOnceInTheStore and TestOpenEndRefusesAnEndAlreadySet.
func TestRefusalAgainstSetEndLetsOneWin(t *testing.T) {
	t.Parallel()
	for name, order := range map[string][2]endDecision{
		"refusal first": {{decision: carecontract.EndDecisionReinterpreted}, {decision: carecontract.EndDecisionSet, endsOn: calendar.NewDate(2026, 11, 30)}},
		"set end first": {{decision: carecontract.EndDecisionSet, endsOn: calendar.NewDate(2026, 11, 30)}, {decision: carecontract.EndDecisionReinterpreted}},
		"twice refused": {{decision: carecontract.EndDecisionReinterpreted}, {decision: carecontract.EndDecisionReinterpreted}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newNoticeFixture(t, fmt.Sprintf("Träger Rennen Entscheidung %s", name), noticeToday)
			_, err := f.parentDeclare(extraordinary)
			require.NoError(t, err)
			decide := func(decision endDecision) func(context.Context) error {
				return func(txCtx context.Context) error {
					_, err := f.notices.decideEnd(txCtx, f.reviewer, f.contract, decision)
					return err
				}
			}
			won, lost := f.race(t, decide(order[0]), decide(order[1]))

			require.NoError(t, won)
			require.ErrorIs(t, lost, carecontract.ErrNoticeEndNotOpen)
			want := calendar.NewDate(2026, 12, 31)
			if order[0].decision == carecontract.EndDecisionSet {
				want = order[0].endsOn
			}
			assert.Equal(t, &want, f.state(t).endsOn, "the first decision stands")
			assert.Equal(t, 1, f.decisionCount(t))
			mails := f.endMails()
			require.Len(t, mails, 1, "one notice")
			assert.Equal(t, order[0].decision, mails[0].Decision)
		})
	}
}

// A late declaration cannot extend an already expired fixed term. Only a
// manually chosen end is constrained by the day the declaration arrived.
func TestRefusedReasonPreservesAnExpiredFixedTerm(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger abgelaufene Befristung", noticeToday)
	end := calendar.NewDate(2026, 11, 9)
	_, err := f.db.NewUpdate().Table("care_contract.contract_versions").Set("ends_on = ?", end).
		Where("contract_id = ?", f.contract).Exec(f.ctx)
	require.NoError(t, err)
	_, err = f.parentDeclare(extraordinary)
	require.NoError(t, err)
	options, err := f.notices.NoticeOptions(f.ctx, f.reviewer, f.contract, calendar.Date(""))
	require.NoError(t, err)
	require.Equal(t, &end, options.ReinterpretedEndsOn)
	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
	var invalid *carecontract.ValidationError
	require.ErrorAs(t, err, &invalid, "a manually chosen end still cannot precede receipt")
	options, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, end)
	require.NoError(t, err)
	assert.Equal(t, &end, options.Notice.EndsOn)
	assert.Equal(t, &end, f.state(t).endsOn)
	mails := f.endMails()
	require.Len(t, mails, 1)
	assert.Contains(t, mails[0].ConfirmationText, "09.11.2026")
}

type repeatedReasonTemplate struct{ noticeTemplates }

func (templates repeatedReasonTemplate) NoticeTemplate(ctx context.Context, org int64, kind string) (string, error) {
	if kind == carecontract.NoticeTemplateReasonRejected || kind == carecontract.NoticeTemplateEndSet {
		return strings.Repeat("{Grund}\n\n", 4), nil
	}
	return templates.noticeTemplates.NoticeTemplate(ctx, org, kind)
}

// The saved notice and the mail both retain every expanded placeholder,
// even when an allowed template produces more than 8,000 characters.
func TestEndDecisionStoresTheFullExpandedLetter(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, decision := range []string{carecontract.EndDecisionSet, carecontract.EndDecisionReinterpreted} {
		t.Run(decision, func(t *testing.T) {
			f := newNoticeFixture(t, "Träger vollständiges Endschreiben", noticeToday)
			f.notices.templates = repeatedReasonTemplate{}
			input := extraordinary
			input.Reason = strings.Repeat("ä", 2000)
			_, err := f.parentDeclare(input)
			require.NoError(t, err)
			end := calendar.NewDate(2026, 12, 31)
			var options carecontract.NoticeOptions
			if decision == carecontract.EndDecisionSet {
				options, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
			} else {
				options, err = f.notices.RejectNoticeReason(f.ctx, f.reviewer, f.contract, end)
			}
			require.NoError(t, err)
			text := options.Notice.EndNoticeText
			assert.Equal(t, 4, strings.Count(text, input.Reason))
			assert.Contains(t, text, "Letzter Betreuungstag: 31.12.2026")
			mails := f.endMails()
			require.Len(t, mails, 1)
			assert.Equal(t, text, mails[0].ConfirmationText)
		})
	}
}
