package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The cost bearer shares of the monthly contribution through the routes
// (#4687, ADR 0169). The contracts and shares start on careReductionStart,
// like the evidences: they hold for the run month far ahead and the
// contracts stand in the window of the share page.

const careCostSharesPath = "/traeger/care-cost-shares"

type careCostShareSetBody struct {
	Contracts []struct {
		ContractID int64 `json:"contract_id"`
		Shares     []struct {
			ID     int64  `json:"id"`
			Status string `json:"status"`
		} `json:"shares"`
	} `json:"contracts"`
	Expiring int `json:"expiring"`
}

type careCostShareRunBody struct {
	careRunBody
	Invoices []struct {
		ID         int64  `json:"id"`
		ContractID int64  `json:"contract_id"`
		Number     string `json:"number"`
		CostShares []struct {
			Kind           string `json:"kind"`
			CostBearerID   int64  `json:"cost_bearer_id"`
			CostBearerName string `json:"cost_bearer_name"`
			AmountCents    int64  `json:"amount_cents"`
		} `json:"cost_shares"`
		TotalCents int64 `json:"total_cents"`
	} `json:"invoices"`
	CostBearers []struct {
		CostBearerID int64 `json:"cost_bearer_id"`
		InvoiceCount int   `json:"invoice_count"`
		AmountCents  int64 `json:"amount_cents"`
	} `json:"cost_bearers"`
}

// costBearer keeps a cost bearer of the fixture's Träger through its route.
func (f careBillingFixture) costBearer(t *testing.T, name string) int64 {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/cost-bearers",
		map[string]any{"name": name, "address": "Am Kreishaus 1\n12345 Musterstadt", "file_reference": "51-2030", "active": true})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
}

func careCostShare(contractID int64, kind string, bearerID int64, share map[string]any) map[string]any {
	body := map[string]any{"contract_id": contractID, "kind": kind, "cost_bearer_id": bearerID, "valid_from": careReductionStart,
		"valid_until": "2030-07-31", "reference": "Bescheid 51-2030-17", "checked_on": "2026-10-01"}
	for key, value := range share {
		body[key] = value
	}
	return body
}

func (f careBillingFixture) saveCostShare(t *testing.T, body map[string]any) careCostShareSetBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careCostSharesPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careCostShareSetBody](t, rec.Body.Bytes())
}

func (f careBillingFixture) costShareRun(t *testing.T, id int64) careCostShareRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careCostShareRunBody](t, rec.Body.Bytes())
}

