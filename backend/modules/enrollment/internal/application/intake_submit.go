package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
)

// Submit stores a submission: in one transaction the request, its children,
// co-guardians, offering choices and bookings and the confirmation mails.
// Either everything lands or nothing does. The phase carries the window, the
// form schema and the overflow mode.
func (s *Intake) Submit(ctx context.Context, req enrollment.SubmitRequest) (*enrollment.SubmitResult, error) {
	decoded, err := decodeSubmitRequest(req)
	if err != nil {
		return nil, err
	}
	outcome, err := s.submit(ctx, decoded)
	if err != nil {
		return nil, publicError(err)
	}
	return outcome.public()
}

// submissionPlan is everything a submission resolves before its write
// transaction.
type submissionPlan struct {
	phase            *enrollment.Phase
	now              time.Time
	capabilities     enrollment.FormCapabilities
	duplicatePolicy  string
	dupKeys          []enrollment.DuplicateChildKey
	openByID         map[int64]*enrollmentModels.CareOffering
	selections       [][]selection.Selection
	capacityChildren []SubmitChild
	schema           *enrollment.FormSchema
	legalBlocks      []enrollment.LegalBlock
	submissionSource string
	sourceMetadata   map[string]any
	statusToken      string
	statusURL        string
	statusExpiresAt  time.Time
}

