package owners

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
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
