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

// ReplaceEditable rewrites the editable payload of a request while every
// child is still submitted. It keeps the request id, the status token, the
// guardian account link and the original submitted_at.
func (s *Intake) ReplaceEditable(ctx context.Context, token string, incoming enrollment.SubmitRequest) (*enrollment.SubmitResult, error) {
	decoded, err := decodeSubmitRequest(incoming)
	if err != nil {
		return nil, err
	}
	outcome, err := s.replaceEditable(ctx, token, decoded)
	if err != nil {
		return nil, publicError(err)
	}
	return outcome.public()
}

// replacePlan is the state of one replacement edit inside its transaction.
type replacePlan struct {
	req             *enrollmentModels.Request
	children        []*RequestChild
	phase           *enrollment.Phase
	duplicatePolicy string
	capabilities    enrollment.FormCapabilities
	openByID        map[int64]*enrollmentModels.CareOffering
	selections      [][]selection.Selection
	schema          *enrollment.FormSchema
	legalBlocks     []enrollment.LegalBlock
	// eligibilityEnforced is false for trusted-source and rollover-generated
	// requests, which bypassed the self-service gates at creation (#1663).
	eligibilityEnforced bool
	matchedExisting     []*RequestChild
	overrides           map[int]string
}

func (s *Intake) replaceEditable(ctx context.Context, token string, incoming SubmitRequest) (*submitOutcome, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, enrollment.ErrRequestNotFound
	}
	var tenantID int64
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		req, err := s.requestByStatusToken(adminCtx, token, false)
		if err != nil {
			return err
		}
		tenantID = req.TenantID
		return nil
	}); err != nil {
		return nil, err
	}
	outcome := &submitOutcome{}
	if err := s.deps.Runtime.TenantTx(ctx, tenantID, func(txCtx context.Context) error {
		return s.replaceInTenant(txCtx, token, tenantID, incoming, outcome)
	}); err != nil {
		return nil, err
	}
	s.logger().Info("enrollment request edited by parent",
		slog.Int64("request_id", outcome.Request.ID),
		slog.Int64("tenant_id", tenantID),
		slog.Int("children", len(outcome.Children)))
	outcome.StatusURL = enrollment.StatusURL(s.deps.ParentsURL, outcome.Request.StatusToken)
	return outcome, nil
}

func (s *Intake) replaceInTenant(ctx context.Context, token string, tenantID int64, incoming SubmitRequest, outcome *submitOutcome) error {
	plan, editReq, err := s.lockEditableRequest(ctx, token, tenantID, incoming)
	if err != nil {
		return err
	}
	if err := s.planReplacementCatalog(ctx, &editReq, plan); err != nil {
		return err
	}
	if err := s.planReplacementContract(ctx, &editReq, plan); err != nil {
		return err
	}
	if err := s.prepareReplacementWrite(ctx, &editReq, plan); err != nil {
		return err
	}
	if outcome.Warnings, err = s.replaceRequestPayload(ctx, &editReq, plan); err != nil {
		return err
	}
	if err := createRequestGuardians(ctx, s.deps.Guardians, plan.req.ID, editReq.AdditionalGuardians, nil, "edit replace: create request guardian %d: %w"); err != nil {
		return err
	}
	if outcome.Children, err = s.createReplacementChildren(ctx, &editReq, plan); err != nil {
		return err
	}
	outcome.Request = plan.req
	if len(plan.overrides) == 0 || editReq.SuppressSubmissionEmails {
		return nil
	}
	if err := notifyDecisions(ctx, s.deps.Notifications, decisionGeneration{
		Request: plan.req, Children: outcome.Children, Phase: plan.phase,
		ImmediateChildIDs: childIDsForStatus(outcome.Children, enrollmentModels.ChildStatusWaitlisted), ParentsURL: s.deps.ParentsURL,
	}); err != nil {
		return fmt.Errorf("edit replace: notify capacity decisions: %w", err)
	}
	return nil
}

// lockEditableRequest locks the request and its children, checks the edit is
// allowed and binds the incoming payload to the stored request.
func (s *Intake) lockEditableRequest(ctx context.Context, token string, tenantID int64, incoming SubmitRequest) (*replacePlan, SubmitRequest, error) {
	req, err := s.requestByStatusToken(ctx, token, true)
	if err != nil {
		return nil, incoming, err
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, req.ID, true)
	if err != nil {
		return nil, incoming, fmt.Errorf("edit replace: lock children: %w", err)
	}
	if err := s.ensureRequestEditable(ctx, req, children); err != nil {
		return nil, incoming, err
	}
	editReq := incoming
	editReq.TenantID, editReq.PhaseID = tenantID, req.PhaseID
	editReq.GuardianEmail, editReq.GuardianAccountID = req.GuardianEmail, req.GuardianAccountID
	if editReq.ConsentFlags == nil {
		editReq.ConsentFlags = map[string]any{}
	}
	if editReq.CustomData == nil {
		editReq.CustomData = map[string]any{}
	}
	if editReq.Children == nil {
		editReq.Children = []SubmitChild{}
	}
	return &replacePlan{req: req, children: children}, editReq, nil
}

