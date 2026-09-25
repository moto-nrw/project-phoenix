package application

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// editDraft collects the persisted request that reopens the public form.
type editDraft struct {
	req           *enrollmentModels.Request
	children      []*RequestChild
	childIDs      []int64
	guardians     []*enrollment.RequestGuardian
	school        *enrollment.School
	links         []*enrollment.RequestChildOfferingRecord
	phase         *enrollment.Phase
	schema        *enrollment.FormSchema
	openOfferings []*enrollmentModels.CareOffering
	legalTexts    enrollment.LegalTexts
	editMode      string
	capabilities  enrollment.FormCapabilities
	gradeLevelMax int
}

// EditDraft loads the persisted request that reopens the public form. It is
// token-gated like the status page and enforces the edit window up front, so
// a locked request never leaks a stale editable draft.
func (s *Intake) EditDraft(ctx context.Context, token string) (*enrollment.EditDraft, error) {
	draft, err := s.editDraft(ctx, token)
	if err != nil {
		return nil, publicError(err)
	}
	return draft.public()
}

func (s *Intake) editDraft(ctx context.Context, token string) (*editDraft, error) {
	draft := &editDraft{}
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		return s.loadDraftRequest(adminCtx, token, draft)
	}); err != nil {
		return nil, err
	}
	if err := s.deps.Runtime.TenantTx(ctx, draft.req.TenantID, func(txCtx context.Context) error {
		return s.loadDraftForm(txCtx, draft)
	}); err != nil {
		return nil, err
	}
	return draft, nil
}

// loadDraftRequest reads the request, its children, co-guardians and school
// behind the token in the administrative transaction.
func (s *Intake) loadDraftRequest(adminCtx context.Context, token string, draft *editDraft) error {
	req, err := s.requestByStatusToken(adminCtx, token, false)
	if err != nil {
		return err
	}
	draft.req = req
	tenantCtx := s.deps.Runtime.WithTenant(adminCtx, req.TenantID)
	if draft.children, err = decodedChildrenOfRequest(tenantCtx, s.deps.Children, req.ID, false); err != nil {
		return fmt.Errorf("edit draft: list children: %w", err)
	}
	if s.deps.Guardians != nil {
		if draft.guardians, err = s.deps.Guardians.RequestGuardians(tenantCtx, []int64{req.ID}); err != nil {
			return fmt.Errorf("edit draft: list guardians: %w", err)
		}
	}
	for _, c := range draft.children {
		draft.childIDs = append(draft.childIDs, c.ID)
	}
	if s.deps.Schools != nil {
		if draft.school, err = s.deps.Schools.FindSchool(adminCtx, req.TenantID); err != nil {
			return fmt.Errorf("edit draft: load school: %w", err)
		}
	}
	return nil
}

// loadDraftForm resolves the form the draft reopens in the tenant
// transaction: capabilities, edit gates, phase, current bookings, schema,
// offerings and legal texts.
func (s *Intake) loadDraftForm(ctx context.Context, draft *editDraft) error {
	var err error
	if draft.capabilities, err = s.formCapabilities(ctx); err != nil {
		return fmt.Errorf("edit draft: resolve form capabilities: %w", err)
	}
	if draft.gradeLevelMax, err = s.resolveGradeMax(ctx); err != nil {
		return fmt.Errorf("edit draft: %w", err)
	}
	if err := s.loadDraftPhase(ctx, draft); err != nil {
		return err
	}
	if draft.schema, err = s.schemaForEditableRequest(ctx, draft.req); err != nil {
		return err
	}
	if err := s.loadDraftOfferings(ctx, draft); err != nil {
		return err
	}
	draft.capabilities = effectiveFormCapabilities(draft.capabilities, draft.openOfferings)
	texts, err := s.legalTexts(ctx)
	if err != nil {
		return err
	}
	draft.legalTexts = applyTemplateLegalBlocks(texts, draft.schema)
	return nil
}

