// Router-level tests for the signature of the Betreuungsvertrag (#4679):
// the one-time mail link, the paper contract, the check of the checksums,
// the right "Verträge verwalten", the Träger boundary and the reminders.
package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	careContractCompose "github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careContractSignatureBody struct {
	Method         string `json:"method"`
	Channel        string `json:"channel"`
	SignerName     string `json:"signer_name"`
	DocumentSHA256 string `json:"document_sha256"`
	Intact         bool   `json:"intact"`
	PaperFilename  string `json:"paper_filename"`
	// WithdrawalEndsOn is the last day of the withdrawal period the
	// signature names (#4682 compares it with the notice page).
	WithdrawalEndsOn *string `json:"withdrawal_ends_on"`
}

type careContractSignedDetailBody struct {
	Status    string                     `json:"status"`
	Signature *careContractSignatureBody `json:"signature"`
	Reminders []struct {
		Number int `json:"number"`
	} `json:"reminders"`
}

func signatureImage(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 60, 20))
	for x := 5; x < 55; x++ {
		img.Set(x, 10, color.NRGBA{A: 255})
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return base64.StdEncoding.EncodeToString(out.Bytes())
}

// sentCareContract creates and sends the contract of a child and answers
// its ID and the one-time link of the mail.
func sentCareContract(t *testing.T, f careContractFixture, child int64) (int64, string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts",
		map[string]any{"contracts": []any{map[string]any{"school_id": f.school, "request_child_id": child}}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes())
	require.Len(t, created.Created, 1, rec.Body.String())
	id := created.Created[0].ID
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts/send", map[string]any{"contract_ids": []int64{id}})
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	return id, lastSigningLink(t, f, id)
}

