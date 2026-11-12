package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	notification "github.com/moto-ogs/project-phoenix/workflows/holidayclosurenotification"
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

func TestDeletingUnpreparedNewClosureWithdrawsItsCommands(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	for _, state := range []string{"pending", "leased", "edited", "confirmed_twice"} {
		t.Run(state, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger vor Verarbeitung aufgehoben "+state, db)
			day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			confirm := func() {
				for _, notice := range w.schoolNotices(t) {
					if notice.Status != "suggested" {
						continue
					}
					require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
						_, err := events.ConfirmHolidayClosureNotice(ctx, notice.ID, notice.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: notice.Token})
						return err
					}))
				}
			}
			confirm()
			var leasedEvent schoolcalendar.HolidayClosureEvent
			if state == "leased" {
				require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
					event, leased, err := events.LeaseHolidayClosureEvent(ctx, w.organization, time.Minute)
					leasedEvent = event
					require.True(t, leased)
					return err
				}))
			}
			if state == "edited" || state == "confirmed_twice" {
				_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: "2030-07-12", EndDate: "2030-07-12", Reason: "Verschoben"}})
				require.NoError(t, err)
				if state == "confirmed_twice" {
					confirm()
				}
			}
			require.NoError(t, w.records.DeleteClosingDay(w.schoolCtx(), day.ID))
			if state == "leased" {
				err := testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
					return events.LockLeasedHolidayClosureEvent(ctx, w.organization, leasedEvent.ID, leasedEvent.LeaseToken)
				})
				require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureLeaseLost, "the former worker cannot prepare the withdrawn command")
			}
			states := w.events(t)
			require.NotEmpty(t, states, "confirmed history remains until retirement")
			for _, state := range states {
				assert.Equal(t, "no_mail", state)
			}
			for _, notice := range w.schoolNotices(t) {
				assert.False(t, notice.CanNotify)
			}
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Empty(t, w.mails(t), "withdrawal must prevent a previously leased command preparing mail")
		})
	}
}

