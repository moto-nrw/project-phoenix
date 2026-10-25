package settingshttp_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// retentionWindowKeys are the retention windows the moto team records per
// school (ADR 0048): the school reads them and never writes them.
var retentionWindowKeys = []string{
	"gdpr.privacy_consent_retention_days",
	"gdpr.staff_message_retention_days",
	"gdpr.time_tracking_retention_days",
	"gdpr.student_change_log_retention_days",
	"gdpr.pwa_usage_retention_days",
	"gdpr.timetable_retention_days",
	"feedback.data_retention_days",
	"enrollment.rejected_retention_days",
	"gdpr.status_day_retention_days",
	"gdpr.attendance_retention_days",
	"gdpr.guardian_retention_days",
	"gdpr.parent_message_retention_days",
	"gdpr.withdrawn_enrollment_retention_days",
	"gdpr.staff_sick_note_retention_days",
	"gdpr.student_retention_days",
}

func schemaItems(t *testing.T, ctx *settingsTestContext) map[string]map[string]any {
	t.Helper()
	req := testutil.NewAuthenticatedRequest(t, "GET", "/schema", nil, testutil.WithTestTenant(t))
	rr := testutil.ExecuteRequest(ctx.resource.SettingsRouter(), req)
	testutil.AssertSuccessResponse(t, rr, 200)

	items := map[string]map[string]any{}
	data, _ := testutil.ParseJSONResponse(t, rr.Body.Bytes())["data"].(map[string]any)
	tabs, _ := data["tabs"].([]any)
	for _, tab := range tabs {
		categories, _ := tab.(map[string]any)["categories"].([]any)
		for _, category := range categories {
			entries, _ := category.(map[string]any)["items"].([]any)
			for _, entry := range entries {
				item := entry.(map[string]any)
				items[item["key"].(string)] = item
			}
		}
	}
	return items
}

func TestSettingsSchemaShowsRetentionWindowsReadOnly(t *testing.T) {
	t.Parallel()
	items := schemaItems(t, setupSettingsModule(t))

	for _, key := range retentionWindowKeys {
		item, ok := items[key]
		require.True(t, ok, "the school sees its retention window %s", key)
		assert.Equal(t, false, item["writable"], "%s is read-only for the school", key)
		assert.Equal(t, "operator_managed", item["access_policy"], key)
	}
	_, visible := items["gdpr.data_cleanup_enabled"]
	assert.False(t, visible, "the school cannot see or switch off the nightly deletion")
	require.Contains(t, items, "gdpr.data_cleanup_time")
	assert.Equal(t, true, items["gdpr.data_cleanup_time"]["writable"], "the school keeps choosing the time of the nightly deletion")
}

func TestSettingsSchoolCannotWriteRetentionWindowsOrTheDeletionSwitch(t *testing.T) {
	t.Parallel()
	ctx := setupSettingsModule(t)
	router := ctx.resource.SettingsRouter()

	writes := map[string]any{"gdpr.data_cleanup_enabled": false}
	for _, key := range retentionWindowKeys {
		writes[key] = 365
	}
	for key, value := range writes {
		put := testutil.NewAuthenticatedRequest(t, "PUT", "/values/"+key, map[string]any{"value": value}, testutil.WithTestTenant(t))
		testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, put), 403)
		reset := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/"+key, nil, testutil.WithTestTenant(t))
		testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, reset), 403)
	}

	count, err := ctx.db.NewSelect().
		Table("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key LIKE '%retention_days' OR setting_key = 'gdpr.data_cleanup_enabled'").
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "a refused write must not store an override")

	put := testutil.NewAuthenticatedRequest(t, "PUT", "/values/gdpr.data_cleanup_time", map[string]any{"value": "03:30"}, testutil.WithTestTenant(t))
	testutil.AssertSuccessResponse(t, testutil.ExecuteRequest(router, put), 200)
}

// The school sees its federal state read-only (#4775): the moto team sets
// it, because it decides the statutory holidays of the Arbeitszeitkonto,
// the Dienstplan and the Regeltermine.
func TestSettingsSchoolSeesTheFederalStateButCannotWriteIt(t *testing.T) {
	t.Parallel()
	ctx := setupSettingsModule(t)
	items := schemaItems(t, ctx)
	item, ok := items["operations.federal_state"]
	require.True(t, ok, "the school sees its federal state")
	assert.Equal(t, false, item["writable"])
	assert.Equal(t, "operator_managed", item["access_policy"])
	assert.Equal(t, "DE-NW", item["value"])

	router := ctx.resource.SettingsRouter()
	put := testutil.NewAuthenticatedRequest(t, "PUT", "/values/operations.federal_state", map[string]any{"value": "DE-BY"}, testutil.WithTestTenant(t))
	testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, put), 403)
	reset := testutil.NewAuthenticatedRequest(t, "DELETE", "/values/operations.federal_state", nil, testutil.WithTestTenant(t))
	testutil.AssertErrorResponse(t, testutil.ExecuteRequest(router, reset), 403)

	count, err := ctx.db.NewSelect().
		Table("config.setting_values").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("setting_key = 'operations.federal_state'").
		Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count, "a refused write must not store an override")
}