// lastSigningLink reads the link of the newest mail of the contract.
func lastSigningLink(t *testing.T, f careContractFixture, contractID int64) string {
	t.Helper()
	var payload string
	require.NoError(t, f.db.NewRaw(`SELECT payload::text FROM platform.email_outbox WHERE organization_id = ? AND kind = 'care_contract'
		AND related_entity_id = ? ORDER BY id DESC LIMIT 1`, f.office.organization, contractID).Scan(context.Background(), &payload))
	var mail struct {
		SigningURL string `json:"signing_url"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &mail))
	_, link, found := strings.Cut(mail.SigningURL, "/vertrag-unterschreiben/")
	require.True(t, found, "the mail carries the link to sign: %s", mail.SigningURL)
	return link
}

func publicSigning(t *testing.T, router http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var payload bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&payload).Encode(body))
	}
	req := httptest.NewRequest(method, "/public/care-contract-signing/"+path, &payload)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func signedDetail(t *testing.T, f careContractFixture, id int64) careContractSignedDetailBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careContractSignedDetailBody](t, rec.Body.Bytes())
}

// TestCareContractSigningPageNamesTheTraegersPhoneAndEmail pins #6200: the
// public signing link has no account of the Träger, yet its withdrawal
// notice names phone and e-mail like the PDF does (Gestaltungshinweis 2 zu
// Anlage 1 zu Art. 246a EGBGB), the Träger's own for withdrawals before the
// contact person's.
func TestCareContractSigningPageNamesTheTraegersPhoneAndEmail(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Widerrufskontakt")
	testpkg.OwnTraegerMasterData(t, f.db, f.office.organization)
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO platform.traeger_profiles
		(organization_id, contact_name, contact_phone, contact_email, withdrawal_email)
		VALUES (?, 'Frau Beispiel', '0211 123 45', 'frau.beispiel@traeger.example', 'widerruf@traeger.example')`, f.office.organization)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	_, link := sentCareContract(t, f, f.child)

	rec := publicSigning(t, f.traeger, http.MethodGet, link, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	view := decodeSchoolPortalData[struct {
		WithdrawalNotice []string `json:"withdrawal_notice"`
	}](t, rec.Body.Bytes())
	require.Greater(t, len(view.WithdrawalNotice), 1)
	assert.Contains(t, view.WithdrawalNotice[1], "Telefon: 0211 123 45, E-Mail: widerruf@traeger.example)",
		"the contact person's phone, the Träger's own e-mail for withdrawals")
	assert.NotContains(t, rec.Body.String(), "frau.beispiel@traeger.example", "the own e-mail replaces the contact person's")
}

// TestCareContractMailLinkSignsOnceOnlyItsContract pins the invariants of
// the one-time link (#4679): reading leaves it open, signing needs the
// withdrawal notice and a visible signature, the link signs exactly once,
// names only its contract and its Träger, and expires. The Träger-Büro then
// reads the signature with its checksums, and a changed ledger entry shows
// as broken.
func TestCareContractMailLinkSignsOnceOnlyItsContract(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Unterschrift")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	sibling := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Ben", "erika.muster@example.org")
	other, otherLink := sentCareContract(t, f, sibling)
	signature := map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true}

	rec := publicSigning(t, f.traeger, http.MethodGet, link, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	view := decodeSchoolPortalData[struct {
		ContractID       int64    `json:"contract_id"`
		Status           string   `json:"status"`
		WithdrawalNotice []string `json:"withdrawal_notice"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, contract, view.ContractID, "the link names its contract")
	assert.Equal(t, "versendet", view.Status)
	assert.NotEmpty(t, view.WithdrawalNotice)
	assert.NotContains(t, rec.Body.String(), "child_birth_date", "the link without an account shows no birth date")
	assert.NotContains(t, rec.Body.String(), `"terms"`, "the terms stand in the PDF, not in the view")
	rec = publicSigning(t, f.traeger, http.MethodGet, link+"/document", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")), "the frozen PDF")

	// A token under another Träger, a forged token and a malformed link
	// open nothing.
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Unterschrift Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	_, token, _ := strings.Cut(link, ".")
	for name, forged := range map[string]string{
		"other Träger": fmt.Sprintf("%d.%s", foreign.organization, token),
		"forged token": fmt.Sprintf("%d.%s", f.office.organization, strings.Repeat("A", len(token))),
		"no Träger":    token,
	} {
		rec = publicSigning(t, f.traeger, http.MethodPost, forged+"/sign", signature)
		assert.Equal(t, http.StatusGone, rec.Code, "%s: %s", name, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "care_contract.signing_link_invalid", name)
	}

	withoutNotice := map[string]any{"method": "drawn", "image_png": signatureImage(t)}
	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign", withoutNotice)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.withdrawal_notice_required")
	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign", map[string]any{"method": "drawn", "image_png": "bm8=", "withdrawal_notice_acknowledged": true})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.signature_invalid")

	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign", signature)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"status":"aktiv"`, "the signature concludes the contract")

	// The link is used: neither reading nor signing works again.
	for _, attempt := range []struct{ method, path string }{{http.MethodGet, link}, {http.MethodPost, link + "/sign"}} {
		rec = publicSigning(t, f.traeger, attempt.method, attempt.path, signature)
		assert.Equal(t, http.StatusGone, rec.Code, "%s %s", attempt.method, rec.Body.String())
	}
	assert.Equal(t, "versendet", signedDetail(t, f, other).Status, "the link signed only its own contract")

	detail := signedDetail(t, f, contract)
	assert.Equal(t, "aktiv", detail.Status)
	require.NotNil(t, detail.Signature)
	assert.Equal(t, "mail_link", detail.Signature.Channel)
	assert.Equal(t, "Erika Muster", detail.Signature.SignerName)
	assert.True(t, detail.Signature.Intact, "ledger, PDF and version match their checksums")
	var mails int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE organization_id = ? AND related_entity_id = ?
		AND payload->>'purpose' = 'signed' AND payload->>'pdf' IS NOT NULL`, f.office.organization, contract).Scan(context.Background(), &mails))
	assert.Equal(t, 1, mails, "the confirmation carries the signed PDF")

	// Independent edits of displayed metadata must break the evidence check.
	for _, change := range []string{"signer_name = 'Anderer Name'", "signed_at = signed_at + INTERVAL '1 day'"} {
		_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signatures SET `+change+` WHERE contract_id = ?`, contract)
		require.NoError(t, err)
		assert.False(t, signedDetail(t, f, contract).Signature.Intact, "changed metadata: %s", change)
		_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signatures AS s
			SET signer_name = a.signer_name, signed_at = a.signed_at FROM audit.traeger_signatures AS a
			WHERE s.audit_signature_id = a.id AND s.contract_id = ?`, contract)
		require.NoError(t, err)
	}

	// A changed ledger entry no longer matches its record hash.
	_, err := f.db.ExecContext(context.Background(), `UPDATE audit.traeger_signatures SET signer_name = 'Jemand anderes'
		WHERE organization_id = ? AND care_contract_version_id = (SELECT contract_version_id FROM care_contract.contract_signatures WHERE contract_id = ?)`,
		f.office.organization, contract)
	require.NoError(t, err)
	assert.False(t, signedDetail(t, f, contract).Signature.Intact, "a tampered signature shows as broken")

	// An expired link opens nothing.
	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signing_links
		SET created_at = NOW() - INTERVAL '40 days', expires_at = NOW() - INTERVAL '1 day' WHERE contract_id = ?`, other)
	require.NoError(t, err)
	rec = publicSigning(t, f.traeger, http.MethodPost, otherLink+"/sign", signature)
	assert.Equal(t, http.StatusGone, rec.Code, rec.Body.String())
	assert.Equal(t, "versendet", signedDetail(t, f, other).Status)
}

