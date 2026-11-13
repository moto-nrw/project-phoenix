package parentportal_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// signaturePNG is a drawn signature: a dark stroke on a transparent canvas.
func signaturePNG(t *testing.T) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for x := 5; x < 35; x++ {
		img.Set(x, 10, color.NRGBA{A: 255})
	}
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, img))
	return base64.StdEncoding.EncodeToString(out.Bytes())
}

// blankPNG is an untouched canvas.
func blankPNG(t *testing.T) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, png.Encode(&out, image.NewNRGBA(image.Rect(0, 0, 40, 20))))
	return base64.StdEncoding.EncodeToString(out.Bytes())
}

type signingViewBody struct {
	ContractID       int64    `json:"contract_id"`
	Status           string   `json:"status"`
	DocumentSHA256   string   `json:"document_sha256"`
	WithdrawalNotice []string `json:"withdrawal_notice"`
	Signature        *struct {
		Method           string         `json:"method"`
		SignerName       string         `json:"signer_name"`
		SignedAt         time.Time      `json:"signed_at"`
		EarlyStart       bool           `json:"early_start_requested"`
		WithdrawalEndsOn *calendar.Date `json:"withdrawal_ends_on"`
	} `json:"signature"`
}

// TestContractSigningInTheInboxNeedsTheOwnAddressAndTheChildsSubmitRight
// pins the invariants of #4679 for the parents portal: a parent signs only
// a contract sent to the own address, only for a child whose relationship
// grants submitting an enrollment, only with the withdrawal notice
// confirmed and a visible signature, and only once. The signature binds the
// frozen PDF and makes the version effective.
func TestContractSigningInTheInboxNeedsTheOwnAddressAndTheChildsSubmitRight(t *testing.T) {
	t.Parallel()
	portal, _, _, db, mails := documentInboxWithMails(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	sibling := testpkg.CreateTestStudent(t, db, "Mia", "Schneider", "1a")
	siblingLink := testpkg.CreateTestStudentGuardianLink(t, db, sibling.ID, chain.GuardianProfileID, "primary_guardian")
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	sent := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	otherFamily := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Tom", "andere.familie@example.org")
	siblings := inboxContract(t, db, organization, chain.TenantID, sibling.ID, "Mia", chain.Email)
	signature := parentService.ContractSignatureInput{Method: "drawn", ImagePNG: signaturePNG(t), NoticeAcknowledged: true, EarlyStart: true}

	raw, err := portal.ContractSigningView(ctx, chain.AccountID, organization, sent)
	require.NoError(t, err)
	var view signingViewBody
	require.NoError(t, json.Unmarshal(raw, &view))
	assert.Equal(t, "versendet", view.Status)
	assert.Len(t, view.WithdrawalNotice, 6, "the parents read the model withdrawal notice before they sign (#6105)")
	assert.Contains(t, strings.Join(view.WithdrawalNotice, " "), "ab dem Tag des Vertragsabschlusses",
		"the signing page prints the notice of the PDF, not a wording of its own")
	assert.Len(t, view.DocumentSHA256, 64, "the checksum of the PDF they sign")

	_, err = portal.SignContract(ctx, chain.AccountID, organization, otherFamily, signature)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "a contract sent to another family is no one's to sign here")

	// Without the right to submit an enrollment for Mia, her contract is
	// not to be signed, although it went to the same address.
	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, db, siblingLink.ID, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true,
	}))
	_, err = portal.SignContract(ctx, chain.AccountID, organization, siblings, signature)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "the relationship must grant enrollment.submit")
	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	eligibility := make(map[int64]bool)
	for _, document := range documents {
		eligibility[document.ID] = document.CanSign
	}
	require.Contains(t, eligibility, siblings, "view-only contracts remain downloadable")
	assert.False(t, eligibility[siblings], "the sibling's viewing right does not permit signing")
	assert.True(t, eligibility[sent], "the other child's explicit submit right permits signing")
	_, err = portal.ContractDocumentFile(ctx, chain.AccountID, organization, siblings)
	assert.NoError(t, err, "reading stays with the inbox right")

	withoutNotice := signature
	withoutNotice.NoticeAcknowledged = false
	_, err = portal.SignContract(ctx, chain.AccountID, organization, sent, withoutNotice)
	assert.ErrorIs(t, err, parentService.ErrContractWithdrawalNoticeRequired)
	blank := signature
	blank.ImagePNG = blankPNG(t)
	_, err = portal.SignContract(ctx, chain.AccountID, organization, sent, blank)
	assert.ErrorIs(t, err, parentService.ErrContractSignatureInvalid, "an untouched canvas is no signature")

	raw, err = portal.SignContract(ctx, chain.AccountID, organization, sent, signature)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &view))
	assert.Equal(t, "aktiv", view.Status, "the signature concludes the contract")
	require.NotNil(t, view.Signature)
	assert.Equal(t, "drawn", view.Signature.Method)
	confirmed := *view.Signature
	raw, err = portal.ContractSigningView(ctx, chain.AccountID, organization, sent)
	require.NoError(t, err)
	view = signingViewBody{}
	require.NoError(t, json.Unmarshal(raw, &view))
	require.NotNil(t, view.Signature, "reload retains the confirmation")
	assert.Equal(t, confirmed.Method, view.Signature.Method)
	assert.Equal(t, confirmed.SignerName, view.Signature.SignerName)
	assert.WithinDuration(t, confirmed.SignedAt, view.Signature.SignedAt, time.Microsecond)
	assert.Equal(t, confirmed.EarlyStart, view.Signature.EarlyStart)
	assert.Equal(t, confirmed.WithdrawalEndsOn, view.Signature.WithdrawalEndsOn)
	assert.True(t, view.Signature.EarlyStart)
	assert.NotNil(t, view.Signature.WithdrawalEndsOn)

	_, err = portal.SignContract(ctx, chain.AccountID, organization, sent, signature)
	assert.ErrorIs(t, err, parentService.ErrContractNotSignable, "a contract signs once")

	var stored struct {
		Channel        string `bun:"channel"`
		SignerAccount  int64  `bun:"signer_account_id"`
		DocumentSHA256 string `bun:"document_sha256"`
		PDFSHA256      string `bun:"pdf_sha256"`
		EarlyStart     bool   `bun:"early_start_requested"`
		Version        string `bun:"version_status"`
		AuditDocument  string `bun:"audit_document"`
	}
	require.NoError(t, db.NewRaw(`SELECT s.channel, s.signer_account_id, s.document_sha256, d.pdf_sha256, s.early_start_requested,
			v.status AS version_status, a.document_sha256 AS audit_document
		FROM care_contract.contract_signatures AS s
		JOIN care_contract.contract_documents AS d ON d.id = s.contract_document_id
		JOIN care_contract.contract_versions AS v ON v.id = s.contract_version_id
		JOIN audit.traeger_signatures AS a ON a.id = s.audit_signature_id
		WHERE s.contract_id = ?`, sent).Scan(ctx, &stored))
	assert.Equal(t, "parent_account", stored.Channel)
	assert.Equal(t, chain.AccountID, stored.SignerAccount)
	assert.Equal(t, stored.PDFSHA256, stored.DocumentSHA256, "the signature names the frozen PDF")
	assert.Equal(t, stored.PDFSHA256, stored.AuditDocument, "the ledger names the same PDF")
	assert.True(t, stored.EarlyStart)
	assert.Equal(t, "wirksam", stored.Version)
	assert.Equal(t, []string{"signed"}, mails.Purposes(), "the confirmation with the PDF and the withdrawal notice")
}

