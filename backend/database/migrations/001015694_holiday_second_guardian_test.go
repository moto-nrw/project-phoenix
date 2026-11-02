package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHolidaySecondGuardianMigrationPreservesPublishedPeriodsAndRegistrations(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferienmigration", Slug: fmt.Sprintf("ferien-migration-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	require.NoError(t, holidaySecondGuardianDown(ctx, db))
	var period, registration int64
	var token string
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Winterferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id, public_token`, organization).Scan(ctx, &period, &token))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now()) RETURNING id`, organization, period).Scan(ctx, &registration))
	require.NoError(t, holidaySecondGuardianUp(ctx, db))
	var preserved bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM enrollment.holiday_registrations r
  JOIN enrollment.holiday_periods p ON p.id = r.holiday_period_id AND p.organization_id = r.organization_id
  WHERE r.id = ? AND p.public_token = ? AND p.status = 'published' AND r.guardian_email = 'eva@example.org'
   AND r.second_guardian_first_name IS NULL AND r.second_guardian_last_name IS NULL
   AND r.second_guardian_email IS NULL AND r.second_guardian_phone IS NULL)`, registration, token).Scan(ctx, &preserved))
	assert.True(t, preserved, "the existing public link and registration are unchanged")
	for _, names := range [][2]any{{"Tom", nil}, {nil, "Muster"}, {" ", "Muster"}, {"Tom", " "}} {
		_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET second_guardian_first_name = ?,
   second_guardian_last_name = ? WHERE id = ?`, names[0], names[1], registration)
		assert.ErrorContains(t, err, "chk_holiday_registrations_second_guardian")
	}
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET second_guardian_email = 'tom@example.org' WHERE id = ?`, registration)
	assert.ErrorContains(t, err, "chk_holiday_registrations_second_guardian", "contact details without names are refused")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET second_guardian_first_name = 'Tom',
  second_guardian_last_name = 'Muster' WHERE id = ?`, registration)
	require.NoError(t, err, "a second guardian needs no e-mail or phone")
}
