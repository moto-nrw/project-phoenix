//go:build testhooks

package settingshttp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// These are resource-router budgets, not Serve-host/latency measurements.
// Fixture construction and JWT issuance are outside the measured request.
type settingsBudgetFixture struct {
	db       *bun.DB
	router   chi.Router
	module   owners.SettingsTestModule
	account  int64
	operator int64
	org      int64
	token    string
	admin    testutil.Claims
	counter  *testpkg.QueryCounter
}

func newSettingsBudgetFixture(t *testing.T, db *bun.DB) settingsBudgetFixture {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "settings-budget")
	operator := testpkg.CreateTestOperator(t, db)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Settings budget", Slug: fmt.Sprintf("settings-budget-%d", account.ID), Active: true,
	})
	_, err := db.NewUpdate().TableExpr("platform.schools").Set("organization_id = ?", org.ID).
		Where("id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	ctx := testpkg.WithTestTenantRuntime(t, context.Background())
	router, token, err := api.NewSettingsBudgetRouterForTest(ctx, testpkg.NewDatabase(t, db), operator.ID)
	require.NoError(t, err)
	module, err := owners.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	admin := testutil.AdminTestClaims(int(account.ID))
	admin.TenantID = testpkg.Tenant(t)
	return settingsBudgetFixture{db: db, router: router, module: module, account: account.ID,
		operator: operator.ID, org: org.ID, token: token, admin: admin, counter: testpkg.CaptureQueries(t, db)}
}

func (f settingsBudgetFixture) get(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	f.counter.Reset()
	req := testutil.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.RemoteAddr = "127.0.0.1:12345"
	rec := testutil.ExecuteRequestForTest(t, f.router, req)
	encoded, err := json.Marshal(f.counter.Queries())
	require.NoError(t, err)
	t.Logf("path=%s statements=%d SQL=%s", path, f.counter.Total(), encoded)
	return rec
}

func settingsBudgetData(t *testing.T, rec *httptest.ResponseRecorder) json.RawMessage {
	t.Helper()
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Status string          `json:"status"`
		Data   json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.Equal(t, "success", envelope.Status)
	return envelope.Data
}

func (f settingsBudgetFixture) noSettingsDML(t *testing.T, audit bool) {
	t.Helper()
	for _, operation := range []string{"UPDATE", "DELETE", "MERGE"} {
		require.Empty(t, f.counter.Operation(operation))
	}
	if audit {
		require.Len(t, f.counter.Operation("INSERT"), 1, "only the mandatory operator access audit writes")
		require.Contains(t, f.counter.Operation("INSERT")[0], "platform.operator_audit_log")
		require.EqualValues(t, 1, f.counter.WriteRows())
	} else {
		require.Empty(t, f.counter.Operation("INSERT"))
		require.Zero(t, f.counter.WriteRows())
	}
	for _, sql := range f.counter.Queries() {
		require.NotContains(t, sql, "pg_advisory", "GET must not acquire write locks")
	}
}

