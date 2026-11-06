package parentportal_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
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
		Method     string `json:"method"`
		SignerName string `json:"signer_name"`
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
	assert.Len(t, view.WithdrawalNotice, 4, "the parents read the withdrawal notice before they sign")
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
	assert.Equal(t, "unterschrieben", view.Status, "signed before its effective date 2027-08-01")
	require.NotNil(t, view.Signature)
	assert.Equal(t, "drawn", view.Signature.Method)

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
