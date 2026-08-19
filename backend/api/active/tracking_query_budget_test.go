package active

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	configRepository "github.com/moto-nrw/project-phoenix/database/repositories/config"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type trackingSettingValuesCounter struct {
	count atomic.Int32
}

func (c *trackingSettingValuesCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if strings.HasPrefix(strings.TrimSpace(query), "select") &&
		strings.Contains(query, "config.setting_values") {
		c.count.Add(1)
	}
	return ctx
}

func (*trackingSettingValuesCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

// JWT-safe tenant IDs: UniqueTestTenantID values exceed 2^53 and corrupt when
// the tenant_id claim round-trips through JSON, so JWT-driving tests use a
// small offset counter (same pattern as api/rooms/system_filter_test.go).
var trackingBudgetTenantCounter int64 = 890_000 + time.Now().UnixNano()%100_000

// TestTrackingIndicatorsIssuesOneSettingValuesQuery drives the REAL router
// (ProtectedTenantGroup chain, so the request cache middleware is attached the
// way production attaches it) and asserts the four settings the handler reads
// (toggle + three labels) cost exactly one config.setting_values query
// (issue #2065).
// Deliberately NOT parallel: the test installs a query hook on the SHARED
// package pool and asserts a query budget, so any test running beside it is
// counted too.
func TestTrackingIndicatorsIssuesOneSettingValuesQuery(t *testing.T) {
	testutil.SeedTestJWTConfig()
	db := testpkg.SetupTestDB(t)

	tenantID := atomic.AddInt64(&trackingBudgetTenantCounter, 1)
	testpkg.EnsureTestTenant(t, db, tenantID)
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_audit WHERE tenant_id = ?`, tenantID)
		_, _ = db.ExecContext(ctx, `DELETE FROM config.setting_values WHERE tenant_id = ?`, tenantID)
	})

	valueRepo := configRepository.NewSettingValueRepository(db)
	auditRepo := configRepository.NewSettingAuditRepository(db)
	settings := configSvc.NewSettingsService(valueRepo, auditRepo, nil, db, slog.Default())

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

	counter := &trackingSettingValuesCounter{}
	db.AddQueryHook(counter)

	req := httptest.NewRequest("POST", "/tracking-indicators", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := testutil.ExecuteWithAuth(t, router, req, testutil.AdminTestClaimsForTenant(1, tenantID))
	queries := counter.count.Load()

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp struct {
		Data struct {
			Labels []string `json:"labels"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, []string{"Bibliothek"}, resp.Data.Labels,
		"empty label keys must still be skipped by the per-key reads")
	assert.Equal(t, int32(1), queries,
		"toggle + three label keys must share one config.setting_values query")
}
