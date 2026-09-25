package application

import (
	"context"
	"fmt"
	"maps"
	"slices"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// proposal is a validated, normalized change request payload ready to store
// or apply.
type proposal struct {
	request    SubmitRequest
	selections [][]selection.Selection
	phase      *enrollment.Phase
	offerings  map[int64]*enrollmentModels.CareOffering
}

// proposalOptions tune prepareProposed to its two callers. Creation locks the
// children taken over into care; the approval restores the hidden bookings a
// disabled capability kept in the stored proposal.
type proposalOptions struct {
	allowStoredHiddenOfferings bool
	lockTakenOverChildren      bool
}

// prepareProposed validates a proposal the way a submission is validated —
// capabilities, offerings, classes, eligibility, guardians, consents and form
// fields — and returns it normalized. Creation and approval both run it.
func (s *ChangeRequests) prepareProposed(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild, incoming SubmitRequest, capabilities enrollment.FormCapabilities, allowStoredHiddenOfferings, lockTakenOverChildren bool) (*proposal, error) {
	opts := proposalOptions{allowStoredHiddenOfferings: allowStoredHiddenOfferings, lockTakenOverChildren: lockTakenOverChildren}
	editReq, err := s.bindProposal(ctx, req, children, incoming, opts)
	if err != nil {
		return nil, err
	}
	if opts.allowStoredHiddenOfferings && !capabilities.CareOfferingsEnabled {
		// A disabled proposal snapshot holds the stored, hidden links so it
		// stays stable across setting toggles; they are restored from the
		// locked rows below, not validated as fresh parent input.
		clearHiddenProposalOfferings(&editReq)
	}
	phase, err := s.intake.intakePhase(ctx, req.PhaseID)
	if err != nil && !s.deps.Runtime.NotFound(err) {
		return nil, err
	}
	if err != nil || phase == nil || !phase.IsActive {
		return nil, enrollment.ErrEnrollmentDisabled
	}
	prepared := &proposal{phase: phase}
	capabilities, err = s.materializeProposal(ctx, &editReq, children, capabilities, opts, prepared)
	if err != nil {
		return nil, err
	}
	if err := s.validateProposal(ctx, req, &editReq, children, capabilities, opts, prepared); err != nil {
		return nil, err
	}
	prepared.request = editReq
	return prepared, nil
}

// bindProposal binds the incoming payload to the stored request, pins the
// children's identities and, on creation, keeps every child taken over into
// care exactly as stored.
func (s *ChangeRequests) bindProposal(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild, incoming SubmitRequest, opts proposalOptions) (SubmitRequest, error) {
	editReq := incoming
	editReq.TenantID, editReq.PhaseID = req.TenantID, req.PhaseID
	editReq.GuardianEmail, editReq.GuardianAccountID = req.GuardianEmail, req.GuardianAccountID
	if editReq.ConsentFlags == nil {
		editReq.ConsentFlags = map[string]any{}
	}
	if editReq.CustomData == nil {
		editReq.CustomData = map[string]any{}
	}
	if len(editReq.Children) != len(children) {
		return editReq, fmt.Errorf("%w: child count changes are not supported for change requests", enrollment.ErrChangeRequestInvalidData)
	}
	if err := validateChangeRequestChildIdentity(children, editReq.Children); err != nil {
		return editReq, err
	}
	if opts.lockTakenOverChildren {
		if err := s.keepTakenOverChildren(ctx, req, children, &editReq); err != nil {
			return editReq, err
		}
	}
	return editReq, nil
}

// keepTakenOverChildren refuses a proposal that changes a child already taken
// over into care and pins those children to their stored data.
func (s *ChangeRequests) keepTakenOverChildren(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild, editReq *SubmitRequest) error {
	persisted, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return err
	}
	if err := ensureTakenOverChildrenUnchanged(children, persisted, submitSnapshot(*editReq)); err != nil {
		return err
	}
	persistedSubmit, err := snapshotToSubmitRequest(persisted)
	if err != nil {
		return err
	}
	for i, child := range children {
		if childTakenOver(child) {
			editReq.Children[i] = persistedSubmit.Children[i]
		}
	}
	return nil
}

