package application

import (
	"context"
	"encoding/json"
	"os"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The export side of the DATEV settings (#4694): the readiness check gives
// every case of backend/testdata/datev-settings-cases.json the verdict the
// list names. TestDatevSettingsSaveMatchesTheExportCases in the settings
// registry runs the same list against the save validation.
func TestDatevReadinessMatchesTheSettingsCases(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../../testdata/datev-settings-cases.json")
	require.NoError(t, err)
	var list struct {
		Base  map[string]any `json:"base"`
		Cases []struct {
			Key    string `json:"key"`
			Value  any    `json:"value"`
			Export string `json:"export"`
		} `json:"cases"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.NotEmpty(t, list.Cases)

	settingsOf := func(values map[string]any) parentbilling.DatevSettings {
		text := func(key string) parentbilling.DatevSetting {
			value, _ := values[key].(string)
			return parentbilling.DatevSetting{Key: key, Value: value}
		}
		locked, _ := values["parent_billing.datev_locked"].(bool)
		costCenterRequired, _ := values["parent_billing.datev_cost_center_required"].(bool)
		return parentbilling.DatevSettings{
			ConsultantNumber: text("parent_billing.datev_consultant_number"),
			ClientNumber:     text("parent_billing.datev_client_number"),
			AccountLength:    text("parent_billing.datev_account_length"),
			ChartOfAccounts:  text("parent_billing.datev_chart_of_accounts"),
			FiscalYearStart:  text("parent_billing.datev_fiscal_year_start"),
			RevenueAccount:   text("parent_billing.datev_revenue_account"),
			Locked:           locked,
			// A Pflicht-KOST1 is a rule of the bookings, not of the settings.
			CostCenterRequired: costCenterRequired,
		}
	}
	readiness := func(values map[string]any) parentbilling.DatevReadiness {
		port := &datevSettingsDouble{byOrganization: map[int64]parentbilling.DatevSettings{7: settingsOf(values)}}
		result, err := newDatevExportForTest(port).DatevReadiness(context.Background(), parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 3})
		require.NoError(t, err)
		return result
	}
	require.True(t, readiness(list.Base).Ready, "the base of the case list is exportable")

	for _, c := range list.Cases {
		values := map[string]any{}
		for key, value := range list.Base {
			values[key] = value
		}
		values[c.Key] = c.Value
		got := readiness(values)
		verdict := "ok"
		switch {
		case slices.Contains(got.InvalidSettings, c.Key):
			verdict = "invalid"
		case slices.Contains(got.MissingSettings, c.Key):
			verdict = "missing"
		}
		assert.Equal(t, c.Export, verdict, "%s = %#v", c.Key, c.Value)
	}
}
