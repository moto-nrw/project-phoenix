// Router-level tests for the debtor number per child (#7007): with the
// Träger setting "Debitorennummer gilt je: Kind" the list of families names
// the children of a family's contracts before any invoice, the office sets
// a number per child, the monthly run copies each child's own number, the
// mandate import gives siblings different numbers without a conflict, and no
// other Träger sees or sets them. With the default nothing changes.
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

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const childDebtorNumberPath = "/traeger/billing-families/child-debtor-number"

type listedChild struct {
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	DebtorNumber string `json:"debtor_number"`
}

type childListedFamily struct {
	FamilyEmail        string        `json:"family_email"`
	RecipientFirstName string        `json:"recipient_first_name"`
	Children           []listedChild `json:"children"`
}

// siblingContracts stores two contracts of one family, Mia and Noah, and
// answers them with the family's address.
func siblingContracts(t *testing.T, f careBillingFixture, school int64) (mia, noah int64, family string) {
	t.Helper()
	mia = careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	noah = careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
	family = partyEmail(t, f, mia)
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = ? WHERE contract_id = ?`, family, noah)
	require.NoError(t, err)
	return mia, noah, family
}

// childScope sets the Träger to a number per child, as its administration.
func childScope(t *testing.T, f careBillingFixture) {
	t.Helper()
	admin := traegerColleague(t, reviewFixture{db: f.db, traeger: f.traeger, office: f.office}, identityaccess.TraegerRoleAdministration)
	rec := traegerJSON(t, f.traeger, admin, http.MethodPut, "/traeger/settings/values/"+configModels.KeyParentBillingDebtorNumberScope,
		map[string]any{"value": "child"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func listChildFamilies(t *testing.T, f careBillingFixture, token string) []childListedFamily {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, "/traeger/billing-families/children", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]childListedFamily](t, rec.Body.Bytes())
}

func setChildNumber(t *testing.T, f careBillingFixture, token, family, child, number string) int {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodPut, childDebtorNumberPath, map[string]any{"family_email": family,
		"child_first_name": child, "child_last_name": "Beispiel", "child_birth_date": nil, "debtor_number": number}).Code
}

func TestChildDebtorNumbersReachTheMonthlyInvoiceOfEachChild(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Nummer je Kind")
	school := f.office.schools[0]
	mia, noah, family := siblingContracts(t, f, school)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, careMonth, 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, debtorNumberFormatPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"scope":"payer"`, "the payer is the default")
	childScope(t, f)
	assert.Equal(t, []childListedFamily{{FamilyEmail: family, RecipientFirstName: "Eva", Children: []listedChild{
		{FirstName: "Mia", LastName: "Beispiel"}, {FirstName: "Noah", LastName: "Beispiel"},
	}}}, listChildFamilies(t, f, f.office.token), "the family of the contracts is listed with both children before any invoice")

	assert.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, " "+strings.ToUpper(family), "mia ", "10 0001"))
	assert.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, family, "Noah", "10 0002"))
	assert.Equal(t, http.StatusBadRequest, setChildNumber(t, f, f.office.token, family, "Noah", "10\n0002"), "one line, as at the family")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, childDebtorNumberPath, map[string]any{"family_email": family,
		"child_first_name": "Lia", "child_last_name": "Beispiel", "debtor_number": "1"})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, common.CodeParentBillingChildNotFound, decodeProblem(t, rec).Code, "a child of no contract of the family")
	assert.Equal(t, []listedChild{{"Mia", "Beispiel", "10 0001"}, {"Noah", "Beispiel", "10 0002"}},
		listChildFamilies(t, f, f.office.token)[0].Children, "the names stay as the contract has them")
	var lastUsed string
	require.NoError(t, f.db.NewRaw(`SELECT MIN(last_used_on)::text FROM parent_billing.child_debtor_numbers WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &lastUsed))
	assert.Greater(t, lastUsed, careMonth, "a number prepared for a contract ahead is kept until the contract needs it")

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	numbers := map[int64]string{}
	for _, invoice := range decodeSchoolPortalData[struct {
		Invoices []struct {
			ContractID   int64  `json:"contract_id"`
			DebtorNumber string `json:"debtor_number"`
		} `json:"invoices"`
	}](t, rec.Body.Bytes()).Invoices {
		numbers[invoice.ContractID] = invoice.DebtorNumber
	}
	assert.Equal(t, map[int64]string{mia: "10 0001", noah: "10 0002"}, numbers, "each sibling's invoice carries the child's own number")

	assert.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, family, "Noah", ""), "an empty number removes it")
	assert.Equal(t, []listedChild{{"Mia", "Beispiel", "10 0001"}, {"Noah", "Beispiel", ""}}, listChildFamilies(t, f, f.office.token)[0].Children)
	var trail []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT action FROM parent_billing.child_debtor_number_events WHERE organization_id = ? ORDER BY action`,
		f.office.organization).Scan(context.Background(), &trail))
	assert.Equal(t, []string{"delete", "read", "save"}, trail, "every read and change is in the trail")

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Nummer je Kind Fremd", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, "/traeger/billing-families/children", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "10 0001", "another Träger sees no number")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPut, childDebtorNumberPath, map[string]any{"family_email": family,
		"child_first_name": "Mia", "child_last_name": "Beispiel", "debtor_number": "9"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "nor sets one: %s", rec.Body.String())
}