func clearHiddenProposalOfferings(editReq *SubmitRequest) {
	for i := range editReq.Children {
		editReq.Children[i].OfferingIDs = nil
		editReq.Children[i].OfferingDays = nil
	}
}

// materializeProposal loads the catalog each child may book — the open
// offerings plus the ones it holds now — and materializes the picks, or,
// with care offerings off, restores the bookings in force.
func (s *ChangeRequests) materializeProposal(ctx context.Context, editReq *SubmitRequest, children []*RequestChild, capabilities enrollment.FormCapabilities, opts proposalOptions, prepared *proposal) (enrollment.FormCapabilities, error) {
	prepared.offerings = map[int64]*enrollmentModels.CareOffering{}
	var catalogs []map[int64]*enrollmentModels.CareOffering
	openOfferings := []*enrollmentModels.CareOffering{}
	if capabilities.CareOfferingsEnabled {
		var err error
		if openOfferings, err = s.deps.Offerings.ListActiveByPhase(ctx, prepared.phase.ID); err != nil {
			return capabilities, fmt.Errorf("change request: load phase offerings: %w", err)
		}
		if catalogs, prepared.offerings, err = s.changeRequestOfferingCatalogs(ctx, children, offeringsByID(openOfferings), s.intake.currentOfferingSelectionDate(prepared.phase)); err != nil {
			return capabilities, err
		}
	}
	capabilityOfferings := openOfferings
	if len(prepared.offerings) > 0 {
		capabilityOfferings = slices.Collect(maps.Values(prepared.offerings))
	}
	capabilities = effectiveFormCapabilities(capabilities, capabilityOfferings)
	if err := normalizeChangeRequestChildren(editReq, children, capabilities, opts.lockTakenOverChildren); err != nil {
		return capabilities, err
	}
	if capabilities.CareOfferingsEnabled {
		selections, err := materializeChangeRequestChildren(editReq.Children, children, catalogs, prepared.phase.CareOfferingSelectionMode, opts.lockTakenOverChildren)
		prepared.selections = selections
		return capabilities, err
	}
	return capabilities, s.restoreProposalOfferings(ctx, editReq, children, prepared)
}

// restoreProposalOfferings copies the bookings in force now into the
// proposal while care offerings are off; a superseded booking restored here
// would be written back as a live one.
func (s *ChangeRequests) restoreProposalOfferings(ctx context.Context, editReq *SubmitRequest, children []*RequestChild, prepared *proposal) error {
	childIDs := make([]int64, 0, len(children))
	for _, child := range children {
		childIDs = append(childIDs, child.ID)
	}
	existingLinks, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, s.deps.Children, childIDs, s.intake.currentOfferingSelectionDate(prepared.phase))
	if err != nil {
		return fmt.Errorf("change request: preserve child offerings: %w", err)
	}
	prepared.selections = preservedOfferingSelections(children, editReq.Children, existingLinks)
	applyPreservedOfferingSelections(editReq.Children, prepared.selections)
	return nil
}

// validateProposal checks the proposal like a submission and sanitizes its
// answers over the stored ones.
func (s *ChangeRequests) validateProposal(ctx context.Context, req *enrollmentModels.Request, editReq *SubmitRequest, children []*RequestChild, capabilities enrollment.FormCapabilities, opts proposalOptions, prepared *proposal) error {
	schema, err := s.schemaForRequest(ctx, req)
	if err != nil {
		return err
	}
	legalBlocks, err := s.legalBlocksForRequest(ctx, schema)
	if err != nil {
		return err
	}
	if err := normalizeAdditionalGuardians(editReq); err != nil {
		return err
	}
	if err := s.intake.validateSubmission(ctx, changeRequestValidationSubmission(*editReq, children, opts.lockTakenOverChildren), legalBlocks, capabilities); err != nil {
		return err
	}
	// The class rules hang off the tenant setting and the phase (#1833),
	// like on submit and on a replacement edit.
	if err := s.normalizeChangeRequestSchoolClasses(ctx, prepared.phase, editReq, children, opts.lockTakenOverChildren); err != nil {
		return err
	}
	if err := s.validateProposalEligibility(ctx, req, *editReq, children, prepared.phase, opts); err != nil {
		return err
	}
	if err := s.validateAccountLinkedGuardianEdits(ctx, req, *editReq); err != nil {
		return err
	}
	editReq.ConsentFlags = filterConsentFlags(editReq.ConsentFlags, legalBlocks)
	if err := validateSubmittedFields(schema, changeRequestValidationSubmission(*editReq, children, opts.lockTakenOverChildren), prepared.offerings, children); err != nil {
		return err
	}
	mergeProposalAnswers(schema, editReq, req.CustomData, children, prepared.offerings, opts.lockTakenOverChildren)
	return nil
}

