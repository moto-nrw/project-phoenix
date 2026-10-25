package owners

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// SignatureLedger is the Audit Platform's signature store (#4785) over the
// Security Runtime's SHA-256 content identity. Every consumer appends and
// reads inside its own tenant unit of work.
func SignatureLedger() auditlog.SignatureLedger {
	return auditCompose.NewSignatureLedger(securityruntime.Fingerprint)
}

// CommunicationSignatures binds the ledger to Communication's reader of the
// signatures of signed Erklärungen.
type CommunicationSignatures struct{ Ledger auditlog.SignatureLedger }

func (c CommunicationSignatures) DeclarationSignatures(ctx context.Context, ids []int64) (map[int64]communication.DeclarationSignature, error) {
	stored, err := c.Ledger.Signatures(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]communication.DeclarationSignature, len(stored))
	for id, s := range stored {
		out[id] = communication.DeclarationSignature{
			IsDeclaration: s.Purpose == auditlog.SignaturePurposeDeclaration, StudentID: s.StudentID,
			Method: s.Method, TypedName: s.TypedName, PNG: s.PNG, ImageSHA256: s.ImageSHA256,
			DocumentSHA256: s.DocumentSHA256, SignedAt: s.SignedAt, IntegrityOK: s.IntegrityOK,
		}
	}
	return out, nil
}

// ParentDeclarationSignatures binds the ledger to the parent portal's port
// (#4785): it checks the image with the ledger's own rules and maps a refused
// image onto the flow's error.
type ParentDeclarationSignatures struct{ Ledger auditlog.SignatureLedger }

var _ parentportal.SignatureLedger = ParentDeclarationSignatures{}

func (p ParentDeclarationSignatures) RecordDeclarationSignature(ctx context.Context, entry parentportal.DeclarationSignatureEntry) (int64, error) {
	image, err := auditlog.ParseSignatureImage(entry.Input.Method, entry.Input.ImagePNG, entry.Input.TypedName)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", parentportal.ErrDeclarationSignatureInvalid, err)
	}
	studentID, accountID := entry.StudentID, entry.SignerAccountID
	stored, err := p.Ledger.RecordSignature(ctx, auditlog.SignatureEntry{
		Purpose: auditlog.SignaturePurposeDeclaration, StudentID: &studentID, SignerAccountID: &accountID,
		SignerName: entry.SignerName, DocumentSHA256: entry.DocumentSHA256, SignedAt: entry.SignedAt, Image: image,
	})
	if errors.Is(err, auditlog.ErrSignatureInvalid) {
		return 0, fmt.Errorf("%w: %v", parentportal.ErrDeclarationSignatureInvalid, err)
	}
	if err != nil {
		return 0, err
	}
	return stored.ID, nil
}

func (p ParentDeclarationSignatures) DeclarationSignatures(ctx context.Context, ids []int64) (map[int64]*parentportal.DeclarationSignature, error) {
	stored, err := p.Ledger.Signatures(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*parentportal.DeclarationSignature, len(stored))
	for id, s := range stored {
		out[id] = &parentportal.DeclarationSignature{
			ID: s.ID, IsDeclaration: s.Purpose == auditlog.SignaturePurposeDeclaration, StudentID: s.StudentID,
			Method: s.Method, TypedName: s.TypedName, PNG: s.PNG, ImageSHA256: s.ImageSHA256,
			DocumentSHA256: s.DocumentSHA256, SignedAt: s.SignedAt, IntegrityOK: s.IntegrityOK,
		}
	}
	return out, nil
}

// EnrollmentSignatures binds the ledger to the enrollment intake (#4786):
// the request is the signature's subject, a public enrollment has no
// account, and a refused image becomes the intake's error.
type EnrollmentSignatures struct{ Ledger auditlog.SignatureLedger }

var _ enrollmentOwner.IntakeSignatureLedger = EnrollmentSignatures{}

func (e EnrollmentSignatures) RecordEnrollmentSignature(ctx context.Context, entry enrollmentOwner.EnrollmentSignatureEntry) (int64, error) {
	image, err := auditlog.ParseSignatureImage(entry.Signature.Method, entry.Signature.ImagePNG, entry.Signature.TypedName)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", enrollmentOwner.ErrSignatureInvalid, err)
	}
	requestID := entry.RequestID
	stored, err := e.Ledger.RecordSignature(ctx, auditlog.SignatureEntry{
		Purpose: auditlog.SignaturePurposeEnrollment, EnrollmentRequestID: &requestID, SignerAccountID: entry.SignerAccountID,
		SignerName: entry.SignerName, DocumentSHA256: entry.DocumentSHA256, SignedAt: entry.SignedAt, Image: image,
	})
	if errors.Is(err, auditlog.ErrSignatureInvalid) {
		return 0, fmt.Errorf("%w: %v", enrollmentOwner.ErrSignatureInvalid, err)
	}
	if err != nil {
		return 0, err
	}
	return stored.ID, nil
}

func (e EnrollmentSignatures) LatestEnrollmentSignatures(ctx context.Context, requestIDs []int64) (map[int64]*enrollmentOwner.RequestSignature, error) {
	stored, err := e.Ledger.LatestEnrollmentSignatures(ctx, requestIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]*enrollmentOwner.RequestSignature, len(stored))
	for requestID, s := range stored {
		if s.EnrollmentRequestID == nil || *s.EnrollmentRequestID != requestID {
			continue
		}
		out[requestID] = &enrollmentOwner.RequestSignature{
			ID: s.ID, RequestID: requestID, SignerName: s.SignerName, Method: s.Method, TypedName: s.TypedName,
			PNG: s.PNG, ImageSHA256: s.ImageSHA256, DocumentSHA256: s.DocumentSHA256, SignedAt: s.SignedAt, IntegrityOK: s.IntegrityOK,
		}
	}
	return out, nil
}
