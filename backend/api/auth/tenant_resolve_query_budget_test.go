// Query-budget guard for the public GET /auth/tenant/resolve endpoint
// (issue #2065): the shell-settings batch prefetches every key the handler
// needs — including enrollment.grade_level_max for the separate hard-fail
// resolve — so one request issues exactly ONE config.setting_values query and
// zero additional tenant transactions for settings.
package auth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type settingValuesQueryCounter struct {
	count atomic.Int32
}

func (c *settingValuesQueryCounter) BeforeQuery(ctx context.Context, event *bun.QueryEvent) context.Context {
	query := strings.ToLower(event.Query)
	if strings.HasPrefix(strings.TrimSpace(query), "select") &&
		strings.Contains(query, "config.setting_values") {
		c.count.Add(1)
	}
	return ctx
}

func (*settingValuesQueryCounter) AfterQuery(context.Context, *bun.QueryEvent) {}

func TestResolveTenant_IssuesOneSettingValuesQuery(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	db, authRoute := setupAuthDependenciesRoute(t)
	_, slug := newTenantResolveScope(t, db)

	schoolRepo := platformRepo.NewSchoolRepository(db)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.InvitationService, platformSvc.NewSchoolService(schoolRepo), db)
	resource.SettingsService = authRoute.SettingsService

	// The blanket attachment in api/base.go is what production requests run
	// through; mirror it here so the request cache is present.
	router := chi.NewRouter()
	router.Use(apiCommon.RequestSettingsCacheMiddleware)
	router.Mount("/auth", resource.Router())

	counter := &settingValuesQueryCounter{}
	db.AddQueryHook(counter)

	req := httptest.NewRequest("GET", "/auth/tenant/resolve?slug="+slug, nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	queries := counter.count.Load()

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	var resp resolveResp
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 4, resp.Data.GradeLevelMax,
		"grade cap must still resolve correctly through the batch + cache path")
	assert.Equal(t, int32(1), queries,
		"tenant resolve must load config.setting_values exactly once per request")
}
