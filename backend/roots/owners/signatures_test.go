package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
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

	own, other := int64(7), int64(8)
	adapter := EnrollmentSignatures{Ledger: declarationSignatureLedgerStub{signatures: map[int64]*auditlog.Signature{
		7: {ID: 1, Purpose: auditlog.SignaturePurposeEnrollment, EnrollmentRequestID: &own, SignerName: "Anna", IntegrityOK: true},
		9: {ID: 2, Purpose: auditlog.SignaturePurposeEnrollment, EnrollmentRequestID: &other},
	}}}
	got, err := adapter.LatestEnrollmentSignatures(context.Background(), []int64{7, 9})
	if err != nil {
		t.Fatalf("LatestEnrollmentSignatures() error = %v", err)
	}
	if got[7] == nil || got[7].RequestID != 7 || got[7].SignerName != "Anna" || !got[7].IntegrityOK {
		t.Errorf("own request signature = %#v", got[7])
	}
	if _, leaked := got[9]; leaked {
		t.Errorf("a signature of another request must not be bound to request 9")
	}

	_, err = adapter.RecordEnrollmentSignature(context.Background(), enrollmentCompose.EnrollmentSignatureEntry{
		RequestID: 7, SignerName: "Anna", Signature: enrollmentOwner.SubmitSignature{Method: "drawn", ImagePNG: "a2VpbiBCaWxk"},
	})
	if !errors.Is(err, enrollmentOwner.ErrSignatureInvalid) {
		t.Errorf("RecordEnrollmentSignature() error = %v, want ErrSignatureInvalid", err)
	}
}
