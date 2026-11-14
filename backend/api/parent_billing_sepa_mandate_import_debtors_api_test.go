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

// The import takes the column "kinder-Debitorennummer" of OGS Connect over
// per family (#4693, decision E-c): a family whose rows carry one number gets
// it, siblings with different numbers are a conflict and get none, and a
// family with another number keeps it.
func TestSEPAMandateImportTakesOverDebtorNumbersPerFamily(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Debitorenimport")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	noah := careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
	lia := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	ben := careContract(t, f.db, f.office.organization, school, "Ben", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath,
		map[string]any{"contract_id": lia, "recipient_kind": "applicant", "debtor_number": "30 0001"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	csv := strings.Join([]string{
		"Mandatsreferenz;Unterschriftsdatum;IBAN;Kontoinhaber;E-Mail;Lastschriftart;kinder-Debitorennummer",
		fmt.Sprintf("OGS-D1;15.08.2024;DE89 3704 0044 0532 0130 00;Eva Beispiel;%s;FRST;10 0001", partyEmail(t, f, mia)),
		fmt.Sprintf("OGS-D2;15.08.2024;%s;Eva Beispiel;%s;FRST;20 0001", sepaIBAN, partyEmail(t, f, noah)),
		fmt.Sprintf("OGS-D3;15.08.2024;%s;Eva Beispiel;%s;FRST;20 0002", sepaIBAN, partyEmail(t, f, noah)),
		fmt.Sprintf("OGS-D4;15.08.2024;%s;Eva Beispiel;%s;FRST;30 0002", sepaIBAN, partyEmail(t, f, lia)),
		fmt.Sprintf(";;;;%s;;70002", partyEmail(t, f, ben)),
		fmt.Sprintf("OGS-D5;15.08.2024;%s;Eva Beispiel;%s;FRST;70001", sepaIBAN, partyEmail(t, f, ben)),
	}, "\r\n") + "\r\n"
	fields := map[string]string{"mapping": `{"columns":{"reference":"Mandatsreferenz","signed_on":"Unterschriftsdatum","iban":"IBAN",` +
		`"account_holder":"Kontoinhaber","payer":"E-Mail","sequence":"Lastschriftart","debtor_number":"kinder-Debitorennummer"},` +
		`"payer_key":"email","creditor_from_traeger":true,"confirmed_creditor_id":"DE98ZZZ09999999999"}`}
	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs.csv", []byte(csv), fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"new"`)
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"siblings_differ"`)
	assert.Contains(t, rec.Body.String(), `"debtor_number_status":"differs"`)
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, 1, preview.Counts["debtor_numbers"])
	assert.Equal(t, 5, preview.Counts["debtor_conflicts"], "a sibling without a mandate still contributes its debtor number")
	assert.Equal(t, 1, preview.Counts["without_mandate"])
	assert.Zero(t, preview.Counts["invalid"])
	assert.Equal(t, 4, preview.Counts["ready"], "a sibling without a mandate does not reserve the payer")
	require.Len(t, preview.Rows, 6, "the mandate-free sibling shows its number conflict")
	assert.Equal(t, 6, preview.Rows[4].Line, "the mandate-free conflict names its file line")
	assert.Equal(t, 7, preview.Rows[5].Line, "the original file line is preserved")

	rec = applyImport(t, f.traeger, f.office.token, "ogs.csv", []byte(csv), fields)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	numbers := map[string]string{}
	rows, err := f.db.QueryContext(context.Background(), `SELECT family_email, COALESCE(debtor_number, '') FROM parent_billing.family_billing
		WHERE organization_id = ?`, f.office.organization)
	require.NoError(t, err)
	defer func() { require.NoError(t, rows.Close()) }()
	for rows.Next() {
		var family, number string
		require.NoError(t, rows.Scan(&family, &number))
		numbers[family] = number
	}
	require.NoError(t, rows.Err())
	assert.Equal(t, map[string]string{partyEmail(t, f, mia): "10 0001", partyEmail(t, f, lia): "30 0001"}, numbers,
		"one number taken over, the siblings' conflict and the existing number left alone")
	var events int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.family_billing_events WHERE organization_id = ? AND action = 'save'`,
		f.office.organization).Scan(context.Background(), &events))
	assert.Equal(t, 2, events, "the office's deviation and the imported number are in the trail")
}
