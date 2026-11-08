//go:build testhooks

package settingshttp_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	settingsHTTP "github.com/moto-nrw/project-phoenix/modules/settings/http"
	"github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestSettingsHomeLayoutGETQueryBudget(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	token := testutil.MintTestJWT(t, f.admin)
	home := f.module.Settings.(settingsHTTP.HomeLayoutOperations)
	other := testpkg.CreateTestAccount(t, f.db, "settings-layout-other")
	require.NoError(t, home.SetHomeLayout(testpkg.Ctx(t), testpkg.Tenant(t), other.ID,
		map[string]bool{"section.foreign_account": false}, nil, nil, nil, nil, true))
	foreign := testpkg.NewTenantScope(t, f.db)
	_, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, foreign.TenantID, "Foreign", "Layout")
	require.NoError(t, home.SetHomeLayout(foreign.Context(), foreign.TenantID, foreignAccount.ID,
		map[string]bool{"section.foreign_tenant": false}, nil, nil, nil, nil, true))
	overrides := map[string]bool{}
	policies := map[string]string{}
	keys := []string{}
	spans, cols, rows := map[string]int{}, map[string]int{}, map[string]int{}
	blocks := []map[string]any{}
	var baseline int
	var statements testpkg.QueryMeasurement
	for _, n := range []int{0, 3, 8} {
		for len(keys) < n {
			i := len(keys)
			key := fmt.Sprintf("section.budget_%02d", i)
			keys = append(keys, key)
			overrides[key] = i%2 == 0
			spans[key], cols[key], rows[key] = 2, 2, i*2
			policies[key] = "required"
			if i%2 == 0 {
				policies[key] = "disabled"
			}
			blocks = append(blocks, map[string]any{"key": key, "span": 2, "col": 2, "row": i * 2})
		}
		if n > 0 {
			// Persist choices before policy prescriptions: both remembered
			// true and false choices must survive the school settling a block.
			require.NoError(t, home.SetHomeLayout(testpkg.Ctx(t), testpkg.Tenant(t), f.account, overrides, keys, spans, cols, rows, true))
			require.NoError(t, home.SetHomeBlockPolicies(testpkg.Ctx(t), testpkg.Tenant(t), f.account, f.admin.Permissions, policies))
		}
		t.Logf("home_layout persisted_slots=%d", n)
		raw := settingsBudgetData(t, f.get(t, "/api/settings/home-layout", token))
		want, err := json.Marshal(map[string]any{"overrides": overrides, "blocks": blocks, "policies": policies, "can_manage_policies": true})
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(raw), "exact personal choice, placement order/grid and school prescription DTO")
		f.noSettingsDML(t, false)
		require.Len(t, f.counter.Selects("config.home_layouts"), 1)
		require.Len(t, f.counter.Selects("config.home_block_policies"), 1)
		statements = f.counter.Measurement(f.counter.Queries())
		if n > 0 {
			require.Len(t, statements.Statements, baseline)
		}
		baseline = len(statements.Statements)
	}
	reader := f.admin
	reader.IsAdmin = false
	reader.Permissions = nil
	raw := settingsBudgetData(t, f.get(t, "/api/settings/home-layout", testutil.MintTestJWT(t, reader)))
	var view map[string]any
	require.NoError(t, json.Unmarshal(raw, &view))
	require.Equal(t, false, view["can_manage_policies"], "every tenant account can read its home layout")
	f.noSettingsDML(t, false)
	querybudgets.AssertQueryBudget(t, "api.settings.home_layout", statements)
}

