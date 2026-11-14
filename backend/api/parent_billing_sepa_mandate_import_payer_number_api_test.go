// Router-level tests for the payer by debtor number (#7009): a file in the
// shape of the OGS Connect master data names no contract and no address,
// only the debtor number of each row. The import finds the payer by the
// number moto keeps, the child's or the family's by the Träger's setting,
// stores one mandate for siblings with the same account and reference, and
// records every number it reads in its trail.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ogsConnectMapping maps a file in the shape of the OGS Connect master data
// with the payer column on the debtor number.
func ogsConnectMapping() map[string]string {
	return map[string]string{"mapping": `{"columns":{"reference":"Mandatsreferenz","signed_on":"Unterschriftsdatum","iban":"IBAN",` +
		`"account_holder":"Kontoinhaber","payer":"Debitor-Nr.","sequence":"Lastschriftart","debtor_number":"Debitor-Nr."},` +
		`"payer_key":"debtor_number","creditor_from_traeger":true,"confirmed_creditor_id":"DE98ZZZ09999999999"}`}
}

func ogsConnectFile(rows ...string) []byte {
	return []byte(strings.Join(append([]string{"Nachname Kind;Vorname Kind;Debitor-Nr.;Kontoinhaber;IBAN;Mandatsreferenz;Unterschriftsdatum;Lastschriftart"},
		rows...), "\r\n") + "\r\n")
}

func countMandates(t *testing.T, f careBillingFixture) int {
	t.Helper()
	var mandates int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.sepa_mandates WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &mandates))
	return mandates
}

func TestSEPAMandateImportFindsThePayerByTheChildsDebtorNumber(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Zahler über Kindernummer")
	_, _, family := siblingContracts(t, f, f.office.schools[0])
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	childScope(t, f)
	require.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, family, "Mia", "60101"))
	require.Equal(t, http.StatusOK, setChildNumber(t, f, f.office.token, family, "Noah", "60102"))

	file := ogsConnectFile(
		fmt.Sprintf("Beispiel;Mia;60101;Eva Beispiel;%s;OGS-S1;15.08.2024 10:12;RCUR", sepaIBAN),
		fmt.Sprintf("Beispiel;Noah;60102;Eva Beispiel;%s;OGS-S1;15.08.2024 10:12;RCUR", sepaIBAN),
		fmt.Sprintf("Fremd;Ole;99999;Ole Fremd;%s;OGS-S2;15.08.2024;RCUR", sepaIBAN),
	)
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "stammdaten.csv", file, ogsConnectMapping())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	require.Len(t, preview.Rows, 3)
	assert.Equal(t, "ready", preview.Rows[0].Status, "the child's number names the contract of the child")
	assert.NotZero(t, preview.Rows[0].ContractID)
	assert.Equal(t, "same_mandate", preview.Rows[1].Status, "the sibling repeats the family's mandate")
	assert.Contains(t, rec.Body.String(), `"same_mandate_line":2`)
	assert.Equal(t, "invalid", preview.Rows[2].Status)
	assert.Equal(t, "payer_unknown", preview.Rows[2].Problems[0].Code, "a number moto does not keep names no payer")
	assert.Equal(t, 1, preview.Counts["same_mandate"])
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.child_debtor_number_events WHERE organization_id = ? AND action = 'read'`,
		f.office.organization).Scan(context.Background(), &reads))
	assert.GreaterOrEqual(t, reads, 2, "the numbers the rows name are read in their trail")

	rec = applyImport(t, f.traeger, f.office.token, "stammdaten.csv", file, ogsConnectMapping())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 1, countMandates(t, f), "siblings with one account and one reference have one mandate")

	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "stammdaten.csv", file, ogsConnectMapping())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, "duplicate", again.Rows[0].Status, "a repeated import skips the stored mandate")
	assert.Equal(t, "duplicate", again.Rows[1].Status, "also in the sibling's row")
}

func TestSEPAMandateImportFindsThePayerByTheFamilysDebtorNumber(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Zahler über Familiennummer")
	mia, _, _ := siblingContracts(t, f, f.office.schools[0])
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath,
		map[string]any{"contract_id": mia, "recipient_kind": "applicant", "debtor_number": "70001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	file := ogsConnectFile(
		fmt.Sprintf("Beispiel;Mia;70001;Eva Beispiel;%s;OGS-F1;15.08.2024;RCUR", sepaIBAN),
		fmt.Sprintf("Beispiel;Noah;70001;Eva Beispiel;%s;OGS-F1;15.08.2024;RCUR", sepaIBAN),
	)
	rec = applyImport(t, f.traeger, f.office.token, "stammdaten.csv", file, ogsConnectMapping())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 1, countMandates(t, f), "the family's number names the family; both rows give its one mandate")
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.family_billing_events WHERE organization_id = ? AND action = 'read'`,
		f.office.organization).Scan(context.Background(), &reads))
	assert.Positive(t, reads, "the family's number read is in its trail")
}