func paperUpload(t *testing.T, f careContractFixture, token string, id int64, filename string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	require.NoError(t, form.WriteField("signer_name", "Erika Muster"))
	require.NoError(t, form.WriteField("signed_on", "2026-10-30"))
	part, err := form.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, form.Close())
	req := httptest.NewRequest(http.MethodPost, contractPath(id, "/paper-signature"), &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	return rec
}

// TestCareContractSignedOnPaperNeedsAPDFAndTheRight pins the paper contract
// (#4679, option D): only a PDF counts, only the right "Verträge verwalten"
// of the own Träger records it, it concludes a draft with its frozen PDF,
// and a concluded contract signs no second time.
func TestCareContractSignedOnPaperNeedsAPDFAndTheRight(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Papier")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	draft := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0].ID
	pdf := []byte("%PDF-1.4 unterschriebener Vertrag")

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	assert.Equal(t, http.StatusForbidden, paperUpload(t, f, billing, draft, "vertrag.pdf", pdf).Code, "the right is required")
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Papier Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, foreign.organization)
	assert.Equal(t, http.StatusNotFound, paperUpload(t, f, foreign.token, draft, "vertrag.pdf", pdf).Code, "another Träger reaches nothing")

	rec = paperUpload(t, f, f.office.token, draft, "vertrag.pdf", []byte("kein PDF"))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.paper_document_invalid", "the content decides, not the name")
	rec = paperUpload(t, f, f.office.token, draft, "vertrag.png", pdf)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "only a .pdf")

	rec = paperUpload(t, f, f.office.token, draft, "vertrag.pdf", pdf)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	detail := signedDetail(t, f, draft)
	assert.Equal(t, "aktiv", detail.Status)
	require.NotNil(t, detail.Signature)
	assert.Equal(t, "paper", detail.Signature.Method)
	assert.Equal(t, "office", detail.Signature.Channel)
	assert.True(t, detail.Signature.Intact)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(draft, "/paper-signature"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, pdf, rec.Body.Bytes())
	var paperReads int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log WHERE tenant_id = ?
		AND resource_type = 'care_contracts' AND actor_role = 'traeger'
		AND metadata->>'view' = 'care_contract_paper_document'
		AND metadata->'contract_ids' @> to_jsonb(?::bigint)`, f.school, draft).Scan(context.Background(), &paperReads))
	assert.Equal(t, 1, paperReads, "the signed scan is in the school's access log")

	rec = paperUpload(t, f, f.office.token, draft, "vertrag.pdf", pdf)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.contract_not_signable")
	var frozen int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_documents WHERE contract_id = ?`, draft).Scan(context.Background(), &frozen))
	assert.Equal(t, 1, frozen, "the paper contract froze the PDF the parents signed")

	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signatures SET paper_pdf = ? WHERE contract_id = ?`, []byte("%PDF-1.4 anderer Vertrag"), draft)
	require.NoError(t, err)
	assert.False(t, signedDetail(t, f, draft).Signature.Intact, "replacing the signed scan breaks its original checksum")

	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signatures SET paper_pdf = ? WHERE contract_id = ?`, pdf, draft)
	require.NoError(t, err)

	// Deleting the recording account retains the historical paper proof.
	recorder := testpkg.CreateTestAccount(t, f.db, "papier-archiv@example.org")
	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signatures SET recorded_by_account_id = ? WHERE contract_id = ?`, recorder.ID, draft)
	require.NoError(t, err)
	_, err = f.db.ExecContext(context.Background(), `DELETE FROM auth.accounts WHERE id = ?`, recorder.ID)
	require.NoError(t, err)
	assert.True(t, signedDetail(t, f, draft).Signature.Intact, "account deletion retains the proof")
}

type reminderTraeger struct{ id int64 }

func (r reminderTraeger) ActiveTraegerIDs(context.Context) ([]int64, error) {
	return []int64{r.id}, nil
}

type reminderDays struct{ days int }

func (r reminderDays) SignatureReminderDays(context.Context, int64) (int, error) { return r.days, nil }

type reminderSender struct{}

func (reminderSender) TraegerLetterhead(context.Context, int64) (string, error) {
	return "Träger e.V.", nil
}

// TestCareContractRemindersFollowTheTraegersDays pins the reminders
// (#4679): a sent contract without signature is reminded once its last mail
// is as old as the Träger's days, at most three times, never after it was
// signed and never with 0 days.
func TestCareContractRemindersFollowTheTraegersDays(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Erinnerung")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	waiting, _ := sentCareContract(t, f, f.child)
	sibling := careContractApplication(t, f.db, f.school, f.phase, f.offering, "Ben", "erika.muster@example.org")
	signed, signedLink := sentCareContract(t, f, sibling)
	rec := publicSigning(t, f.traeger, http.MethodPost, signedLink+"/sign",
		map[string]any{"method": "typed", "typed_name": "Erika Muster", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	age := func(days int) {
		t.Helper()
		for _, statement := range []string{
			`UPDATE care_contract.contract_mailings SET sent_at = NOW() - make_interval(days => ?) WHERE organization_id = ?`,
			`UPDATE care_contract.contract_reminders SET sent_at = NOW() - make_interval(days => ?) WHERE organization_id = ?`,
		} {
			_, err := f.db.ExecContext(ctx, statement, days, f.office.organization)
			require.NoError(t, err)
		}
	}
	mails := &owners.CareContractTestMails{}
	run := func(days int) {
		t.Helper()
		reminders := careContractCompose.NewReminders(careContractCompose.ReminderPorts{
			Organizations: reminderTraeger{id: f.office.organization}, Settings: reminderDays{days: days}, Sender: reminderSender{},
			Post: mails, Runtime: owners.CareContractRuntime{}, Links: owners.CareContractLinks("http://parents.localhost:3000"),
		})
		require.NoError(t, reminders.RemindDue(ctx))
	}

	run(7)
	assert.Empty(t, mails.Purposes(), "the mail is fresh")
	age(8)
	run(0)
	assert.Empty(t, mails.Purposes(), "0 days sends no reminder")
	for range 4 {
		run(7)
		age(8)
	}
	assert.Equal(t, []string{carecontract.MailPurposeReminder, carecontract.MailPurposeReminder, carecontract.MailPurposeReminder},
		mails.Purposes(), "at most three reminders, none for the signed contract")
	assert.Len(t, signedDetail(t, f, waiting).Reminders, 3)
	assert.Empty(t, signedDetail(t, f, signed).Reminders)

	assert.Equal(t, "aktiv", signedDetail(t, f, signed).Status)
	assert.Equal(t, "versendet", signedDetail(t, f, waiting).Status)
}

// concludedRow is a contract as the conclusion of #4680 leaves it.
type concludedRow struct {
	Status        string     `bun:"status"`
	ConcludedAt   *time.Time `bun:"concluded_at"`
	Method        *string    `bun:"conclusion_method"`
	VersionStatus string     `bun:"version_status"`
	SignedAt      time.Time  `bun:"signed_at"`
	Conclusions   int        `bun:"conclusions"`
}

func readConcluded(t *testing.T, f careContractFixture, contractID int64) concludedRow {
	t.Helper()
	var row concludedRow
	require.NoError(t, f.db.NewRaw(`SELECT c.status, c.concluded_at, c.conclusion_method, v.status AS version_status, s.signed_at,
			(SELECT COUNT(*) FROM care_contract.contract_events e WHERE e.contract_id = c.id AND e.action = 'conclude') AS conclusions
		FROM care_contract.contracts c
		JOIN care_contract.contract_versions v ON v.contract_id = c.id
		JOIN care_contract.contract_signatures s ON s.contract_version_id = v.id
		WHERE c.id = ?`, contractID).Scan(context.Background(), &row))
	return row
}

// assertConcludedBySignature checks a signed contract the way the one
// conclusion transition of #4680 leaves it: aktiv with concluded_at at the
// instant of the signature and the method of ADR 0088 section 3, its version
// wirksam, one conclusion in the trail, and chk_contracts_active_concluded
// in place and kept by every contract.
func assertConcludedBySignature(t *testing.T, f careContractFixture, contractID int64, method string) {
	t.Helper()
	row := readConcluded(t, f, contractID)
	assert.Equal(t, "aktiv", row.Status)
	require.NotNil(t, row.ConcludedAt, "an active contract is concluded")
	assert.True(t, row.ConcludedAt.Equal(row.SignedAt), "concluded when signed")
	require.NotNil(t, row.Method)
	assert.Equal(t, method, *row.Method)
	assert.Equal(t, "wirksam", row.VersionStatus)
	assert.Equal(t, 1, row.Conclusions, "the trail records the conclusion once")

	var constraint string
	require.NoError(t, f.db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
		WHERE conrelid = 'care_contract.contracts'::regclass AND conname = 'chk_contracts_active_concluded'`).Scan(context.Background(), &constraint))
	assert.Contains(t, constraint, "concluded_at IS NOT NULL", "the constraint stays as #4680 defines it")
	var violations int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contracts
		WHERE status = 'aktiv' AND (concluded_at IS NULL OR conclusion_method IS NULL)`).Scan(context.Background(), &violations))
	assert.Zero(t, violations, "no active contract without its conclusion")
}

// TestCareContractSignedByMailLinkIsConcludedElectronically: a signature
// over the one-time link concludes the contract through concludeContract
// with einfach_elektronisch (#4679, #4680).
func TestCareContractSignedByMailLinkIsConcludedElectronically(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Abschluss Link")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	rec := publicSigning(t, f.traeger, http.MethodPost, link+"/sign",
		map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assertConcludedBySignature(t, f, contract, "einfach_elektronisch")
}

// TestCareContractSignedOnPaperIsConcludedOnPaper: a paper contract the
// Träger-Büro records, even from a draft, concludes the contract through
// concludeContract with papier (#4679, #4680).
func TestCareContractSignedOnPaperIsConcludedOnPaper(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Abschluss Papier")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	draft := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0].ID
	rec = paperUpload(t, f, f.office.token, draft, "vertrag.pdf", []byte("%PDF-1.4 unterschriebener Vertrag"))
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assertConcludedBySignature(t, f, draft, "papier")
}

// A planned amendment must not replace the PDF used to verify the signature
// of the version currently shown in the contract detail.
func TestCareContractSignatureSurvivesAFutureAmendment(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Geplante Änderung")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	rec := publicSigning(t, f.traeger, http.MethodPost, link+"/sign",
		map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	original := signedDetail(t, f, contract).Signature
	require.NotNil(t, original)
	require.True(t, original.Intact)

	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `UPDATE care_contract.contract_versions SET status = 'abgeloest' WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	var version int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_versions (organization_id, contract_id, version_number,
		status, effective_from, child_first_name, child_last_name, school_name, term_kind, notice_months, notice_to,
		special_termination, special_termination_note, statute_required, statute_title, statute_url, contract_partner, source)
		SELECT organization_id, contract_id, 2, 'wirksam', effective_from + INTERVAL '100 years', child_first_name,
		child_last_name, school_name, term_kind, notice_months, notice_to, special_termination, special_termination_note,
		statute_required, statute_title, statute_url, contract_partner, 'buchungsaenderung'
		FROM care_contract.contract_versions WHERE contract_id = ? AND version_number = 1 RETURNING id`, contract).Scan(ctx, &version))
	assert.Equal(t, original, signedDetail(t, f, contract).Signature, "a planned version without a PDF leaves the original evidence intact")

	_, err = f.db.ExecContext(ctx, `INSERT INTO care_contract.contract_documents (organization_id, contract_id,
		contract_version_id, template_version, filename, pdf) VALUES (?, ?, ?, 1, 'aenderung.pdf', ?)`,
		f.office.organization, contract, version, []byte("%PDF-1.4 Geplante Änderung"))
	require.NoError(t, err)
	assert.Equal(t, original, signedDetail(t, f, contract).Signature, "a different planned PDF must not damage the original evidence")
}

// The sender is resolved after the candidate scan but before contract locks.
// Commit a corrected mailing there to reproduce a resend winning that race.
type reminderResendSender struct{ resend func() }

func (s reminderResendSender) TraegerLetterhead(context.Context, int64) (string, error) {
	s.resend()
	return "Träger e.V.", nil
}

func TestCareContractRemindersRecheckMailingsAfterCandidateSelection(t *testing.T) {
	t.Parallel()
	for _, fresh := range []bool{true, false} {
		t.Run(fmt.Sprintf("fresh=%t", fresh), func(t *testing.T) {
			testpkg.OwnTenant(t)
			f := newCareContractFixture(t, "Träger Vertrag Erinnerung Adresskorrektur")
			require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
			contract, _ := sentCareContract(t, f, f.child)
			ctx := testpkg.WithPackageTenantRuntime(context.Background())
			_, err := f.db.ExecContext(ctx, `UPDATE care_contract.contract_mailings SET sent_at = NOW() - INTERVAL '8 days' WHERE contract_id = ?`, contract)
			require.NoError(t, err)
			mails := &owners.CareContractTestMails{}
			sender := reminderResendSender{resend: func() {
				age := 8
				if fresh {
					age = 0
				}
				_, err := f.db.ExecContext(ctx, `INSERT INTO care_contract.contract_mailings (organization_id, contract_id,
					contract_document_id, recipient_email, sent_at, outbox_id)
					SELECT organization_id, contract_id, contract_document_id, 'corrected@example.org',
					NOW() - make_interval(days => ?), ? FROM care_contract.contract_mailings WHERE contract_id = ?`, age, testpkg.UniqueTestTenantID(t), contract)
				require.NoError(t, err)
			}}
			reminders := careContractCompose.NewReminders(careContractCompose.ReminderPorts{
				Organizations: reminderTraeger{id: f.office.organization}, Settings: reminderDays{days: 7}, Sender: sender,
				Post: mails, Runtime: owners.CareContractRuntime{}, Links: owners.CareContractLinks("http://parents.localhost:3000"),
			})
			require.NoError(t, reminders.RemindDue(ctx))
			if fresh {
				assert.Empty(t, mails.Purposes(), "a resend restarts the reminder interval")
				assert.Empty(t, signedDetail(t, f, contract).Reminders)
			} else {
				assert.Equal(t, []string{carecontract.MailPurposeReminder}, mails.Purposes())
				var recipient string
				require.NoError(t, f.db.NewRaw(`SELECT recipient_email FROM care_contract.contract_reminders WHERE contract_id = ?`, contract).Scan(ctx, &recipient))
				assert.Equal(t, "corrected@example.org", recipient, "use the mailing committed after the candidate scan")
			}
		})
	}
}

// signingStatus reads the contract's status and how many signatures it has.
func signingStatus(t *testing.T, f careContractFixture, contractID int64) (string, int) {
	t.Helper()
	var row struct {
		Status     string `bun:"status"`
		Signatures int    `bun:"signatures"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT c.status, (SELECT COUNT(*) FROM care_contract.contract_signatures s WHERE s.contract_id = c.id) AS signatures
		FROM care_contract.contracts c WHERE c.id = ?`, contractID).Scan(context.Background(), &row))
	return row.Status, row.Signatures
}

// TestCareContractUsedLinkNeverOpensAgain pins the single use of a link on
// its own: a link once used opens nothing and signs nothing, also while its
// contract would still take a signature. It goes red when opening a link
// skips its used mark.
func TestCareContractUsedLinkNeverOpensAgain(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Link benutzt")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	contract, link := sentCareContract(t, f, f.child)
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_signing_links SET used_at = NOW() WHERE contract_id = ?`, contract)
	require.NoError(t, err)

	rec := publicSigning(t, f.traeger, http.MethodGet, link, nil)
	assert.Equal(t, http.StatusGone, rec.Code, "a used link opens nothing: %s", rec.Body.String())
	rec = publicSigning(t, f.traeger, http.MethodPost, link+"/sign",
		map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
	assert.Equal(t, http.StatusGone, rec.Code, "a used link signs nothing: %s", rec.Body.String())
	status, signatures := signingStatus(t, f, contract)
	assert.Equal(t, "versendet", status, "the contract still waits for a signature")
	assert.Zero(t, signatures)
}

