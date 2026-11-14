package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidayHealthRequirementMigrationKeepsExistingFormsOptional pins
// #6686: an existing form version keeps allergies and medication optional,
// and the CHECK takes only the two settings.
func TestHolidayHealthRequirementMigrationKeepsExistingFormsOptional(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Gesundheit", Slug: fmt.Sprintf("ferien-gesundheit-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	require.NoError(t, holidayHealthRequirementDown(ctx, db))

	var period int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Weihnachtsferien', '2026-11-16', '2026-12-01', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))

	require.NoError(t, holidayHealthRequirementUp(ctx, db))

	var optional bool
	require.NoError(t, db.NewRaw(`SELECT bool_and(health_requirement = 'optional') FROM enrollment.holiday_period_forms
  WHERE holiday_period_id = ?`, period).Scan(ctx, &optional))
	assert.True(t, optional, "existing versions keep the health fields optional")
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_period_forms SET health_requirement = 'off' WHERE holiday_period_id = ?`, period)
	assert.ErrorContains(t, err, "chk_holiday_period_forms_health_requirement")
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_period_forms SET health_requirement = 'required' WHERE holiday_period_id = ?`, period)
	require.NoError(t, err)
}
