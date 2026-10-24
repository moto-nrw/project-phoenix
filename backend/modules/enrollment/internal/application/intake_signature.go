package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// The signature of an enrollment (#4786): a simple electronic signature with
// a drawn signature, stored by the Audit Platform (ADR 0103). The intake
// decides when a parent has to sign, checks that a signature is there before
// the write transaction, and records it in that transaction once the request
// has its id. Every parent edit of a signed form signs again; the newest
// signature belongs to the request's current version.

// signedEnrollmentFormat versions the canonical form of the signed document.
const signedEnrollmentFormat = 1

// EnrollmentSignatureEntry is one signature to store for a request.
type EnrollmentSignatureEntry struct {
	RequestID       int64
	SignerAccountID *int64
	SignerName      string
	DocumentSHA256  string
	SignedAt        time.Time
	Signature       enrollmentNative.SubmitSignature
}

// IntakeSignatures is the Audit Platform's signature ledger as the intake
// uses it, inside the request's tenant transaction. RecordEnrollmentSignature
// wraps enrollment.ErrSignatureInvalid when the signature checks refuse the
// image or the typed name. LatestEnrollmentSignatures returns, per request of
// the school in context, its newest signature; requests without one are
// absent.
type IntakeSignatures interface {
	RecordEnrollmentSignature(ctx context.Context, entry EnrollmentSignatureEntry) (int64, error)
	LatestEnrollmentSignatures(ctx context.Context, requestIDs []int64) (map[int64]*enrollmentNative.RequestSignature, error)
}

// signatureRequired reports whether a submission of this form needs the
// parent's signature. A staff-entered enrollment has no parent signing it.
func signatureRequired(schema *enrollmentNative.FormSchema, source string) bool {
	if schema == nil || !schema.CoreRequirements.Required(enrollmentNative.CoreRequirementSignature) {
		return false
	}
	return strings.TrimSpace(source) != enrollmentNative.RequestSourceAdminManual
}

// requireSignature refuses a submission or edit of a signed form without a
// signature, before any write.
func (s *Intake) requireSignature(required bool, signature *enrollmentNative.SubmitSignature) error {
	if !required {
		return nil
	}
	if signature == nil || strings.TrimSpace(signature.ImagePNG) == "" {
		return enrollmentNative.InvalidInput(enrollmentNative.CodeSignatureRequired, "signature", enrollmentNative.ErrSignatureRequired)
	}
	if s.deps.Signatures == nil || s.deps.Fingerprint == nil {
		return fmt.Errorf("enrollment intake: signature ledger is not bound")
	}
	return nil
}

// signedEnrollmentDocument is what the parent signs: the stored submission
// with its offering choices and the legal snapshot entry of this submission.
type signedEnrollmentDocument struct {
	Format              int                        `json:"format"`
	TenantID            int64                      `json:"tenant_id"`
	RequestID           int64                      `json:"request_id"`
	PhaseID             int64                      `json:"phase_id"`
	SchemaID            *int64                     `json:"schema_id"`
	GuardianFirstName   string                     `json:"guardian_first_name"`
	GuardianLastName    string                     `json:"guardian_last_name"`
	GuardianEmail       string                     `json:"guardian_email"`
	GuardianPhone       *string                    `json:"guardian_phone"`
	AdditionalGuardians []signedEnrollmentGuardian `json:"additional_guardians"`
	ConsentFlags        map[string]any             `json:"consent_flags"`
	CustomData          map[string]any             `json:"custom_data"`
	Children            []signedEnrollmentChild    `json:"children"`
	LegalSnapshot       LegalBlocksSnapshotEntry   `json:"legal_snapshot"`
}

type signedEnrollmentGuardian struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email"`
	Phone     *string `json:"phone"`
}

type signedEnrollmentChild struct {
	FirstName         string                       `json:"first_name"`
	LastName          string                       `json:"last_name"`
	DateOfBirth       string                       `json:"date_of_birth"`
	TargetGradeLevel  *int16                       `json:"target_grade_level"`
	TargetSchoolClass *string                      `json:"target_school_class"`
	CustomData        map[string]any               `json:"custom_data"`
	Offerings         []signedEnrollmentSelections `json:"offerings"`
}

type signedEnrollmentSelections struct {
	OfferingID   int64    `json:"offering_id"`
	SelectedDays []string `json:"selected_days"`
}