// TestCareContractChangedAfterTheSendIsNotSigned pins the comparison with
// what was sent: when the frozen PDF or the version changes after the send,
// the link refuses the signature and the contract stays unsigned. It goes
// red when the signature skips the comparison.
func TestCareContractChangedAfterTheSendIsNotSigned(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]string{
		"PDF":     `UPDATE care_contract.contract_documents SET pdf = '%PDF-1.4 anderer Inhalt'::bytea WHERE contract_id = ?`,
		"version": `UPDATE care_contract.contract_versions SET child_first_name = 'Jemand anderes' WHERE contract_id = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newCareContractFixture(t, "Träger Vertrag geändert "+name)
			require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
			contract, link := sentCareContract(t, f, f.child)
			_, err := f.db.ExecContext(context.Background(), change, contract)
			require.NoError(t, err)

			rec := publicSigning(t, f.traeger, http.MethodPost, link+"/sign",
				map[string]any{"method": "drawn", "image_png": signatureImage(t), "withdrawal_notice_acknowledged": true})
			assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), "care_contract.contract_not_signable")
			status, signatures := signingStatus(t, f, contract)
			assert.Equal(t, "versendet", status, "a changed contract is not concluded")
			assert.Zero(t, signatures, "a changed contract is not signed")
		})
	}
}