// validateProposalEligibility applies the eligible classes and grades — a
// proper subset of the offered ones — and the enrolled-status gate of the
// children whose identity the proposal rewrites (#1663). Trusted-source and
// rollover-generated requests keep the exemption their creation had.
func (s *ChangeRequests) validateProposalEligibility(ctx context.Context, req *enrollmentModels.Request, editReq SubmitRequest, children []*RequestChild, phase *enrollment.Phase, opts proposalOptions) error {
	if isTrustedEnrollmentSource(req.SubmissionSource) || hasRolloverGeneratedChild(children) {
		return nil
	}
	validationReq := changeRequestValidationSubmission(editReq, children, opts.lockTakenOverChildren)
	if err := validateChildGradeEligibility(phase, validationReq.Children); err != nil {
		return err
	}
	if err := validateChildClassEligibility(phase, validationReq.Children); err != nil {
		return err
	}
	return s.validateChangedChildIdentityEligibility(ctx, phase, editReq, children)
}

// validateChangedChildIdentityEligibility re-applies the enrolled-status gate
// to the children whose submitted identity the proposal rewrites. An
// unchanged child is skipped: its own approval may have created the matching
// student, and probing it would refuse every later change request (#1663).
func (s *ChangeRequests) validateChangedChildIdentityEligibility(ctx context.Context, phase *enrollment.Phase, editReq SubmitRequest, children []*RequestChild) error {
	for i := range editReq.Children {
		if i < len(children) && sameSubmittedIdentity(children[i], editReq.Children[i]) {
			continue
		}
		if err := s.intake.validateChildEnrolledStatus(ctx, phase, editReq.TenantID, editReq.Children[i], i); err != nil {
			return err
		}
	}
	return nil
}

func mergeProposalAnswers(schema *enrollment.FormSchema, editReq *SubmitRequest, storedGuardianData map[string]any, children []*RequestChild, offerings map[int64]*enrollmentModels.CareOffering, skipTakenOver bool) {
	byKey := buildFieldsByKey(schema)
	rawGuardian := editReq.CustomData
	existingCustomData := existingChildCustomDataBySubmittedIdentity(children, editReq.Children)
	for i := range editReq.Children {
		if skipTakenOver && childTakenOver(children[i]) {
			continue
		}
		childCtx := childVisibilityContext(rawGuardian, editReq.Children[i], offerings, byKey)
		sanitizedChild := sanitizeVisibleAnswers(schema, true, editReq.Children[i].CustomData, childCtx)
		pruneChildScheduleAnswers(schema, sanitizedChild, relevantCareDaysForChild(editReq.Children[i], offerings))
		editReq.Children[i].CustomData = mergeEditableCustomData(existingCustomData[i], sanitizedChild, schema, true)
	}
	editReq.CustomData = mergeEditableCustomData(storedGuardianData,
		sanitizeVisibleAnswers(schema, false, rawGuardian, fieldVisibilityContext{guardianAnswers: rawGuardian, fieldsByKey: byKey}),
		schema, false)
}

