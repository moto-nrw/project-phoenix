//go:build testhooks

package settingshttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSettingsSchemaCascadeAndIsolation(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	token := testutil.MintTestJWT(t, f.admin)
	rights := []string{"traeger_overview:read", "traeger_settings:manage"}
	key := records.KeyParentNewsEnabled
	foreign := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{Name: "Foreign settings", Slug: fmt.Sprintf("foreign-settings-%d", f.account), Active: true})
	require.NoError(t, f.module.Traeger.SetValue(context.Background(), foreign.ID, f.account, rights, key, false))
	stages := []struct {
		name      string
		apply     func()
		value     bool
		source    string
		isDefault bool
	}{
		{"default", func() {}, true, "default", true},
		{"traeger", func() {
			require.NoError(t, f.module.Traeger.SetValue(context.Background(), f.org, f.account, rights, key, false))
		}, false, "traeger", true},
		{"school equal registry default", func() {
			require.NoError(t, f.module.TenantSettings.SetValue(testpkg.Ctx(t), testpkg.Tenant(t), f.account, f.admin.Permissions, key, true))
		}, true, "school", false},
		{"school reset", func() {
			require.NoError(t, f.module.TenantSettings.ResetValue(testpkg.Ctx(t), testpkg.Tenant(t), f.account, f.admin.Permissions, key))
		}, false, "traeger", true},
		{"traeger reset", func() {
			require.NoError(t, f.module.Traeger.ResetValue(context.Background(), f.org, f.account, rights, key))
		}, true, "default", true},
	}
	for _, stage := range stages {
		stage.apply()
		t.Log(stage.name)
		for _, route := range []struct{ portal, path, token string }{
			{"tenant", "/api/settings/schema", token},
			{"operator", fmt.Sprintf("/operator/schools/%d/settings/schema", testpkg.Tenant(t)), f.token},
		} {
			items := assertBudgetSchema(t, settingsBudgetData(t, f.get(t, route.path, route.token)), route.portal)
			require.Equal(t, stage.value, items[key].Value)
			require.EqualValues(t, stage.source, items[key].Source)
			require.Equal(t, stage.isDefault, items[key].IsDefault)
			require.Equal(t, stage.source != "default", items[key].TraegerChangedAt != nil)
			f.noSettingsDML(t, route.portal == "operator")
		}
	}
	// A schema read must retain hidden dependencies rather than persisting a
	// projected value or exposing operator-only fields to the tenant.
	require.NoError(t, f.module.TenantSettings.SetValue(testpkg.Ctx(t), testpkg.Tenant(t), f.account, f.admin.Permissions, records.KeyParentNotesEnabled, false))
	items := assertBudgetSchema(t, settingsBudgetData(t, f.get(t, "/api/settings/schema", token)), "tenant")
	require.False(t, items[records.KeyParentMessageStaffNameVisible].Visible)
	f.noSettingsDML(t, false)

	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, f.db, foreign.ID, "Foreign school")
	foreignClaims := f.admin
	foreignClaims.TenantID = foreignSchool
	items = assertBudgetSchema(t, settingsBudgetData(t, f.get(t, "/api/settings/schema", testutil.MintTestJWT(t, foreignClaims))), "tenant")
	require.Equal(t, false, items[key].Value, "the other school inherits only its own Träger")
	require.True(t, items[records.KeyParentNotesEnabled].Value.(bool), "the original school's override cannot leak")
	f.noSettingsDML(t, false)
}

