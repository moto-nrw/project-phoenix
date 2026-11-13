// Router-level tests of withdrawal and termination (#4682) on a contract the
// parents signed (#4679): the signature starts the withdrawal period and
// carries the early start, and an ended contract takes no signature.
package api

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// A contract signed online is withdrawn within the 14 days that start with
// the signature, whose acknowledged notice the parents read, and with the
// early start they asked for when they signed. Withdrawn, it takes no paper
// contract. Red without the signature in the notice state: the period would
// run 12 months and 14 days and the early start would be lost.
func TestSignedContractIsWithdrawnAndTakesNoFurtherSignature(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Unterschrift Widerruf")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)

	rec := publicSigning(t, f.traeger, http.MethodPost, link+"/sign", map[string]any{"method": "drawn",
		"image_png": signatureImage(t), "withdrawal_notice_acknowledged": true, "early_start_requested": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// The signature concluded the contract; its Berlin day is the day of
	// the signature, read from the row rather than the wall clock.
	signedOn := concludedOn(t, f, contract)

	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	options := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusActive, options.Status, "the signature concluded the contract")
	assert.True(t, options.EarlyStart, "the early start of the signature counts")
	require.NotNil(t, options.WithdrawalUntil)
	base := signedOn.AddDays(carecontract.WithdrawalDays)
	assert.False(t, options.WithdrawalUntil.Before(base), "never shorter than 14 days from the signature")
	assert.LessOrEqual(t, base.DaysUntil(*options.WithdrawalUntil), 4, "14 days from the signature, moved by weekend and holidays only")
	detail := signedDetail(t, f, contract)
	require.NotNil(t, detail.Signature)
	require.NotNil(t, detail.Signature.WithdrawalEndsOn)
	assert.Equal(t, options.WithdrawalUntil.String(), *detail.Signature.WithdrawalEndsOn,
		"the signature and the notice page name the same last day")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, path, map[string]any{"kind": "widerruf", "received_on": signedOn.String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, carecontract.StatusWithdrawn, decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes()).Status)

	rec = paperUpload(t, f, f.office.token, contract, "vertrag.pdf", []byte("%PDF-1.4 unterschrieben"))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_signable")
	assert.Equal(t, carecontract.StatusWithdrawn, signedDetail(t, f, contract).Status)
}

// A sent contract the parents signed on paper is terminated like any other.
// Terminated, it takes no signature: neither the link of its mail, which
// the parents never used, nor a second paper contract. Several guards hold
// it: the status checks of the link, of the signature and of the paper
// contract, and behind them the one transition of #4680, which concludes a
// contract only once. Removing the first three alone leaves it green; the
// test pins the outcome, whichever guard refuses.
func TestPaperContractIsTerminatedAndTakesNoFurtherSignature(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Papier Kündigung")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	pdf := []byte("%PDF-1.4 unterschrieben")
	rec := paperUpload(t, f, f.office.token, contract, "vertrag.pdf", pdf)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	options := decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes())
	assert.Equal(t, carecontract.StatusActive, options.Status, "the paper contract concluded it")
	require.NotNil(t, options.OrdinaryEndsOn)

	today := concludedOn(t, f, contract).String()
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, path, map[string]any{"kind": "kuendigung",
		"termination_kind": "ordentlich", "received_on": today})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, carecontract.StatusTerminated, decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes()).Status)

	assertLinkClosed(t, f, link, "terminated")
	signature := map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true}
	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign", signature)
	assert.NotEqual(t, http.StatusOK, rec.Code, "the unused link signs no terminated contract: %s", rec.Body.String())
	rec = paperUpload(t, f, f.office.token, contract, "vertrag.pdf", pdf)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_signable")
	detail := signedDetail(t, f, contract)
	assert.Equal(t, carecontract.StatusTerminated, detail.Status)
	require.NotNil(t, detail.Signature)
	assert.Equal(t, carecontract.SignaturePaper, detail.Signature.Method, "the paper contract stays the only signature")
}

// assertLinkClosed checks that the unused link of an ended contract shows
// neither the contract nor its PDF (#4682): the status check of the link
// answers signing_link_invalid. Red without that check, which alone stands
// between the link and a read; the contract's PDF is still the one sent.
func assertLinkClosed(t *testing.T, f careContractFixture, link, ended string) {
	t.Helper()
	for _, path := range []string{link, link + "/document"} {
		rec := publicSigning(t, f.traeger, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusGone, rec.Code, "%s contract, GET %s: %s", ended, path, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "care_contract.signing_link_invalid", "%s contract, GET %s", ended, path)
	}
}

// A paper contract the parents got the withdrawal notice for (the mail with
// the PDF was delivered) is withdrawn within the period. Withdrawn, the
// unused link of that mail shows neither the contract nor its PDF.
func TestWithdrawnContractClosesTheLinkOfItsMail(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Papier Widerruf")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW(),
		delivery_state = 'delivered', delivery_state_at = NOW()
		WHERE organization_id = ? AND related_entity_type = 'care_contract' AND related_entity_id = ?`, f.office.organization, contract)
	require.NoError(t, err)
	rec := paperUpload(t, f, f.office.token, contract, "vertrag.pdf", []byte("%PDF-1.4 unterschrieben"))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = publicSigning(t, f.traeger, http.MethodGet, link, nil)
	assert.Equal(t, http.StatusGone, rec.Code, "a concluded contract closes the link already: %s", rec.Body.String())

	path := "/traeger/contract-notices/" + strconv.FormatInt(contract, 10)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NotNil(t, decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes()).WithdrawalUntil, "the delivered notice opens the period")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, path, map[string]any{"kind": "widerruf",
		"received_on": concludedOn(t, f, contract).String()})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, carecontract.StatusWithdrawn, decodeSchoolPortalData[noticeOptionsBody](t, rec.Body.Bytes()).Status)

	assertLinkClosed(t, f, link, "withdrawn")
}

// concludedOn is the Berlin day the contract was concluded, read from its
// row: the day a received declaration may carry at the earliest.
func concludedOn(t *testing.T, f careContractFixture, contract int64) calendar.Date {
	t.Helper()
	var day calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT (concluded_at AT TIME ZONE 'Europe/Berlin')::date FROM care_contract.contracts WHERE id = ?`,
		contract).Scan(context.Background(), &day))
	return day
}