// TestContractSignedInTheInboxIsConcludedElectronically: a signature in the
// parents portal concludes the contract through the one transition of
// #4680 (concludeContract): aktiv with concluded_at at the instant of the
// signature and einfach_elektronisch, its version wirksam, the conclusion
// in the trail under the parent's account, and chk_contracts_active_concluded
// kept.
func TestContractSignedInTheInboxIsConcludedElectronically(t *testing.T) {
	t.Parallel()
	portal, _, _, db, _ := documentInboxWithMails(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	sent := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)

	_, err := portal.SignContract(ctx, chain.AccountID, organization, sent,
		parentService.ContractSignatureInput{Method: "drawn", ImagePNG: signaturePNG(t), NoticeAcknowledged: true})
	require.NoError(t, err)

	var row struct {
		Status        string     `bun:"status"`
		ConcludedAt   *time.Time `bun:"concluded_at"`
		Method        *string    `bun:"conclusion_method"`
		VersionStatus string     `bun:"version_status"`
		SignedAt      time.Time  `bun:"signed_at"`
		Conclusions   int        `bun:"conclusions"`
	}
	require.NoError(t, db.NewRaw(`SELECT c.status, c.concluded_at, c.conclusion_method, v.status AS version_status, s.signed_at,
			(SELECT COUNT(*) FROM care_contract.contract_events e
				WHERE e.contract_id = c.id AND e.action = 'conclude' AND e.account_id = ?) AS conclusions
		FROM care_contract.contracts c
		JOIN care_contract.contract_versions v ON v.contract_id = c.id
		JOIN care_contract.contract_signatures s ON s.contract_version_id = v.id
		WHERE c.id = ?`, chain.AccountID, sent).Scan(ctx, &row))
	assert.Equal(t, "aktiv", row.Status)
	require.NotNil(t, row.ConcludedAt, "an active contract is concluded")
	assert.True(t, row.ConcludedAt.Equal(row.SignedAt), "concluded when signed")
	require.NotNil(t, row.Method)
	assert.Equal(t, "einfach_elektronisch", *row.Method)
	assert.Equal(t, "wirksam", row.VersionStatus)
	assert.Equal(t, 1, row.Conclusions, "the trail records the conclusion once, under the parent's account")

	var violations int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM care_contract.contracts
		WHERE status = 'aktiv' AND (concluded_at IS NULL OR conclusion_method IS NULL)`).Scan(ctx, &violations))
	assert.Zero(t, violations, "chk_contracts_active_concluded holds for every contract")
}

// TestContractChangedAfterTheSendIsNotSignedInTheInbox: in the parents
// portal too, a contract whose frozen PDF changed after the send is not
// signed; it stays waiting and unsigned.
func TestContractChangedAfterTheSendIsNotSignedInTheInbox(t *testing.T) {
	t.Parallel()
	portal, _, _, db, _ := documentInboxWithMails(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	sent := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	_, err := db.ExecContext(ctx, `UPDATE care_contract.contract_documents SET pdf = '%PDF-1.4 anderer Inhalt'::bytea WHERE contract_id = ?`, sent)
	require.NoError(t, err)

	_, err = portal.SignContract(ctx, chain.AccountID, organization, sent,
		parentService.ContractSignatureInput{Method: "drawn", ImagePNG: signaturePNG(t), NoticeAcknowledged: true})
	require.Error(t, err, "a changed contract is not signed")
	var row struct {
		Status     string `bun:"status"`
		Signatures int    `bun:"signatures"`
	}
	require.NoError(t, db.NewRaw(`SELECT c.status, (SELECT COUNT(*) FROM care_contract.contract_signatures s WHERE s.contract_id = c.id) AS signatures
		FROM care_contract.contracts c WHERE c.id = ?`, sent).Scan(ctx, &row))
	assert.Equal(t, "versendet", row.Status)
	assert.Zero(t, row.Signatures)
}

// TestContractWithoutASendToCompareOffersNoSigning: a contract with no
// signing link, such as one sent before the links existed, has nothing to
// compare a signature with; the inbox offers no signing for it and refuses
// one, while reading stays.
func TestContractWithoutASendToCompareOffersNoSigning(t *testing.T) {
	t.Parallel()
	portal, _, _, db, _ := documentInboxWithMails(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, chain.TenantID).Scan(ctx, &organization))
	testpkg.OwnTraegerCareContracts(t, db, organization)
	sent := inboxContract(t, db, organization, chain.TenantID, chain.StudentID, "Felix", chain.Email)
	_, err := db.ExecContext(ctx, `DELETE FROM care_contract.contract_signing_links WHERE contract_id = ?`, sent)
	require.NoError(t, err)

	documents, err := portal.ListBillingDocuments(ctx, chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	var listed bool
	for _, document := range documents {
		if document.ID == sent {
			listed = true
			assert.False(t, document.CanSign, "no signing offered without a send to compare with")
		}
	}
	require.True(t, listed, "the contract stays readable in the inbox")
	_, err = portal.SignContract(ctx, chain.AccountID, organization, sent,
		parentService.ContractSignatureInput{Method: "drawn", ImagePNG: signaturePNG(t), NoticeAcknowledged: true})
	require.Error(t, err, "and refuses a signature")
}
