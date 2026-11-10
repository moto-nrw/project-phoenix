// Router-level tests of the revenue accounts and cost centers (#4821): the
// release copies them onto every position, a later change reaches no
// issued position and no file, the Sollstellung offers them as fields, and
// the right "Finanz-Export" and the Träger bound every route.
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

const bookingAccountsPath = financeExportsPath + "/booking-accounts"

type bookingAccountsBody struct {
	StandardAccount string `json:"standard_account"`
	Schools         []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
	Offerings []struct {
		SchoolID       int64    `json:"school_id"`
		CareOfferingID int64    `json:"care_offering_id"`
		ModuleNames    []string `json:"module_names"`
	} `json:"offerings"`
	RevenueAccounts []struct {
		Scope   string `json:"scope"`
		ScopeID int64  `json:"scope_id"`
		Account string `json:"account"`
	} `json:"revenue_accounts"`
	CostCenters []struct {
		ID        int64  `json:"id"`
		Scope     string `json:"scope"`
		ScopeID   int64  `json:"scope_id"`
		ValidFrom string `json:"valid_from"`
		Code      string `json:"code"`
	} `json:"cost_centers"`
}

func bookingAccountsOf(t *testing.T, f careBillingFixture) bookingAccountsBody {
	t.Helper()
	rec := exportGet(t, f, "/booking-accounts")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[bookingAccountsBody](t, rec.Body.Bytes())
}

func TestBookingAccountsAreFrozenOntoTheIssuedPositions(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Erlöskonten")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-K-MIA"))
	page := bookingAccountsOf(t, f)
	require.NotEmpty(t, page.Offerings, "the Angebote of the prices")
	offering := page.Offerings[0]
	assert.Equal(t, f.office.schools[0], offering.SchoolID)

	put := func(scope string, scopeID int64, account string) int {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, bookingAccountsPath+"/revenue-accounts",
			map[string]any{"scope": scope, "scope_id": scopeID, "account": account})
		return rec.Code
	}
	addCenter := func(scope string, scopeID int64, from, code string) int {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, bookingAccountsPath+"/cost-centers",
			map[string]any{"scope": scope, "scope_id": scopeID, "valid_from": from, "code": code})
		return rec.Code
	}
	require.Equal(t, http.StatusOK, put("offering", offering.CareOfferingID, "4410"))
	// The Träger keeps no DATEV standard yet: the Leistungen carry their own.
	require.Equal(t, http.StatusOK, put("care", 0, "4400"))
	require.Equal(t, http.StatusOK, put("meals", 0, "4420"))
	require.Equal(t, http.StatusCreated, addCenter("school", f.office.schools[0], "2029-01-01", "ISP"))
	// The cost center of next year does not hold in June.
	require.Equal(t, http.StatusCreated, addCenter("school", f.office.schools[0], "2030-01-01", "ISP2030"))
	require.Equal(t, http.StatusConflict, addCenter("school", f.office.schools[0], "2030-01-01", "DOPPELT"), "one per day")
	require.Equal(t, http.StatusBadRequest, put("care", 0, "0000"))
	require.Equal(t, http.StatusBadRequest, addCenter("school", f.office.schools[0], "2029-02-01", "K-1"))

	run := releaseDebitRun(t, f, sepaDebitMonth)
	type frozen struct {
		Account    *string `bun:"revenue_account"`
		CostCenter *string `bun:"cost_center"`
		Frozen     bool    `bun:"accounts_frozen"`
	}
	var positions []frozen
	require.NoError(t, f.db.NewRaw(`SELECT position.revenue_account, position.cost_center, position.accounts_frozen
		FROM parent_billing.care_invoice_positions AS position
		JOIN parent_billing.care_invoices AS invoice ON invoice.id = position.invoice_id
		WHERE invoice.run_id = ? AND position.care_offering_id = ?`, run, offering.CareOfferingID).Scan(context.Background(), &positions))
	require.NotEmpty(t, positions)
	for _, position := range positions {
		require.NotNil(t, position.Account)
		assert.Equal(t, "4410", *position.Account, "the Angebot's account")
		require.NotNil(t, position.CostCenter)
		assert.Equal(t, "ISP", *position.CostCenter, "the KOST1 of June, not of next year")
		assert.True(t, position.Frozen)
	}

	// Later changes reach no issued position: the Sollstellung of the run
	// still writes the accounts of the release.
	require.Equal(t, http.StatusOK, put("offering", offering.CareOfferingID, "4999"))
	require.Equal(t, http.StatusCreated, addCenter("school", f.office.schools[0], "2029-03-01", "NEU"))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, financeExportsPath+"/settings/sollstellung", map[string]any{
		"columns": []map[string]any{{"field": "invoice_number", "header": "Nr"}, {"field": "revenue_account", "header": "Konto"},
			{"field": "cost_center", "header": "KOST1"}},
		"delimiter": ";", "decimal_separator": ",", "date_format": "dd.mm.yyyy", "encoding": "utf-8", "header_row": true,
		"file_name_pattern": "soll-{NR}.csv",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, content := sollstellung(t, f, run)
	assert.Contains(t, content, ";4410;ISP")
	assert.NotContains(t, content, "4999")
	assert.NotContains(t, content, "NEU")

	// Removing a cost center keeps what the release copied.
	page = bookingAccountsOf(t, f)
	for _, center := range page.CostCenters {
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, fmt.Sprintf("%s/cost-centers/%d", bookingAccountsPath, center.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	assert.Empty(t, bookingAccountsOf(t, f).CostCenters)
	_, content = sollstellung(t, f, run)
	assert.Contains(t, content, ";4410;ISP")
}

func TestBookingAccountsRightsAndTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Konten Rechte")
	careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, bookingAccountsPath+"/cost-centers",
		map[string]any{"scope": "holiday", "scope_id": 0, "valid_from": "2029-01-01", "code": "FER"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	center := bookingAccountsOf(t, f).CostCenters[0].ID

	leadership := newTraegerOffice(t, f.db, f.traeger, "Träger Konten Leitung", identityaccess.TraegerRoleLeadership)
	rec = exportGet(t, careBillingFixture{db: f.db, traeger: f.traeger, office: leadership}, "/booking-accounts")
	assert.Equal(t, http.StatusForbidden, rec.Code, "the right Finanz-Export")
	rec = traegerJSON(t, f.traeger, leadership.token, http.MethodPut, bookingAccountsPath+"/revenue-accounts",
		map[string]any{"scope": "care", "scope_id": 0, "account": "4400"})
	assert.Equal(t, http.StatusForbidden, rec.Code)

	other := sepaDebitFixture(t, "Träger Konten Fremd")
	page := bookingAccountsOf(t, other)
	assert.Empty(t, page.CostCenters, "no cost center of another Träger")
	rec = traegerJSON(t, other.traeger, other.office.token, http.MethodPost, bookingAccountsPath+"/cost-centers",
		map[string]any{"scope": "school", "scope_id": f.office.schools[0], "valid_from": "2029-01-01", "code": "X"})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.`)
	rec = traegerJSON(t, other.traeger, other.office.token, http.MethodDelete, fmt.Sprintf("%s/cost-centers/%d", bookingAccountsPath, center), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"code":"general.`)
	assert.Len(t, bookingAccountsOf(t, f).CostCenters, 1, "the other Träger deleted nothing")
}