// With a number per child the import gives each sibling the number of its
// row: through the contract, or the address of a family of one child. An
// address of a family of several children names no child.
func TestSEPAMandateImportTakesOverDebtorNumbersPerChild(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Debitorenimport je Kind")
	school := f.office.schools[0]
	mia, noah, family := siblingContracts(t, f, school)
	lia := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	childScope(t, f)

	byContract := strings.Join([]string{
		"Mandatsreferenz;Unterschriftsdatum;IBAN;Kontoinhaber;Vertrag;Lastschriftart;kinder-Debitorennummer",
		fmt.Sprintf("OGS-K1;15.08.2024;DE89 3704 0044 0532 0130 00;Eva Beispiel;%d;FRST;20 0001", mia),
		fmt.Sprintf("OGS-K2;15.08.2024;%s;Eva Beispiel;%d;FRST;20 0002", sepaIBAN, noah),
	}, "\r\n") + "\r\n"
	fields := map[string]string{"mapping": `{"columns":{"reference":"Mandatsreferenz","signed_on":"Unterschriftsdatum","iban":"IBAN",` +
		`"account_holder":"Kontoinhaber","payer":"Vertrag","sequence":"Lastschriftart","debtor_number":"kinder-Debitorennummer"},` +
		`"payer_key":"contract_id","creditor_from_traeger":true,"confirmed_creditor_id":"DE98ZZZ09999999999"}`}
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs.csv", []byte(byContract), fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), `"debtor_number_status":"siblings_differ"`, "siblings may differ")
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, 2, preview.Counts["debtor_numbers"])
	assert.Zero(t, preview.Counts["debtor_conflicts"])
	rec = applyImport(t, f.traeger, f.office.token, "ogs.csv", []byte(byContract), fields)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	byAddress := strings.Join([]string{
		"Mandatsreferenz;Unterschriftsdatum;IBAN;Kontoinhaber;E-Mail;Lastschriftart;kinder-Debitorennummer",
		fmt.Sprintf("OGS-K3;15.08.2024;%s;Eva Beispiel;%s;FRST;20 0003", sepaIBAN, family),
		fmt.Sprintf("OGS-K4;15.08.2024;%s;Eva Beispiel;%s;FRST;30 0001", sepaIBAN, partyEmail(t, f, lia)),
	}, "\r\n") + "\r\n"
	fields["mapping"] = strings.NewReplacer(`"payer":"Vertrag"`, `"payer":"E-Mail"`, `"payer_key":"contract_id"`, `"payer_key":"email"`).
		Replace(fields["mapping"])
	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs.csv", []byte(byAddress), fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"no_child"`, "the address of two children names neither")
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"new"`, "the address of one child names it")

	assert.Equal(t, []listedChild{{"Mia", "Beispiel", "20 0001"}, {"Noah", "Beispiel", "20 0002"}},
		childrenOf(listChildFamilies(t, f, f.office.token), family), "the import gave each sibling its own number")

	fields["mapping"] = strings.NewReplacer(`"payer":"E-Mail"`, `"payer":"Vertrag"`, `"payer_key":"email"`, `"payer_key":"contract_id"`).
		Replace(fields["mapping"])
	var reads int
	countReads := func() int {
		require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.child_debtor_number_events WHERE organization_id = ? AND action = 'read'`,
			f.office.organization).Scan(context.Background(), &reads))
		return reads
	}
	before := countReads()
	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs.csv", []byte(byContract), fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"same"`)
	assert.Equal(t, before+2, countReads(), "comparing with the stored numbers is in their trail")
	var familyNumbers int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.family_billing WHERE organization_id = ? AND debtor_number IS NOT NULL`,
		f.office.organization).Scan(context.Background(), &familyNumbers))
	assert.Zero(t, familyNumbers, "no family number")
}

func childrenOf(families []childListedFamily, family string) []listedChild {
	for _, listed := range families {
		if listed.FamilyEmail == family {
			return listed.Children
		}
	}
	return nil
}

func TestFamilyChildrenQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Budget Kinder")
	school := f.office.schools[0]
	_, _, family := siblingContracts(t, f, school)
	childScope(t, f)
	require.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, family, "Mia", "10 0001"))
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, "/traeger/billing-families/children", nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	children := list()
	_, _, other := siblingContracts(t, f, school)
	require.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, other, "Noah", "20 0002"))
	assert.Len(t, list().Statements, len(children.Statements), "the children of any number of families and contracts take the same statements")
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_family_children", children)
}

// A preview confirmed while the Träger kept the number per payer does not
// import into the children once it keeps it per child: the office looks
// again.
func TestSEPAMandateImportPreviewChangesWithTheScope(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Debitorenimport Umstellung")
	lia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	csv := []byte(strings.Join([]string{
		"Mandatsreferenz;Unterschriftsdatum;IBAN;Kontoinhaber;Vertrag;Lastschriftart;kinder-Debitorennummer",
		fmt.Sprintf("OGS-U1;15.08.2024;%s;Eva Beispiel;%d;FRST;40 0001", sepaIBAN, lia),
	}, "\r\n") + "\r\n")
	fields := map[string]string{"mapping": `{"columns":{"reference":"Mandatsreferenz","signed_on":"Unterschriftsdatum","iban":"IBAN",` +
		`"account_holder":"Kontoinhaber","payer":"Vertrag","sequence":"Lastschriftart","debtor_number":"kinder-Debitorennummer"},` +
		`"payer_key":"contract_id","creditor_from_traeger":true,"confirmed_creditor_id":"DE98ZZZ09999999999"}`}
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs.csv", csv, fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	require.Contains(t, rec.Body.String(), `"debtor_number_status":"new"`)

	childScope(t, f)
	confirmed := map[string]string{"fingerprint": preview.Fingerprint, "mapping": fields["mapping"]}
	rec = importForm(t, f.traeger, f.office.token, sepaImportPath, "ogs.csv", csv, confirmed)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_import_changed", code)
}
