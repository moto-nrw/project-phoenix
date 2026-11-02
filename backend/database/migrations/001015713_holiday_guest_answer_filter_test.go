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

func TestHolidayGuestSharedAnswersFilterHealthHistoryAndSchoolScope(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	f := newHolidaySharedAnswersFixture(t, db)
	f.insertVersion(t, f.own, f.period, 2, `[
		{"key":"safe","health":false}, {"key":"health","health":false},
		{"key":"missing","health":false}, {"key":"string","health":false}, {"key":"null","health":false}]`)
	student := testpkg.CreateTestStudentForTenant(t, db, f.host, "Ferien", "Gast", "Feriengruppe")
	child := f.register(t, holidaySharedRegistration{organization: f.own, period: f.period, version: 2, status: "confirmed"})
	var guest int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_guests
		(tenant_id, organization_id, holiday_period_week_id, holiday_registration_child_id, student_id,
		 starts_on, ends_on, time_model_name, time_model_starts_at, time_model_ends_at, answers)
		VALUES (?, ?, ?, ?, ?, '2099-07-06', '2099-07-10', 'Ferien', '08:00', '13:00', ?::jsonb) RETURNING id`,
		f.host, f.own, f.open, child, student.ID,
		`[{"key":"safe","value":"unverändert"},{"key":"health","value":"sensibel"},
		  {"key":"missing"},{"key":"string"},{"key":"null"},{"key":"unknown"}]`).Scan(ctx, &guest))
	read := func(school int64, organization string) string {
		t.Helper()
		var answers string
		f.asSchool(t, school, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, organization); err != nil {
				return err
			}
			return tx.NewRaw(`SELECT enrollment.holiday_guest_shared_answers(?)::text`, guest).Scan(ctx, &answers)
		})
		return answers
	}
	assert.Contains(t, read(f.host, ""), "sensibel", "the originally safe copy is readable")
	assert.Equal(t, "[]", read(f.home, ""), "another school of the same Träger cannot name this guest")
	assert.Equal(t, "[]", read(f.foreignSchool, ""), "another Träger's school cannot name this guest")
	assert.Equal(t, "[]", read(f.host, fmt.Sprint(f.own)), "a Träger session cannot use the school function")
	assert.Equal(t, "[]", read(0, ""), "a session without a school gets nothing")

	// An unrelated period with the same key must not revoke the safe copy.
	f.insertVersion(t, f.own, f.insertPeriod(t, f.own), 2, `[{"key":"safe","health":true}]`)
	f.insertVersion(t, f.foreign, f.foreignPeriod, 2, `[{"key":"safe","health":true}]`)
	f.insertVersion(t, f.own, f.period, 3, `[
		{"key":"safe","health":false}, {"key":"health","health":true},
		{"key":"missing"}, {"key":"string","health":"false"}, {"key":"null","health":null}]`)
	assert.JSONEq(t, `[{"key":"safe","value":"unverändert"}]`, read(f.host, ""),
		"every ambiguous or health marker suppresses the copied key, including its label and value")
	f.insertVersion(t, f.own, f.period, 4, `[{"key":"health","health":false}]`)
	assert.JSONEq(t, `[{"key":"safe","value":"unverändert"}]`, read(f.host, ""), "a later safe version cannot undo health history")
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_guests SET ended_on = '2099-07-08' WHERE id = ?`, guest)
	require.NoError(t, err)
	assert.Equal(t, "[]", read(f.host, ""), "an ended guest exposes no copied answers")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_guests SET ended_on = NULL, starts_on = '2020-07-06', ends_on = '2020-07-10' WHERE id = ?`, guest)
	require.NoError(t, err)
	assert.Equal(t, "[]", read(f.host, ""), "an expired copy exposes no answers even before cleanup")
}

func TestHolidayGuestAnswerFilterDownRemovesOnlyItsFunction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	functionExists := func() bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT to_regprocedure('enrollment.holiday_guest_shared_answers(bigint)') IS NOT NULL`).Scan(ctx, &exists))
		return exists
	}
	require.True(t, functionExists())
	require.NoError(t, holidayGuestAnswerFilterDown(ctx, db))
	assert.False(t, functionExists())
	require.NoError(t, holidayGuestAnswerFilterUp(ctx, db))
	assert.True(t, functionExists())
	var executable bool
	require.NoError(t, db.NewRaw(`SELECT has_function_privilege('phoenix_tenant', 'enrollment.holiday_guest_shared_answers(bigint)', 'EXECUTE')
		AND NOT has_function_privilege('public', 'enrollment.holiday_guest_shared_answers(bigint)', 'EXECUTE')`).Scan(ctx, &executable))
	assert.True(t, executable)
}
