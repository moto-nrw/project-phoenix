package http_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestReplacementConfirmationResolvesBlockedPredecessor(t *testing.T) {
	t.Parallel()
	for _, channel := range []string{"school", "traeger"} {
		t.Run(channel, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger abgelöster Auftrag", testpkg.SetupIsolatedTestDB(t))
			account := testpkg.CreateTestAccount(t, w.db, "closure.approval@example.org")
			day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
			first := w.schoolNotices(t)[0]
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, first.ID, first.Revision,
					schoolcalendar.HolidayClosureActor{Channel: "school", AccountID: account.ID, ImpactToken: first.Token})
				return err
			}))
			var priorID int64
			require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
				event, leased, err := events.LeaseHolidayClosureEvent(ctx, w.organization, time.Minute)
				require.NoError(t, err)
				require.True(t, leased)
				priorID = event.ID
				return events.FinishHolidayClosureEvent(ctx, w.organization, event.ID, event.LeaseToken,
					schoolcalendar.HolidayClosureOutcome{State: "blocked", ErrorCode: "processing_failed"})
			}))
			var history string
			const historySQL = `SELECT jsonb_agg(to_jsonb(entry) ORDER BY entry.id)::text FROM calendar.holiday_closure_authorizations AS entry WHERE tenant_id=? AND event_id=?`
			require.NoError(t, w.db.NewRaw(historySQL, w.school, priorID).Scan(w.ctx, &history))
			resolve := func() error {
				return tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, tx bun.Tx) error {
					_, err := tx.ExecContext(ctx, `UPDATE calendar.holiday_closure_events
     SET state='no_mail', error_code='superseded', lease_token=NULL, lease_until=NULL,
      processed_at=CURRENT_TIMESTAMP, completed_at=CURRENT_TIMESTAMP WHERE tenant_id=? AND id=?`, w.school, priorID)
					return err
				})
			}
			require.ErrorContains(t, resolve(), "only takes up a failed", "a school cannot resolve work without a confirmed replacement")
			_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID,
				ClosingDayFields: schoolcalendar.ClosingDayFields{StartDate: "2030-07-12", EndDate: "2030-07-12", Reason: "Verschoben"}})
			require.NoError(t, err)
			require.ErrorContains(t, resolve(), "only takes up a failed", "an unconfirmed edit cannot resolve work either")
			var next schoolcalendar.HolidayClosureNotice
			require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				notices, err := events.SchoolHolidayClosureNotices(ctx)
				require.NoError(t, err)
				for _, notice := range notices {
					if notice.Status == schoolcalendar.HolidayClosureSuggested {
						next = notice
					}
				}
				return nil
			}))
			require.NotZero(t, next.ProposalID)
			assert.Equal(t, []string{"blocked"}, w.events(t), "an edit alone does not resolve the confirmed work")
			err = tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, next.ProposalID, next.Revision,
					schoolcalendar.HolidayClosureActor{Channel: "school", AccountID: account.ID, ImpactToken: "rollback"})
				require.NoError(t, err)
				return context.Canceled
			})
			require.ErrorIs(t, err, context.Canceled)
			assert.Equal(t, []string{"blocked"}, w.events(t), "replacement and predecessor settlement roll back together")
			if channel == "school" {
				notice := w.schoolNotices(t)
				for _, item := range notice {
					if item.ID == next.ProposalID {
						_, err = w.notices.NotifyFromSchool(w.ctx, w.school, account.ID, item.ID, item.Revision, item.Token)
					}
				}
			} else {
				pending, pendingErr := w.notices.PendingHolidayClosures(w.ctx, w.organization)
				require.NoError(t, pendingErr)
				_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, account.ID, pending.Token)
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"no_mail", "prepared"}, w.events(t))
			require.Len(t, w.mails(t), 2)
			_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='sent', sent_at=NOW() WHERE organization_id=?`, w.organization)
			require.NoError(t, err)
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{"no_mail", "completed"}, w.events(t))
			var preserved string
			require.NoError(t, w.db.NewRaw(historySQL, w.school, priorID).Scan(w.ctx, &preserved))
			assert.Equal(t, history, preserved, "supersession preserves the original authorization verbatim")
			for _, notice := range w.schoolNotices(t) {
				assert.NotEqual(t, "failed", string(notice.Status))
			}
			pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
			require.NoError(t, err)
			for _, notice := range pending.Notices {
				assert.NotEqual(t, "failed", string(notice.Status))
			}
			require.NoError(t, tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
				overdue, err := events.OverdueHolidayClosureEvents(ctx, 15*time.Minute)
				require.NoError(t, err)
				assert.Zero(t, overdue, "superseded work no longer triggers the watchdog")
				return nil
			}))
			_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET state='blocked',
    prepared_at=NOW(), completed_at=NULL WHERE tenant_id=? AND id=?`, w.school, priorID)
			require.NoError(t, err)
			require.ErrorContains(t, resolve(), "only takes up a failed", "even a linked confirmed replacement cannot discard prepared mail")
		})
	}
}
