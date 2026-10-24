package messaging

import (
	"context"
	"errors"
	"time"
)

// Signatures of Erklärungen (#4785). A school can ask guardians to sign their
// consent with finger, pen or mouse (or the typed name as the keyboard
// alternative). The signature is a simple electronic signature with a drawn
// signature; the flow decides who may sign, the root-bound ledger stores the
// image append-only in the flow's unit of work.

var (
	// ErrDeclarationSignatureRequired: the school asks for a signature and
	// the agreement carries none. 400.
	ErrDeclarationSignatureRequired = errors.New("parent: the declaration requires a signature")
	// ErrDeclarationSignatureInvalid: the signature image or typed name does
	// not pass the checks. 400.
	ErrDeclarationSignatureInvalid = errors.New("parent: the signature is not valid")
)

// DeclarationSignatureInput is a signature as the portal sends it: the
// method, the base64 PNG and, for the keyboard alternative, the typed name.
type DeclarationSignatureInput struct {
	Method    string
	ImagePNG  string
	TypedName string
}

// DeclarationSignatureEntry is one signature to store for an answer.
// DocumentSHA256 is the content hash of the version signed.
type DeclarationSignatureEntry struct {
	StudentID       int64
	SignerAccountID int64
	SignerName      string
	DocumentSHA256  string
	SignedAt        time.Time
	Input           DeclarationSignatureInput
}

// DeclarationSignature is a stored signature. IntegrityOK is the ledger's
// own check of image and row.
type DeclarationSignature struct {
	ID             int64
	Method         string
	TypedName      string
	PNG            []byte
	ImageSHA256    string
	DocumentSHA256 string
	SignedAt       time.Time
	IntegrityOK    bool
}

// SignatureLedger is the flow's port to the shared signature store. Both
// calls run in the caller's tenant unit of work; RecordDeclarationSignature
// checks the image and returns ErrDeclarationSignatureInvalid for a bad one,
// DeclarationSignatures returns only rows of that school.
type SignatureLedger interface {
	RecordDeclarationSignature(context.Context, DeclarationSignatureEntry) (int64, error)
	DeclarationSignatures(ctx context.Context, ids []int64) (map[int64]*DeclarationSignature, error)
}

// recordDeclarationSignature stores the signature an agreement needs. It
// returns nil for an answer that is not signed: a refusal or a revocation
// never needs one, and a signature sent with one is ignored.
func (s *Service) recordDeclarationSignature(ctx context.Context, tenantID, accountID int64, version *DeclarationVersion, signer *DeclarationSignerContext, in DeclarationInput, requiresSignature bool, now time.Time) (*int64, error) {
	if !requiresSignature || in.Action != DeclarationActionAgreed {
		return nil, nil
	}
	if in.Signature == nil {
		return nil, ErrDeclarationSignatureRequired
	}
	if s.Signatures == nil {
		return nil, errors.New("parent: signature ledger is not wired")
	}
	id, err := s.Signatures.RecordDeclarationSignature(s.Runtime.WithTenantID(ctx, tenantID), DeclarationSignatureEntry{
		StudentID: in.StudentID, SignerAccountID: accountID, SignerName: signerName(signer),
		DocumentSHA256: version.ContentHash, SignedAt: now, Input: *in.Signature,
	})
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// requireDeclarationSignature refuses an unsigned agreement before any
// transaction opens; the locked write checks again against the reloaded
// settings.
func requireDeclarationSignature(requiresSignature bool, in DeclarationInput) error {
	if requiresSignature && in.Action == DeclarationActionAgreed && in.Signature == nil {
		return ErrDeclarationSignatureRequired
	}
	return nil
}

// loadProofSignatures attaches the signatures of the guardian's signed
// answers to the proof.
func (s *Service) loadProofSignatures(ctx context.Context, proof *DeclarationProof, tenantID int64) error {
	ids := make([]int64, 0)
	for _, sub := range proof.Submissions {
		if sub.SignatureID != nil {
			ids = append(ids, *sub.SignatureID)
		}
	}
	proof.Signatures = map[int64]*DeclarationSignature{}
	if len(ids) == 0 {
		return nil
	}
	if s.Signatures == nil {
		return errors.New("parent: signature ledger is not wired")
	}
	signatures, err := s.Signatures.DeclarationSignatures(s.Runtime.WithTenantID(ctx, tenantID), ids)
	if err != nil {
		return err
	}
	proof.Signatures = signatures
	return nil
}

// proofSignatureIntact reports whether a signed answer's signature is stored,
// unchanged, and was given on the version the answer names.
func proofSignatureIntact(proof *DeclarationProof, sub *DeclarationSubmission) bool {
	if sub.SignatureID == nil {
		return true
	}
	signature, ok := proof.Signatures[*sub.SignatureID]
	return ok && signature.IntegrityOK && signature.DocumentSHA256 == sub.ContentHash
}