// Mia books Mon until 14:00 and Tue until 16:30 (77,50 €), Lea the same.
// The Jugendamt takes all of Mia's contribution, Bildung und Teilhabe 25 €
// of Lea's. The release keeps both shares on the invoices; the list for the
// Jugendamt names Mia and the download stands in the trail. A later change
// of a share never reaches an issued invoice.
func TestCareCostSharesSplitTheContributionAndStayOnTheIssuedInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Kostenträger Kette")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	lea := careContract(t, f.db, f.office.organization, school, "Lea", careReductionStart, sjrServices()...)
	pricedSchools(t, f)
	youthOffice := f.costBearer(t, "Jugendamt Musterkreis")
	jobcenter := f.costBearer(t, "Jobcenter Musterkreis")
	f.saveCostShare(t, careCostShare(mia, "youth_office", youthOffice, map[string]any{"share_basis_points": 10000}))
	set := f.saveCostShare(t, careCostShare(lea, "education_participation", jobcenter, map[string]any{"share_cents": 2500}))
	require.Len(t, set.Contracts, 2)

	run := f.startRun(t, careMonth)
	preview := f.costShareRun(t, run.ID)
	totals, shares := map[int64]int64{}, map[int64]int64{}
	for _, invoice := range preview.Invoices {
		totals[invoice.ContractID] = invoice.TotalCents
		for _, share := range invoice.CostShares {
			shares[invoice.ContractID] += share.AmountCents
		}
	}
	assert.Equal(t, map[int64]int64{mia: 0, lea: 5250}, totals, "the family pays the rest")
	assert.Equal(t, map[int64]int64{mia: 7750, lea: 2500}, shares)
	assert.Equal(t, int64(5250), preview.PreviewCents, "the run sums what the families pay")
	require.Len(t, preview.CostBearers, 2, "the preview names what each cost bearer pays")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careCostShareRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 2)
	var miaNumber string
	for _, invoice := range released.Invoices {
		require.Len(t, invoice.CostShares, 1, "the issued invoice keeps its share")
		assert.Equal(t, totals[invoice.ContractID], invoice.TotalCents)
		if invoice.ContractID == mia {
			miaNumber = invoice.Number
			assert.Equal(t, "Jugendamt Musterkreis", invoice.CostShares[0].CostBearerName)
		}
	}

	// A later change of the share and of the cost bearer's name never
	// reaches the issued invoice.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, "/traeger/cost-bearers/"+strconv.FormatInt(youthOffice, 10),
		map[string]any{"name": "Jugendamt neu", "active": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	shareID := set.Contracts[0].Shares[0].ID
	for _, contract := range set.Contracts {
		if contract.ContractID == mia {
			shareID = contract.Shares[0].ID
		}
	}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careCostSharesPath+"/"+strconv.FormatInt(shareID, 10),
		careCostShare(mia, "youth_office", youthOffice, map[string]any{"share_basis_points": 1000}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := f.costShareRun(t, run.ID)
	for _, invoice := range again.Invoices {
		assert.Equal(t, totals[invoice.ContractID], invoice.TotalCents)
		if invoice.ContractID == mia {
			assert.Equal(t, "Jugendamt Musterkreis", invoice.CostShares[0].CostBearerName, "the invoice keeps its copy")
			assert.Equal(t, int64(7750), invoice.CostShares[0].AmountCents)
		}
	}

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, "/cost-shares/"+strconv.FormatInt(youthOffice, 10)+"/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	csv := rec.Body.String()
	assert.Contains(t, csv, "Rechnungsnummer;Schule;Kind;Art;Monatsbeitrag in Euro;Anteil;Übernommener Betrag in Euro;Status")
	assert.Contains(t, csv, miaNumber+";")
	assert.Contains(t, csv, "Jugendamt;77,50;100 %;77,50;Ausgestellt")
	assert.NotContains(t, csv, "Lea", "the list names only the children of its cost bearer")
	assert.NotContains(t, csv, "Bescheid 51-2030-17", "the list carries no reference to the decision")
	var exports int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events
		WHERE run_id = ? AND action = 'export' AND details->>'list' = 'cost_share'`, run.ID).Scan(context.Background(), &exports))
	assert.Equal(t, 1, exports, "the download stands in the run's trail")
}

// A share entered after the review changes what the release would issue
// for an excluded invoice only: the count and the sum stay, but the
// fingerprint covers the share and the release answers 409.
func TestCareReviewReleaseRefusesACostShareAddedSinceTheReview(t *testing.T) {
	t.Parallel()
	changedSinceReview(t, "Träger Prüfphase neuer Kostenträger-Anteil", func(f careBillingFixture, _, mia int64) {
		bearer := f.costBearer(t, "Jugendamt Prüfphase")
		f.saveCostShare(t, careCostShare(mia, "youth_office", bearer, map[string]any{"share_basis_points": 5000}))
	})
}

// The share page and its routes need the right "Abrechnung"; another Träger
// finds no share and changes none; every read and change stands in the
// trail of the contract; a cost bearer a share names stays.
func TestCareCostSharesNeedTheRightStayWithTheTraegerAndAreRecorded(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Kostenträger Grenze")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	bearer := f.costBearer(t, "Jugendamt Grenze")
	set := f.saveCostShare(t, careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 10000}))
	var shareID int64
	for _, entry := range set.Contracts {
		if entry.ContractID == contract {
			require.Len(t, entry.Shares, 1)
			shareID = entry.Shares[0].ID
		}
	}
	require.NotZero(t, shareID)
	item := careCostSharesPath + "/" + strconv.FormatInt(shareID, 10)

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, careCostSharesPath, nil},
		{http.MethodPost, careCostSharesPath, careCostShare(contract, "other_subsidy", bearer, map[string]any{"share_cents": 100})},
		{http.MethodPut, item, careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 1})},
		{http.MethodDelete, item, nil},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Kostenträger Grenze Fremd", identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	for _, request := range requests[1:] {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, rec.Code,
			"%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, careCostSharesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "Bescheid 51-2030-17", "no share of another Träger")
	var kept int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_cost_shares WHERE id = ?`, shareID).Scan(context.Background(), &kept))
	assert.Equal(t, 1, kept, "the other Träger deleted nothing")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, "/traeger/cost-bearers/"+strconv.FormatInt(bearer, 10), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a cost bearer a share names stays: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careCostSharesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM parent_billing.care_cost_share_events
		WHERE organization_id = ? AND care_contract_id = ? ORDER BY id`, f.office.organization, contract).
		Scan(context.Background(), &actions))
	assert.Equal(t, []string{"create", "read", "read"}, actions)
	var anonymous int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_cost_share_events
		WHERE care_contract_id = ? AND actor_account_id IS NULL`, contract).Scan(context.Background(), &anonymous))
	assert.Zero(t, anonymous, "every entry of the trail names the account")
}

// A share without an end, with two amounts or with a reference that is too
// long never reaches the table.
func TestCareCostSharesAreCheckedBeforeTheyAreSaved(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Kostenträger Prüfung")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careReductionStart, sjrServices()...)
	bearer := f.costBearer(t, "Jugendamt Prüfung")
	for name, body := range map[string]map[string]any{
		"without an end":     careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 100, "valid_until": nil}),
		"with two amounts":   careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 100, "share_cents": 100}),
		"above 100 percent":  careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 10001}),
		"of an unknown kind": careCostShare(contract, "grandparents", bearer, map[string]any{"share_basis_points": 100}),
		"with a long reference": careCostShare(contract, "youth_office", bearer,
			map[string]any{"share_basis_points": 100, "reference": strings.Repeat("x", 121)}),
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careCostSharesPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.care_cost_share_invalid", name)
	}
	f.saveCostShare(t, careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 100}))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careCostSharesPath,
		careCostShare(contract, "youth_office", bearer, map[string]any{"share_basis_points": 100, "valid_from": "2029-03-01"}))
	assert.Equal(t, http.StatusConflict, rec.Code, "two shares of one kind in the same time: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_cost_share_overlap")
}

// The share page reads the same statements for any number of contracts and
// shares.
func TestCareCostShareListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Kostenträger Budget")
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, careCostSharesPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	bearer := f.costBearer(t, "Jugendamt Budget")
	first := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careReductionStart, sjrServices()...)
	f.saveCostShare(t, careCostShare(first, "youth_office", bearer, map[string]any{"share_basis_points": 10000}))
	shares := list()
	second := careContract(t, f.db, f.office.organization, f.office.schools[1], "Noah", careReductionStart, sjrServices()...)
	f.saveCostShare(t, careCostShare(second, "education_participation", bearer, map[string]any{"share_cents": 2500}))
	f.saveCostShare(t, careCostShare(second, "youth_office", bearer, map[string]any{"share_basis_points": 5000}))
	assert.Len(t, list().Statements, len(shares.Statements), "the share page reads the same statements for any number of shares")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_cost_shares", shares)
}
