package operator_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// putModule switches a module of school tenantID through the operator route.
func putModule(t *testing.T, ctx *operatorSettingsTestContext, tenantID int64, key string, booked bool) map[string]any {
	t.Helper()
	path := fmt.Sprintf("/schools/%d/settings/values/%s", tenantID, key)
	rr := testutil.ExecuteRequest(ctx.router, newOperatorRequest(t, http.MethodPut, path, map[string]any{"value": booked}))
	if rr.Code == http.StatusOK {
		return nil
	}
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	return testutil.ParseJSONResponse(t, rr.Body.Bytes())
}

// The moto team books each module per school (#4892): switching one school
// leaves every other school's booking unchanged when the settings service
// resolves it outside the request, as jobs and portals will (#4895).
func TestOperatorBooksModulesPerSchoolWithoutCrosstalk(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, ctx.db)
	background := context.Background()

	require.Nil(t, putModule(t, ctx, schoolA, "module.zeiterfassung", false))

	bookedA, err := ctx.settings.ResolveBoolForTenant(background, schoolA, "module.zeiterfassung")
	require.NoError(t, err)
	assert.False(t, bookedA, "school A cancelled Zeiterfassung")
	bookedB, err := ctx.settings.ResolveBoolForTenant(background, schoolB, "module.zeiterfassung")
	require.NoError(t, err)
	assert.True(t, bookedB, "school B keeps the registry default")

	count, err := ctx.db.NewSelect().TableExpr("config.setting_values").
		Where("tenant_id = ?", schoolB).Where("setting_key LIKE 'module.%'").Count(background)
	require.NoError(t, err)
	assert.Zero(t, count, "a booking of school A writes nothing for school B")
}

// Prerequisites hold in both directions (#4892): booking a module without the
// module it requires and cancelling a prerequisite a booked module needs are
// both refused with their own code. A reset restores "booked" and is checked
// as a booking.
func TestOperatorModulePrerequisitesHoldInBothDirections(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)
	school := testpkg.Tenant(t)

	refused := putModule(t, ctx, school, "module.online_anmeldung", false)
	require.NotNil(t, refused, "Ferienbetreuung is booked by default and needs Online-Anmeldung")
	assert.Equal(t, "settings.module_required_by", refused["code"])
	details, _ := refused["details"].(map[string]any)
	assert.Equal(t, "Online-Anmeldung", details["module"])
	assert.Equal(t, "Ferienbetreuung", details["dependents"])

	require.Nil(t, putModule(t, ctx, school, "module.ferienbetreuung", false))
	require.Nil(t, putModule(t, ctx, school, "module.online_anmeldung", false))

	refused = putModule(t, ctx, school, "module.ferienbetreuung", true)
	require.NotNil(t, refused, "Ferienbetreuung cannot be booked without Online-Anmeldung")
	assert.Equal(t, "settings.module_requires", refused["code"])
	details, _ = refused["details"].(map[string]any)
	assert.Equal(t, "Ferienbetreuung", details["module"])
	assert.Equal(t, "Online-Anmeldung", details["required"])

	reset := testutil.ExecuteRequest(ctx.router, newOperatorRequest(t, http.MethodDelete, schoolPath(t, "/settings/values/module.ferienbetreuung"), nil))
	require.Equal(t, http.StatusBadRequest, reset.Code, "a reset books the module again and needs the prerequisite")

	require.Nil(t, putModule(t, ctx, school, "module.online_anmeldung", true))
	require.Nil(t, putModule(t, ctx, school, "module.ferienbetreuung", true))

	refused = putModule(t, ctx, school, "module.vertraege", false)
	require.NotNil(t, refused, "Elternbeiträge is booked by default and needs Verträge")
	assert.Equal(t, "settings.module_required_by", refused["code"])
}

// Every module switch is recorded in the settings audit (#4892).
func TestOperatorModuleSwitchIsAudited(t *testing.T) {
	t.Parallel()
	ctx := setupOperatorSettingsRoute(t)
	school := testpkg.Tenant(t)

	require.Nil(t, putModule(t, ctx, school, "module.personalakte", false))
	require.Nil(t, putModule(t, ctx, school, "module.personalakte", true))

	var entries []struct {
		NewValue string `bun:"new_value"`
		Action   string `bun:"action"`
	}
	require.NoError(t, ctx.db.NewSelect().TableExpr("config.setting_audit").
		ColumnExpr("new_value::text AS new_value, action").
		Where("tenant_id = ?", school).Where("setting_key = ?", "module.personalakte").
		OrderExpr("id").Scan(context.Background(), &entries))
	require.Len(t, entries, 2)
	assert.Equal(t, "false", entries[0].NewValue)
	assert.Equal(t, "true", entries[1].NewValue)
}