func TestSettingsPayrollStatusGETQueryBudget(t *testing.T) {
	t.Parallel()
	f := newSettingsBudgetFixture(t, testpkg.SetupIsolatedTestDB(t))
	token := testutil.MintTestJWT(t, f.admin)
	foreign := testpkg.NewTenantScope(t, f.db)
	testpkg.CreateTestStaffForTenant(t, f.db, foreign.TenantID, "Foreign", "Payroll")
	settings := map[string]any{
		records.KeyPayrollLohnartRegelarbeit: "100", records.KeyPayrollLohnartKrank: "200", records.KeyPayrollEinheitKrank: records.PayrollUnitDays,
		records.KeyPayrollDatevBeraternummer: "1234567", records.KeyPayrollDatevMandantennummer: "12345",
		records.KeyPayrollDispatchAuto: true, records.KeyPayrollDispatchChannel: records.PayrollDispatchChannelEmail,
		records.KeyPayrollDispatchEmail: "payroll@example.invalid",
	}
	for key, value := range settings {
		require.NoError(t, f.module.TenantSettings.SetValue(testpkg.Ctx(t), testpkg.Tenant(t), f.account, f.admin.Permissions, key, value))
	}
	expected := records.PayrollStatus{
		Categories: []records.PayrollCategoryStatus{
			{ID: "regelarbeit", Label: "Regelarbeit", Number: "100", SettingKey: records.KeyPayrollLohnartRegelarbeit},
			{ID: "plus_stunden", Label: "Plus-Stunden", SettingKey: records.KeyPayrollLohnartPlusStunden},
			{ID: "auszahlung", Label: "Auszahlung", SettingKey: records.KeyPayrollLohnartAuszahlung},
			{ID: "freizeitausgleich", Label: "Freizeitausgleich", SettingKey: records.KeyPayrollLohnartFreizeitausgleich},
			{ID: "krank", Label: "Krank", Number: "200", SettingKey: records.KeyPayrollLohnartKrank, Unit: records.PayrollUnitDays, UnitRequired: true, UnitSettingKey: records.KeyPayrollEinheitKrank},
			{ID: "urlaub", Label: "Urlaub", SettingKey: records.KeyPayrollLohnartUrlaub, UnitRequired: true, UnitSettingKey: records.KeyPayrollEinheitUrlaub},
			{ID: "fortbildung", Label: "Fortbildung", SettingKey: records.KeyPayrollLohnartFortbildung, UnitRequired: true, UnitSettingKey: records.KeyPayrollEinheitFortbildung},
		},
		Beraternummer: "1234567", Mandantennummer: "12345", LodasHeaderComplete: true, ConfiguredCategories: 2, TotalCategories: 7,
		Dispatch: records.PayrollDispatchSettings{Auto: true, Channel: records.PayrollDispatchChannelEmail, Format: records.PayrollDispatchFormatCSV, Email: "payroll@example.invalid"},
	}
	created, missing, baseline := 0, 0, 0
	var statements testpkg.QueryMeasurement
	for _, n := range []int{0, 3, 8} {
		for created < n {
			staff := testpkg.CreateTestStaff(t, f.db, "Budget", fmt.Sprintf("Payroll-%d", created))
			var number *string
			if created == 1 || created%3 == 2 {
				value := ""
				if created%3 == 2 {
					value = fmt.Sprintf("PN%04d", created)
				}
				number = &value
			}
			_, err := f.db.NewUpdate().TableExpr("users.staff_employment_profiles").Set("personnel_number = ?", number).
				Where("membership_id = ?", staff.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			if created%3 != 2 {
				missing++
			}
			created++
		}
		expected.StaffTotal, expected.StaffWithoutPersonnelNumber = n, missing
		t.Logf("payroll persisted_staff=%d", n)
		raw := settingsBudgetData(t, f.get(t, "/api/settings/payroll-status", token))
		want, err := json.Marshal(expected)
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(raw), "exact categories/order/units/header/dispatch and personnel counts, never names")
		f.noSettingsDML(t, false)
		require.Len(t, f.counter.Selects("config.setting_values"), 1)
		require.NotEmpty(t, f.counter.Selects("users.staff_school_memberships"))
		statements = f.counter.Measurement(f.counter.Queries())
		if n == 0 {
			querybudgets.AssertQueryBudget(t, "api.settings.payroll_status.empty", statements)
		}
		// N=0 legitimately avoids hydration; compare populated sizes only.
		if n == 8 {
			require.Len(t, statements.Statements, baseline)
		}
		baseline = len(statements.Statements)
	}
	querybudgets.AssertQueryBudget(t, "api.settings.payroll_status", statements)
}
