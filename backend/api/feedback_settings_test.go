package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffFeedbackRoutesRespectSchoolSetting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module, config, err := NewFeedbackTestModuleWithSettings(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	setEnabled := func(enabled bool) {
		require.NoError(t, config.Settings.SetValue(ctx, settings.KeyFeedbackEnabled, enabled, nil, nil))
	}
	student := testpkg.CreateTestStudent(t, db, "Feedback", "Setting", "1a")
	setEnabled(true)
	entry, err := module.Feedback.Submit(ctx, feedback.CreateEntry{
		Value: feedback.ValuePositive, Day: "2026-08-31", Time: "12:34:56", StudentID: student.ID,
	})
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/feedback", newFeedbackResource(module.Feedback).Router())
	_, account := testpkg.CreateTestStaffWithAccount(t, db, "Feedback", "Staff")
	token := testutil.MintTestJWT(t, testutil.AdminTestClaimsForTenant(int(account.ID), testpkg.Tenant(t)))
	body := map[string]any{"value": "positive", "day": "2026-08-31", "time": "12:34:56", "student_id": student.ID}
	routes := []struct {
		method, path string
		body         any
		status       int
	}{
		{http.MethodPost, "/feedback/", body, http.StatusCreated},
		{http.MethodPost, "/feedback/batch", map[string]any{"entries": []any{body}}, http.StatusCreated},
		{http.MethodGet, "/feedback/", nil, http.StatusOK},
		{http.MethodGet, fmt.Sprintf("/feedback/%d", entry.ID), nil, http.StatusOK},
		{http.MethodGet, fmt.Sprintf("/feedback/student/%d", student.ID), nil, http.StatusOK},
		{http.MethodGet, "/feedback/date/2026-08-31", nil, http.StatusOK},
		{http.MethodGet, "/feedback/mensa", nil, http.StatusOK},
		{http.MethodGet, "/feedback/date-range?start_date=2026-08-01&end_date=2026-08-31", nil, http.StatusOK},
	}
	states := []struct {
		name   string
		status int
		code   string
	}{
		{"registry default", http.StatusForbidden, "feedback.disabled"},
		{"disabled", http.StatusForbidden, "feedback.disabled"},
		{"unreadable setting", http.StatusInternalServerError, "general.server"},
		{"enabled", http.StatusOK, ""},
	}
	for _, state := range states {
		switch state.name {
		case "registry default":
			require.NoError(t, config.Settings.ResetValue(ctx, settings.KeyFeedbackEnabled, nil, nil))
		case "disabled":
			setEnabled(false)
		case "unreadable setting":
			// Simulate a corrupt persisted value that the typed resolver cannot read.
			_, err = db.NewRaw(`UPDATE config.setting_values SET value = '"unreadable"'::jsonb WHERE tenant_id = ? AND setting_key = ?`, testpkg.Tenant(t), settings.KeyFeedbackEnabled).Exec(ctx)
			require.NoError(t, err)
		case "enabled":
			setEnabled(true)
		}
		for _, route := range routes {
			request := testutil.NewAuthenticatedRequest(t, route.method, route.path, route.body, testutil.WithJWTBearer(token))
			response := testutil.ExecuteRequest(router, request)
			if state.code == "" {
				assert.Equal(t, route.status, response.Code, "%s %s: %s", route.method, route.path, response.Body.String())
				continue
			}
			assert.Equal(t, state.status, response.Code, "%s: %s %s", state.name, route.method, route.path)
			var problem struct{ Code string }
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &problem))
			assert.Equal(t, state.code, problem.Code, "%s: %s %s", state.name, route.method, route.path)
		}
		if state.code != "" {
			setEnabled(true)
			count, countErr := module.Feedback.CountForStudent(ctx, student.ID)
			require.NoError(t, countErr)
			assert.Equal(t, 1, count, "refused single and batch submissions must not persist")
		}
	}
	setEnabled(false)
	request := testutil.NewAuthenticatedRequest(t, http.MethodDelete, fmt.Sprintf("/feedback/%d", entry.ID), nil, testutil.WithJWTBearer(token))
	response := testutil.ExecuteRequest(router, request)
	assert.Equal(t, http.StatusOK, response.Code, "switching feedback off must not prevent erasure")
}
