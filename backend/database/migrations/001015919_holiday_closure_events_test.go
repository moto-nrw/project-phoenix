package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The read channel of the worker (ADR 0196 section 3.4) is a read-only
// SECURITY DEFINER function under a fixed search path, owned by a role
// without BYPASSRLS and without write rights, and only phoenix_tenant may
// call it.
func TestHolidayClosureReaderIsNarrow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	var definer, fixedPath bool
	var owner string
	require.NoError(t, db.NewRaw(`SELECT prosecdef, proconfig @> ARRAY['search_path=pg_catalog, pg_temp'], pg_get_userbyid(proowner)
		FROM pg_proc WHERE oid = 'calendar.traeger_holiday_closure_calendar(bigint[], date, date)'::regprocedure`).Scan(ctx, &definer, &fixedPath, &owner))
	assert.True(t, definer)
	assert.True(t, fixedPath)
	assert.Equal(t, "phoenix_holiday_closure_reader", owner)
	var bypass, login bool
	require.NoError(t, db.NewRaw(`SELECT rolbypassrls, rolcanlogin FROM pg_roles WHERE rolname = 'phoenix_holiday_closure_reader'`).Scan(ctx, &bypass, &login))
	assert.False(t, bypass, "no general BYPASSRLS")
	assert.False(t, login)
	for _, table := range []string{"schedule.closing_days", "config.setting_values", "platform.schools", "calendar.holiday_closure_revisions"} {
		var writes bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_holiday_closure_reader', ?, 'INSERT')
			OR has_table_privilege('phoenix_holiday_closure_reader', ?, 'UPDATE')
			OR has_table_privilege('phoenix_holiday_closure_reader', ?, 'DELETE')`, table, table, table).Scan(ctx, &writes))
		assert.False(t, writes, "the reader writes nothing: %s", table)
	}
	for role, allowed := range map[string]bool{"phoenix_tenant": true, "phoenix_auth": false, "phoenix_demo": false} {
		var may bool
		require.NoError(t, db.NewRaw(`SELECT has_function_privilege(?, 'calendar.traeger_holiday_closure_calendar(bigint[], date, date)', 'EXECUTE')`, role).
			Scan(ctx, &may))
		assert.Equal(t, allowed, may, role)
	}
}

// holidayClosureFixture is a Träger with a school, a booked holiday week
// and a closing day of the school in it nobody told yet.
type holidayClosureFixture struct {
	organization, school, closingDay, registration, week int64
}

func newHolidayClosureFixture(t *testing.T, db *bun.DB, name string) holidayClosureFixture {
	t.Helper()
	ctx := context.Background()
	f := holidayClosureFixture{organization: testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("umstieg-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID}
	f.school = testpkg.CreateTestSchoolInOrganization(t, db, f.organization, name+" OGS")
	var period, model, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2030-01-01', '2030-06-01', 'published', NOW())
		RETURNING id`, f.organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2030-07-08', '2030-07-12', ?) RETURNING id`, f.organization, period, f.school).Scan(ctx, &f.week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, f.organization, f.week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Ferien', 'eva@example.org', '0123', NOW()) RETURNING id`, f.organization, period).Scan(ctx, &f.registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Mia', 'Ferien', '2019-03-04') RETURNING id`, f.organization, period, f.registration).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		f.organization, period, child, f.week, model)
	require.NoError(t, err)
	require.NoError(t, db.NewRaw(`INSERT INTO schedule.closing_days (tenant_id, start_date, end_date, reason)
		VALUES (?, '2030-07-09', '2030-07-10', 'Pädagogische Tage') RETURNING id`, f.school).Scan(ctx, &f.closingDay))
	return f
}

