// Router-level tests for the holiday invoice per child (#7007): with the
// Träger setting "Debitorennummer gilt je: Kind" the confirmation of a
// registration of siblings takes a number per child, the run issues an
// invoice per child with its child's number, and a cancelled child's invoice
// is issued again for that child alone. A registration billed as a whole
// keeps that shape after the switch.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

type holidayChildNumbersBody struct {
	DebtorNumber string `json:"debtor_number"`
	Children     []struct {
		ChildID      int64  `json:"child_id"`
		FirstName    string `json:"first_name"`
		DebtorNumber string `json:"debtor_number"`
	} `json:"children"`
}

type perChildDocumentBody struct {
	ID                  int64  `json:"id"`
	Kind                string `json:"kind"`
	Number              string `json:"number"`
	RegistrationID      int64  `json:"registration_id"`
	RegistrationChildID int64  `json:"registration_child_id"`
	DebtorNumber        string `json:"debtor_number"`
	ReferenceNumber     string `json:"reference_number"`
	Children            []struct {
		FirstName string `json:"first_name"`
	} `json:"children"`
}

type perChildRunBody struct {
	billingRunBody
	Documents []perChildDocumentBody `json:"documents"`
}

// setTraegerSetting sets a Träger setting as its administration.
func setTraegerSetting(t *testing.T, f reviewFixture, key string, value any) {
	t.Helper()
	admin := traegerColleague(t, f, identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, admin, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func holidayChildNumbers(t *testing.T, f reviewFixture, registrationID int64) holidayChildNumbersBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registrationID)+"/family-debtor-number", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[holidayChildNumbersBody](t, rec.Body.Bytes())
}

func perChildRun(t *testing.T, f reviewFixture, token string, id int64) perChildRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[perChildRunBody](t, rec.Body.Bytes())
}

