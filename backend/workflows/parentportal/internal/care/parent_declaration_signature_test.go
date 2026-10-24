package care_test

// Integration tests for signed Erklärungen (#4785): an agreement needs a
// valid signature when the school asks for one, the signature is stored in
// the same unit of work with the checksum of the signed version, the proof
// shows only the guardian's own signature, and a later change of the version
// is reported.

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signedConsent() communication.ParentDeclarationSettings {
	settings := consent(communication.DeclarationSignersAny, false, false)
	settings.RequiresSignature = true
	return settings
}

func drawnSignature(t *testing.T) *parentportal.DeclarationSignatureInput {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 400, 120))
	for x := 30; x < 370; x++ {
		img.Set(x, 60+(x%9), color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return &parentportal.DeclarationSignatureInput{Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString(buf.Bytes())}
}

func (s *declarationSetup) submitSigned(accountID int64, action string, signature *parentportal.DeclarationSignatureInput) (*parentportal.DeclarationSubmission, bool, error) {
	return s.svc.SubmitDeclaration(s.ctx, accountID, s.announcement.ID, parentportal.DeclarationInput{
		StudentID: s.chain.StudentID, VersionID: s.version.ID, Action: action, Signature: signature,
	}, nil)
}

func (s *declarationSetup) signatureCount(t *testing.T) int {
	t.Helper()
	var count int
	require.NoError(t, s.db.NewRaw(`SELECT count(*) FROM audit.signatures WHERE tenant_id = ?`, s.chain.TenantID).Scan(context.Background(), &count))
	return count
}

func TestDeclarationSignature_AgreementNeedsAValidSignature(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, signedConsent(), nil)
	assert.True(t, s.feedDeclaration(t, s.chain.AccountID).RequiresSignature)

	_, _, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, nil)
	require.ErrorIs(t, err, parentportal.ErrDeclarationSignatureRequired)
	_, _, err = s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed,
		&parentportal.DeclarationSignatureInput{Method: "drawn", ImagePNG: base64.StdEncoding.EncodeToString([]byte("kein Bild"))})
	require.ErrorIs(t, err, parentportal.ErrDeclarationSignatureInvalid)
	assert.Empty(t, s.history(t))
	assert.Zero(t, s.signatureCount(t), "a refused agreement stores no signature")

	submission, created, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)
	require.True(t, created)
	require.NotNil(t, submission.SignatureID)
	assert.Equal(t, parentportal.DeclarationMethodSimpleElectronicSignature, submission.Method)
	assert.Equal(t, 1, s.signatureCount(t))

	// A double click returns the same proof and stores no second signature.
	again, created, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)
	assert.False(t, created)
	assert.Equal(t, submission.ID, again.ID)
	assert.Equal(t, 1, s.signatureCount(t))
}

func TestDeclarationSignature_RefusalNeedsNoSignature(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, signedConsent(), nil)
	submission, created, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionDeclined, drawnSignature(t))
	require.NoError(t, err)
	require.True(t, created)
	assert.Nil(t, submission.SignatureID, "a refusal ignores a signature")
	assert.Equal(t, parentportal.DeclarationMethodSimpleElectronic, submission.Method)
	assert.Zero(t, s.signatureCount(t))
}

func TestDeclarationSignature_UnsignedConsentIgnoresASignature(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, consent(communication.DeclarationSignersAny, false, false), nil)
	submission, _, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)
	assert.Nil(t, submission.SignatureID)
	assert.Zero(t, s.signatureCount(t))
}

func TestDeclarationSignature_ProofShowsTheOwnSignatureAndAChangedVersion(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, signedConsent(), nil)
	co := testpkg.CreateTestCoGuardianForStudent(t, s.db, s.chain.StudentID, "Anna", "Schneider")
	mine, _, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)
	theirs, _, err := s.submitSigned(co.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)

	proof, err := s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	require.True(t, proof.IntegrityOK)
	require.Len(t, proof.Signatures, 1, "the proof carries only the own signature")
	signature := proof.Signatures[*mine.SignatureID]
	require.NotNil(t, signature)
	assert.NotEmpty(t, signature.PNG)
	assert.Equal(t, s.version.ContentHash, signature.DocumentSHA256)
	assert.NotContains(t, proof.Signatures, *theirs.SignatureID)

	// The wording changes after signing: the signature still names the old
	// checksum, so the proof reports the change.
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE users.parent_announcement_declaration_versions SET body = body || ' geändert' WHERE id = ?`, s.version.ID)
	require.NoError(t, err)
	proof, err = s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	assert.False(t, proof.IntegrityOK)
}

func TestDeclarationSignature_ChangedSignatureIsReported(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, signedConsent(), nil)
	mine, _, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE audit.signatures SET document_sha256 = repeat('b', 64) WHERE id = ?`, *mine.SignatureID)
	require.NoError(t, err)
	proof, err := s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	assert.False(t, proof.IntegrityOK)
}

// The wording and the answer are rewritten consistently, each with a fresh
// checksum: only the checksum the signature stored still names the signed
// wording, and the proof reports the change through it.
func TestDeclarationSignature_SignatureChecksumRevealsAConsistentRewrite(t *testing.T) {
	t.Parallel()
	s := newDeclarationSetup(t, signedConsent(), nil)
	mine, _, err := s.submitSigned(s.chain.AccountID, parentportal.DeclarationActionAgreed, drawnSignature(t))
	require.NoError(t, err)

	rewritten := *s.version
	rewritten.Body += " Neuer Treffpunkt."
	rewritten.ContentHash = owners.SecurityFingerprints{}.Content(api.ParentDeclarationCanonicalContent(&rewritten))
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE users.parent_announcement_declaration_versions SET body = ?, content_hash = ? WHERE id = ?`,
		rewritten.Body, rewritten.ContentHash, rewritten.ID)
	require.NoError(t, err)
	answer := *mine
	answer.ContentHash = rewritten.ContentHash
	answer.RecordHash = owners.SecurityFingerprints{}.Content(api.ParentDeclarationCanonicalRecord(&answer))
	_, err = s.db.ExecContext(context.Background(),
		`UPDATE users.parent_announcement_declaration_submissions SET content_hash = ?, record_hash = ? WHERE id = ?`,
		answer.ContentHash, answer.RecordHash, answer.ID)
	require.NoError(t, err)

	proof, err := s.svc.DeclarationProof(s.ctx, s.chain.AccountID, s.announcement.ID, s.chain.StudentID)
	require.NoError(t, err)
	signature := proof.Signatures[*mine.SignatureID]
	require.NotNil(t, signature)
	assert.True(t, signature.IntegrityOK, "the signature row itself is unchanged")
	assert.Equal(t, s.version.ContentHash, signature.DocumentSHA256)
	assert.False(t, proof.IntegrityOK, "the signed wording no longer matches")
}