// Section 7 of ADR 0196: the carry-over turns an open case of #6443 into a
// proposal, never a command or a mail; it skips a case whose days were all
// told, and running it again adds nothing.
func TestHolidayClosureCarryOverProposesWithoutMail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	open := newHolidayClosureFixture(t, db, "Umstieg offen")
	told := newHolidayClosureFixture(t, db, "Umstieg erledigt")
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_closure_notices (organization_id, holiday_registration_id, holiday_period_week_id, closed_on)
		VALUES (?, ?, ?, '2030-07-09'), (?, ?, ?, '2030-07-10')`,
		told.organization, told.registration, told.week, told.organization, told.registration, told.week)
	require.NoError(t, err)

	for range 2 {
		_, err := db.ExecContext(ctx, holidayClosureCarryOverSQL)
		require.NoError(t, err)
	}
	var proposals []struct {
		Organization int64  `bun:"organization_id"`
		Source       int64  `bun:"source_id"`
		State        string `bun:"state"`
	}
	require.NoError(t, db.NewRaw(`SELECT organization_id, source_id, state FROM calendar.holiday_closure_proposals
		WHERE organization_id IN (?, ?)`, open.organization, told.organization).Scan(ctx, &proposals))
	require.Len(t, proposals, 1, "once, and only for the case with untold days")
	assert.Equal(t, open.organization, proposals[0].Organization)
	assert.Equal(t, open.closingDay, proposals[0].Source)
	assert.Equal(t, "open", proposals[0].State, "a proposal, not a confirmed command")
	var events, mails int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM calendar.holiday_closure_events`).Scan(ctx, &events))
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE kind = 'holiday_closure_notice'`).Scan(ctx, &mails))
	assert.Zero(t, events)
	assert.Zero(t, mails, "the migration sends nothing")
	var state string
	require.NoError(t, db.NewRaw(`SELECT DISTINCT state FROM enrollment.holiday_closure_notices WHERE organization_id = ?`, told.organization).Scan(ctx, &state))
	assert.Equal(t, "closed", state, "existing notices count as told closed days")
}

// The field guard of the command (ADR 0196 section 3.2): a school unit only
// takes a failed command up again, a Träger unit never writes who requested
// it, and nobody changes the command fields.
func TestHolidayClosureEventFieldGuard(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	f := newHolidayClosureFixture(t, db, "Feldwächter")
	var proposal, event int64
	require.NoError(t, db.NewRaw(`INSERT INTO calendar.holiday_closure_proposals (tenant_id, organization_id, source_kind, source_id,
			operation_id, reason, current_start, current_end, span_from, span_to, revision)
		VALUES (?, 0, 'school_closing_day', ?, gen_random_uuid(), 'Pädagogische Tage', '2030-07-09', '2030-07-10', '2030-07-09', '2030-07-10', 1)
		RETURNING id`, f.school, f.closingDay).Scan(ctx, &proposal))
	require.NoError(t, db.NewRaw(`INSERT INTO calendar.holiday_closure_events (tenant_id, organization_id, proposal_id, operation_id,
			source_kind, source_id, revision, span_from, span_to, confirmed_channel, state)
		SELECT tenant_id, organization_id, id, operation_id, source_kind, source_id, revision, span_from, span_to, 'school', 'prepared'
		FROM calendar.holiday_closure_proposals WHERE id = ? RETURNING id`, proposal).Scan(ctx, &event))
	scoped := func(setting, value, statement string) error {
		return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			_, err := tx.ExecContext(ctx, statement, event)
			return err
		})
	}
	school, traeger := fmt.Sprint(f.school), fmt.Sprint(f.organization)
	require.ErrorContains(t, scoped("app.current_tenant_id", school, `UPDATE calendar.holiday_closure_events SET state = 'pending' WHERE id = ?`),
		"only takes up a failed", "a prepared command is not the school's to restart")
	require.ErrorContains(t, scoped("app.current_tenant_id", school, `UPDATE calendar.holiday_closure_events SET span_to = '2030-07-12' WHERE id = ?`),
		"never change", "the command fields are fixed")
	require.ErrorContains(t, scoped("app.current_org_id", traeger, `UPDATE calendar.holiday_closure_events SET requested_channel = 'traeger' WHERE id = ?`),
		"does not request", "the Träger unit processes, it does not request")
	require.NoError(t, scoped("app.current_org_id", traeger, `UPDATE calendar.holiday_closure_events SET state = 'failed' WHERE id = ?`))
	require.NoError(t, scoped("app.current_tenant_id", school, `UPDATE calendar.holiday_closure_events
		SET state = 'pending', requested_channel = 'school', requested_at = NOW() WHERE id = ?`), "a failed command is the school's to take up")
	var accountColumns int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns WHERE table_schema = 'calendar'
		AND table_name IN ('holiday_closure_events', 'holiday_closure_proposals') AND column_name LIKE '%account%'`).Scan(ctx, &accountColumns))
	assert.Zero(t, accountColumns, "neither proposal nor command names a person")
}
