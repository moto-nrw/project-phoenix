package migrations

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The erasure of 1.15.939 is the erasure of 1.15.927 whose statement for the
// mails of a registration also deletes the offer of a parent account (ADR
// 0216), and nothing else changed.
func TestParentAccountOfferErasureIsTheErasureOf927WithTheOffer(t *testing.T) {
	t.Parallel()
	before := `'holiday_closure_notice', 'holiday_registration_accepted')`
	after := `'holiday_closure_notice', 'holiday_registration_accepted',
							'holiday_parent_account_offer')`
	commentBefore := "-- mail of the Träger, named by the registration alone.\n"
	commentAfter := "-- mail of the Träger, named by the registration alone. So is\n\t\t\t\t\t-- the offer of a parent account (ADR 0216).\n"
	require.Equal(t, 1, strings.Count(demoVisitorErasureHolidayAcceptanceSQL, before), "the statement for the mails of a registration is in the erasure of 1.15.927 exactly once")
	require.Equal(t, 1, strings.Count(demoVisitorErasureHolidayAcceptanceSQL, commentBefore))
	want := strings.Replace(strings.Replace(demoVisitorErasureHolidayAcceptanceSQL, before, after, 1), commentBefore, commentAfter, 1)
	assert.Equal(t, want, demoVisitorErasureParentAccountOfferSQL)
}

// TestHolidayRegistrationParentAccountGoesWithTheAccount pins the link of
// 1.15.939 (ADR 0216 points 1 and 10): a registration names at most one
// account, keeps its row when the account goes, and the read by account and
// the claim by address each have their index.
func TestHolidayRegistrationParentAccountGoesWithTheAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger Konto an der Anmeldung", Slug: fmt.Sprintf("traeger-konto-anmeldung-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	account := testpkg.CreateTestAccount(t, db, "holiday-registration-link")
	var period, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on, registration_closes_on)
		VALUES (?, 'Herbstferien', '2027-08-02', '2027-09-03') RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at, parent_account_id, parent_account_requested)
		VALUES (?, ?, 'Eva', 'Muster', 'Eva@Example.org', '0123', NOW(), ?, TRUE) RETURNING id`, organization, period, account.ID).Scan(ctx, &registration))

	var requested bool
	var linked *int64
	require.NoError(t, db.NewRaw(`SELECT parent_account_id, parent_account_requested FROM enrollment.holiday_registrations WHERE id = ?`, registration).
		Scan(ctx, &linked, &requested))
	require.NotNil(t, linked)
	assert.Equal(t, account.ID, *linked)
	assert.True(t, requested)

	var indexes []string
	require.NoError(t, db.NewRaw(`SELECT indexdef FROM pg_indexes WHERE schemaname = 'enrollment' AND tablename = 'holiday_registrations'
		AND indexname IN ('idx_holiday_registrations_parent_account', 'idx_holiday_registrations_unclaimed_email') ORDER BY indexname`).Scan(ctx, &indexes))
	require.Len(t, indexes, 2)
	assert.Contains(t, indexes[0], "WHERE (parent_account_id IS NOT NULL)")
	assert.Contains(t, indexes[1], "lower(guardian_email)")
	assert.Contains(t, indexes[1], "WHERE (parent_account_id IS NULL)")

	_, err := db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err)
	var kept int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.holiday_registrations WHERE id = ? AND parent_account_id IS NULL`, registration).Scan(ctx, &kept))
	assert.Equal(t, 1, kept, "the registration stays without the account")
}

// The rollback writes back exactly the CHECK of 1.15.927, and the CHECK of
// this migration declares every kind of it and the offer.
func TestHolidayParentAccountKindsBuildOnTheAcceptanceMail(t *testing.T) {
	t.Parallel()
	assert.Equal(t, holidayAcceptanceEmailOutboxTraegerKindCheckSQL, holidayParentAccountRestoredKindCheckSQL)
	assert.Equal(t, validateHolidayAcceptanceKindCheckSQL, validateHolidayParentAccountKindCheckSQL)
	for _, kind := range append(slices.Clone(holidayAcceptanceEmailOutboxTraegerKinds), "holiday_parent_account_offer") {
		assert.Contains(t, holidayParentAccountOfferKindCheckSQL, "'"+kind+"'")
		assert.Contains(t, holidayParentAccountOfferTraegerKinds, kind)
	}
}

// TestHolidayRegistrationKeepsTheAccountAtItsTraeger pins the lasting
// retention of 1.15.939 (ADR 0216 point 10): past the 30 days of its setup
// link an access at a live Träger keeps its account while a holiday
// registration at that Träger names the account or, not yet claimed, its
// address; a registration at another Träger or one that went keeps nothing.
func TestHolidayRegistrationKeepsTheAccountAtItsTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) (int64, int64) {
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
			Name: name, Slug: fmt.Sprintf("traeger-konto-frist-%d", testpkg.UniqueTestTenantID(t)), Active: true,
		}).ID
		return organization, testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	}
	own, ownSchool := newTraeger("Träger Konto Frist")
	other, otherSchool := newTraeger("Träger Konto Fremd")
	account := testpkg.CreateTestAccount(t, db, "holiday-account-retention")
	testpkg.UnclaimTestAccount(t, db, account.ID)
	testpkg.OwnTestAccount(t, db, account.ID)
	unused := func() bool {
		t.Helper()
		var result bool
		require.NoError(t, db.NewRaw(`SELECT auth.account_is_unused(?, ?)`, account.ID, account.Email).Scan(ctx, &result))
		return result
	}
	_, err := db.ExecContext(ctx, `INSERT INTO auth.traeger_parent_access (account_id, organization_id, created_at)
		VALUES (?, ?, NOW() - INTERVAL '90 days')`, account.ID, own)
	require.NoError(t, err)
	require.True(t, unused(), "a long unused access alone keeps nothing")

	foreign := testpkg.CreateTestHolidayRegistration(t, db, other, otherSchool, "Herbstferien", account.Email, "Lia")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET parent_account_id = ? WHERE id = ?`, account.ID, foreign.RegistrationID)
	require.NoError(t, err)
	assert.True(t, unused(), "a registration at another Träger keeps no access at this one")

	unclaimed := testpkg.CreateTestHolidayRegistration(t, db, own, ownSchool, "Winterferien", strings.ToUpper(account.Email), "Ben")
	assert.False(t, unused(), "a registration of the account's address at the Träger keeps it until the sign-in claims it")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET parent_account_id = ? WHERE id = ?`, account.ID, unclaimed.RegistrationID)
	require.NoError(t, err)
	assert.False(t, unused(), "a registration linked to the account keeps it")

	_, err = db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registrations WHERE id = ?`, unclaimed.RegistrationID)
	require.NoError(t, err)
	assert.True(t, unused(), "once the registration went with its retention, the account follows")
}