// releasePerChildRun starts a run of the period, releases it and answers
// the issued run.
func releasePerChildRun(t *testing.T, f reviewFixture, token string) perChildRunBody {
	t.Helper()
	run := perChildRun(t, f, token, startBillingRun(t, f, token, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run.billingRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return perChildRun(t, f, token, run.ID)
}

func TestHolidayInvoicePerChildCarriesTheNumberOfEachChild(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienrechnung je Kind")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberOnConfirm, true)

	proposal := holidayChildNumbers(t, f, f.siblings)
	require.Len(t, proposal.Children, 2, "the confirmation asks for a number per child")
	first, second := proposal.Children[0], proposal.Children[1]
	assert.Empty(t, first.DebtorNumber)

	confirm := holidayConfirmPath(f.period.ID, f.siblings)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, confirm, map[string]any{"child_debtor_numbers": []map[string]any{
		{"child_id": first.ChildID, "debtor_number": "60101"}, {"child_id": second.ChildID, "debtor_number": " "},
	}})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, []string{fmt.Sprintf("child_debtor_number_%d", second.ChildID)}, fields, "the Träger asks for a number of every child")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, confirm, map[string]any{"child_debtor_numbers": []map[string]any{
		{"child_id": first.ChildID, "debtor_number": "60101"}, {"child_id": first.ChildID + second.ChildID, "debtor_number": "1"},
	}})
	require.Equal(t, http.StatusNotFound, rec.Code, "a child of another registration: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, confirm, map[string]any{"child_debtor_numbers": []map[string]any{
		{"child_id": first.ChildID, "debtor_number": "60101"}, {"child_id": second.ChildID, "debtor_number": "60102"},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, f.again),
		map[string]any{"debtor_number": "70001"}).Code, "a registration of one child keeps its single number")

	stored := holidayChildNumbers(t, f, f.siblings)
	assert.Equal(t, "60101", stored.Children[0].DebtorNumber, "the child keeps its number")
	assert.Equal(t, "60102", stored.Children[1].DebtorNumber)

	// The office corrects a number after the confirmation.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, holidayContributionPath(f.period.ID, f.siblings)+"/child-debtor-numbers",
		map[string]any{"children": []map[string]any{{"child_id": second.ChildID, "debtor_number": "60103"}}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	setNumberRange(t, f, billing, "FE-{NNNN}")
	released := releasePerChildRun(t, f, billing)
	type issued struct {
		registration, child int64
		debtor              string
		children            int
	}
	var got []issued
	for _, document := range released.Documents {
		got = append(got, issued{document.RegistrationID, document.RegistrationChildID, document.DebtorNumber, len(document.Children)})
	}
	assert.Equal(t, []issued{
		{f.siblings, first.ChildID, "60101", 1}, {f.siblings, second.ChildID, "60103", 1}, {f.again, againChild(t, f), "70001", 1},
	}, got, "each child its invoice with its number")

	// A cancelled invoice keeps its child number after switching to payer scope.
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "payer")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(released.ID, released.Documents[1].ID),
		map[string]any{"reason": "Falsche Debitorennummer"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberOnConfirm, false)
	newRegistration := f.submitChild(t, f.period, f.offer, "eva.muster@example.org", first.FirstName, 0)
	rec = f.confirmWith(t, newRegistration, map[string]any{"debtor_number": "80001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.changeDebtorNumber(t, f.office.token, newRegistration, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := perChildRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	require.Len(t, again.Documents, 2)
	assert.Zero(t, again.Documents[1].RegistrationChildID)
	assert.Equal(t, "80001", again.Documents[1].DebtorNumber)
	assert.Equal(t, second.ChildID, again.Documents[0].RegistrationChildID)
	assert.Equal(t, "60103", again.Documents[0].DebtorNumber)
	assert.Equal(t, released.Documents[1].Number, again.Documents[0].ReferenceNumber)
}

// againChild is the only child of the second registration of the fixture.
func againChild(t *testing.T, f reviewFixture) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM enrollment.holiday_registration_children WHERE holiday_registration_id = ?`, f.again).
		Scan(context.Background(), &id))
	return id
}

// A registration billed as a whole before the switch stays one invoice: its
// Storno is issued again as one invoice, and the contribution counts it as
// billed.
func TestHolidayInvoiceOfAWholeRegistrationKeepsItsShapeAfterTheSwitch(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienrechnung Umstellung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	released := releasePerChildRun(t, f, billing)
	require.Len(t, released.Documents, 1)
	assert.Zero(t, released.Documents[0].RegistrationChildID)

	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, f.siblings), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), `"correction_needed":true`, "the invoice of the whole still bills it")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(released.ID, released.Documents[0].ID),
		map[string]any{"reason": "Anschrift falsch"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	again := perChildRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	require.Len(t, again.Documents, 1, "the invoice of the whole is issued again as one")
	assert.Zero(t, again.Documents[0].RegistrationChildID)
	assert.Len(t, again.Documents[0].Children, 2)
}

func TestHolidayConfirmationRequiresChildNumbersWithoutAnArray(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Pflichtnummern ohne Kinderliste")
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberOnConfirm, true)
	rec := f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "70001"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, fields := registrationRefusalOf(t, rec)
	proposal := holidayChildNumbers(t, f, f.siblings)
	for _, child := range proposal.Children {
		assert.Contains(t, fields, fmt.Sprintf("child_debtor_number_%d", child.ChildID))
	}
}

func TestHolidayChildNumbersKeepUnchangedLegacyValues(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Alte Kindernummern")
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	proposal := holidayChildNumbers(t, f, f.siblings)
	first, second := proposal.Children[0], proposal.Children[1]
	inputs := []map[string]any{{"child_id": first.ChildID, "debtor_number": "OLD-1"}, {"child_id": second.ChildID, "debtor_number": "OLD-2"}}
	rec := f.confirmWith(t, f.siblings, map[string]any{"child_debtor_numbers": inputs})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	fiveDigits(t, f)
	path := holidayContributionPath(f.period.ID, f.siblings) + "/child-debtor-numbers"
	inputs[1]["debtor_number"] = "60102"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"children": inputs})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "OLD-1", holidayChildNumbers(t, f, f.siblings).Children[0].DebtorNumber)
	inputs[0]["debtor_number"] = "NEW-1"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, map[string]any{"children": inputs})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// A later registration of the same children submits the stored proposal too.
	repeated := f.submitChild(t, f.period, f.offer, "eva.muster@example.org", first.FirstName, 0)
	children := holidayChildNumbers(t, f, repeated)
	require.Len(t, children.Children, 1)
	rec = f.confirmWith(t, repeated, map[string]any{"child_debtor_numbers": []map[string]any{
		{"child_id": children.Children[0].ChildID, "debtor_number": "OLD-1"},
	}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestHolidayChildNumberRemovalsAreBatched(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Kindernummern gemeinsam entfernen")
	setTraegerSetting(t, f, configModels.KeyParentBillingDebtorNumberScope, "child")
	proposal := holidayChildNumbers(t, f, f.siblings)
	var inputs []map[string]any
	for i, child := range proposal.Children {
		inputs = append(inputs, map[string]any{"child_id": child.ChildID, "debtor_number": fmt.Sprint(60101 + i)})
	}
	rec := f.confirmWith(t, f.siblings, map[string]any{"child_debtor_numbers": inputs})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, input := range inputs {
		input["debtor_number"] = ""
	}
	counter := testpkg.CaptureQueriesForContext(t, f.db)
	tracked := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.traeger.ServeHTTP(w, r.WithContext(counter.Context(r.Context())))
	})
	rec = traegerJSON(t, tracked, f.office.token, http.MethodPut,
		holidayContributionPath(f.period.ID, f.siblings)+"/child-debtor-numbers", map[string]any{"children": inputs})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Len(t, counter.Matching(func(sql string) bool {
		return strings.Contains(sql, "update parent_billing.child_debtor_numbers as child")
	}), 1, "all removals use one update")
	assert.Len(t, counter.Matching(func(sql string) bool {
		return strings.Contains(sql, "insert into parent_billing.child_debtor_number_events")
	}), 1, "all removal events use one insert")
	for _, child := range holidayChildNumbers(t, f, f.siblings).Children {
		assert.Empty(t, child.DebtorNumber)
	}
	var removed int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.child_debtor_number_events
		WHERE organization_id = ? AND action = 'delete'`, f.office.organization).Scan(context.Background(), &removed))
	assert.Equal(t, len(inputs), removed, "every cleared number gets its event")
}