func assertBudgetSchema(t *testing.T, raw json.RawMessage, portal string) map[string]*records.ResolvedSetting {
	t.Helper()
	var schema records.SettingsSchema
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	require.NoError(t, decoder.Decode(&schema))
	defs := records.DefaultRegistry().AllDefinitions()
	items := make(map[string]*records.ResolvedSetting)
	tabRank := make(map[string]int)
	for i, key := range records.TabOrder {
		tabRank[key] = i
	}
	var extra []string
	for _, tab := range schema.Tabs {
		if _, ok := tabRank[tab.Key]; !ok {
			extra = append(extra, tab.Key)
		}
	}
	sort.Strings(extra)
	for i, key := range extra {
		tabRank[key] = len(records.TabOrder) + i
	}
	lastTab := -1
	for _, tab := range schema.Tabs {
		require.Greater(t, tabRank[tab.Key], lastTab)
		lastTab = tabRank[tab.Key]
		require.Equal(t, tab.Key, tab.Label)
		lastCategory := -1
		lastCategoryKey := ""
		for _, category := range tab.Categories {
			require.NotEmpty(t, category.Items)
			require.Equal(t, category.Key, category.Label)
			minimum := category.Items[0].SortOrder
			require.GreaterOrEqual(t, minimum, lastCategory)
			if minimum == lastCategory {
				require.Greater(t, category.Key, lastCategoryKey)
			}
			lastCategory, lastCategoryKey = minimum, category.Key
			last := -1
			for _, item := range category.Items {
				require.NotContains(t, items, item.Key)
				items[item.Key] = item
				def := defs[item.Key]
				require.NotNil(t, def)
				require.Equal(t, tab.Key, def.Tab)
				require.Equal(t, category.Key, def.Category)
				require.GreaterOrEqual(t, item.SortOrder, last)
				last = item.SortOrder
				// Full definition DTO, including validation/options/dependency, is
				// compared independently of the resolved state.
				want := records.ResolvedSetting{Key: def.Key, Label: def.Label, Description: def.Description,
					Type: def.Type, Default: def.Default, SortOrder: def.SortOrder, AccessPolicy: def.AccessPolicy,
					Levels: def.Levels, Validation: def.Validation, Options: def.Options, DependsOn: def.DependsOn,
					Value: item.Value, IsDefault: item.IsDefault, Writable: item.Writable, Visible: item.Visible,
					Source: item.Source, TraegerChangedAt: item.TraegerChangedAt}
				switch item.Key {
				case records.KeyParentSickReportsEnabled, records.KeyParentExcusedReportsEnabled:
					want.Type, want.Default = records.FieldSelect, "approval"
					want.Options = &records.SelectOptions{Static: []records.SelectOption{{Label: "Aus", Value: "off"}, {Label: "Sofort übernehmen", Value: "immediate"}, {Label: "Erst bestätigen", Value: "approval"}}}
					require.Equal(t, "approval", item.Value)
				case records.KeyParentRequestReviewScope, records.KeyParentAbsenceReviewScope:
					want.Default = records.ParentRequestReviewScopeAdmins
					want.Options = &records.SelectOptions{Static: append([]records.SelectOption(nil), def.Options.Static[1:]...)}
					require.Equal(t, records.ParentRequestReviewScopeAdmins, item.Value)
				}
				wantJSON, err := json.Marshal(want)
				require.NoError(t, err)
				gotJSON, err := json.Marshal(item)
				require.NoError(t, err)
				require.JSONEq(t, string(wantJSON), string(gotJSON))
			}
		}
	}
	for key, def := range defs {
		visible := def.AccessPolicy != records.AccessOperatorOnly
		if portal == "operator" {
			visible = def.AccessPolicy != records.AccessAdminOnly
		}
		if portal == "traeger" {
			visible = def.Levels.AtTraeger()
		}
		if portal != "traeger" {
			switch key {
			case records.KeyParentSickNoteEnabled, records.KeyParentSickRequiresApproval, records.KeyParentExcusedRequiresApproval, records.KeyParentRequestGroupLeaderReviewEnabled:
				visible = false
			}
		}
		if visible {
			require.Contains(t, items, key)
		} else {
			require.NotContains(t, items, key)
		}
	}
	if portal != "traeger" {
		require.False(t, items[records.KeyBrandingPrimaryColor].Writable, "Träger-only branding is never a school/operator write")
		require.Equal(t, portal == "operator", items[records.KeyModuleNFCAnwesenheit].Writable, "operator-managed module switch")
	}
	if portal != "operator" && portal != "traeger" {
		require.Equal(t, "••••••", items[records.KeyOGSDevicePIN].Value, "password default is masked too")
		require.False(t, items[records.KeyModuleNFCAnwesenheit].Writable)
	}
	return items
}

