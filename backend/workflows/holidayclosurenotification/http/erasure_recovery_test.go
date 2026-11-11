package http_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestConfirmedNoticeWithoutRemainingBookingsNeedsNoMail(t *testing.T) {
	t.Parallel()
	for _, change := range []string{"cancelled", "deleted"} {
		t.Run(change, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger ohne Buchungen", testpkg.SetupIsolatedTestDB(t))
			w.createClosingDay(t, "2030-07-08", "2030-07-08")
			n := w.schoolNotices(t)[0]
			events := schoolCalendarCompose.NewHolidayClosureEvents()
			require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
				_, err := events.ConfirmHolidayClosureNotice(ctx, n.ID, n.Revision, schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: n.Token})
				return err
			}))
			statement := `UPDATE enrollment.holiday_registrations SET status='cancelled' WHERE organization_id=?`
			if change == "deleted" {
				statement = `DELETE FROM enrollment.holiday_registrations WHERE organization_id=?`
			}
			_, err := w.db.ExecContext(w.ctx, statement, w.organization)
			require.NoError(t, err)
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{"no_mail"}, w.events(t))
			assert.Empty(t, w.mails(t))
			var history int
			require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_authorizations WHERE tenant_id=?`, w.school).Scan(w.ctx, &history))
			assert.Equal(t, 1, history, "the click remains recorded")
		})
	}
}

func TestHolidayClosureMailExpiresWithAnyIncludedRegistration(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Mailfrist", testpkg.SetupIsolatedTestDB(t))
	bookHolidayWeek(t, w.db, w.organization, w.school, "familie.eins@example.org")
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	n := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `UPDATE enrollment.holiday_registrations SET retention_days=365 WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `UPDATE enrollment.holiday_registrations SET retention_days=92 WHERE organization_id=? AND id=(SELECT min(id) FROM enrollment.holiday_registrations WHERE organization_id=?)`, w.organization, w.organization)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='dead_letter', dead_letter_at='2030-07-08', last_error='mailbox unavailable' WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	retain := func(day calendar.Date) {
		require.NoError(t, tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
			_, err := owners.NewHolidayRegistrationRetention(func() calendar.Date { return day }).DeleteExpiredHolidayRegistrations(ctx)
			return err
		}))
	}
	last := calendar.NewDate(2030, 7, 12)
	retain(last.AddDays(92))
	require.Len(t, w.mails(t), 2, "failed mail stays recoverable through the last kept day")
	retain(last.AddDays(93))
	require.Len(t, w.mails(t), 1, "a bundled mail goes with its first expired registration")
	assert.Equal(t, "familie.zwei@example.org", w.mails(t)[0].Recipient, "another family's recoverable mail stays")
	_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='sent', sent_at=NOW() WHERE organization_id=?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Equal(t, []string{"completed"}, w.events(t), "intentional erasure is no transport failure")
}