// changeRequestOfferingCatalogs returns each child's catalog — the open
// offerings plus the ones it holds on the given day, even if deactivated —
// and all of them combined.
func (s *ChangeRequests) changeRequestOfferingCatalogs(ctx context.Context, children []*RequestChild, openByID map[int64]*enrollmentModels.CareOffering, onDate calendar.Date) ([]map[int64]*enrollmentModels.CareOffering, map[int64]*enrollmentModels.CareOffering, error) {
	childIDs := make([]int64, 0, len(children))
	childIndexByID := make(map[int64]int, len(children))
	for i, child := range children {
		if child == nil {
			continue
		}
		childIDs = append(childIDs, child.ID)
		childIndexByID[child.ID] = i
	}
	links, err := enrollment.OfferingSelectionRecordsForChildrenAt(ctx, s.deps.Children, childIDs, onDate)
	if err != nil {
		return nil, nil, fmt.Errorf("change request: load current child offerings: %w", err)
	}
	currentByChild, currentIDs := heldOfferingsByChild(links, childIndexByID, len(children), openByID)
	combinedByID, err := s.combinedOfferingCatalog(ctx, openByID, currentIDs)
	if err != nil {
		return nil, nil, err
	}
	catalogs := make([]map[int64]*enrollmentModels.CareOffering, len(children))
	for i := range children {
		catalog := maps.Clone(openByID)
		for id := range currentByChild[i] {
			if offering := combinedByID[id]; offering != nil {
				catalog[id] = offering
			}
		}
		catalogs[i] = catalog
	}
	return catalogs, combinedByID, nil
}

// heldOfferingsByChild groups the held offerings by child index and collects
// the held offerings the open catalog lacks.
func heldOfferingsByChild(links []*enrollment.RequestChildOfferingRecord, childIndexByID map[int64]int, childCount int, openByID map[int64]*enrollmentModels.CareOffering) ([]map[int64]bool, map[int64]bool) {
	currentIDs := make(map[int64]bool)
	currentByChild := make([]map[int64]bool, childCount)
	for _, link := range links {
		if link == nil {
			continue
		}
		idx, ok := childIndexByID[link.RequestChildID]
		if !ok {
			continue
		}
		if currentByChild[idx] == nil {
			currentByChild[idx] = map[int64]bool{}
		}
		currentByChild[idx][link.CareOfferingID] = true
		if openByID[link.CareOfferingID] == nil {
			currentIDs[link.CareOfferingID] = true
		}
	}
	return currentByChild, currentIDs
}

// combinedOfferingCatalog adds the currently held, inactive offerings to the
// open ones.
func (s *ChangeRequests) combinedOfferingCatalog(ctx context.Context, openByID map[int64]*enrollmentModels.CareOffering, currentIDs map[int64]bool) (map[int64]*enrollmentModels.CareOffering, error) {
	combined := maps.Clone(openByID)
	if combined == nil {
		combined = map[int64]*enrollmentModels.CareOffering{}
	}
	if len(currentIDs) == 0 {
		return combined, nil
	}
	current, err := s.deps.Offerings.ListByIDs(ctx, slices.Collect(maps.Keys(currentIDs)))
	if err != nil {
		return nil, fmt.Errorf("change request: load current inactive offerings: %w", err)
	}
	for _, offering := range current {
		if offering != nil {
			combined[offering.ID] = offering
		}
	}
	return combined, nil
}

// materializeChangeRequestChildren validates each child's picks against its
// own catalog. On creation, children taken over into care are skipped.
func materializeChangeRequestChildren(children []SubmitChild, existing []*RequestChild, catalogs []map[int64]*enrollmentModels.CareOffering, selectionMode string, skipTakenOver bool) ([][]selection.Selection, error) {
	out := make([][]selection.Selection, len(children))
	for i := range children {
		if skipTakenOver && i < len(existing) && childTakenOver(existing[i]) {
			continue
		}
		catalog := map[int64]*enrollmentModels.CareOffering{}
		if i < len(catalogs) && catalogs[i] != nil {
			catalog = catalogs[i]
		}
		selections, err := materializeChildOfferingSelection(i, &children[i], catalog, selectionMode)
		if err != nil {
			return nil, err
		}
		if i < len(existing) && existing[i] == nil {
			return nil, fmt.Errorf("%w: child %d missing existing row", enrollment.ErrChangeRequestInvalidData, i)
		}
		out[i] = selections
	}
	return out, nil
}

