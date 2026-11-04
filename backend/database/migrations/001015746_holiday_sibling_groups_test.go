package migrations

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holidayAuditActionsBefore1_15_746 is the list of audit actions of
// 1.15.745: every action of the migrations before, edit (1.15.741) and
// proof_upload (1.15.745) included.
var holidayAuditActionsBefore1_15_746 = []string{
	"list", "read", "export", "confirm", "cancel", "rebook", "freeze", "reopen", "proof", "health", "debtor_number",
	"edit", "proof_upload",
}

// TestHolidaySiblingGroupsMigrationKeepsEveryAuditAction pins #5628: the
// migration redefines the CHECK of the audit actions of the holiday
// registrations, which 1.15.741 and 1.15.745 redefined before. After it the
// trail takes every earlier action and the two sibling actions; its down
// migration removes only the sibling actions.
func TestHolidaySiblingGroupsMigrationKeepsEveryAuditAction(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Ferien Geschwister", Slug: fmt.Sprintf("ferien-geschwister-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerContributionRules(t, db, organization)

	var period, registration int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods
  (organization_id, name, registration_opens_on, registration_closes_on, status, published_at)
  VALUES (?, 'Sommerferien', '2026-01-01', '2099-12-31', 'published', now()) RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations
  (organization_id, holiday_period_id, guardian_first_name, guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
  VALUES (?, ?, 'Eva', 'Muster', 'eva@example.org', '0151 2345678', now()) RETURNING id`, organization, period).Scan(ctx, &registration))
	// A list or an export names the period, every other action one
	// registration.
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

	for _, action := range append(append([]string{}, holidayAuditActionsBefore1_15_746...), "sibling_merge", "sibling_split") {
		assert.NoError(t, record(action), "after the migration the trail takes %q", action)
	}
	assert.Error(t, record("sibling_join"), "an unknown action stays refused")

	_, err := db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registration_audit WHERE action IN ('sibling_merge', 'sibling_split')`)
	require.NoError(t, err)
	require.NoError(t, holidaySiblingGroupsDown(ctx, db))
	defer func() { require.NoError(t, holidaySiblingGroupsUp(ctx, db)) }()
	for _, action := range holidayAuditActionsBefore1_15_746 {
		assert.NoError(t, record(action), "the down migration keeps %q", action)
	}
	assert.Error(t, record("sibling_merge"), "the down migration removes the sibling actions")
}
