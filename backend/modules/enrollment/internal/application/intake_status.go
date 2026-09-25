package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// The status link: a request behind its public token, the parent's edits of
// it before the first decision, the withdrawal and the renewal confirmation.

// StatusByToken loads a request and its children for the public status
// page. The caller runs it in an administrative transaction: the token is
// the only authentication.
func (s *Intake) StatusByToken(ctx context.Context, token string) (*enrollment.RequestStatus, error) {
	req, children, err := s.statusByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return publicRequestStatus(req, children)
}

func publicRequestStatus(req *enrollmentModels.Request, children []*RequestChild) (*enrollment.RequestStatus, error) {
	request, err := requestInput(req)
	if err != nil {
		return nil, err
	}
	values, err := childInputs(children)
	if err != nil {
		return nil, err
	}
	return &enrollment.RequestStatus{Request: request, Children: values}, nil
}

// requestByStatusToken reads the request behind a token that has not
// expired; any miss is ErrRequestNotFound.
func (s *Intake) requestByStatusToken(ctx context.Context, token string, lock bool) (*enrollmentModels.Request, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, enrollment.ErrRequestNotFound
	}
	req, err := decodedRequestByToken(ctx, s.deps.Requests, token, lock)
	if err != nil || req == nil {
		return nil, enrollment.ErrRequestNotFound
	}
	if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
		return nil, enrollment.ErrRequestNotFound
	}
	return req, nil
}