// newSignedEnrollmentDocument builds the signed document from the stored
// request, the stored children's submitted values and their materialized
// offering choices.
func newSignedEnrollmentDocument(request *Request, children []SubmitChild, guardians []SubmitGuardian, selections [][]selection.Selection) signedEnrollmentDocument {
	doc := signedEnrollmentDocument{
		Format: signedEnrollmentFormat, TenantID: request.TenantID, RequestID: request.ID, PhaseID: request.PhaseID,
		SchemaID: request.SchemaID, GuardianFirstName: request.GuardianFirstName, GuardianLastName: request.GuardianLastName,
		GuardianEmail: request.GuardianEmail, GuardianPhone: request.GuardianPhone,
		AdditionalGuardians: make([]signedEnrollmentGuardian, 0, len(guardians)),
		ConsentFlags:        request.ConsentFlags, CustomData: request.CustomData,
		Children: make([]signedEnrollmentChild, 0, len(children)),
	}
	if n := len(request.LegalBlocksSnapshot); n > 0 {
		doc.LegalSnapshot = request.LegalBlocksSnapshot[n-1]
	}
	for _, g := range guardians {
		doc.AdditionalGuardians = append(doc.AdditionalGuardians, signedEnrollmentGuardian{FirstName: g.FirstName, LastName: g.LastName, Email: g.Email, Phone: g.Phone})
	}
	for i, c := range children {
		child := signedEnrollmentChild{
			FirstName: strings.TrimSpace(c.FirstName), LastName: strings.TrimSpace(c.LastName), DateOfBirth: c.DateOfBirth.String(),
			TargetGradeLevel: c.TargetGradeLevel, TargetSchoolClass: c.TargetSchoolClass, CustomData: c.CustomData,
			Offerings: []signedEnrollmentSelections{},
		}
		if i < len(selections) {
			for _, sel := range selections[i] {
				child.Offerings = append(child.Offerings, signedEnrollmentSelections{OfferingID: sel.OfferingID, SelectedDays: sel.SelectedDays})
			}
		}
		doc.Children = append(doc.Children, child)
	}
	return doc
}

// recordSignature stores the parent's signature of the request's current
// version inside the write transaction. The document checksum covers the
// stored submission with its legal snapshot.
func (s *Intake) recordSignature(ctx context.Context, request *Request, signature *enrollmentNative.SubmitSignature, doc signedEnrollmentDocument) error {
	encoded, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("signature: encode signed document: %w", err)
	}
	signerName := strings.TrimSpace(request.GuardianFirstName + " " + request.GuardianLastName)
	if _, err := s.deps.Signatures.RecordEnrollmentSignature(ctx, EnrollmentSignatureEntry{
		RequestID: request.ID, SignerAccountID: request.GuardianAccountID, SignerName: signerName,
		DocumentSHA256: s.deps.Fingerprint(encoded), SignedAt: time.Now(), Signature: *signature,
	}); err != nil {
		return signatureWriteError(err)
	}
	return nil
}

// signatureWriteError marks a refused image as the parent's input.
func signatureWriteError(err error) error {
	if errors.Is(err, enrollmentNative.ErrSignatureInvalid) {
		return enrollmentNative.InvalidInput(enrollmentNative.CodeSignatureInvalid, "signature", err)
	}
	return fmt.Errorf("signature: record: %w", err)
}

// RequestSignature returns the newest signature of a request of the school
// in context, nil when it has none.
func (s *Intake) RequestSignature(ctx context.Context, requestID int64) (*enrollmentNative.RequestSignature, error) {
	if s.deps.Signatures == nil {
		return nil, nil
	}
	signatures, err := s.deps.Signatures.LatestEnrollmentSignatures(ctx, []int64{requestID})
	if err != nil {
		return nil, fmt.Errorf("signature: read: %w", err)
	}
	signature := signatures[requestID]
	if signature == nil || signature.RequestID != requestID {
		return nil, nil
	}
	return signature, nil
}

// SignatureProofByStatusToken loads the request behind a valid status token
// with its co-guardians and newest signature. The token is the only
// authentication: the caller runs the lookup in an administrative
// transaction, and every read after it runs in the tenant transaction of the
// request the token names, for that request alone.
func (s *Intake) SignatureProofByStatusToken(ctx context.Context, token string) (*enrollmentNative.RequestSignatureProof, error) {
	req, children, err := s.statusByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	status, err := publicRequestStatus(req, children)
	if err != nil {
		return nil, err
	}
	proof := &enrollmentNative.RequestSignatureProof{Status: status}
	if s.deps.Runtime.IsAdminTx(ctx) {
		ctx = s.deps.Runtime.DetachTransaction(ctx)
	}
	err = s.deps.Runtime.TenantTx(s.deps.Runtime.WithTenant(ctx, req.TenantID), req.TenantID, func(txCtx context.Context) error {
		if s.deps.Guardians != nil {
			if proof.Guardians, err = s.deps.Guardians.RequestGuardians(txCtx, []int64{req.ID}); err != nil {
				return fmt.Errorf("signature proof: co-guardians: %w", err)
			}
		}
		proof.Signature, err = s.RequestSignature(txCtx, req.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return proof, nil
}