// loadDraftPhase applies the edit gates and loads the phase with the
// bookings in force now; the form presents exactly the world the save will
// accept.
func (s *Intake) loadDraftPhase(ctx context.Context, draft *editDraft) error {
	draft.editMode = editModeForChildren(draft.children)
	if draft.editMode == enrollment.EditModeDirectEdit {
		if err := s.ensureRequestEditable(ctx, draft.req, draft.children); err != nil {
			return err
		}
	} else if err := s.ensureChangeRequestDraftAvailable(ctx, draft.req, draft.children); err != nil {
		return err
	}
	phase, err := s.loadPhaseForEditableRequest(ctx, draft.req.PhaseID)
	if err != nil {
		return err
	}
	if draft.editMode == enrollment.EditModeChangeRequest && !phase.EnrollmentWindowOpen(time.Now()) {
		return enrollment.ErrEnrollmentWindowClosed
	}
	draft.phase = phase
	// After a dated change the unscoped read returns every interval, so the
	// parent would find the superseded and the current booking both ticked.
	if draft.links, err = enrollment.OfferingSelectionRecordsForChildrenAt(ctx, s.deps.Children, draft.childIDs, s.currentOfferingSelectionDate(phase)); err != nil {
		return fmt.Errorf("edit draft: list child offerings: %w", err)
	}
	// The edit paths re-run the eligibility gates for an enforced draft, so
	// its classes narrow to the eligible subset; an exempt draft keeps the
	// full list and drops the grade restriction (#1663).
	if isTrustedEnrollmentSource(draft.req.SubmissionSource) || hasRolloverGeneratedChild(draft.children) {
		clearGradeRestrictionForEligibilityExemptForm(phase)
	} else {
		narrowOfferedClassesToEligibleForForm(phase)
	}
	return nil
}

func (s *Intake) schemaForEditableRequest(ctx context.Context, req *enrollmentModels.Request) (*enrollment.FormSchema, error) {
	if req != nil && req.SchemaID != nil {
		return s.intakeSchema(ctx, *req.SchemaID)
	}
	return nil, nil
}

// loadDraftOfferings loads the open offerings. A change request keeps a
// booked offering that was deactivated since; a direct edit cannot reopen on
// a booking the form no longer offers.
func (s *Intake) loadDraftOfferings(ctx context.Context, draft *editDraft) error {
	draft.openOfferings = []*enrollmentModels.CareOffering{}
	if !draft.capabilities.CareOfferingsEnabled {
		return nil
	}
	list, err := s.deps.Offerings.ListActiveByPhase(ctx, draft.phase.ID)
	if err != nil {
		return fmt.Errorf("edit draft: list active offerings: %w", err)
	}
	draft.openOfferings = list
	openByID := offeringsByID(list)
	missing := make(map[int64]struct{})
	for _, link := range draft.links {
		if _, ok := openByID[link.CareOfferingID]; ok {
			continue
		}
		if draft.editMode != enrollment.EditModeChangeRequest {
			return enrollment.ErrEditNotAllowed
		}
		missing[link.CareOfferingID] = struct{}{}
	}
	if len(missing) == 0 {
		return nil
	}
	current, err := s.deps.Offerings.ListByIDs(ctx, slices.Collect(maps.Keys(missing)))
	if err != nil {
		return fmt.Errorf("edit draft: list current inactive offerings: %w", err)
	}
	for _, offering := range current {
		if offering == nil || offering.PhaseID != draft.phase.ID {
			continue
		}
		draft.openOfferings = append(draft.openOfferings, offering)
		delete(missing, offering.ID)
	}
	if len(missing) > 0 {
		return enrollment.ErrEditNotAllowed
	}
	return nil
}

func (d *editDraft) public() (*enrollment.EditDraft, error) {
	request, err := requestInput(d.req)
	if err != nil {
		return nil, err
	}
	children, err := childInputs(d.children)
	if err != nil {
		return nil, err
	}
	linksByChild := make(map[int64][]*enrollment.RequestChildOfferingRecord, len(d.children))
	for _, link := range d.links {
		linksByChild[link.RequestChildID] = append(linksByChild[link.RequestChildID], link)
	}
	return &enrollment.EditDraft{
		Request: request, Children: children, Guardians: d.guardians, OfferingsByChild: linksByChild,
		Phase: d.phase, School: d.school, Schema: d.schema, OpenOfferings: publicCareOfferings(d.openOfferings),
		LegalTexts: d.legalTexts, EditMode: d.editMode,
		CollectSchoolClass: d.capabilities.CollectSchoolClass, CollectGradeLevel: d.capabilities.CollectGradeLevel,
		CareOfferingsEnabled: d.capabilities.CareOfferingsEnabled, GradeLevelMax: d.gradeLevelMax,
	}, nil
}
