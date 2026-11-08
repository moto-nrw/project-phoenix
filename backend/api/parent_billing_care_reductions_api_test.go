package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The rule chain of the monthly contribution through the routes (#4685,
// ADR 0164). The contracts start on careReductionStart, open-ended: they are
// in force for the run month far ahead and in the window of the evidence
// page, whatever day the suite runs.

const (
	careRulesPath      = "/traeger/care-rules"
	careEvidencesPath  = "/traeger/care-evidences"
	careReductionStart = "2027-06-01"
)

type careRuleBody struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type careRuleSetBody struct {
	Rules []careRuleBody `json:"rules"`
}

type careEvidenceSetBody struct {
	Contracts []struct {
		ContractID int64 `json:"contract_id"`
		Evidences  []struct {
			ID     int64  `json:"id"`
			Kind   string `json:"kind"`
			RuleID *int64 `json:"rule_id"`
		} `json:"evidences"`
	} `json:"contracts"`
	Levels []careRuleBody `json:"levels"`
}

type careReductionRunBody struct {
	ID           int64 `json:"id"`
	Status       string
	PreviewCount int   `json:"preview_count"`
	PreviewCents int64 `json:"preview_cents"`
	Invoices     []struct {
		ContractID int64 `json:"contract_id"`
		Reductions []struct {
			Kind           string `json:"kind"`
			Name           string `json:"name"`
			ChildRank      *int   `json:"child_rank"`
			FamilyChildren *int   `json:"family_children"`
			AmountCents    int64  `json:"amount_cents"`
		} `json:"reductions"`
		TotalCents int64 `json:"total_cents"`
	} `json:"invoices"`
}

func careRule(kind, name string, schools []int64, extra map[string]any) map[string]any {
	body := map[string]any{"kind": kind, "name": name, "valid_from": "2029-09-01", "school_ids": schools}
	for key, value := range extra {
		body[key] = value
	}
	return body
}

func (f careBillingFixture) saveRule(t *testing.T, body map[string]any) careRuleSetBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRulesPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careRuleSetBody](t, rec.Body.Bytes())
}

func ruleNamed(t *testing.T, set careRuleSetBody, name string) int64 {
	t.Helper()
	for _, rule := range set.Rules {
		if rule.Name == name {
			return rule.ID
		}
	}
	t.Fatalf("no rule %q", name)
	return 0
}

func careEvidence(contractID int64, kind string, ruleID *int64) map[string]any {
	return map[string]any{"contract_id": contractID, "kind": kind, "rule_id": ruleID, "valid_from": careReductionStart,
		"evidence": "Einkommensteuerbescheid 2025", "checked_on": "2026-10-01"}
}