func TestSettingsGETPortalAndPermissionBoundaries(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	paths := []string{"/api/settings/schema", "/api/settings/home-layout", "/api/settings/payroll-status",
		fmt.Sprintf("/operator/schools/%d/settings/schema", testpkg.Tenant(t)), "/traeger/settings/schema"}
	for _, path := range paths {
		for _, scope := range []string{"school", "parent", "caterer"} {
			claims := f.admin
			claims.Scope = scope
			rec := f.get(t, path, testutil.MintTestJWT(t, claims))
			status := http.StatusUnauthorized
			if path == paths[3] {
				status = http.StatusForbidden
			}
			require.Equal(t, status, rec.Code, rec.Body.String())
			if status == http.StatusForbidden {
				assertSettingsProblem(t, rec.Body.Bytes(), status, "general.permission", "This endpoint requires operator authentication")
			} else {
				assertSettingsProblem(t, rec.Body.Bytes(), status, "general.permission")
			}
			require.Empty(t, f.counter.Queries(), "wrong portal must be rejected before a DB frame")
		}
		rec := f.get(t, path, "")
		require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
		assertSettingsProblem(t, rec.Body.Bytes(), http.StatusUnauthorized, "general.permission")
		require.Empty(t, f.counter.Queries())
	}
	reader := f.admin
	reader.IsAdmin = false
	reader.Permissions = []string{"config:read"}
	rec := f.get(t, "/api/settings/payroll-status", testutil.MintTestJWT(t, reader))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assertSettingsProblem(t, rec.Body.Bytes(), http.StatusForbidden, "general.permission")
	require.Empty(t, f.counter.Queries())
	reader.Permissions = nil
	rec = f.get(t, "/api/settings/schema", testutil.MintTestJWT(t, reader))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assertSettingsProblem(t, rec.Body.Bytes(), http.StatusForbidden, "general.permission")
	require.Empty(t, f.counter.Queries())
	reader.Scope, reader.OrgID, reader.TenantID = "org", f.org, 0
	rec = f.get(t, "/traeger/settings/schema", testutil.MintTestJWT(t, reader))
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	assertSettingsProblem(t, rec.Body.Bytes(), http.StatusForbidden, "general.permission")
	require.Empty(t, f.counter.Queries())
	reader.Permissions = []string{"traeger_overview:read"}
	items := assertBudgetSchema(t, settingsBudgetData(t, f.get(t, "/traeger/settings/schema", testutil.MintTestJWT(t, reader))), "traeger")
	for _, item := range items {
		require.False(t, item.Writable, "read-only Träger session")
	}
	f.noSettingsDML(t, false)
}

func assertSettingsProblem(t *testing.T, body []byte, status int, code string, detail ...string) {
	t.Helper()
	message := http.StatusText(status)
	if len(detail) > 0 {
		message = detail[0]
	}
	if status == http.StatusUnauthorized {
		require.JSONEq(t, `{"status":401,"detail":"token unauthorized","code":"general.permission"}`, string(body))
	} else {
		anchor := "anleitung-zugriff-pruefen"
		if status == http.StatusInternalServerError {
			anchor = "anleitung-unerwarteter-fehler"
		}
		if status == http.StatusServiceUnavailable {
			anchor = "anleitung-erneut-versuchen"
		}
		want, err := json.Marshal(map[string]any{"type": "https://moto-app.de/help/fehlermeldungen#" + anchor, "title": http.StatusText(status), "status": status, "detail": message, "instance": "", "code": code})
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(body))
	}
	t.Logf("refusal body=%s", body)
}

func TestSettingsGETCancellationPreservesReadOnlyFailure(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	token := testutil.MintTestJWT(t, f.admin)
	traeger := f.admin
	traeger.Scope, traeger.OrgID, traeger.TenantID = "org", f.org, 0
	traeger.Permissions = []string{"traeger_overview:read"}
	traegerToken := testutil.MintTestJWT(t, traeger)
	for _, path := range []string{"/api/settings/schema", "/api/settings/home-layout", "/api/settings/payroll-status", "/traeger/settings/schema"} {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		requestToken := token
		if path == "/traeger/settings/schema" {
			requestToken = traegerToken
		}
		req := testutil.NewRequest(http.MethodGet, path, nil, testutil.WithJWTBearer(requestToken)).WithContext(ctx)
		f.counter.Reset()
		rec := testutil.ExecuteRequestForTest(t, f.router, req)
		if path == "/traeger/settings/schema" {
			// The service carries the cancellation: the client went away,
			// so 499 instead of a server error.
			require.Equal(t, 499, rec.Code, rec.Body.String())
			require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Client Closed Request","status":499,"detail":"settings service error in traeger_schema: unit of work: start transaction: context canceled","instance":"","code":"general.unavailable"}`, rec.Body.String())
		} else {
			// The request transaction cannot begin for a client that went
			// away: 499 instead of a server error.
			require.Equal(t, 499, rec.Code, rec.Body.String())
			require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-gerade-nicht-erreichbar","title":"Client Closed Request","status":499,"detail":"unit of work: start transaction: context canceled","instance":"","code":"general.unavailable"}`, rec.Body.String())
		}
		f.noSettingsDML(t, false)
		t.Logf("canceled path=%s SQL=%q", path, f.counter.Queries())
	}
}