// planReplacementCatalog loads the phase, the duplicate policy, the
// capabilities and the open catalog and materializes the new picks.
func (s *Intake) planReplacementCatalog(ctx context.Context, editReq *SubmitRequest, plan *replacePlan) error {
	phase, err := s.loadPhaseForEditableRequest(ctx, plan.req.PhaseID)
	if err != nil {
		return err
	}
	plan.phase = phase
	if plan.duplicatePolicy, err = s.deps.Settings.DuplicateHandling(ctx); err != nil {
		return fmt.Errorf("edit replace: resolve duplicate handling: %w", err)
	}
	if plan.capabilities, err = s.formCapabilities(ctx); err != nil {
		return fmt.Errorf("edit replace: resolve form capabilities: %w", err)
	}
	openOfferings := []*enrollmentModels.CareOffering{}
	if plan.capabilities.CareOfferingsEnabled {
		if openOfferings, err = s.deps.Offerings.ListActiveByPhase(ctx, phase.ID); err != nil {
			return fmt.Errorf("edit replace: load phase offerings: %w", err)
		}
	}
	plan.capabilities = effectiveFormCapabilities(plan.capabilities, openOfferings)
	if err := normalizeSubmissionForCapabilities(editReq, plan.capabilities); err != nil {
		return err
	}
	plan.openByID = offeringsByID(openOfferings)
	plan.selections, err = materializeChildrenOfferingSelections(editReq.Children, plan.openByID, selectionModeFor(phase, plan.capabilities))
	return err
}

// planReplacementContract validates the edited payload like a submission.
// The linked_parents authorization carries over from the original
// submission; the per-child gates run again unless the request is exempt.
func (s *Intake) planReplacementContract(ctx context.Context, editReq *SubmitRequest, plan *replacePlan) error {
	var err error
	if plan.schema, err = s.schemaForEditableRequest(ctx, plan.req); err != nil {
		return err
	}
	texts, err := s.legalTexts(ctx)
	if err != nil {
		return fmt.Errorf("edit replace: resolve legal blocks: %w", err)
	}
	plan.legalBlocks = applyTemplateLegalBlocks(texts, plan.schema).Blocks
	if err := normalizeAdditionalGuardians(editReq); err != nil {
		return err
	}
	if err := s.validateSubmission(ctx, *editReq, plan.legalBlocks, plan.capabilities); err != nil {
		return err
	}
	if err := s.validateAndNormalizeSchoolClasses(ctx, plan.phase, editReq.Children); err != nil {
		return err
	}
	plan.eligibilityEnforced = !isTrustedEnrollmentSource(plan.req.SubmissionSource) && !hasRolloverGeneratedChild(plan.children)
	if plan.eligibilityEnforced {
		if err := s.validatePhaseChildEligibility(ctx, plan.phase, *editReq); err != nil {
			return err
		}
	}
	editReq.ConsentFlags = filterConsentFlags(editReq.ConsentFlags, plan.legalBlocks)
	if err := validateSubmittedFields(plan.schema, *editReq, plan.openByID, plan.children); err != nil {
		return err
	}
	mergeReplacementAnswers(plan.schema, editReq, plan.req.CustomData, plan.children, plan.openByID)
	return nil
}

// mergeReplacementAnswers sanitizes the edited answers and merges them over
// the stored ones, so keys the reopened form cannot render survive the edit.
func mergeReplacementAnswers(schema *enrollment.FormSchema, editReq *SubmitRequest, storedGuardianData map[string]any, children []*RequestChild, openByID map[int64]*enrollmentModels.CareOffering) {
	byKey := buildFieldsByKey(schema)
	rawGuardian := editReq.CustomData
	existingCustomData := existingChildCustomDataBySubmittedIdentity(children, editReq.Children)
	for i := range editReq.Children {
		childCtx := childVisibilityContext(rawGuardian, editReq.Children[i], openByID, byKey)
		sanitizedChild := sanitizeVisibleAnswers(schema, true, editReq.Children[i].CustomData, childCtx)
		pruneChildScheduleAnswers(schema, sanitizedChild, relevantCareDaysForChild(editReq.Children[i], openByID))
		editReq.Children[i].CustomData = mergeEditableCustomData(existingCustomData[i], sanitizedChild, schema, true)
	}
	editReq.CustomData = mergeEditableCustomData(storedGuardianData,
		sanitizeVisibleAnswers(schema, false, rawGuardian, fieldVisibilityContext{guardianAnswers: rawGuardian, fieldsByKey: byKey}),
		schema, false)
}

