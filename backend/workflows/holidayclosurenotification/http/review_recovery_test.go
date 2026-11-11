package http_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func TestMovedNoticeOnlyApprovesItsOwnRanges(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger genauer Umfang")
	day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
	_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-12", EndDate: "2030-07-12", Reason: "Verschoben",
	}})
	require.NoError(t, err)
	other := w.createClosingDay(t, "2030-07-09", "2030-07-09")
	for _, notice := range w.schoolNotices(t) {
		if notice.ClosingDayID != day.ID {
			continue
		}
		_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
		require.NoError(t, err)
	}
	require.Len(t, w.mails(t), 2)
	var payload enrollment.HolidayClosureMail
	require.NoError(t, json.Unmarshal(w.mails(t)[0].Payload, &payload))
	assert.Equal(t, []string{"Freitag, 12.07.2030"}, payload.Children[0].Weeks[0].ClosedDays)
	var outstanding bool
	for _, notice := range w.schoolNotices(t) {
		if notice.ClosingDayID == other.ID && notice.Status == "suggested" {
			outstanding = true
		}
	}
	assert.True(t, outstanding, "the independent Tuesday still requires its own approval")
}

func TestOneBatchKeepsApprovalForOverlappingNotices(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger überlappende Bestätigungen")
	w.createClosingDay(t, "2030-07-08", "2030-07-09")
	w.createClosingDay(t, "2030-07-09", "2030-07-10")
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	require.Len(t, pending.Notices, 2)
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.NoError(t, err)
	assert.Equal(t, []string{"prepared", "prepared"}, w.events(t), "one approved operation cannot invalidate another's unchanged bookings")
	require.Len(t, w.mails(t), 4)
}

func TestTransferredSchoolReplacesTheOldProposalOwner(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger vor Wechsel")
	next := newClosureWorld(t, "Träger nach Wechsel")
	day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
	before := w.schoolNotices(t)[0]
	_, err := w.db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, next.organization, w.school)
	require.NoError(t, err)
	bookHolidayWeek(t, w.db, next.organization, w.school, "neue.familie@example.org")
	_, err = w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-09", EndDate: "2030-07-09", Reason: "Neuer Träger",
	}})
	require.NoError(t, err)
	after := w.schoolNotices(t)
	require.Len(t, after, 1)
	assert.Equal(t, 1, after[0].Families, "only the current organization's family is counted")
	pending, err := w.notices.PendingHolidayClosures(w.ctx, next.organization)
	require.NoError(t, err)
	require.Len(t, pending.Notices, 1)
	_, err = w.notices.NotifyFromTraeger(w.ctx, w.organization, 0, before.ID, before.Revision, before.Token)
	require.Error(t, err, "the previous organization cannot approve the renewed proposal")
}

func TestPreparationHoldsSchoolMembershipUntilCommit(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger gesicherte Zugehörigkeit")
	other := newClosureWorld(t, "Träger paralleler Wechsel")
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
		schools, err := events.HolidayClosureCalendars(ctx, w.organization, []int64{w.school}, calendar.NewDate(2030, 7, 8), calendar.NewDate(2030, 7, 12))
		require.NoError(t, err)
		require.Contains(t, schools, w.school)
		err = w.db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '100ms'`); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, other.organization, w.school)
			return err
		})
		require.ErrorContains(t, err, "lock timeout", "a transfer must wait until mail preparation commits")
		return nil
	}))
}

func TestRetryAlsoRecoversLegacyBlockingMail(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Vorgänger nachholen", testpkg.SetupIsolatedTestDB(t))
	_, err := w.db.ExecContext(context.Background(), `INSERT INTO platform.email_outbox (organization_id, kind, idempotency_key,
  recipient, payload, status, dead_letter_at, last_error) VALUES (?, 'holiday_closure_notice', 'legacy-closure',
  '{"address":"familie.eins@example.org"}', '{}', 'dead_letter', NOW(), 'mailbox unavailable')`, w.organization)
	require.NoError(t, err)
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	notice := w.schoolNotices(t)[0]
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	failed := w.schoolNotices(t)[0]
	require.Equal(t, "failed", failed.Status, "the blocking predecessor must be visible and recoverable")
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, failed.ID, failed.Revision, failed.Token)
	require.NoError(t, err)
	var state string
	require.NoError(t, w.db.NewRaw(`SELECT status FROM platform.email_outbox WHERE organization_id = ? AND idempotency_key = 'legacy-closure'`, w.organization).Scan(context.Background(), &state))
	assert.Equal(t, "pending", state, "the existing predecessor is retried before its correction")
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status='dead_letter',last_error=? WHERE organization_id=? AND idempotency_key='legacy-closure'`, deliveryContract.DispatchOutcomeUnknown, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	unknown := w.schoolNotices(t)[0]
	assert.Equal(t, "unknown", unknown.Status, "an unclear predecessor must never offer a blind retry")
	assert.False(t, unknown.CanNotify)

}