func (s *Intake) statusByToken(ctx context.Context, token string) (*enrollmentModels.Request, []*RequestChild, error) {
	req, err := s.requestByStatusToken(ctx, token, false)
	if err != nil {
		return nil, nil, err
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	children, err := decodedChildrenOfRequest(tenantCtx, s.deps.Children, req.ID, false)
	if err != nil {
		return nil, nil, fmt.Errorf("status: list children: %w", err)
	}
	// status_reason is admin-internal free text; it reaches a parent only
	// when the phase opts in.
	if !s.statusReasonVisibleToParent(tenantCtx, req.PhaseID) {
		for _, c := range children {
			c.StatusReason = nil
		}
	}
	return req, children, nil
}

// statusReasonVisibleToParent fails closed: when the phase cannot be loaded
// the reason is redacted.
func (s *Intake) statusReasonVisibleToParent(ctx context.Context, phaseID int64) bool {
	phase, err := s.intakePhase(ctx, phaseID)
	if err != nil {
		s.logger().Warn("status: phase load failed; redacting status reason",
			slog.Int64("phase_id", phaseID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return phase.ShowStatusReasonToParent
}

// EditModeForStatus returns the edit path the status page may offer. It
// mirrors the draft's gates without loading the draft.
func (s *Intake) EditModeForStatus(ctx context.Context, status *enrollment.RequestStatus) (string, error) {
	if status == nil || status.Request == nil {
		return enrollment.EditModeNone, nil
	}
	req, err := requestValue(status.Request)
	if err != nil {
		return enrollment.EditModeNone, err
	}
	children, err := childValues(status.Children)
	if err != nil {
		return enrollment.EditModeNone, err
	}
	return s.editModeForStatus(s.deps.Runtime.WithTenant(ctx, req.TenantID), req, children)
}

func (s *Intake) editModeForStatus(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) (string, error) {
	mode := editModeForChildren(children)
	switch mode {
	case enrollment.EditModeDirectEdit:
		if s.ensureRequestEditable(ctx, req, children) != nil {
			return enrollment.EditModeNone, nil
		}
		if _, err := s.loadPhaseForEditableRequest(ctx, req.PhaseID); err != nil {
			return editModeGateResult(err)
		}
		return enrollment.EditModeDirectEdit, nil
	case enrollment.EditModeChangeRequest:
		if s.ensureChangeRequestDraftAvailable(ctx, req, children) != nil {
			return enrollment.EditModeNone, nil
		}
		phase, err := s.loadPhaseForEditableRequest(ctx, req.PhaseID)
		if err != nil {
			return editModeGateResult(err)
		}
		if !phase.EnrollmentWindowOpen(time.Now()) {
			return enrollment.EditModeNone, nil
		}
		return enrollment.EditModeChangeRequest, nil
	default:
		return enrollment.EditModeNone, nil
	}
}

// editModeGateResult turns a closed phase gate into "no edit" and reports
// every other failure.
func editModeGateResult(err error) (string, error) {
	if errors.Is(err, enrollment.ErrEnrollmentDisabled) || errors.Is(err, enrollment.ErrInvalidSubmission) {
		return enrollment.EditModeNone, nil
	}
	return enrollment.EditModeNone, err
}

func editModeForChildren(children []*RequestChild) string {
	if len(children) == 0 {
		return enrollment.EditModeDirectEdit
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return enrollment.EditModeChangeRequest
		}
	}
	return enrollment.EditModeDirectEdit
}

// ensureRequestEditable allows a direct edit while the request is not
// withdrawn, edits are allowed and every child is still submitted.
func (s *Intake) ensureRequestEditable(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) error {
	if req == nil || req.WithdrawnAt != nil || !s.allowSubmissionEdit(ctx) || len(children) == 0 {
		return enrollment.ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return enrollment.ErrEditNotAllowed
		}
	}
	return nil
}

// ensureChangeRequestDraftAvailable allows a change request while the
// request is decided in part, nothing is withdrawn and at least one child is
// not yet taken over into care.
func (s *Intake) ensureChangeRequestDraftAvailable(ctx context.Context, req *enrollmentModels.Request, children []*RequestChild) error {
	if req == nil || req.WithdrawnAt != nil || !s.allowSubmissionEdit(ctx) || len(children) == 0 {
		return enrollment.ErrEditNotAllowed
	}
	if editModeForChildren(children) != enrollment.EditModeChangeRequest {
		return enrollment.ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status == enrollmentModels.ChildStatusWithdrawn {
			return enrollment.ErrEditNotAllowed
		}
	}
	if allChildrenTakenOver(children) {
		return enrollment.ErrEditNotAllowed
	}
	return nil
}

// GuardiansByStatusToken loads the co-guardians behind a status token. The
// caller runs it in an administrative transaction.
func (s *Intake) GuardiansByStatusToken(ctx context.Context, token string) ([]*enrollment.RequestGuardian, error) {
	if s.deps.Guardians == nil {
		return nil, nil
	}
	req, err := s.requestByStatusToken(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return s.deps.Guardians.RequestGuardians(s.deps.Runtime.WithTenant(ctx, req.TenantID), []int64{req.ID})
}

// Edit patches the guardian fields while every child is still submitted.
func (s *Intake) Edit(ctx context.Context, token string, patch enrollment.EditPatch) error {
	consentFlags, err := decodeJSONMap(patch.ConsentFlags, "consent flags")
	if err != nil {
		return err
	}
	customData, err := decodeJSONMap(patch.CustomData, "custom data")
	if err != nil {
		return err
	}
	req, children, err := s.statusByToken(ctx, token)
	if err != nil {
		return err
	}
	if !s.allowSubmissionEdit(ctx) {
		return enrollment.ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status != enrollmentModels.ChildStatusSubmitted {
			return enrollment.ErrEditNotAllowed
		}
	}
	if err := applyEditPatch(req, patch, consentFlags, customData); err != nil {
		return err
	}
	if s.deps.Runtime.IsAdminTx(ctx) {
		// The token lookup runs in an administrative transaction; the write
		// opens a tenant transaction instead of reusing its unscoped role.
		ctx = s.deps.Runtime.DetachTransaction(ctx)
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	return s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		return s.writeEditPatch(txCtx, req, consentFlags != nil)
	})
}

func applyEditPatch(req *enrollmentModels.Request, patch enrollment.EditPatch, consentFlags, customData map[string]any) error {
	if patch.GuardianFirstName != nil {
		req.GuardianFirstName = strings.TrimSpace(*patch.GuardianFirstName)
	}
	if patch.GuardianLastName != nil {
		req.GuardianLastName = strings.TrimSpace(*patch.GuardianLastName)
	}
	if patch.GuardianPhone != nil {
		v := strings.TrimSpace(*patch.GuardianPhone)
		// Same canonical phone check as the submission.
		if err := contact.ValidateOptionalPhone(v); err != nil {
			return enrollment.ErrInvalidGuardianPhone
		}
		req.GuardianPhone = &v
	}
	if consentFlags != nil {
		req.ConsentFlags = consentFlags
	}
	if customData != nil {
		req.CustomData = customData
	}
	return nil
}

// writeEditPatch filters the patched guardian answers against the request's
// immutable schema and the consent flags against its legal contract.
func (s *Intake) writeEditPatch(ctx context.Context, req *enrollmentModels.Request, consentPatched bool) error {
	var schema *enrollment.FormSchema
	if req.SchemaID != nil {
		loaded, err := s.intakeSchema(ctx, *req.SchemaID)
		if err != nil {
			return fmt.Errorf("edit: load pinned schema: %w", err)
		}
		schema = loaded
		byKey := buildFieldsByKey(schema)
		req.CustomData = sanitizeVisibleAnswers(schema, false, req.CustomData,
			fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey})
	}
	if consentPatched {
		blocks, err := s.resolveSubmissionLegalBlocks(ctx, schema)
		if err != nil {
			return fmt.Errorf("edit: resolve legal blocks: %w", err)
		}
		req.ConsentFlags = filterConsentFlags(req.ConsentFlags, blocks)
	}
	return updateDecodedRequest(ctx, s.deps.Requests, req, false)
}

// Withdraw moves one child, or every open child when childID is 0, to
// withdrawn. An approved child has a student and goes through the admin.
func (s *Intake) Withdraw(ctx context.Context, token string, childID int64) error {
	var req *enrollmentModels.Request
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		var loadErr error
		req, _, loadErr = s.statusByToken(adminCtx, token)
		return loadErr
	}); err != nil {
		return err
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	return s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		return s.withdrawInTenant(txCtx, token, childID)
	})
}