func mutableChangeRequestChildren(submitted []SubmitChild, existing []*RequestChild) []SubmitChild {
	mutable := make([]SubmitChild, 0, len(submitted))
	for i, child := range submitted {
		if i < len(existing) && childTakenOver(existing[i]) {
			continue
		}
		mutable = append(mutable, child)
	}
	return mutable
}

func normalizeChangeRequestChildren(req *SubmitRequest, existing []*RequestChild, capabilities enrollment.FormCapabilities, skipTakenOver bool) error {
	for i := range req.Children {
		if skipTakenOver && i < len(existing) && childTakenOver(existing[i]) {
			continue
		}
		child := SubmitRequest{Children: []SubmitChild{req.Children[i]}}
		if err := normalizeSubmissionForCapabilities(&child, capabilities); err != nil {
			return err
		}
		req.Children[i] = child.Children[0]
	}
	return nil
}

func (s *ChangeRequests) normalizeChangeRequestSchoolClasses(ctx context.Context, phase *enrollment.Phase, req *SubmitRequest, existing []*RequestChild, skipTakenOver bool) error {
	for i := range req.Children {
		if skipTakenOver && i < len(existing) && childTakenOver(existing[i]) {
			continue
		}
		child := []SubmitChild{req.Children[i]}
		if err := s.intake.validateAndNormalizeSchoolClasses(ctx, phase, child); err != nil {
			return err
		}
		req.Children[i] = child[0]
	}
	return nil
}

func changeRequestValidationSubmission(req SubmitRequest, existing []*RequestChild, skipTakenOver bool) SubmitRequest {
	if !skipTakenOver {
		return req
	}
	req.Children = mutableChangeRequestChildren(req.Children, existing)
	return req
}

// applyPreservedOfferingSelections copies hidden stored links into the
// canonical proposal, so a later setting toggle does not read as a removal.
func applyPreservedOfferingSelections(children []SubmitChild, selections [][]selection.Selection) {
	for i := range children {
		children[i].OfferingIDs = nil
		children[i].OfferingDays = nil
		if i >= len(selections) {
			continue
		}
		for _, pick := range selections[i] {
			children[i].OfferingIDs = append(children[i].OfferingIDs, pick.OfferingID)
			if len(pick.SelectedDays) > 0 {
				children[i].OfferingDays = append(children[i].OfferingDays, SubmitOfferingDays{OfferingID: pick.OfferingID, SelectedDays: copyDays(pick.SelectedDays)})
			}
		}
	}
}

func (s *ChangeRequests) schemaForRequest(ctx context.Context, req *enrollmentModels.Request) (*enrollment.FormSchema, error) {
	if req.SchemaID == nil {
		return nil, nil
	}
	schema, err := s.intake.intakeSchema(ctx, *req.SchemaID)
	if err != nil {
		return nil, fmt.Errorf("change request: load schema: %w", err)
	}
	return schema, nil
}

func (s *ChangeRequests) legalBlocksForRequest(ctx context.Context, schema *enrollment.FormSchema) ([]enrollment.LegalBlock, error) {
	texts, err := s.intake.legalTexts(ctx)
	if err != nil {
		return nil, err
	}
	return applyTemplateLegalBlocks(texts, schema).Blocks, nil
}

func validateChangeRequestChildIdentity(existing []*RequestChild, incoming []SubmitChild) error {
	if len(existing) != len(incoming) {
		return fmt.Errorf("%w: child count changes are not supported for change requests", enrollment.ErrChangeRequestInvalidData)
	}
	for i, child := range existing {
		if incoming[i].ID <= 0 {
			return fmt.Errorf("%w: child %d id is required", enrollment.ErrChangeRequestInvalidData, i)
		}
		if incoming[i].ID != child.ID {
			return fmt.Errorf("%w: child order or identity changes are not supported for change requests", enrollment.ErrChangeRequestInvalidData)
		}
	}
	return nil
}
