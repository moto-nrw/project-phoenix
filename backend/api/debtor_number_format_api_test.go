// Router-level tests for the format of a debtor number (#6675) and the
// follow-ups of the family's number (#6692): every path that writes a
// number the office entered checks the Träger's format, a stored number is
// never checked again, a correction of a typo needs no choice, two families
// of one sibling group never overwrite each other silently, and the form of
// a care contract never deletes what the holiday care remembered.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const debtorNumberFormatPath = "/traeger/billing-families/debtor-number-format"

// setTraegerValue writes one Träger setting through the Träger settings.
func setTraegerValue(t *testing.T, f reviewFixture, key string, value any) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// fiveDigits sets the format of Bissendorf: five digits, nothing else.
func fiveDigits(t *testing.T, f reviewFixture) {
	t.Helper()
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberDigitsOnly, true)
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberMinDigits, 5)
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberMaxDigits, 5)
}

// assertFormatRefused checks the refusal of a number in another format: a
// field error on debtor_number with the code and the values its text names.
func assertFormatRefused(t *testing.T, rec *httptest.ResponseRecorder, code string, details map[string]any) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	problem := decodeProblem(t, rec)
	assert.Equal(t, code, problem.Code)
	assert.Equal(t, details, problem.Details)
	_, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, []string{"debtor_number"}, fields)
}

func TestDebtorNumberFormatIsOffByDefault(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Format Aus")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, debtorNumberFormatPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"digits_only":false,"min_digits":0,"max_digits":0}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())))

	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711-A"}).Code,
		"without the setting any one-line text is a number, as before")
	assert.Equal(t, "10 4711-A", f.debtorNumberDetail(t, f.siblings).DebtorNumber)
}

func TestEveryPathChecksTheFormatOfTheTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Format Fünf")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, map[string]any{"debtor_number": "LEGACY 1"}).Code)
	fiveDigits(t, f)
	exact := map[string]any{"digits": float64(5), "example": "20815"}

	rec := confirmationSettings(t, f, f.office.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"debtor_number_format":{"digits_only":true,"min_digits":5,"max_digits":5}`,
		"the confirmation names the format before the office types")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, debtorNumberFormatPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"digits_only":true,"min_digits":5,"max_digits":5}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())))

	// Confirmation.
	assertFormatRefused(t, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20 815"}),
		common.CodeParentBillingDebtorNumberDigits, exact)
	assert.Equal(t, "received", f.debtorNumberDetail(t, f.siblings).Status, "nothing is confirmed")
	assert.Equal(t, "-", f.familyNumber(t, "eva.muster@example.org"), "and nothing remembered")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": " 20815 "}).Code)
	assert.Equal(t, "20815", f.familyNumber(t, "eva.muster@example.org"))

	// Change of a registration's number.
	assertFormatRefused(t, f.changeDebtorNumber(t, f.office.token, f.siblings, "2081"), common.CodeParentBillingDebtorNumberDigits, exact)
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.siblings).DebtorNumber)

	// The stored number in the old format stays, and is no reason to refuse
	// anything else: removing it is no number.
	assert.Equal(t, "LEGACY 1", f.debtorNumberDetail(t, f.again).DebtorNumber, "a stored number is never changed")
	require.Equal(t, http.StatusOK, f.changeDebtorNumber(t, f.office.token, f.again, "").Code)

	// The list of families.
	assertFormatRefused(t, traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "eva.muster@example.org", "debtor_number": "2081A"}), common.CodeParentBillingDebtorNumberDigits, exact)
	assert.Equal(t, "20815", f.familyNumber(t, "eva.muster@example.org"))

	// A range names both ends; the ends count in either order.
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberMinDigits, 8)
	setTraegerValue(t, f, configModels.KeyParentBillingDebtorNumberMaxDigits, 6)
	assertFormatRefused(t, traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "eva.muster@example.org", "debtor_number": "20815"}),
		common.CodeParentBillingDebtorNumberDigitRange, map[string]any{"min_digits": float64(6), "max_digits": float64(8), "example": "208154"})
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "eva.muster@example.org", "debtor_number": "2081547"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestContractFormChecksTheFormatAndKeepsTheHolidayFamily(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Format Vertrag")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'eva.muster@example.org'
		WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "40300"}).Code)
	holder := func() (string, bool) {
		t.Helper()
		var rows []struct {
			HolderFirstName *string `bun:"holder_first_name"`
			LastUsedOn      *string `bun:"last_used_on"`
		}
		require.NoError(t, f.db.NewRaw(`SELECT holder_first_name, last_used_on::text AS last_used_on FROM parent_billing.family_billing
			WHERE organization_id = ? AND family_email = ?`, f.office.organization, "eva.muster@example.org").Scan(context.Background(), &rows))
		if len(rows) == 0 || rows[0].HolderFirstName == nil {
			return "", false
		}
		return *rows[0].HolderFirstName, rows[0].LastUsedOn != nil
	}
	save := func(body map[string]any) *httptest.ResponseRecorder {
		body["contract_id"] = contract
		return traegerJSON(t, f.traeger, billing, http.MethodPut, familyBillingPath, body)
	}

	fiveDigits(t, f)
	assertFormatRefused(t, save(map[string]any{"recipient_kind": "applicant", "debtor_number": "40 300"}),
		common.CodeParentBillingDebtorNumberDigits, map[string]any{"digits": float64(5), "example": "20815"})
	assert.Equal(t, "40300", f.familyNumber(t, "eva.muster@example.org"))

	rec := save(map[string]any{"recipient_kind": "applicant", "payment_method": "direct_debit", "debtor_number": "40300"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// Back to the contract, with the number the form shows: the deviation
	// goes, the family keeps its number and what the holiday care knows.
	rec = save(map[string]any{"recipient_kind": "applicant", "debtor_number": "40300"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "40300", f.familyNumber(t, "eva.muster@example.org"))
	// Nothing deviates and the office removed the number: the row stays,
	// because the holiday care still needs the family (#6692).
	rec = save(map[string]any{"recipient_kind": "applicant", "debtor_number": ""})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "", f.familyNumber(t, "eva.muster@example.org"), "the number goes, as the office asked")
	name, used := holder()
	assert.Equal(t, "Eva", name, "the parent of the holiday registration stays")
	assert.True(t, used, "and the period of the family")
	assert.Contains(t, f.familyTrail(t, "eva.muster@example.org"), "save")
	assert.NotContains(t, f.familyTrail(t, "eva.muster@example.org"), "delete")
}

// A wrong digit of a number the family took from this registration is a
// correction: registration and family change together, without the choice.
func TestCorrectingATypoNeedsNoChoice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Tippfehler")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	later := f.submitFamily(t, "eva.muster@example.org")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20851"}).Code)
	require.Equal(t, "20851", f.familyNumber(t, "eva.muster@example.org"))

	rec := f.changeDebtorNumber(t, f.office.token, f.siblings, "20815")
	require.Equal(t, http.StatusOK, rec.Code, "the family had the number of this registration: %s", rec.Body.String())
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.siblings).DebtorNumber)
	assert.Equal(t, "20815", f.familyNumber(t, "eva.muster@example.org"), "the family takes the correction")
	assert.Equal(t, "20815", f.proposedNumber(t, later))

	// Another remembered number still asks.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "eva.muster@example.org", "debtor_number": "30100"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.changeDebtorNumber(t, f.office.token, f.siblings, "20816")
	require.Equal(t, http.StatusConflict, rec.Code, "the family's number came from elsewhere: %s", rec.Body.String())
	assert.Equal(t, "30100", f.familyNumber(t, "eva.muster@example.org"))
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.siblings).DebtorNumber)
}

// Two families in one sibling group, each with its own number: the group's
// number never overwrites one of them silently, and the refusal names the
// family whose number differs.
func TestSiblingGroupOfTwoFamiliesNamesTheFamilyInConflict(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Familie Zwei Nummern")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	require.Equal(t, http.StatusOK, f.merge(t, f.office.token, f.siblings, f.again).Code)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, billingFamilyDebtorNumberPath,
		map[string]any{"family_email": "papa.muster@example.org", "debtor_number": "30100"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20815"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	refusal := decodeProblem(t, rec)
	assert.Equal(t, common.CodeParentBillingFamilyDebtorNumberConflict, refusal.Code)
	assert.Equal(t, map[string]any{"family_debtor_number": "30100", "debtor_number": "20815", "family_email": "papa.muster@example.org"},
		refusal.Details, "the refusal names the sibling's family and its number")
	assert.Equal(t, "received", f.debtorNumberDetail(t, f.siblings).Status)
	assert.Equal(t, "30100", f.familyNumber(t, "papa.muster@example.org"), "no silent overwrite")
	assert.Equal(t, "-", f.familyNumber(t, "eva.muster@example.org"))

	rec = f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "20815", "family_debtor_number": "keep"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "30100", f.familyNumber(t, "papa.muster@example.org"), "keep leaves the sibling's family")
	assert.Equal(t, "20815", f.familyNumber(t, "eva.muster@example.org"), "the family without a number takes it")
	assert.Equal(t, "20815", f.debtorNumberDetail(t, f.again).DebtorNumber, "the group pays with one number")
}

func TestFamilyDebtorNumberWriteQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Budget Familiennummer")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	set := func(number string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodPut, billingFamilyDebtorNumberPath,
			strings.NewReader(fmt.Sprintf(`{"family_email":"eva.muster@example.org","debtor_number":%q}`, number)))
		req.Header.Set("Authorization", "Bearer "+billing)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	first := set("20815")
	for i := range 3 {
		require.Equal(t, http.StatusOK, f.confirmWith(t, f.submitFamily(t, fmt.Sprintf("weitere%d@example.org", i)), nil).Code)
	}
	assert.Len(t, set("20816").Statements, len(first.Statements), "the write reads and writes the same statements for any number of families")
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_family_debtor_number", first)
}
