package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type declarationSignatureLedgerStub struct {
	signatures map[int64]*auditlog.Signature
}

func (s declarationSignatureLedgerStub) RecordSignature(context.Context, auditlog.SignatureEntry) (*auditlog.Signature, error) {
	return nil, errors.New("unexpected signature write")
}

func (s declarationSignatureLedgerStub) Signatures(context.Context, []int64) (map[int64]*auditlog.Signature, error) {
	return s.signatures, nil
}

func (s declarationSignatureLedgerStub) LatestEnrollmentSignatures(context.Context, []int64) (map[int64]*auditlog.Signature, error) {
	return s.signatures, nil
}

func TestParentDeclarationSignaturesPreservesPurposeAndStudent(t *testing.T) {
	t.Parallel()

	studentID := int64(53)
	adapter := ParentDeclarationSignatures{Ledger: declarationSignatureLedgerStub{signatures: map[int64]*auditlog.Signature{
		41: {ID: 41, Purpose: auditlog.SignaturePurposeDeclaration, StudentID: &studentID},
		42: {ID: 42, Purpose: auditlog.SignaturePurposeEnrollment},
	}}}

	got, err := adapter.DeclarationSignatures(context.Background(), []int64{41, 42})
	if err != nil {
		t.Fatalf("DeclarationSignatures() error = %v", err)
	}
	if !got[41].IsDeclaration || got[41].StudentID == nil || *got[41].StudentID != studentID {
		t.Errorf("declaration signature binding = %#v, want declaration for student %d", got[41], studentID)
	}
	if got[42].IsDeclaration || got[42].StudentID != nil {
		t.Errorf("enrollment signature binding = %#v, want non-declaration without student", got[42])
	}
}

// The enrollment binding (#4786) keeps a signature only under the request it
// names and maps a refused image onto the intake's error before any write.
func TestEnrollmentSignaturesBindTheRequestAndRefuseInvalidImages(t *testing.T) {
	t.Parallel()

	// Arbitrary map keys: the stub ledger never reaches a database.
	own, other := int64(4786), int64(4787)
	adapter := EnrollmentSignatures{Ledger: declarationSignatureLedgerStub{signatures: map[int64]*auditlog.Signature{
		own:     {ID: 1, Purpose: auditlog.SignaturePurposeEnrollment, EnrollmentRequestID: &own, SignerName: "Anna", IntegrityOK: true},
		own + 2: {ID: 2, Purpose: auditlog.SignaturePurposeEnrollment, EnrollmentRequestID: &other},
	}}}
	got, err := adapter.LatestEnrollmentSignatures(context.Background(), []int64{own, own + 2})
	if err != nil {
		t.Fatalf("LatestEnrollmentSignatures() error = %v", err)
	}
	if got[own] == nil || got[own].RequestID != own || got[own].SignerName != "Anna" || !got[own].IntegrityOK {
		t.Errorf("own request signature = %#v", got[own])
	}
	if _, leaked := got[own+2]; leaked {
		t.Errorf("a signature of another request must not be bound to this request")
	}

	_, err = adapter.RecordEnrollmentSignature(context.Background(), enrollmentOwner.EnrollmentSignatureEntry{
		RequestID: own, SignerName: "Anna", Signature: enrollmentOwner.SubmitSignature{Method: "drawn", ImagePNG: "a2VpbiBCaWxk"},
	})
	if !errors.Is(err, enrollmentOwner.ErrSignatureInvalid) {
		t.Errorf("RecordEnrollmentSignature() error = %v, want ErrSignatureInvalid", err)
	}
}
