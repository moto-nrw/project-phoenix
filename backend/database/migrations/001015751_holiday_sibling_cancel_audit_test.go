package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHolidaySiblingCancelAuditKeepsEveryAuditAction pins #5662: the
// migration redefines the CHECK of the audit actions of the holiday
// registrations once more. After it the trail takes every action of
// 1.15.746 and sibling_cancel; its down migration removes only
// sibling_cancel.
func TestHolidaySiblingCancelAuditKeepsEveryAuditAction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Geschwister Storno", Slug: fmt.Sprintf("ferien-geschwister-storno-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)

	var period, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Sommerferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now()) RETURNING id`, organization, period).Scan(ctx, &registration))
	record := func(action string) error {
		var named *int64
		if action != "list" && action != "export" {
			named = &registration
		}
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_audit
  (organization_id, holiday_period_id, holiday_registration_id, action) VALUES (?, ?, ?, ?)`,
			organization, period, named, action)
		return err
	}
	before := append(append([]string{}, holidayAuditActionsBefore1_15_746...), "sibling_merge", "sibling_split")

	for _, action := range append(append([]string{}, before...), "sibling_cancel") {
		assert.NoError(t, record(action), "after the migration the trail takes %q", action)
	}
	assert.Error(t, record("sibling_join"), "an unknown action stays refused")

	require.NoError(t, holidaySiblingCancelAuditDown(ctx, db))
	defer func() { require.NoError(t, holidaySiblingCancelAuditUp(ctx, db)) }()
	for _, action := range before {
		assert.NoError(t, record(action), "the down migration keeps %q", action)
	}
	assert.Error(t, record("sibling_cancel"), "the down migration removes sibling_cancel")
}