// prepareReplacementWrite takes the dedup lock, pins a rollover edit to its
// children's identities, keeps the slots the request already holds and runs
// the capacity gate.
func (s *Intake) prepareReplacementWrite(ctx context.Context, editReq *SubmitRequest, plan *replacePlan) error {
	plan.matchedExisting = matchExistingChildrenBySubmittedIdentity(plan.children, editReq.Children)
	emailLC := lowerTrim(plan.req.GuardianEmail)
	if err := s.deps.Requests.AcquireSubmissionDedupLock(ctx, plan.phase.ID, enrollment.SubmissionDedupLockKey(emailLC)); err != nil {
		return fmt.Errorf("edit replace: acquire dedup lock: %w", err)
	}
	existingChildIDs := make([]int64, 0, len(plan.children))
	for _, child := range plan.children {
		existingChildIDs = append(existingChildIDs, child.ID)
	}
	if hasRolloverGeneratedChild(plan.children) {
		if err := validateRolloverEditIdentity(plan.children, editReq.Children); err != nil {
			return err
		}
	}
	// One point-in-time read serves both uses: preserving a hidden selection
	// means preserving the one in force now.
	activeLinks, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, s.deps.Children, existingChildIDs, s.currentOfferingSelectionDate(plan.phase))
	if err != nil {
		return fmt.Errorf("edit replace: load active child offerings: %w", err)
	}
	if !plan.capabilities.CareOfferingsEnabled {
		plan.selections = preservedOfferingSelections(plan.children, editReq.Children, activeLinks)
	}
	plan.overrides = map[int]string{}
	if plan.capabilities.CareOfferingsEnabled {
		plan.overrides, err = s.applyCapacityOverflow(ctx, plan.phase, editReq.Children, preservedClaims(activeLinks), nil)
	}
	return err
}

// preservedClaims counts, per offering, the children already holding it.
func preservedClaims(links []*enrollment.RequestChildOfferingRecord) map[int64]int {
	claims := make(map[int64]int, len(links))
	seen := make(map[[2]int64]struct{}, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		key := [2]int64{link.RequestChildID, link.CareOfferingID}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		claims[link.CareOfferingID]++
	}
	return claims
}

func validateRolloverEditIdentity(existing []*RequestChild, incoming []SubmitChild) error {
	if len(incoming) != len(existing) {
		return enrollment.ErrEditNotAllowed
	}
	for i, child := range existing {
		next := incoming[i]
		if strings.TrimSpace(next.FirstName) != strings.TrimSpace(child.FirstName) ||
			strings.TrimSpace(next.LastName) != strings.TrimSpace(child.LastName) ||
			next.DateOfBirth != child.DateOfBirth {
			return enrollment.ErrEditNotAllowed
		}
	}
	return nil
}

// replaceRequestPayload deletes the old co-guardians and children, applies
// the duplicate policy and writes the edited guardian fields with an
// appended legal evidence entry.
func (s *Intake) replaceRequestPayload(ctx context.Context, editReq *SubmitRequest, plan *replacePlan) ([]enrollment.SubmissionWarning, error) {
	if s.deps.Guardians != nil {
		if err := s.deps.Guardians.DeleteRequestGuardians(ctx, plan.req.ID); err != nil {
			return nil, err
		}
	}
	if err := s.deps.Children.DeleteRequestChildren(ctx, plan.req.ID); err != nil {
		return nil, err
	}
	warnings, err := s.checkReplacementDuplicates(ctx, plan, duplicateChildKeys(editReq.Children))
	if err != nil {
		return nil, err
	}
	req := plan.req
	req.GuardianFirstName = strings.TrimSpace(editReq.GuardianFirstName)
	req.GuardianLastName = strings.TrimSpace(editReq.GuardianLastName)
	req.GuardianPhone = editReq.GuardianPhone
	req.ConsentFlags = editReq.ConsentFlags
	req.CustomData = editReq.CustomData
	// Append-only evidence: the edit re-confirms the legal blocks in force
	// now next to the earlier entries, which keep the pre-edit answers.
	snapshotEntry, err := legalBlocksSnapshotEntry(plan.legalBlocks, editReq.ConsentFlags, time.Now())
	if err != nil {
		return nil, err
	}
	req.LegalBlocksSnapshot = append(req.LegalBlocksSnapshot, snapshotEntry)
	if err := updateDecodedRequest(ctx, s.deps.Requests, req, false); err != nil {
		return nil, err
	}
	return warnings, nil
}