func TestDemoErasureResolvesBundledMailOfRemainingHostSchool(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name, state string
		onlyFamily  bool
	}{
		{"pending", "pending", false}, {"sent", "sent", false},
		{"only-pending", "pending", true}, {"only-sent", "sent", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			w := newClosureWorldIn(t, "Träger gebündelte Löschung", testpkg.SetupIsolatedTestDB(t))
			bookHolidayWeek(t, w.db, w.organization, w.school, "familie.eins@example.org")
			remaining := 1
			expectedWaiting, expectedFinished := "prepared", "completed"
			if scenario.onlyFamily {
				_, err := w.db.ExecContext(w.ctx, `UPDATE enrollment.holiday_registrations SET guardian_email='familie.eins@example.org' WHERE organization_id=?`, w.organization)
				require.NoError(t, err)
				remaining, expectedWaiting, expectedFinished = 0, "no_mail", "no_mail"
			}
			home := testpkg.CreateTestSchoolInOrganization(t, w.db, w.organization, "Gelöschte Stammschule")
			_, err := w.db.ExecContext(w.ctx, `UPDATE enrollment.holiday_registration_children SET home_school_id=? WHERE organization_id=? AND holiday_registration_id=(SELECT min(id) FROM enrollment.holiday_registrations WHERE organization_id=?)`, home, w.organization, w.organization)
			require.NoError(t, err)
			w.createClosingDay(t, "2030-07-08", "2030-07-08")
			n := w.schoolNotices(t)[0]
			_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
			require.NoError(t, err)
			require.Len(t, w.mails(t), remaining+1)
			_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status=?, sent_at=CASE WHEN ?='sent' THEN NOW() END WHERE organization_id=? AND payload->>'recipient_email'='familie.eins@example.org'`, scenario.state, scenario.state, w.organization)
			require.NoError(t, err)
			account := testpkg.CreateTestAccount(t, w.db, "demo-closure@example.org").ID
			testpkg.EnsureAccountTenant(t, w.db, account, home)
			testpkg.SoftDeleteTestSchool(t, w.db, home)
			_, err = w.db.ExecContext(w.ctx, `INSERT INTO platform.demo_school_states(name,tenant_id,visitor_account_ids,seed_state,status) VALUES (? , ?, ARRAY[?]::bigint[], '{}', 'ready')`, "ogs-closure-erasure", home, account)
			require.NoError(t, err)
			require.NoError(t, w.db.RunInTx(w.ctx, nil, func(ctx context.Context, tx bun.Tx) error {
				if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_demo`); err != nil {
					return err
				}
				var erased int
				if err := tx.NewRaw(`SELECT platform.erase_closed_demo_visitors()`).Scan(ctx, &erased); err != nil {
					return err
				}
				require.Equal(t, 1, erased)
				return nil
			}))
			require.Len(t, w.mails(t), remaining, "the shared mail containing erased data goes whole")
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{expectedWaiting}, w.events(t), "only mail not intentionally erased still needs transport work")
			_, err = w.db.ExecContext(w.ctx, `UPDATE calendar.holiday_closure_events SET state='failed' WHERE organization_id=?`, w.organization)
			require.NoError(t, err)
			_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
			require.NoError(t, err)
			require.Len(t, w.mails(t), remaining, "recovery must never recreate erased content")
			_, err = w.db.ExecContext(w.ctx, `UPDATE platform.email_outbox SET status='sent', sent_at=NOW() WHERE organization_id=?`, w.organization)
			require.NoError(t, err)
			require.NoError(t, w.notices.ProcessDue(w.ctx))
			assert.Equal(t, []string{expectedFinished}, w.events(t))
			var registrations, sends int
			require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE organization_id=?`, w.organization).Scan(w.ctx, &registrations))
			require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_closure_sends WHERE organization_id=?`, w.organization).Scan(w.ctx, &sends))
			assert.Equal(t, 2, registrations)
			assert.Equal(t, 2, sends, "the remaining registration keeps evidence against another send")
		})
	}
}

func TestHolidayMailErasureFailureRollsBackRegistrationRetention(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Löschung fehlgeschlagen", testpkg.SetupIsolatedTestDB(t))
	w.createClosingDay(t, "2030-07-08", "2030-07-08")
	n := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, n.ID, n.Revision, n.Token)
	require.NoError(t, err)
	_, err = w.db.ExecContext(w.ctx, `CREATE FUNCTION public.refuse_mail_erasure() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'erasure refused'; END $$;
  CREATE TRIGGER refuse_mail_erasure BEFORE DELETE ON platform.email_outbox FOR EACH ROW EXECUTE FUNCTION public.refuse_mail_erasure()`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := w.db.ExecContext(w.ctx, `DROP TRIGGER refuse_mail_erasure ON platform.email_outbox; DROP FUNCTION public.refuse_mail_erasure()`)
		require.NoError(t, err)
	})
	err = tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
		_, err := owners.NewHolidayRegistrationRetention(func() calendar.Date { return calendar.NewDate(2031, 7, 14) }).DeleteExpiredHolidayRegistrations(ctx)
		return err
	})
	require.ErrorContains(t, err, "erasure refused")
	var registrations, sends, deletions int
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE organization_id=?`, w.organization).Scan(w.ctx, &registrations))
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_closure_sends WHERE organization_id=?`, w.organization).Scan(w.ctx, &sends))
	require.NoError(t, w.db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registration_deletions WHERE organization_id=?`, w.organization).Scan(w.ctx, &deletions))
	assert.Equal(t, 2, registrations)
	assert.Equal(t, 2, sends)
	assert.Zero(t, deletions)
	assert.Len(t, w.mails(t), 2, "a failed mail erasure rolls back the entire command")
}