func TestIndirectRetryReconcilesTheFailedPredecessor(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Vorgängerstatus abgleichen", testpkg.SetupIsolatedTestDB(t))
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	first := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, first.ID, first.Revision, first.Token)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='dead_letter', dead_letter_at=NOW(), last_error='mailbox unavailable' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	secondDay := w.createClosingDay(t, "2030-07-12", "2030-07-12")
	for _, notice := range w.schoolNotices(t) {
		if notice.ClosingDayID != secondDay.ID {
			continue
		}
		_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"failed", "failed"}, w.events(t))
	for _, notice := range w.schoolNotices(t) {
		if notice.ClosingDayID != secondDay.ID {
			continue
		}
		_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
		require.NoError(t, err)
	}
	assert.Equal(t, []string{"prepared", "prepared"}, w.events(t), "indirectly retried mail must clear its predecessor's error")
	for _, notice := range w.schoolNotices(t) {
		assert.Equal(t, "sending", notice.Status)
		assert.False(t, notice.CanNotify)
	}
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	for _, notice := range pending.Notices {
		assert.Equal(t, "sending", notice.Status)
	}
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='sent', sent_at=NOW() WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	// Leave only the failed predecessor for worker discovery.
	_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET state=CASE WHEN proposal_id=? THEN 'failed' ELSE 'completed' END WHERE organization_id=?`, first.ID, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Equal(t, []string{"completed", "completed"}, w.events(t))
	for _, notice := range w.schoolNotices(t) {
		assert.Equal(t, "sent", notice.Status)
	}
	pending, err = w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	for _, notice := range pending.Notices {
		assert.Equal(t, "sent", notice.Status)
	}
	require.Len(t, w.mails(t), 4, "recovery reuses the original mails")
	_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET completed_at=NOW()-interval '31 days' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Empty(t, w.events(t), "indirect recovery also restores normal retirement")
}

func TestEditedNoticeReplacesAnOutstandingRenewal(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"single", "bulk", "deleted"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger erneute Bestätigung nach Änderung", testpkg.SetupIsolatedTestDB(t))
			day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
			first := w.schoolNotices(t)[0]
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, first.ID, first.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: first.Token})
				return err
			}))
			// An unrelated calendar revision makes the confirmed command await approval.
			w.createClosingDay(t, "2030-08-20", "2030-08-20")
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{"needs_confirmation"}, w.events(t))
			stale := w.schoolNotices(t)[0]
			if mode == "deleted" {
				require.NoError(t, w.records.DeleteClosingDay(w.schoolCtx(), day.ID))
				assert.Equal(t, []string{"no_mail"}, w.events(t),
					"the deletion resolves the command that waited for approval and told nobody, so retirement can proceed")
				require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
					notices, err := events.SchoolHolidayClosureNotices(ctx)
					require.NoError(t, err)
					var cancellation []schoolcalendar.HolidayClosureNotice
					for _, notice := range notices {
						if notice.ClosingDayID != day.ID {
							continue
						}
						if notice.Status == schoolcalendar.HolidayClosureNoMail {
							assert.False(t, notice.CanNotify(), "the withdrawn command is finished")
							continue
						}
						cancellation = append(cancellation, notice)
					}
					require.Len(t, cancellation, 1, "only the nonempty cancellation proposal remains open for this source")
					assert.NotNil(t, cancellation[0].Previous)
					assert.Nil(t, cancellation[0].Current)
					return nil
				}))
				require.NoError(t, w.notices.ProcessDue(w.ctx))
				assert.Empty(t, w.mails(t))
				return
			}
			_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: "2030-07-08", EndDate: "2030-07-12", Reason: "Verschoben"}})
			require.NoError(t, err)
			// Also recover proposals saved before explicit predecessor linking.
			_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_proposals SET supersedes_event_id=NULL WHERE tenant_id=? AND state='open'`, w.school)
			require.NoError(t, err)
			notices := w.schoolNotices(t)
			require.Len(t, notices, 1, "the new source proposal replaces the old renewal in both portals")
			require.NotEqual(t, stale.ID, notices[0].ID)
			err = tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, stale.ID, stale.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: stale.Token})
				return err
			})
			require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureNoticeChanged, "a stale renewal must not reach a uniqueness error")
			pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
			require.NoError(t, err)
			require.Len(t, pending.Notices, 1)
			if mode == "bulk" {
				_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
			} else {
				n := notices[0]
				_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
			}
			require.NoError(t, err)
			require.Len(t, w.mails(t), 2)
			assert.Contains(t, string(w.mails(t)[0].Payload), "12.07.2030")
			err = tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, stale.ID, stale.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: stale.Token})
				return err
			})
			require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureNoticeChanged, "the old command also stays superseded after the newer proposal is confirmed")
			// A later edit must use the latest prepared source, not the old renewal.
			_, err = w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: "2030-07-10", EndDate: "2030-07-10", Reason: "Verkürzt"}})
			require.NoError(t, err)
			var corrections []notification.Notice
			for _, notice := range w.schoolNotices(t) {
				if notice.ClosingDayID == day.ID && notice.Status == "suggested" {
					corrections = append(corrections, notice)
				}
			}
			require.Len(t, corrections, 1, "the reopening of the latest told source remains discoverable")
			correction := corrections[0]
			require.NotNil(t, correction.Previous)
			assert.Equal(t, calendar.NewDate(2030, 7, 12), correction.Previous.EndDate)
			_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, correction.ID, correction.Revision, correction.Token)
			require.NoError(t, err)
			require.Len(t, w.mails(t), 4)
			assert.Contains(t, string(w.mails(t)[1].Payload), "reopened_children")
		})
	}
}

func TestPreparationFailureKeepsTheLastToldSourceRange(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Vorbereitung fehlgeschlagen", testpkg.SetupIsolatedTestDB(t))
	day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
	n := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
	require.NoError(t, err)
	move := func(date string) {
		_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: date, EndDate: date, Reason: "Verschoben"}})
		require.NoError(t, err)
	}
	move("2030-07-12")
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	for _, notice := range w.schoolNotices(t) {
		if notice.Status != "suggested" {
			continue
		}
		require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
			_, err := events.ConfirmHolidayClosureNotice(ctx, notice.ID, notice.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: notice.Token})
			return err
		}))
	}
	require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
		event, leased, err := events.LeaseHolidayClosureEvent(ctx, w.organization, time.Minute)
		require.NoError(t, err)
		require.True(t, leased)
		return events.FinishHolidayClosureEvent(ctx, w.organization, event.ID, event.LeaseToken, schoolcalendar.HolidayClosureOutcome{State: schoolcalendar.HolidayClosureEventPending, ErrorCode: "processing_failed", RetryAt: time.Now().Add(time.Hour)})
	}))
	move("2030-07-11")
	var latest notification.Notice
	for _, notice := range w.schoolNotices(t) {
		if notice.Status == "suggested" {
			latest = notice
		}
	}
	require.NotNil(t, latest.Previous)
	assert.Equal(t, calendar.NewDate(2030, 7, 8), latest.Previous.StartDate)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, latest.ID, latest.Revision, latest.Token)
	require.NoError(t, err)
	require.Len(t, w.mails(t), 4)
	assert.Contains(t, string(w.mails(t)[1].Payload), "Montag, 08.07.2030")
}