func (s *Intake) withdrawInTenant(ctx context.Context, token string, childID int64) error {
	lockedReq, err := decodedRequestByToken(ctx, s.deps.Requests, strings.TrimSpace(token), true)
	if err != nil || lockedReq == nil {
		return enrollment.ErrRequestNotFound
	}
	if lockedReq.StatusTokenExpires != nil && time.Now().After(*lockedReq.StatusTokenExpires) {
		return enrollment.ErrRequestNotFound
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, lockedReq.ID, true)
	if err != nil {
		return fmt.Errorf("withdraw: lock request children: %w", err)
	}
	anyWithdrawn, err := s.withdrawChildren(ctx, children, childID)
	if err != nil || !anyWithdrawn {
		return err
	}
	if childID == 0 {
		withdrawnAt := time.Now()
		if err := s.deps.Requests.SetRequestWithdrawal(ctx, lockedReq.ID, &withdrawnAt); err != nil {
			return err
		}
	}
	if !childrenParentResolved(children) {
		return nil
	}
	return s.enqueueWithdrawDecision(ctx, lockedReq)
}

func (s *Intake) withdrawChildren(ctx context.Context, children []*RequestChild, childID int64) (bool, error) {
	anyWithdrawn := false
	for _, child := range children {
		if childID != 0 && child.ID != childID {
			continue
		}
		if child.Status == enrollmentModels.ChildStatusApproved {
			if childID != 0 {
				return false, enrollment.ErrWithdrawNotAllowed
			}
			continue
		}
		if (&enrollment.RequestChild{Status: child.Status}).IsTerminal() {
			continue
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0); err != nil {
			return false, err
		}
		child.Status, child.StatusReason, anyWithdrawn = enrollmentModels.ChildStatusWithdrawn, nil, true
	}
	return anyWithdrawn, nil
}

func (s *Intake) enqueueWithdrawDecision(ctx context.Context, request *enrollmentModels.Request) error {
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, request.ID, false)
	if err != nil {
		return fmt.Errorf("withdraw: refresh children for decision digest: %w", err)
	}
	phase, err := s.intakePhase(ctx, request.PhaseID)
	if err != nil {
		return fmt.Errorf("withdraw: load phase for decision digest: %w", err)
	}
	if err := notifyDecisions(ctx, s.deps.Notifications, decisionGeneration{
		Request: request, Children: children, Phase: phase, ParentsURL: s.deps.ParentsURL,
	}); err != nil {
		return fmt.Errorf("withdraw: notify completed decision state: %w", err)
	}
	return nil
}

// ConfirmRenewal moves every pending_renewal child of the request to
// submitted. Idempotent: other rows are skipped, so a double click is no
// error.
func (s *Intake) ConfirmRenewal(ctx context.Context, token string) (int, error) {
	req, children, err := s.statusByToken(ctx, token)
	if err != nil {
		return 0, err
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	var confirmed int
	if err := s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		for _, c := range children {
			if c.Status != enrollmentModels.ChildStatusPendingRenewal {
				continue
			}
			if err := s.deps.Children.UpdateChildStatus(txCtx, c.ID, enrollmentModels.ChildStatusSubmitted, nil, 0); err != nil {
				return err
			}
			confirmed++
		}
		return nil
	}); err != nil {
		return 0, err
	}
	if confirmed > 0 {
		s.logger().Info("renewal confirmed by parent",
			slog.Int64("request_id", req.ID),
			slog.Int("children_confirmed", confirmed),
		)
	}
	return confirmed, nil
}