// checkReplacementDuplicates applies the duplicate policy of an edit. Unlike
// a submission it refuses an unsupported policy even without a duplicate.
func (s *Intake) checkReplacementDuplicates(ctx context.Context, plan *replacePlan, dupKeys []enrollment.DuplicateChildKey) ([]enrollment.SubmissionWarning, error) {
	dupes, err := s.deps.Requests.ActiveDuplicateChildren(ctx, plan.phase.ID, plan.req.GuardianEmail, dupKeys, 0)
	if err != nil {
		return nil, fmt.Errorf("edit replace: duplicate check: %w", err)
	}
	var warnings []enrollment.SubmissionWarning
	if len(dupes) > 0 && plan.duplicatePolicy == duplicateHandlingBlock {
		return nil, enrollment.ErrDuplicateEnrollment
	}
	if len(dupes) > 0 && plan.duplicatePolicy == duplicateHandlingWarn {
		warnings = []enrollment.SubmissionWarning{{Code: enrollment.WarningCodeDuplicateEnrollment}}
	}
	switch plan.duplicatePolicy {
	case duplicateHandlingBlock, duplicateHandlingWarn, duplicateHandlingIgnore:
		return warnings, nil
	default:
		return nil, fmt.Errorf("edit replace: unsupported duplicate handling %q", plan.duplicatePolicy)
	}
}

func (s *Intake) createReplacementChildren(ctx context.Context, editReq *SubmitRequest, plan *replacePlan) ([]*RequestChild, error) {
	created := make([]*RequestChild, 0, len(editReq.Children))
	for i, child := range editReq.Children {
		row := newSubmittedChild(plan.req.ID, i, child, plan.overrides)
		if matched := plan.matchedExisting[i]; matched != nil {
			row.ActivationMode, row.ActivateOn = matched.ActivationMode, matched.ActivateOn
			row.RolloverSourceChildID, row.ReviewReason = matched.RolloverSourceChildID, matched.ReviewReason
		}
		// A rollover child resolves its student through the rollover source
		// chain at approval, so a pin would be dead data.
		if row.RolloverSourceChildID == nil {
			pin, err := s.replacementStudentPin(ctx, plan, i, child)
			if err != nil {
				return nil, err
			}
			row.MatchedStudentID = pin
		}
		if err := createDecodedChild(ctx, s.deps.Children, row); err != nil {
			return nil, fmt.Errorf("edit replace: create request child %d: %w", i, err)
		}
		if err := s.recordOfferingSubmission(ctx, row.ID, plan.selections[i], plan.phase.ServiceStartDate, plan.phase.ServiceEndDate); err != nil {
			return nil, fmt.Errorf("edit replace: record child offerings: %w", err)
		}
		created = append(created, row)
	}
	return created, nil
}

// replacementStudentPin carries an existing_students child's pin over an
// edit (#1663). The pin of an unchanged identity survives even if the phase
// audience changed since, so approval still renews the right student; an
// edited identity drops it and, while the phase is existing_students,
// re-resolves by the new identity. The pin is authorized against the stored
// request's identity and kept unique in the phase.
func (s *Intake) replacementStudentPin(ctx context.Context, plan *replacePlan, i int, child SubmitChild) (*int64, error) {
	req := plan.req
	var pin *int64
	if matched := plan.matchedExisting[i]; matched != nil && sameSubmittedIdentity(matched, child) {
		pin = matched.MatchedStudentID
	}
	if plan.phase.Audience == enrollment.PhaseAudienceExistingStudents {
		resolved, err := s.resolveMatchedStudentID(ctx, req.TenantID, plan.phase, i, child)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			pin = resolved
		}
	}
	if err := assertExistingStudentMatchResolved(plan.phase, pin, plan.eligibilityEnforced, i); err != nil {
		return nil, err
	}
	submitter := reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, req.GuardianEmail)
	if pin != nil {
		resolvedSubmitter, err := reEnrollmentSubmitterForPersistedRequest(ctx, s.deps.LateInvites, req)
		if err != nil {
			return nil, fmt.Errorf("edit replace: resolve re-enrollment identity: %w", err)
		}
		submitter = resolvedSubmitter
	}
	if err := s.assertGuardianMayReEnrollStudent(ctx, submitter, pin, req.TenantID, i); err != nil {
		return nil, err
	}
	if err := s.guardMatchedStudentUnique(ctx, plan.phase.ID, pin, 0, i); err != nil {
		return nil, err
	}
	return pin, nil
}
