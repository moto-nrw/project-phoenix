package active

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() { testutil.SeedTestJWTConfig() }

// TestTrackingIndicatorsIssuesOneSettingValuesQuery drives the REAL router
// (ProtectedTenantGroup chain, so the request cache middleware is attached the
// way production attaches it) and asserts the four settings the handler reads
// (toggle + three labels) cost exactly one config.setting_values query
// (issue #2065).
func TestTrackingIndicatorsIssuesOneSettingValuesQuery(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db := testpkg.SetupIsolatedTestDB(t)

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_audit WHERE tenant_id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ?`, tenantID)
	})

	valueRepo := configRepository.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	auditRepo := configRepository.NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	settings := configSvc.NewSettingsService(valueRepo, auditRepo, nil, testpkg.SettingsRuntime(t, db), slog.Default())

	seedCtx := tenant.WithTenantID(context.Background(), tenantID)
	require.NoError(t, settings.SetValue(seedCtx, configModel.KeyTrackingIndicatorsEnabled, true, nil, nil))
	require.NoError(t, settings.SetValue(seedCtx, configModel.KeyTrackingIndicator1, "Bibliothek", nil, nil))

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Budget", "Test", "1a")

	active := &trackingMockActiveService{
		getTrackingIndicatorsFunc: func(_ context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			result := make(map[int64][]bool, len(studentIDs))
			for _, id := range studentIDs {
				result[id] = make([]bool, len(labels))
			}
			return result, nil
		},
	}
	rs := NewResource(active, nil, nil, nil, nil, settings, db, slog.Default())
	router := rs.Router()

	body, err := json.Marshal(map[string]any{"student_ids": []int64{student.ID}})
	require.NoError(t, err)

	counter := testpkg.CaptureQueries(t, db)

	req := httptest.NewRequest("POST", "/tracking-indicators", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := testutil.ExecuteWithAuth(t, router, req, testutil.AdminTestClaimsForTenant(1, tenantID))
	queries := counter.Selects("config.setting_values")

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			Labels []string `json:"labels"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, []string{"Bibliothek"}, resp.Data.Labels,
		"empty label keys must still be skipped by the per-key reads")
	// Toggle + three label keys must share one config.setting_values query.
	testpkg.AssertQueryBudget(t, "api.active.tracking_indicators.setting_values", queries)
}