func TestErasedRegistrationsResolveWaitingCommands(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"prepared", "unknown"} {
		t.Run(state, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger gelöschte Anmeldungen", testpkg.SetupIsolatedTestDB(t))
			w.createClosingDay(t, "2030-07-08", "2030-07-08")
			n := w.schoolNotices(t)[0]
			_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
			require.NoError(t, err)
			_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET state=? WHERE organization_id=?`, state, w.organization)
			require.NoError(t, err)
			_, err = w.db.ExecContext(w.ctx, `DELETE FROM enrollment.holiday_registrations WHERE organization_id=?`, w.organization)
			require.NoError(t, err)
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{"no_mail"}, w.events(t), "erased recipients require no further transport work")
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			require.NoError(t, tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
				due, err := events.DueHolidayClosureTraeger(ctx)
				require.NoError(t, err)
				assert.NotContains(t, due, w.organization, "resolved erasure no longer polls transport")
				overdue, err := events.OverdueHolidayClosureEvents(ctx, 15*time.Minute)
				require.NoError(t, err)
				assert.Zero(t, overdue)
				return nil
			}))
			_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET completed_at=NOW()-interval '31 days' WHERE organization_id=?`, w.organization)
			require.NoError(t, err)
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Empty(t, w.events(t))
			var authorizations int
			require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_authorizations WHERE tenant_id=?`, w.school).Scan(w.ctx, &authorizations))
			assert.Zero(t, authorizations, "authorization history retires with its resolved command")
		})
	}
}

func TestAuthorizationHistoryCommitsWithTheCommandAndAnonymizesActors(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Freigabenachweis", testpkg.SetupIsolatedTestDB(t))
	first := testpkg.CreateTestAccount(t, w.db, "closure.approver@example.org")
	retry := testpkg.CreateTestAccount(t, w.db, "closure.retry@example.org")
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	n := w.schoolNotices(t)[0]
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	err := tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := events.ConfirmHolidayClosureNotice(ctx, n.ID, n.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, AccountID: first.ID, ImpactToken: n.Token})
		if err != nil {
			return err
		}
		return context.Canceled
	})
	require.ErrorIs(t, err, context.Canceled)
	var rolledBack int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_authorizations WHERE tenant_id=?`, w.school).Scan(w.ctx, &rolledBack))
	assert.Zero(t, rolledBack)
	assert.Empty(t, w.events(t))
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, first.ID, n.ID, n.Revision, n.Token)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='dead_letter', dead_letter_at=NOW(), last_error='mailbox unavailable' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	_, err = w.notices.NotifyFromTraeger(w.ctx, w.organization, retry.ID, n.ID, n.Revision, n.Token)
	require.NoError(t, err)
	var accounts []int64
	require.NoError(t, w.db.NewRaw(`SELECT account_id FROM calendar.holiday_closure_authorizations WHERE tenant_id=? ORDER BY id`, w.school).Scan(w.ctx, &accounts))
	assert.Equal(t, []int64{first.ID, retry.ID}, accounts)
	var channels []string
	require.NoError(t, w.db.NewRaw(`SELECT channel FROM calendar.holiday_closure_authorizations WHERE tenant_id=? ORDER BY id`, w.school).Scan(w.ctx, &channels))
	assert.Equal(t, []string{"school", "traeger"}, channels)
	// Renewing approval must preserve the earlier authorization identities too.
	_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET state='needs_confirmation' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := events.ConfirmHolidayClosureNotice(ctx, n.ID, n.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, AccountID: first.ID, ImpactToken: n.Token})
		return err
	}))
	require.NoError(t, w.db.NewRaw(`SELECT account_id FROM calendar.holiday_closure_authorizations WHERE tenant_id=? ORDER BY id`, w.school).Scan(w.ctx, &accounts))
	assert.Equal(t, []int64{first.ID, retry.ID, first.ID}, accounts)
	_, err = w.db.ExecContext(w.ctx, `DELETE FROM auth.accounts WHERE id IN (?, ?)`, first.ID, retry.ID)
	require.NoError(t, err)
	var anonymized int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_authorizations WHERE tenant_id=? AND account_id IS NULL`, w.school).Scan(w.ctx, &anonymized))
	assert.Equal(t, 3, anonymized, "account erasure preserves the authorization history without its actor")
}

func TestOfficeRecoversTraegerMailWithoutAnySchoolCommand(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger eigener Wiederholungsweg", testpkg.SetupIsolatedTestDB(t))
	for _, date := range []calendar.Date{"2030-07-08", "2030-07-12"} {
		require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
			closure := enrollment.HolidayClosure{SchoolIDs: []int64{w.school}, StartsOn: date, EndsOn: date, Reason: "Träger-Schließtag"}
			impact, err := w.closures.HolidayClosureImpact(ctx, w.organization, closure)
			require.NoError(t, err)
			_, err = w.closures.NotifyHolidayClosure(ctx, w.organization, closure, impact.Token)
			return err
		}))
	}
	assert.Empty(t, w.events(t))
	_, err := w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='dead_letter', dead_letter_at=NOW(), last_error='mailbox unavailable' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	raw, err := json.Marshal(pending)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.Equal(t, float64(4), fields["retryable_mails"], "office recovery is visible even without a school proposal")
	other := newClosureWorldIn(t, "Träger fremder Wiederholungsweg", w.db)
	foreign, err := other.notices.PendingHolidayClosures(other.ctx, other.organization)
	require.NoError(t, err)
	assert.Zero(t, foreign.RetryableMails)
	assert.Zero(t, foreign.UnknownMails)
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.NoError(t, err)
	require.Len(t, w.mails(t), 4)
	for _, mail := range w.mails(t) {
		assert.Equal(t, "pending", mail.Status)
	}
	assert.Empty(t, w.events(t), "mail recovery needs no unrelated school command")
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='dead_letter', dead_letter_at=NOW(), last_error=? WHERE organization_id=?`, deliveryContract.DispatchOutcomeUnknown, w.organization)
	require.NoError(t, err)
	unknown, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	assert.Zero(t, unknown.RetryableMails)
	assert.Equal(t, 4, unknown.UnknownMails)
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, unknown.Token)
	require.NoError(t, err)
	for _, mail := range w.mails(t) {
		assert.Equal(t, "dead_letter", mail.Status, "unknown outcomes are never blindly repeated")
	}
}