func TestSuppressionCancelledNoticeCanBeRetried(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Sperre aufgehoben", testpkg.SetupIsolatedTestDB(t))
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	notice := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'cancelled', cancelled_at = NOW(), last_error = ? WHERE organization_id = ?`, deliveryContract.MailHeldByList, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	for _, mail := range w.mails(t) {
		assert.Equal(t, "pending", mail.Status)
	}
}

func TestOfficeConfirmsSchoolProposalsInBulk(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Bestätigungen gesammelt", testpkg.SetupIsolatedTestDB(t))
	for _, day := range []string{"2030-07-08", "2030-07-10", "2030-07-12"} {
		w.createClosingDay(t, day, day)
	}
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	counter := testpkg.CaptureQueries(t, w.db)
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.NoError(t, err)
	locks := counter.Matching(func(query string) bool { return strings.Contains(query, "for update of proposal") })
	writes := counter.Matching(func(query string) bool { return strings.Contains(query, "insert into calendar.holiday_closure_events") })
	assert.Len(t, locks, 1, "all proposals share their guarded read under the calendar lock")
	assert.Len(t, writes, 1, "the school writes its confirmed commands together")
}

func TestBulkConfirmationGuardsAndBudget(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Umfang der Bestätigung", testpkg.SetupIsolatedTestDB(t))
	for _, day := range []string{"2030-07-08", "2030-07-10", "2030-07-12"} {
		w.createClosingDay(t, day, day)
	}
	var confirmations []schoolcalendar.HolidayClosureConfirmation
	for _, notice := range w.schoolNotices(t) {
		confirmations = append(confirmations, schoolcalendar.HolidayClosureConfirmation{ProposalID: notice.ID, Revision: notice.Revision, ImpactToken: notice.Token})
	}
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	actor := schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: w.organization}
	stale := append([]schoolcalendar.HolidayClosureConfirmation(nil), confirmations...)
	stale[1].Revision--
	err := tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := events.ConfirmHolidayClosureNotices(ctx, stale, actor)
		return err
	})
	require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureNoticeChanged)
	assert.Empty(t, w.events(t), "a stale row must leave the entire approved school batch untouched")
	counter := testpkg.CaptureQueries(t, w.db)
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		counter.Reset()
		confirmed, err := events.ConfirmHolidayClosureNotices(ctx, confirmations, actor)
		require.NoError(t, err)
		require.Len(t, confirmed, 3)
		querybudgets.AssertQueryBudget(t, "modules.schoolcalendar.confirm_holiday_closure_notices", counter.Measurement(counter.Queries()))
		return nil
	}))
}

func TestBatchCountsOnlyPreparedFamilyMails(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger vollständig überdeckter Hinweis")
	w.createClosingDay(t, "2030-07-08", "2030-07-09")
	w.createClosingDay(t, "2030-07-09", "2030-07-09")
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	result, err := w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Notices)
	assert.Equal(t, 2, result.Families, "the second approval has no remaining mail")
	assert.Equal(t, []string{"prepared", "no_mail"}, w.events(t))
	assert.Len(t, w.mails(t), 2)
}

func TestUnprocessedMoveKeepsTheLastToldRange(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger zweimal verschoben vor Verarbeitung")
	day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
	first := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, first.ID, first.Revision, first.Token)
	require.NoError(t, err)
	move := func(date string) {
		_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: date, EndDate: date, Reason: "Verschoben"}})
		require.NoError(t, err)
	}
	move("2030-07-12")
	var pending schoolcalendar.HolidayClosureConfirmation
	for _, notice := range w.schoolNotices(t) {
		if notice.Status == "suggested" {
			pending = schoolcalendar.HolidayClosureConfirmation{ProposalID: notice.ID, Revision: notice.Revision, ImpactToken: notice.Token}
		}
	}
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := schoolCalendarCompose.NewHolidayClosureEvents().ConfirmHolidayClosureNotice(ctx, pending.ProposalID, pending.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: pending.ImpactToken})
		return err
	}))
	move("2030-07-11")
	for _, notice := range w.schoolNotices(t) {
		if notice.Status != "suggested" {
			continue
		}
		require.NotNil(t, notice.Previous)
		assert.Equal(t, calendar.NewDate(2030, 7, 8), notice.Previous.StartDate, "the unprocessed Friday was never the told source range")
		_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
		require.NoError(t, err)
	}
	mails := w.mails(t)
	require.Len(t, mails, 4)
	var correction enrollment.HolidayClosureMail
	require.NoError(t, json.Unmarshal(mails[1].Payload, &correction))
	require.Len(t, correction.ReopenedChildren, 1)
	assert.Equal(t, []string{"Montag, 08.07.2030"}, correction.ReopenedChildren[0].Weeks[0].ClosedDays)
	assert.Equal(t, []string{"Donnerstag, 11.07.2030"}, correction.Children[0].Weeks[0].ClosedDays)
}