func (s *Intake) submit(ctx context.Context, req SubmitRequest) (*submitOutcome, error) {
	if !req.SkipRateLimit {
		if err := s.enforceRateLimit(ctx, req); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(req.LateInviteToken) != "" {
		req.AllowClosedPhase = true
		req.SubmissionSource = enrollmentModels.RequestSourceLateInvite
	}
	plan := &submissionPlan{
		submissionSource: enrollment.NormalizedSubmissionSource(req.SubmissionSource),
		sourceMetadata:   cloneSourceMetadata(req.SourceMetadata),
	}
	if err := s.planSubmissionCatalog(ctx, &req, plan); err != nil {
		return nil, err
	}
	if err := s.planSubmissionContract(ctx, &req, plan); err != nil {
		return nil, err
	}
	if err := s.planStatusToken(ctx, plan); err != nil {
		return nil, err
	}
	outcome := &submitOutcome{StatusURL: plan.statusURL}
	if err := s.deps.Runtime.Transactions.RunInTx(ctx, func(txCtx context.Context) error {
		return s.writeSubmission(txCtx, &req, plan, outcome)
	}); err != nil {
		return nil, err
	}
	s.logger().Info("enrollment request submitted",
		slog.Int64("request_id", outcome.Request.ID),
		slog.Int64("tenant_id", req.TenantID),
		slog.Int("children", len(outcome.Children)))
	return outcome, nil
}

// planSubmissionCatalog loads the phase, checks its tenant and window,
// resolves the capabilities and the duplicate policy, and materializes the
// children's offering picks against the phase's open catalog.
func (s *Intake) planSubmissionCatalog(ctx context.Context, req *SubmitRequest, plan *submissionPlan) error {
	phase, err := s.loadPhaseForSubmission(ctx, req.PhaseID)
	if err != nil {
		return err
	}
	// The parent submit path resolves the phase under an administrative
	// transaction, so another school's phase would load cleanly. Refuse it
	// with the not-found shape before any decision consumes it (#1663).
	if phase.TenantID != req.TenantID {
		return fmt.Errorf("%w: phase %d not found", enrollment.ErrInvalidSubmission, req.PhaseID)
	}
	plan.phase, plan.now = phase, time.Now()
	if !req.AllowClosedPhase && !phase.EnrollmentWindowOpen(plan.now) {
		return enrollment.ErrEnrollmentWindowClosed
	}
	if plan.capabilities, err = s.formCapabilities(ctx); err != nil {
		return fmt.Errorf("submit: resolve form capabilities: %w", err)
	}
	if plan.duplicatePolicy, err = s.deps.Settings.DuplicateHandling(ctx); err != nil {
		return fmt.Errorf("submit: resolve duplicate handling: %w", err)
	}
	plan.dupKeys = duplicateChildKeys(req.Children)
	openOfferings := []*enrollmentModels.CareOffering{}
	if plan.capabilities.CareOfferingsEnabled {
		if openOfferings, err = s.deps.Offerings.ListActiveByPhase(ctx, phase.ID); err != nil {
			return fmt.Errorf("submit: load phase offerings: %w", err)
		}
	}
	plan.capabilities = effectiveFormCapabilities(plan.capabilities, openOfferings)
	if err := normalizeSubmissionForCapabilities(req, plan.capabilities); err != nil {
		return err
	}
	plan.openByID = offeringsByID(openOfferings)
	if plan.selections, err = materializeChildrenOfferingSelections(req.Children, plan.openByID, selectionModeFor(phase, plan.capabilities)); err != nil {
		return err
	}
	plan.capacityChildren = childrenWithMaterializedOfferingSelections(req.Children, plan.selections)
	return nil
}

// selectionModeFor is the phase's care-offering selection mode, optional
// while care offerings are off.
func selectionModeFor(phase *enrollment.Phase, capabilities enrollment.FormCapabilities) string {
	if !capabilities.CareOfferingsEnabled {
		return enrollment.PhaseCareOfferingSelectionOptional
	}
	return phase.CareOfferingSelectionMode
}

func duplicateChildKeys(children []SubmitChild) []enrollment.DuplicateChildKey {
	keys := make([]enrollment.DuplicateChildKey, 0, len(children))
	for _, c := range children {
		keys = append(keys, enrollment.DuplicateChildKey{FirstName: c.FirstName, LastName: c.LastName})
	}
	return keys
}

// planSubmissionContract resolves the pinned schema and the legal contract,
// validates the guardian, the children, the classes, the eligibility and the
// form fields, and strips the answers the parent could not see.
func (s *Intake) planSubmissionContract(ctx context.Context, req *SubmitRequest, plan *submissionPlan) error {
	schema, err := s.resolveSubmissionSchema(ctx, plan.phase)
	if err != nil {
		return fmt.Errorf("submit: load schema: %w", err)
	}
	plan.schema = schema
	if plan.legalBlocks, err = s.resolveSubmissionLegalBlocks(ctx, schema); err != nil {
		return fmt.Errorf("submit: resolve legal blocks: %w", err)
	}
	if req.ExternalConsentConfirmed {
		req.ConsentFlags = ensureRequiredConsentFlags(req.ConsentFlags, plan.legalBlocks)
	}
	if err := normalizeAdditionalGuardians(req); err != nil {
		return err
	}
	if err := s.validateSubmission(ctx, *req, plan.legalBlocks, plan.capabilities); err != nil {
		return err
	}
	if err := s.validateAndNormalizeSchoolClasses(ctx, plan.phase, req.Children); err != nil {
		return err
	}
	// Eligibility runs after the class canonicalization, so it validates the
	// stored class, not the raw client value (#1663).
	if err := s.validatePhaseEligibility(ctx, plan.phase, *req); err != nil {
		return err
	}
	req.ConsentFlags = filterConsentFlags(req.ConsentFlags, plan.legalBlocks)
	if err := validateSubmittedFields(schema, *req, plan.openByID); err != nil {
		return err
	}
	if schema != nil {
		sanitizeSubmittedAnswers(schema, req, plan.openByID)
	}
	return nil
}

// validateSubmittedFields runs the required-field, companion-note and
// schedule checks of the form.
func validateSubmittedFields(schema *enrollment.FormSchema, req SubmitRequest, openByID map[int64]*enrollmentModels.CareOffering, existingChildren ...[]*RequestChild) error {
	if err := validateRequiredCustomFields(schema, req, openByID); err != nil {
		return err
	}
	if err := validateAccompaniedCompanionNote(schema, req, openByID); err != nil {
		return err
	}
	return validateConstrainedSchedules(schema, req, openByID, existingChildren...)
}

// sanitizeSubmittedAnswers drops the answers of hidden and undeclared fields
// and the schedule entries on days the child cannot schedule. Conditions read
// the raw answers, like the client and the required-field check.
func sanitizeSubmittedAnswers(schema *enrollment.FormSchema, req *SubmitRequest, openByID map[int64]*enrollmentModels.CareOffering) {
	byKey := buildFieldsByKey(schema)
	rawGuardian := req.CustomData
	for i := range req.Children {
		childCtx := childVisibilityContext(rawGuardian, req.Children[i], openByID, byKey)
		req.Children[i].CustomData = sanitizeVisibleAnswers(schema, true, req.Children[i].CustomData, childCtx)
		pruneChildScheduleAnswers(schema, req.Children[i].CustomData, relevantCareDaysForChild(req.Children[i], openByID))
	}
	req.CustomData = sanitizeVisibleAnswers(schema, false, rawGuardian,
		fieldVisibilityContext{guardianAnswers: rawGuardian, fieldsByKey: byKey})
}

func (s *Intake) planStatusToken(ctx context.Context, plan *submissionPlan) error {
	token, err := s.newStatusToken()
	if err != nil {
		return fmt.Errorf("submit: generate status token: %w", err)
	}
	plan.statusToken = token
	plan.statusExpiresAt = time.Now().Add(s.resolveStatusTokenExpiry(ctx))
	plan.statusURL = enrollment.StatusURL(s.deps.ParentsURL, token)
	return nil
}

// writeSubmission is the write transaction of a submission.
func (s *Intake) writeSubmission(ctx context.Context, req *SubmitRequest, plan *submissionPlan, outcome *submitOutcome) error {
	// Serialize concurrent submits of one (phase, guardian email), so two
	// parallel requests cannot both pass the duplicate check.
	emailLC := lowerTrim(req.GuardianEmail)
	if err := s.deps.Requests.AcquireSubmissionDedupLock(ctx, plan.phase.ID, enrollment.SubmissionDedupLockKey(emailLC)); err != nil {
		return fmt.Errorf("submit: acquire dedup lock: %w", err)
	}
	lateInvite, err := s.submissionLateInvite(ctx, req, plan)
	if err != nil {
		return err
	}
	// A late-invite holder may correct the contact address, but an
	// accountless renewal stays authorized as the invited guardian (#1663,
	// #2164).
	reEnrollEmail := emailLC
	if lateInvite != nil && req.GuardianAccountID == nil {
		reEnrollEmail = lateInvite.GuardianEmail
	}
	submitter := reEnrollmentSubmitterFor(plan.submissionSource, req.GuardianAccountID, reEnrollEmail)
	if outcome.Warnings, err = s.checkSubmissionDuplicates(ctx, plan.phase.ID, req.GuardianEmail, plan.dupKeys, plan.duplicatePolicy, "submit"); err != nil {
		return err
	}
	overrides := map[int]string{}
	if plan.capabilities.CareOfferingsEnabled {
		if overrides, err = s.applyCapacityOverflow(ctx, plan.phase, plan.capacityChildren, nil, nil); err != nil {
			return err
		}
	}
	request, err := s.createSubmittedRequest(ctx, req, plan, lateInvite)
	if err != nil {
		return err
	}
	outcome.Request = request
	if err := createRequestGuardians(ctx, s.deps.Guardians, request.ID, req.AdditionalGuardians, nil, "submit: create request guardian %d: %w"); err != nil {
		return err
	}
	if outcome.Children, err = s.createSubmittedChildren(ctx, req, plan, request, overrides, submitter); err != nil {
		return err
	}
	return s.notifySubmission(ctx, req, plan, outcome, len(overrides) > 0)
}

// submissionLateInvite resolves and locks the late invite of a submission
// and records it in the source metadata.
func (s *Intake) submissionLateInvite(ctx context.Context, req *SubmitRequest, plan *submissionPlan) (*enrollment.LateInvite, error) {
	if strings.TrimSpace(req.LateInviteToken) == "" {
		return nil, nil
	}
	invite, err := s.findLateInviteForSubmit(ctx, req.LateInviteToken, plan.phase.ID, plan.now)
	if err != nil {
		return nil, err
	}
	plan.sourceMetadata["late_invite_id"] = invite.ID
	return invite, nil
}

// checkSubmissionDuplicates applies the duplicate policy under the dedup
// lock. Rejected and withdrawn rows do not count, so a parent can re-apply
// after a denial.
func (s *Intake) checkSubmissionDuplicates(ctx context.Context, phaseID int64, guardianEmail string, dupKeys []enrollment.DuplicateChildKey, policy, errPrefix string) ([]enrollment.SubmissionWarning, error) {
	dupes, err := s.deps.Requests.ActiveDuplicateChildren(ctx, phaseID, guardianEmail, dupKeys, 0)
	if err != nil {
		return nil, fmt.Errorf("%s: duplicate check: %w", errPrefix, err)
	}
	if len(dupes) == 0 {
		return nil, nil
	}
	switch policy {
	case duplicateHandlingBlock:
		return nil, enrollment.ErrDuplicateEnrollment
	case duplicateHandlingWarn:
		return []enrollment.SubmissionWarning{{Code: enrollment.WarningCodeDuplicateEnrollment}}, nil
	case duplicateHandlingIgnore:
		return nil, nil
	default:
		return nil, fmt.Errorf("%s: unsupported duplicate handling %q", errPrefix, policy)
	}
}

func (s *Intake) createSubmittedRequest(ctx context.Context, req *SubmitRequest, plan *submissionPlan, lateInvite *enrollment.LateInvite) (*enrollmentModels.Request, error) {
	var schemaID *int64
	if plan.schema != nil {
		id := plan.schema.ID
		schemaID = &id
	}
	snapshotEntry, err := legalBlocksSnapshotEntry(plan.legalBlocks, req.ConsentFlags, time.Now())
	if err != nil {
		return nil, fmt.Errorf("submit: create request: %w", err)
	}
	statusExpiresAt := plan.statusExpiresAt
	request := &enrollmentModels.Request{
		SchemaID: schemaID, PhaseID: plan.phase.ID, GuardianAccountID: req.GuardianAccountID,
		GuardianFirstName: strings.TrimSpace(req.GuardianFirstName), GuardianLastName: strings.TrimSpace(req.GuardianLastName),
		GuardianEmail: lowerTrim(req.GuardianEmail), GuardianPhone: req.GuardianPhone,
		ConsentFlags: req.ConsentFlags, CustomData: req.CustomData,
		SubmissionSource: plan.submissionSource, SourceMetadata: plan.sourceMetadata,
		StatusToken: plan.statusToken, StatusTokenExpires: &statusExpiresAt, SubmittedAt: time.Now(),
		LegalBlocksSnapshot: []enrollmentModels.LegalBlocksSnapshotEntry{snapshotEntry},
	}
	if err := createDecodedRequest(ctx, s.deps.Requests, request); err != nil {
		return nil, fmt.Errorf("submit: create request: %w", err)
	}
	if lateInvite != nil {
		if err := s.deps.LateInvites.MarkLateInviteUsed(ctx, lateInvite.ID, request.ID, time.Now()); err != nil {
			return nil, fmt.Errorf("submit: mark late invite used: %w", err)
		}
	}
	return request, nil
}

// createRequestGuardians stores the normalized co-guardians in order.
// profileIDs, when given, carries a stamped guardian profile over. Without
// the port nothing is stored.
func createRequestGuardians(ctx context.Context, owner IntakeGuardians, requestID int64, guardians []SubmitGuardian, profileIDs func(SubmitGuardian) *int64, errFormat string) error {
	if owner == nil {
		return nil
	}
	for i, g := range guardians {
		row := &enrollment.RequestGuardian{
			RequestID: requestID, FirstName: g.FirstName, LastName: g.LastName, Email: g.Email, Phone: g.Phone, SortOrder: i,
		}
		if profileIDs != nil {
			row.GuardianProfileID = profileIDs(g)
		}
		if err := owner.CreateRequestGuardian(ctx, row); err != nil {
			return fmt.Errorf(errFormat, i, err)
		}
	}
	return nil
}

func (s *Intake) createSubmittedChildren(ctx context.Context, req *SubmitRequest, plan *submissionPlan, request *enrollmentModels.Request, overrides map[int]string, submitter reEnrollmentSubmitter) ([]*RequestChild, error) {
	created := make([]*RequestChild, 0, len(req.Children))
	for i, child := range req.Children {
		row := newSubmittedChild(request.ID, i, child, overrides)
		matchedStudentID, err := s.resolveMatchedStudentID(ctx, req.TenantID, plan.phase, i, child)
		if err != nil {
			return nil, err
		}
		// !AllowClosedPhase means the eligibility gate ran, so a vanished
		// match is a race, not a fresh create (#1663).
		if err := assertExistingStudentMatchResolved(plan.phase, matchedStudentID, !req.AllowClosedPhase, i); err != nil {
			return nil, err
		}
		if err := s.assertGuardianMayReEnrollStudent(ctx, submitter, matchedStudentID, req.TenantID, i); err != nil {
			return nil, err
		}
		if err := s.guardMatchedStudentUnique(ctx, plan.phase.ID, matchedStudentID, 0, i); err != nil {
			return nil, err
		}
		row.MatchedStudentID = matchedStudentID
		if err := createDecodedChild(ctx, s.deps.Children, row); err != nil {
			return nil, fmt.Errorf("submit: create request child %d: %w", i, err)
		}
		if err := s.recordOfferingSubmission(ctx, row.ID, plan.selections[i], plan.phase.ServiceStartDate, plan.phase.ServiceEndDate); err != nil {
			return nil, fmt.Errorf("submit: record child offerings: %w", err)
		}
		created = append(created, row)
	}
	return created, nil
}

// newSubmittedChild is the stored row of a submitted child; a capacity
// override replaces the submitted status.
func newSubmittedChild(requestID int64, index int, child SubmitChild, overrides map[int]string) *RequestChild {
	status := enrollmentModels.ChildStatusSubmitted
	if override, ok := overrides[index]; ok {
		status = override
	}
	return &RequestChild{
		RequestID: requestID, FirstName: strings.TrimSpace(child.FirstName), LastName: strings.TrimSpace(child.LastName),
		DateOfBirth: child.DateOfBirth, TargetGradeLevel: child.TargetGradeLevel, TargetSchoolClass: child.TargetSchoolClass,
		CustomData: child.CustomData, Status: status, ActivationMode: enrollmentModels.ChildActivationScheduled, SortOrder: index,
	}
}

// notifySubmission notifies the capacity decisions a submission produced and
// enqueues the confirmation mails, unless the caller suppresses them.
func (s *Intake) notifySubmission(ctx context.Context, req *SubmitRequest, plan *submissionPlan, outcome *submitOutcome, capacityDecided bool) error {
	if req.SuppressSubmissionEmails {
		return nil
	}
	if capacityDecided {
		if err := notifyDecisions(ctx, s.deps.Notifications, decisionGeneration{
			Request: outcome.Request, Children: outcome.Children, Phase: plan.phase,
			ImmediateChildIDs: childIDsForStatus(outcome.Children, enrollmentModels.ChildStatusWaitlisted), ParentsURL: s.deps.ParentsURL,
		}); err != nil {
			return fmt.Errorf("submit: notify capacity decisions: %w", err)
		}
	}
	if err := s.enqueueSubmissionEmails(ctx, req.TenantID, outcome.Request, outcome.Children, plan.statusURL); err != nil {
		return fmt.Errorf("submit: enqueue submission emails: %w", err)
	}
	return nil
}