func TestTraegerClosureChangesExcludeTransferredHistoricalSchools(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Schließtag nach Schulwechsel")
	transferred := testpkg.CreateTestSchoolInOrganization(t, w.db, w.organization, "Wechselnde Schule")
	next := testpkg.CreateTestOrganization(t, w.db, &testpkg.Organization{
		Name: "Neuer Träger", Slug: fmt.Sprintf("closure-transfer-%d", transferred), Active: true,
	})
	_, err := w.db.ExecContext(w.ctx, `UPDATE platform.schools SET organization_id=? WHERE id=?`, next.ID, transferred)
	require.NoError(t, err)
	day := calendar.Date("2030-07-08")
	previous := &enrollment.HolidayClosureSpan{SchoolIDs: []int64{w.school, transferred}, StartsOn: day, EndsOn: day}
	for _, deleted := range []bool{false, true} {
		require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
			closure := enrollment.HolidayClosure{SchoolIDs: []int64{w.school}, StartsOn: day, EndsOn: day, Previous: previous, Deleted: deleted}
			_, err := w.closures.HolidayClosureImpact(ctx, w.organization, closure)
			return err
		}), "updates and deletion must ignore schools that no longer belong to the Träger")
	}
	require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
		_, err := w.closures.HolidayClosureImpact(ctx, w.organization, enrollment.HolidayClosure{
			SchoolIDs: []int64{transferred}, StartsOn: day, EndsOn: day, Previous: previous,
		})
		require.ErrorIs(t, err, enrollment.ErrHolidayHostSchoolOutsideTraeger, "newly requested schools must still be authorized")
		return nil
	}))
}