// sameFamily gives a contract the party of another one: the family is the
// recipient's mail address.
func sameFamily(t *testing.T, f careBillingFixture, contractID, like int64) {
	t.Helper()
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = ?
		WHERE contract_id = ?`, fmt.Sprintf("eva.%d@example.org", like), contractID)
	require.NoError(t, err)
}

// Every rule of the chain through the routes: an income level with its
// evidence, half from the second child, the reduction for single parents
// and the cap of the family. The release keeps each reduction on its
// invoice. Prices: until 14:00 32 €, until 16:30 45,50 € per weekday.
func TestCareReductionsRunTheWholeChainThroughTheRoutes(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ermäßigungen Kette")
	school := f.office.schools[0]
	// Mia books Mon until 14:00 (32 €) and Tue until 16:30 (45,50 €):
	// 77,50 €. Noah books Mon until 14:00: 32 €.
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	noah := careContract(t, f.db, f.office.organization, school, "Noah", careReductionStart, sjrServices()[0])
	sameFamily(t, f, noah, mia)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	set := f.saveRule(t, careRule("income_level", "Stufe 2", []int64{school}, map[string]any{"discount_basis_points": 2000}))
	level := ruleNamed(t, set, "Stufe 2")
	f.saveRule(t, careRule("sibling", "Geschwister", []int64{school}, map[string]any{"sibling_from_child": 2, "discount_basis_points": 5000}))
	f.saveRule(t, careRule("single_parent", "Alleinerziehend", []int64{school}, map[string]any{"discount_cents": 500}))
	f.saveRule(t, careRule("cap", "Höchstbetrag", []int64{school}, map[string]any{"cap_cents": 7000}))
	for _, body := range []map[string]any{
		careEvidence(mia, "income_level", &level),
		careEvidence(noah, "single_parent", nil),
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, body)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[careReductionRunBody](t, rec.Body.Bytes())
	// Mia 77,50 € − 20 % (15,50 €) = 62,00 €. Noah is second by base
	// price: 32,00 − 50 % = 16,00 €, − 5,00 € = 11,00 €. Cap 70,00 €:
	// 62,00 + 11,00 = 73,00 €, 3,00 € off Noah → 8,00 €.
	totals := map[int64]int64{}
	reductions := map[int64][]string{}
	for _, invoice := range preview.Invoices {
		totals[invoice.ContractID] = invoice.TotalCents
		for _, reduction := range invoice.Reductions {
			reductions[invoice.ContractID] = append(reductions[invoice.ContractID],
				fmt.Sprintf("%s %d", reduction.Kind, reduction.AmountCents))
		}
	}
	assert.Equal(t, map[int64]int64{mia: 6200, noah: 800}, totals)
	assert.Equal(t, []string{"income_level -1550"}, reductions[mia])
	assert.Equal(t, []string{"sibling -1600", "single_parent -500", "cap -300"}, reductions[noah])

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careReductionRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Invoices, 2)
	for _, invoice := range released.Invoices {
		assert.Equal(t, totals[invoice.ContractID], invoice.TotalCents, "the issued invoice keeps the total")
		if invoice.ContractID == noah {
			require.Len(t, invoice.Reductions, 3, "the issued invoice keeps each reduction")
			assert.Equal(t, 2, *invoice.Reductions[0].ChildRank)
			assert.Equal(t, 2, *invoice.Reductions[0].FamilyChildren)
			assert.Equal(t, "Geschwister", invoice.Reductions[0].Name)
		}
	}
	// A later change of a rule never reaches the issued invoice.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careRulesPath+"/"+strconv.FormatInt(level, 10),
		careRule("income_level", "Stufe 2 neu", []int64{school}, map[string]any{"discount_basis_points": 9000}))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[careReductionRunBody](t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, ""), nil).Body.Bytes())
	for _, invoice := range again.Invoices {
		assert.Equal(t, totals[invoice.ContractID], invoice.TotalCents)
	}
	// Already issued amounts consume the cap even after the income rule
	// changes. A later child cannot make the family pay above 70 euros.
	lea := careContract(t, f.db, f.office.organization, school, "Lea", careReductionStart, sjrServices()[0])
	sameFamily(t, f, lea, mia)
	nextRun := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(nextRun.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	next := decodeSchoolPortalData[careReductionRunBody](t, rec.Body.Bytes())
	require.Len(t, next.Invoices, 1)
	assert.Equal(t, lea, next.Invoices[0].ContractID)
	assert.Zero(t, next.Invoices[0].TotalCents)

}

// A fixed end price differs from a fixed discount, and two scopes count
// apart: Birkenfeld's children do not make an Ispringen child the second.
func TestCareSiblingRulesPriceScopeAndCount(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ermäßigungen Bereiche")
	ispringen, birkenfeld := f.office.schools[0], f.office.schools[1]
	mia := careContract(t, f.db, f.office.organization, ispringen, "Mia", careReductionStart, sjrServices()...)
	noah := careContract(t, f.db, f.office.organization, ispringen, "Noah", careReductionStart, sjrServices()[0])
	lea := careContract(t, f.db, f.office.organization, birkenfeld, "Lea", careReductionStart, sjrServices()[0])
	sameFamily(t, f, noah, mia)
	sameFamily(t, f, lea, mia)
	for _, school := range []int64{ispringen, birkenfeld} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	// Ispringen: the second child pays a fixed 25,00 €. Birkenfeld: 10,00 €
	// off from the second child, counting only Birkenfeld.
	f.saveRule(t, careRule("sibling", "Ispringen", []int64{ispringen}, map[string]any{"sibling_from_child": 2, "price_cents": 2500}))
	f.saveRule(t, careRule("sibling", "Birkenfeld", []int64{birkenfeld}, map[string]any{"sibling_from_child": 2, "discount_cents": 1000}))
	run := f.startRun(t, careMonth)
	preview := decodeSchoolPortalData[careReductionRunBody](t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, ""), nil).Body.Bytes())
	totals := map[int64]int64{}
	for _, invoice := range preview.Invoices {
		totals[invoice.ContractID] = invoice.TotalCents
	}
	// Noah (32,00 €) is second in Ispringen and pays 25,00 €; Lea is alone
	// in Birkenfeld and pays 32,00 €.
	assert.Equal(t, map[int64]int64{mia: 7750, noah: 2500, lea: 3200}, totals)
}

// The routes check a rule and an evidence as a whole and keep two rules of a
// kind apart per school.
func TestCareReductionsAreCheckedBeforeTheyAreSaved(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ermäßigungen Prüfung")
	school := f.office.schools[0]
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Ermäßigungen Prüfung Fremd", identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	for _, body := range []map[string]any{
		careRule("sibling", "", []int64{school}, map[string]any{"sibling_from_child": 2, "discount_basis_points": 5000}),
		careRule("sibling", "Zwei", []int64{school}, map[string]any{"sibling_from_child": 2, "discount_basis_points": 5000, "price_cents": 1}),
		careRule("sibling", "Fremd", []int64{foreign.schools[0]}, map[string]any{"sibling_from_child": 2, "discount_cents": 1}),
		careRule("sibling", "Zählt fremd", []int64{school}, map[string]any{"sibling_from_child": 2, "discount_cents": 1,
			"count_school_ids": []int64{foreign.schools[0]}}),
		careRule("single_parent", "Preis", []int64{school}, map[string]any{"price_cents": 100}),
		careRule("cap", "Ohne Betrag", []int64{school}, nil),
		{"kind": "sibling", "name": "Mitte", "valid_from": "2029-09-15", "school_ids": []int64{school},
			"sibling_from_child": 2, "discount_cents": 1},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRulesPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "%v: %s", body["name"], rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.contribution_rule_invalid")
	}
	set := f.saveRule(t, careRule("sibling", "Erste", []int64{school}, map[string]any{"sibling_from_child": 2, "discount_cents": 100}))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRulesPath,
		careRule("sibling", "Zweite", []int64{school, f.office.schools[1]}, map[string]any{"sibling_from_child": 3, "discount_cents": 100}))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.contribution_rule_overlap")
	// Another school takes its own rule.
	f.saveRule(t, careRule("sibling", "Andere Schule", []int64{f.office.schools[1]}, map[string]any{"sibling_from_child": 2, "discount_cents": 100}))
	// The holiday routes neither change nor delete a care rule.
	sibling := ruleNamed(t, set, "Erste")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, "/traeger/contribution-rules/"+strconv.FormatInt(sibling, 10), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	levels := f.saveRule(t, careRule("income_level", "Stufe 1", []int64{school}, map[string]any{"price_cents": 1000}))
	level := ruleNamed(t, levels, "Stufe 1")
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	foreignContract := careContract(t, f.db, foreign.organization, foreign.schools[0], "Fremd", careReductionStart, sjrServices()...)
	for _, body := range []map[string]any{
		careEvidence(foreignContract, "single_parent", nil),
		careEvidence(contract, "income_level", nil),
		careEvidence(contract, "income_level", &sibling),
		{"contract_id": contract, "kind": "single_parent", "valid_from": careReductionStart, "evidence": "", "checked_on": "2026-10-01"},
		{"contract_id": contract, "kind": "single_parent", "valid_from": careReductionStart, "evidence": "Erklärung", "checked_on": "2099-01-01"},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.care_evidence_invalid")
	}
	// A level of another school would be ignored by the run: refused.
	otherLevels := f.saveRule(t, careRule("income_level", "Stufe Nord", []int64{f.office.schools[1]}, map[string]any{"price_cents": 1000}))
	otherLevel := ruleNamed(t, otherLevels, "Stufe Nord")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(contract, "income_level", &otherLevel))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_evidence_invalid")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(contract, "income_level", &level))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(contract, "income_level", &level))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_evidence_overlap")
	// A level an evidence names stays.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, careRulesPath+"/"+strconv.FormatInt(level, 10), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_rule_in_use")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careRulesPath+"/"+strconv.FormatInt(level, 10),
		careRule("single_parent", "Stufe 1", []int64{school}, map[string]any{"discount_cents": 100}))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_rule_in_use")
}

// Without the right "Abrechnung" every route answers 403; another Träger
// finds neither the rules nor the evidences, and every read and change of an
// evidence stands in its trail.
func TestCareReductionsNeedTheRightStayWithTheTraegerAndAreRecorded(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ermäßigungen Grenze")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	set := f.saveRule(t, careRule("income_level", "Stufe 3", []int64{school}, map[string]any{"discount_basis_points": 1000}))
	level := ruleNamed(t, set, "Stufe 3")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(contract, "income_level", &level))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	evidences := decodeSchoolPortalData[careEvidenceSetBody](t, rec.Body.Bytes())
	var evidenceID int64
	for _, entry := range evidences.Contracts {
		if entry.ContractID == contract {
			require.Len(t, entry.Evidences, 1)
			evidenceID = entry.Evidences[0].ID
		}
	}
	require.NotZero(t, evidenceID, "the evidence page shows the contract with its evidence")
	ruleItem := careRulesPath + "/" + strconv.FormatInt(level, 10)
	evidenceItem := careEvidencesPath + "/" + strconv.FormatInt(evidenceID, 10)

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, careRulesPath, nil},
		{http.MethodPost, careRulesPath, careRule("cap", "Höchstbetrag", []int64{school}, map[string]any{"cap_cents": 1})},
		{http.MethodGet, careEvidencesPath, nil},
		{http.MethodPost, careEvidencesPath, careEvidence(contract, "single_parent", nil)},
		{http.MethodPut, ruleItem, careRule("income_level", "Stufe 3", []int64{school}, map[string]any{"discount_basis_points": 1})},
		{http.MethodPut, evidenceItem, careEvidence(contract, "income_level", &level)},
		{http.MethodDelete, evidenceItem, nil},
		{http.MethodDelete, ruleItem, nil},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Ermäßigungen Grenze Fremd", identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	for _, request := range requests[4:] {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Contains(t, []int{http.StatusNotFound, http.StatusBadRequest}, rec.Code,
			"%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	for _, path := range []string{careRulesPath, careEvidencesPath} {
		rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "Stufe 3", "no rule of another Träger")
		assert.NotContains(t, rec.Body.String(), "Einkommensteuerbescheid", "no evidence of another Träger")
	}
	var kept int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_evidences WHERE id = ?`, evidenceID).Scan(context.Background(), &kept))
	assert.Equal(t, 1, kept, "the other Träger deleted nothing")

	// The trail: the create and the read after it, then one more read.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careEvidencesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM parent_billing.care_evidence_events
		WHERE organization_id = ? AND care_contract_id = ? ORDER BY id`, f.office.organization, contract).
		Scan(context.Background(), &actions))
	assert.Equal(t, []string{"create", "read", "read"}, actions)
	var anonymous int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_evidence_events
		WHERE care_contract_id = ? AND actor_account_id IS NULL`, contract).Scan(context.Background(), &anonymous))
	assert.Zero(t, anonymous, "every entry of the trail names the account")
}

// The rule list and the evidence page read the same statements for any
// number of rules, contracts and evidences.
func TestCareReductionListsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ermäßigungen Budget")
	school := f.office.schools[0]
	list := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	set := f.saveRule(t, careRule("income_level", "Stufe A", []int64{school}, map[string]any{"discount_cents": 100}))
	level := ruleNamed(t, set, "Stufe A")
	first := careContract(t, f.db, f.office.organization, school, "Mia", careReductionStart, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(first, "income_level", &level))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rules, evidences := list(careRulesPath), list(careEvidencesPath)
	f.saveRule(t, careRule("cap", "Deckel", []int64{school}, map[string]any{"cap_cents": 1000}))
	second := careContract(t, f.db, f.office.organization, f.office.schools[1], "Noah", careReductionStart, sjrServices()...)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careEvidencesPath, careEvidence(second, "single_parent", nil))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Len(t, list(careRulesPath).Statements, len(rules.Statements), "the rules read the same statements for any number of rules")
	assert.Len(t, list(careEvidencesPath).Statements, len(evidences.Statements), "the evidences read the same statements for any number of contracts")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_rules", rules)
	querybudgets.AssertQueryBudget(t, "api.traeger.care_evidences", evidences)
}
