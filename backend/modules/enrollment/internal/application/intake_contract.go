package application

import (
	"context"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
)

// eligibilityGate is how much of the phase eligibility (#1663) a payload
// must pass. The trust behind it is decided once per path: a submission
// trusts its caller (staff entering a paper enrollment, the holder of a late
// invite), an edit trusts the stored source of the request.
type eligibilityGate int

const (
	// eligibilityAll runs the audience gate and the per-child gates.
	eligibilityAll eligibilityGate = iota
	// eligibilityChildren runs the per-child gates only.
	eligibilityChildren
	// eligibilityNone runs no eligibility gate.
	eligibilityNone
)

// submissionEligibility is the gate of a new submission: a trusted caller
// passes without one.
func submissionEligibility(req SubmitRequest) eligibilityGate {
	if req.AllowClosedPhase {
		return eligibilityNone
	}
	return eligibilityAll
}

// storedRequestEligibility is the gate of a change to a stored request (a
// parent's edit, its draft, a change request): the audience was authorized
// at submission, and a request from a trusted source or with a child the
// rollover carried forward keeps the exemption its creation had.
func storedRequestEligibility(source string, children []*enrollmentNative.RequestChild) eligibilityGate {
	if isTrustedEnrollmentSource(source) || hasRolloverGeneratedChild(children) {
		return eligibilityNone
	}
	return eligibilityChildren
}

// intakePayload is what the submission and the edit check a payload
// against. existingChildren are the stored children an edit pairs the
// payload with; a submission has none.
type intakePayload struct {
	phase            *enrollmentNative.Phase
	schema           *enrollmentNative.FormSchema
	legalBlocks      []enrollmentNative.LegalBlock
	capabilities     enrollmentNative.FormCapabilities
	openByID         map[int64]*offeringContract.CareOfferingRecord
	eligibility      eligibilityGate
	consentConfirmed bool
	existingChildren []*enrollmentNative.RequestChild
}

// validateIntakePayload is the one validation pipeline of the submission and
// the edit: consents, co-guardians, guardian and children, classes,
// eligibility and form fields, in that order. Eligibility runs after the
// class canonicalization, so it validates the stored class, not the raw
// client value (#1663).
func (s *Intake) validateIntakePayload(ctx context.Context, req *SubmitRequest, p intakePayload) error {
	if p.consentConfirmed {
		req.ConsentFlags = ensureRequiredConsentFlags(req.ConsentFlags, p.legalBlocks)
	}
	if err := normalizeAdditionalGuardians(req); err != nil {
		return err
	}
	if err := s.validateSubmission(ctx, *req, p.legalBlocks, p.capabilities); err != nil {
		return err
	}
	if err := s.validateAndNormalizeSchoolClasses(ctx, p.phase, req.Children); err != nil {
		return err
	}
	if err := s.validateEligibility(ctx, p.phase, *req, p.eligibility); err != nil {
		return err
	}
	req.ConsentFlags = filterConsentFlags(req.ConsentFlags, p.legalBlocks)
	if err := validateSubmittedFields(p.schema, *req, p.openByID, p.existingChildren); err != nil {
		return err
	}
	dropUnaskedHolidayCareWish(p.schema, req.Children)
	return nil
}

func (s *Intake) validateEligibility(ctx context.Context, phase *enrollmentNative.Phase, req SubmitRequest, gate eligibilityGate) error {
	switch gate {
	case eligibilityAll:
		return s.validatePhaseEligibility(ctx, phase, req)
	case eligibilityChildren:
		return s.validatePhaseChildEligibility(ctx, phase, req)
	default:
		return nil
	}
}
