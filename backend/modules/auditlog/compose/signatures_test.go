package compose

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"hash/fnv"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func signatureTestPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 300, 100))
	for x := 20; x < 280; x++ {
		img.Set(x, 50+(x%7), color.NRGBA{A: 255})
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func signatureTestEntry(t *testing.T, studentID, accountID int64) auditlog.SignatureEntry {
	t.Helper()
	image, err := auditlog.ParseSignatureImage(auditlog.SignatureMethodDrawn, base64.StdEncoding.EncodeToString(signatureTestPNG(t)), "")
	require.NoError(t, err)
	return auditlog.SignatureEntry{
		Purpose: auditlog.SignaturePurposeDeclaration, StudentID: &studentID, SignerAccountID: &accountID,
		SignerName: "Klaus Richter", DocumentSHA256: strings.Repeat("a", 64),
		SignedAt: time.Date(2026, 10, 24, 9, 30, 0, 123456789, time.UTC), Image: image,
	}
}

// testFingerprint stands in for the Security Runtime's SHA-256, which the
// root binds and this package may not import: a 64-digit hex identity that
// changes with the content.
func testFingerprint(content []byte) string {
	out := ""
	for i := byte(0); i < 4; i++ {
		h := fnv.New64a()
		_, _ = h.Write([]byte{i})
		_, _ = h.Write(content)
		out += fmt.Sprintf("%016x", h.Sum64())
	}
	return out
}

func newTestSignatureLedger(t *testing.T) auditlog.SignatureLedger {
	t.Helper()
	ledger, err := NewSignatureLedger(testFingerprint)
	require.NoError(t, err)
	return ledger
}

// A signature is stored in the caller's tenant transaction, read back
// verified, and invisible to another school.
func TestSignatureLedgerStoresAndReadsWithinTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.Tenant(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantB)
	student := testpkg.CreateTestStudent(t, db, "Signatur", "Kind", "1a")
	account := testpkg.CreateTestAccount(t, db, "signature-signer")
	ledger := newTestSignatureLedger(t)
	entry := signatureTestEntry(t, student.ID, account.ID)

	var stored *auditlog.Signature
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantA, func(ctx context.Context) error {
		var err error
		stored, err = ledger.RecordSignature(ctx, entry)
		return err
	}))
	require.Positive(t, stored.ID)
	require.Equal(t, tenantA, stored.TenantID)
	require.True(t, stored.IntegrityOK)
	require.Equal(t, testFingerprint(entry.Image.PNG), stored.ImageSHA256)

	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantA, func(ctx context.Context) error {
		got, err := ledger.Signatures(ctx, []int64{stored.ID})
		require.NoError(t, err)
		require.Contains(t, got, stored.ID)
		read := got[stored.ID]
		require.True(t, read.IntegrityOK)
		require.Equal(t, entry.Image.PNG, read.PNG)
		require.Equal(t, stored.RecordHash, read.RecordHash)
		require.Equal(t, entry.DocumentSHA256, read.DocumentSHA256)
		require.Equal(t, &account.ID, read.SignerAccountID)
		return nil
	}))
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantB, func(ctx context.Context) error {
		got, err := ledger.Signatures(ctx, []int64{stored.ID})
		require.NoError(t, err)
		require.Empty(t, got, "another school must not read the signature")
		return nil
	}))
}

// A changed image or row is reported, not hidden.
func TestSignatureLedgerDetectsChangedRows(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Signatur", "Prüfung", "1a")
	account := testpkg.CreateTestAccount(t, db, "signature-tamper")
	ledger := newTestSignatureLedger(t)

	var stored *auditlog.Signature
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		var err error
		stored, err = ledger.RecordSignature(ctx, signatureTestEntry(t, student.ID, account.ID))
		return err
	}))
	// The request roles cannot update the table; the test database owner can,
	// which stands in for a change outside the application.
	_, err := db.ExecContext(context.Background(), `UPDATE audit.signatures SET signer_name = 'Jemand anders' WHERE id = ?`, stored.ID)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		got, err := ledger.Signatures(ctx, []int64{stored.ID})
		require.NoError(t, err)
		require.False(t, got[stored.ID].IntegrityOK)
		return nil
	}))
}

func TestSignatureLedgerRefusesWithoutTenantTransactionAndBadEntries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	student := testpkg.CreateTestStudent(t, db, "Signatur", "Ohne", "1a")
	account := testpkg.CreateTestAccount(t, db, "signature-refused")
	ledger := newTestSignatureLedger(t)
	entry := signatureTestEntry(t, student.ID, account.ID)

	_, err := ledger.RecordSignature(testpkg.Ctx(t), entry)
	require.Error(t, err, "tenant without transaction")

	bad := entry
	bad.DocumentSHA256 = "nicht-hex"
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		_, err := ledger.RecordSignature(ctx, bad)
		require.ErrorIs(t, err, auditlog.ErrSignatureInvalid)
		bad = entry
		bad.Purpose = "contract"
		_, err = ledger.RecordSignature(ctx, bad)
		require.ErrorIs(t, err, auditlog.ErrSignatureInvalid)
		bad = entry
		bad.StudentID = nil
		_, err = ledger.RecordSignature(ctx, bad)
		require.ErrorIs(t, err, auditlog.ErrSignatureInvalid, "a signature needs a subject")
		return nil
	}))
	_, err = NewSignatureLedger(nil)
	require.Error(t, err)
}

// An enrollment signs before a child exists (#4786): the request is the
// subject, and a public enrollment has no account.
func TestSignatureLedgerStoresAnEnrollmentSignatureWithoutAccount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := testpkg.CreateTestEnrollmentRequestAwaitingGuardian(t, db, phase.ID, "signatur@example.test", "signature-status-token")
	requestID := request.ID
	ledger := newTestSignatureLedger(t)
	entry := signatureTestEntry(t, 0, 0)
	entry.Purpose, entry.StudentID, entry.SignerAccountID, entry.EnrollmentRequestID = auditlog.SignaturePurposeEnrollment, nil, nil, &requestID
	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, tenantID, func(ctx context.Context) error {
		stored, err := ledger.RecordSignature(ctx, entry)
		require.NoError(t, err)
		require.Nil(t, stored.SignerAccountID)
		got, err := ledger.Signatures(ctx, []int64{stored.ID})
		require.NoError(t, err)
		require.True(t, got[stored.ID].IntegrityOK)
		require.Equal(t, &requestID, got[stored.ID].EnrollmentRequestID)
		return nil
	}))
}
