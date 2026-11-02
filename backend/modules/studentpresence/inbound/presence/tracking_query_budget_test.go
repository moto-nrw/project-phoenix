package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	tenantsettings "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

func init() { testutil.SeedTestJWTConfig() }

// TrackingSettings is the settings service the budget test seeds and the
// tracking route reads.
type TrackingSettings interface {
	Settings
	SetValue(ctx context.Context, key string, value any, changedBy *int64, userPermissions []string) error
}

// ComposeTrackingSettings composes the root's settings service over db. The
// root imports this package, so these internal tests cannot import it;
// root_composition_test.go binds it.
var ComposeTrackingSettings func(t *testing.T, db *bun.DB) TrackingSettings

// TestTrackingIndicatorsIssuesOneSettingValuesQuery drives the REAL router
// (ProtectedTenantGroup chain, so the request cache middleware is attached the
// way production attaches it) and asserts the four settings the handler reads
// (toggle + three labels) cost exactly one config.setting_values query
// (issue #2065).
func TestTrackingIndicatorsIssuesOneSettingValuesQuery(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	settings := ComposeTrackingSettings(t, db)

	seedCtx := tenant.WithTenantID(context.Background(), tenantID)
	require.NoError(t, settings.SetValue(seedCtx, tenantsettings.KeyTrackingIndicatorsEnabled, true, nil, nil))
	require.NoError(t, settings.SetValue(seedCtx, tenantsettings.KeyTrackingIndicator1, "Bibliothek", nil, nil))
	// Slot 2 defaults to "Hausaufgaben" since #3738; the explicit empty
	// value keeps this test on the skipped-empty-slot path.
	require.NoError(t, settings.SetValue(seedCtx, tenantsettings.KeyTrackingIndicator2, "", nil, nil))

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Budget", "Test", "1a")

	active := &stubPresenceOperations{
		trackingIndicators: func(_ context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			result := make(map[int64][]bool, len(studentIDs))
			for _, id := range studentIDs {
				result[id] = make([]bool, len(labels))
			}
			return result, nil
		},
	}
	// This route does not read visits; an unexpected presence call must fail.
	rs := NewResource(active, nil, nil, nil, settings, scopeTestTenantRoutes(testpkg.TenantSessionGatesWithCaches(scopeTestSettingsCache)), slog.Default(), struct{ PresenceQueries }{}, requestRuntimeForTest(func(context.Context, int64, int64) context.Context { panic("tracking must not bind staff") }), authorizationForTest(), tenantsettings.NewDisplaySettings(settings))
	router := rs.Router()

	body, err := json.Marshal(map[string]any{"student_ids": []int64{student.ID}})
	require.NoError(t, err)

	counter := testpkg.CaptureQueries(t, db)

	req := httptest.NewRequest("POST", "/tracking-indicators", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := testutil.ExecuteWithAuth(t, router, req, testutil.AdminTestClaimsForTenant(1, tenantID))
	queries := counter.Selects("config.setting_values")

	require.Equal(t, testutil.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			Labels []string `json:"labels"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, []string{"Bibliothek"}, resp.Data.Labels,
		"empty label keys must still be skipped by the per-key reads")
	// Toggle + three label keys must share one config.setting_values query.
	querybudgets.AssertQueryBudget(t, "api.active.tracking_indicators.setting_values", queries)
}