func TestSettingsSchemaGETQueryBudgets(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	admin := testutil.MintTestJWT(t, f.admin)
	traegerClaims := f.admin
	traegerClaims.Scope, traegerClaims.OrgID, traegerClaims.TenantID = "org", f.org, 0
	traegerClaims.IsAdmin = false
	traegerClaims.Permissions = []string{"traeger_overview:read", "traeger_settings:manage"}
	traeger := testutil.MintTestJWT(t, traegerClaims)
	// N is the number of persisted Träger overrides, not registry definitions.
	keys := []string{records.KeyParentNewsEnabled, records.KeyParentCarePickupRequestEnabled,
		records.KeyParentCareModeRequestEnabled, records.KeyParentGuardianManagementEnabled,
		records.KeyParentMasterDataEditEnabled, records.KeyParentMasterDataRequestEnabled,
		records.KeyEnrollmentLegalAGBText, records.KeyEnrollmentLegalDSGVOText}
	stored := map[string]any{}
	counts := map[string]int{}
	statements := map[string][]string{}
	for _, n := range []int{0, 3, 8} {
		for len(stored) < n {
			key := keys[len(stored)]
			def := records.GetDefinition(key)
			value := any("Budget legal text")
			if def.Type == records.FieldBoolean {
				value = !def.Default.(bool)
			}
			require.NoError(t, f.module.Traeger.SetValue(context.Background(), f.org, f.account,
				traegerClaims.Permissions, key, value))
			stored[key] = value
		}
		for _, route := range []struct{ portal, path, token string }{
			{"tenant", "/api/settings/schema", admin},
			{"operator", fmt.Sprintf("/operator/schools/%d/settings/schema", testpkg.Tenant(t)), f.token},
			{"traeger", "/traeger/settings/schema", traeger},
		} {
			t.Logf("portal=%s persisted_overrides=%d", route.portal, n)
			rec := f.get(t, route.path, route.token)
			items := assertBudgetSchema(t, settingsBudgetData(t, rec), route.portal)
			for _, key := range keys {
				item := items[key]
				require.NotNil(t, item)
				value, ok := stored[key]
				if ok {
					require.Equal(t, value, item.Value, key)
					require.EqualValues(t, "traeger", item.Source)
					require.NotNil(t, item.TraegerChangedAt)
				} else {
					require.Equal(t, records.GetDefinition(key).Default, item.Value, key)
					require.EqualValues(t, "default", item.Source)
					require.Nil(t, item.TraegerChangedAt)
				}
				require.True(t, item.Writable, key)
				require.Equal(t, route.portal != "traeger" || !ok, item.IsDefault, key)
			}
			f.noSettingsDML(t, route.portal == "operator")
			if n > 0 {
				require.Equal(t, counts[route.portal], f.counter.Total())
			}
			counts[route.portal] = f.counter.Total()
			statements[route.portal] = f.counter.Queries()
		}
	}
	querybudgets.AssertQueryBudget(t, "api.settings.schema", statements["tenant"])
	querybudgets.AssertQueryBudget(t, "operator.schools.settings.schema", statements["operator"])
	querybudgets.AssertQueryBudget(t, "traeger.settings.schema", statements["traeger"])
	var audits []testpkg.OperatorSettingsReadAuditSnapshot
	err := testpkg.WithAdminTx(t, testpkg.Ctx(t), f.db, func(ctx context.Context, tx bun.Tx) error {
		var err error
		audits, err = testpkg.ReadOperatorSettingsReadAuditSnapshots(ctx, tx, f.operator, testpkg.Tenant(t))
		return err
	})
	require.NoError(t, err)
	require.Len(t, audits, 3, "one committed audit per operator schema GET")
	for _, audit := range audits {
		require.Equal(t, f.operator, audit.OperatorID)
		require.Equal(t, testpkg.Tenant(t), audit.ResourceID)
		require.Equal(t, "127.0.0.1", audit.RequestIP)
		require.JSONEq(t, fmt.Sprintf(`{"view":"school_settings_schema","school_id":%d}`, testpkg.Tenant(t)), string(audit.Changes))
	}
}
