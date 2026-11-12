package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/internal/adapters/postgres"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// TestHolidayAllocationStatementsKeepTheirTraegerBeneathRLS pins the Träger
// filter of every statement of the allocation by criteria (#4700) beneath
// RLS, as #5952 and #6001 do: in the Träger transaction of A, as the owner
// of the tables, each statement is asked for B's criteria, answers,
// bookings and decisions. Each assertion goes red when its statement drops
// its organization_id filter.
func TestHolidayAllocationStatementsKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newPlacesWeek(t, db, "Träger Vergabe A")
	foreign := newPlacesWeek(t, db, "Träger Vergabe B")
	bg := context.Background()
	t.Cleanup(func() {
		_, err := db.ExecContext(bg, `DELETE FROM enrollment.holiday_allocation_criteria WHERE organization_id IN (?, ?)`, own.organization, foreign.organization)
		require.NoError(t, err)
	})
	_, err := db.ExecContext(bg, `UPDATE enrollment.holiday_periods SET allocation_mode = 'criteria' WHERE id IN (?, ?)`, own.period, foreign.period)
	require.NoError(t, err)
	_, err = db.ExecContext(bg, `UPDATE enrollment.holiday_registration_bookings SET placement = 'pending' WHERE placement = 'waitlisted'
		AND holiday_period_id = ?`, foreign.period)
	require.NoError(t, err)
	var criterion, child, booking int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_allocation_criteria (organization_id, name, question, scope, position)
		VALUES (?, 'Berufstätig', 'Sind Sie berufstätig?', 'family', 1) RETURNING id`, foreign.organization).Scan(bg, &criterion))
	require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ?`, foreign.waiting).Scan(bg, &child))
	require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.holiday_registration_bookings WHERE holiday_registration_child_id = ?`, child).Scan(bg, &booking))
	_, err = db.ExecContext(bg, `INSERT INTO enrollment.holiday_registration_criteria (organization_id, holiday_period_id, holiday_registration_id, criterion_id, stated, checked, checked_at)
		VALUES (?, ?, ?, ?, true, false, NOW())`, foreign.organization, foreign.period, foreign.waiting, criterion)
	require.NoError(t, err)
	_, err = db.ExecContext(bg, `INSERT INTO enrollment.holiday_allocation_decisions (organization_id, holiday_period_id, holiday_registration_id,
			holiday_registration_child_id, holiday_period_week_id, week_starts_on, week_ends_on, placement, proposal, rank)
		VALUES (?, ?, ?, ?, ?, '2027-07-05', '2027-07-09', 'placed', 'placed', 1)`, foreign.organization, foreign.period, foreign.waiting, child, foreign.week)
	require.NoError(t, err)
	store := postgres.NewHolidayRegistrationStore()
	yes := true

	ctx := testpkg.ContextWithTenantRuntime(bg, testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		criteria, err := store.ReadHolidayCriteria(ctx, own.organization)
		require.NoError(t, err)
		assert.Empty(t, criteria, "A reads no criterion of B")
		recordCriteria, err := store.ReadHolidayRecordCriteria(ctx, own.organization, foreign.period, foreign.waiting)
		require.NoError(t, err)
		assert.Empty(t, recordCriteria, "A exports no criterion of B, including answered ones")

		require.NoError(t, store.WriteHolidayCriteria(ctx, own.organization, []enrollment.HolidayCriterionInput{
			{ID: criterion, Name: "Übernommen", Question: "Von A?", Scope: enrollment.HolidayCriterionChild, Active: false},
		}, 0))
		require.NoError(t, store.WriteHolidayCriteria(ctx, own.organization, nil, 0), "an empty list archives A's criteria only")

		answers, err := store.ReadHolidayCriterionAnswers(ctx, own.organization, foreign.period, 0)
		require.NoError(t, err)
		assert.Empty(t, answers, "A reads no answer of B")
		found, err := store.CheckHolidayCriterion(ctx, own.organization, foreign.period, foreign.waiting, criterion, 0, &yes, 0)
		require.NoError(t, err)
		assert.False(t, found, "A checks no answer of B")
		found, err = store.CheckHolidayCriterion(ctx, own.organization, foreign.period, foreign.waiting, criterion, 0, nil, 0)
		require.NoError(t, err)
		assert.True(t, found)

		pending, err := store.ReadHolidayPendingBookings(ctx, own.organization, foreign.period)
		require.NoError(t, err)
		assert.Empty(t, pending, "A reads no pending booking of B")
		changed, err := store.DecideHolidayBookings(ctx, own.organization, foreign.week, []int64{booking}, nil)
		require.NoError(t, err)
		assert.Empty(t, changed, "A decides on no booking of B")
		decisions, err := store.ReadHolidayAllocationDecisions(ctx, own.organization, foreign.period)
		require.NoError(t, err)
		assert.Empty(t, decisions, "A reads no decision of B")
		return nil
	}))

	var name, placement string
	var archived bool
	var checked *bool
	require.NoError(t, db.NewRaw(`SELECT name, archived_at IS NOT NULL FROM enrollment.holiday_allocation_criteria WHERE id = ?`, criterion).Scan(bg, &name, &archived))
	assert.Equal(t, "Berufstätig", name, "A changes no criterion of B")
	assert.False(t, archived, "A's list archives no criterion of B")
	require.NoError(t, db.NewRaw(`SELECT checked FROM enrollment.holiday_registration_criteria WHERE criterion_id = ?`, criterion).Scan(bg, &checked))
	require.NotNil(t, checked)
	assert.False(t, *checked, "A neither checks nor takes back a check of B")
	var answers int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registration_criteria WHERE criterion_id = ?`, criterion).Scan(bg, &answers))
	assert.Equal(t, 1, answers, "A takes back no answer of B")
	require.NoError(t, db.NewRaw(`SELECT placement FROM enrollment.holiday_registration_bookings WHERE id = ?`, booking).Scan(bg, &placement))
	assert.Equal(t, "pending", placement)
}
