package enrollment

import (
	"context"
	"errors"
	"time"
)

// Signature of an enrollment (#4786): a form template can require the
// parent to sign the submission with a simple electronic signature with a
// drawn signature. The Audit Platform stores it (ADR 0103); the intake
// decides when it is required, checks it before the write and keeps it with
// the request through the ledger's enrollment subject.

// CoreRequirementSignature makes a form template require the signature on
// every parent submission and every parent edit of it.
const CoreRequirementSignature = "signature"

// Wire codes of a refused signature.
const (
	CodeSignatureRequired = "enrollment.signature_required"
	CodeSignatureInvalid  = "enrollment.signature_invalid"
)

var (
	// ErrSignatureRequired refuses a parent submission or edit without a
	// signature although its form requires one.
	ErrSignatureRequired = errors.New("enrollment: signature required")
	// ErrSignatureInvalid refuses a signature image or typed name the
	// signature checks reject.
	ErrSignatureInvalid = errors.New("enrollment: signature invalid")
)

// SubmitSignature is the signature as the parent gave it: the method (drawn
// or typed), the base64 PNG and, for a typed signature, the typed name.
type SubmitSignature struct {
	Method    string
	ImagePNG  string
	TypedName string
}

// RequestSignature is the stored signature of a request's current version.
// IntegrityOK is false when the image or the stored record no longer matches
// its checksum. DocumentSHA256 is the checksum of the signed submission with
// its legal snapshot.
type RequestSignature struct {
	ID             int64
	RequestID      int64
	SignerName     string
	Method         string
	TypedName      string
	PNG            []byte
	ImageSHA256    string
	DocumentSHA256 string
	SignedAt       time.Time
	IntegrityOK    bool
}

// RequestSignatureProof is a request's signature with the request it
// belongs to, for the proof a parent loads through the status link.
type RequestSignatureProof struct {
	Status    *RequestStatus
	Guardians []*RequestGuardian
	// Schema names the pinned form fields that label the saved answers in
	// the human-readable proof.
	Schema *FormSchema
	// OfferingSelections are the selections effective for the request's
	// phase. The proof labels them as current information, alongside the
	// checksum of the version the parent signed.
	OfferingSelections []*RequestChildOfferingRecord
	OfferingNames      map[int64]string
	Signature          *RequestSignature
	// SignatureRequired reports whether the request's form asks the parent
	// to sign every edit, so the status page offers only the full form.
	SignatureRequired bool
}

// IntakeSignatureProofs reads the signature of a request. Neither read
// decides more than the caller already may: the staff read runs in the
// caller's tenant transaction, the status read behind a valid status token.
type IntakeSignatureProofs interface {
	// SignatureProof loads a request of the school in context with its
	// signature; Signature is nil when it has none, and a request of
	// another school answers ErrRequestNotFound.
	SignatureProof(ctx context.Context, requestID int64) (*RequestSignatureProof, error)
	// SignatureProofByStatusToken loads the request behind a valid status
	// token with its signature; Signature is nil when it has none.
	SignatureProofByStatusToken(ctx context.Context, token string) (*RequestSignatureProof, error)
}

// EnrollmentSignatureEntry is one signature the intake stores for a
// request: who signed, the checksum of the signed submission with its legal
// snapshot, and the signature as given.
type EnrollmentSignatureEntry struct {
	RequestID       int64
	SignerAccountID *int64
	SignerName      string
	DocumentSHA256  string
	SignedAt        time.Time
	Signature       SubmitSignature
}

// IntakeSignatureLedger is the Audit Platform's signature ledger as the
// intake uses it, inside the request's school (ADR 0103); the root binds
// it. RecordEnrollmentSignature wraps ErrSignatureInvalid when the
// signature checks refuse the image or the typed name.
// LatestEnrollmentSignatures returns, per request of the school in context,
// its newest signature; requests without one are absent.
type IntakeSignatureLedger interface {
	RecordEnrollmentSignature(ctx context.Context, entry EnrollmentSignatureEntry) (int64, error)
	LatestEnrollmentSignatures(ctx context.Context, requestIDs []int64) (map[int64]*RequestSignature, error)
}
