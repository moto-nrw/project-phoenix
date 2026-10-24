package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

// NewDeclarationSignatures binds the ledger to the flow's port; nil stays nil.
//
// declarationSignatures binds the Audit Platform signature ledger (#4785) to
// the flow's port: it checks the image with the ledger's own rules and maps
// a refused image onto the flow's error.
type declarationSignatures struct{ ledger auditlog.SignatureLedger }

func NewDeclarationSignatures(ledger auditlog.SignatureLedger) messaging.SignatureLedger {
	if ledger == nil {
		return nil
	}
	return declarationSignatures{ledger: ledger}
}

func (d declarationSignatures) RecordDeclarationSignature(ctx context.Context, entry messaging.DeclarationSignatureEntry) (int64, error) {
	image, err := auditlog.ParseSignatureImage(entry.Input.Method, entry.Input.ImagePNG, entry.Input.TypedName)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", messaging.ErrDeclarationSignatureInvalid, err)
	}
	studentID, accountID := entry.StudentID, entry.SignerAccountID
	stored, err := d.ledger.RecordSignature(ctx, auditlog.SignatureEntry{
		Purpose: auditlog.SignaturePurposeDeclaration, StudentID: &studentID, SignerAccountID: &accountID,
		SignerName: entry.SignerName, DocumentSHA256: entry.DocumentSHA256, SignedAt: entry.SignedAt, Image: image,
	})
	if errors.Is(err, auditlog.ErrSignatureInvalid) {
		return 0, fmt.Errorf("%w: %v", messaging.ErrDeclarationSignatureInvalid, err)
	}
	if err != nil {
		return 0, err
	}
	return stored.ID, nil
}

func (d declarationSignatures) DeclarationSignatures(ctx context.Context, ids []int64) (map[int64]*messaging.DeclarationSignature, error) {
	stored, err := d.ledger.Signatures(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*messaging.DeclarationSignature, len(stored))
	for id, s := range stored {
		out[id] = &messaging.DeclarationSignature{
			ID: s.ID, Method: s.Method, TypedName: s.TypedName, PNG: s.PNG, ImageSHA256: s.ImageSHA256,
			DocumentSHA256: s.DocumentSHA256, SignedAt: s.SignedAt, IntegrityOK: s.IntegrityOK,
		}
	}
	return out, nil
}
