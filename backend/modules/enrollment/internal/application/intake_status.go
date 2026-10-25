package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/contact"
)

// The status link: a request behind its public token, the parent's edits of
// it before the first decision, the withdrawal and the renewal confirmation.

// StatusByToken loads a request and its children for the public status
// page. The caller runs it in an administrative transaction: the token is
// the only authentication.
func (s *Intake) StatusByToken(ctx context.Context, token string) (*enrollmentNative.RequestStatus, error) {
	req, children, err := s.statusByToken(ctx, token)
	if err != nil {
		return nil, err
	}
	return publicRequestStatus(req, children)
}

func publicRequestStatus(req *Request, children []*RequestChild) (*enrollmentNative.RequestStatus, error) {
	request, err := requestInput(req)
	if err != nil {
		return nil, err
	}
	values, err := childInputs(children)
	if err != nil {
		return nil, err
	}
	return &enrollmentNative.RequestStatus{Request: request, Children: values}, nil
}

// requestByStatusToken reads the request behind a token that has not
// expired; any miss is ErrRequestNotFound.
func (s *Intake) requestByStatusToken(ctx context.Context, token string, lock bool) (*Request, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, enrollmentNative.ErrRequestNotFound
	}
	req, err := decodedRequestByToken(ctx, s.deps.Requests, token, lock)
	if (err != nil && s.deps.Runtime.NotFound(err)) || (err == nil && req == nil) {
		return nil, enrollmentNative.ErrRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("load request by status token: %w", err)
	}
	if req.StatusTokenExpires != nil && time.Now().After(*req.StatusTokenExpires) {
		return nil, enrollmentNative.ErrRequestNotFound
	}
	// A school that cancelled Online-Anmeldung closes every status link: it
	// answers like an expired link, so a family learns nothing about the
	// school's contract (#4893). The data stays; booking again reopens it.
	if !s.deps.Settings.OnlineAnmeldungBooked(s.deps.Runtime.WithTenant(ctx, req.TenantID)) {
		return nil, enrollmentNative.ErrRequestNotFound
	}
	return req, nil
}

func (s *Intake) statusByToken(ctx context.Context, token string) (*Request, []*RequestChild, error) {
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
func (s *Intake) EditModeForStatus(ctx context.Context, status *enrollmentNative.RequestStatus) (string, error) {
	if status == nil || status.Request == nil {
		return enrollmentNative.EditModeNone, nil
	}
	req, err := requestValue(status.Request)
	if err != nil {
		return enrollmentNative.EditModeNone, err
	}
	children, err := childValues(status.Children)
	if err != nil {
		return enrollmentNative.EditModeNone, err
	}
	return s.editModeForStatus(s.tenantContext(ctx, req.TenantID), req, children)
}

// tenantContext scopes ctx to the tenant of a request the caller handed in.
func (s *Intake) tenantContext(ctx context.Context, tenantID int64) context.Context {
	return s.deps.Runtime.WithTenant(ctx, tenantID)
}

func (s *Intake) editModeForStatus(ctx context.Context, req *Request, children []*RequestChild) (string, error) {
	mode := editModeForChildren(children)
	switch mode {
	case enrollmentNative.EditModeDirectEdit:
		if err := s.ensureRequestEditable(ctx, req, children); err != nil {
			return editModeGateResult(err)
		}
		if _, err := s.loadPhaseForEditableRequest(ctx, req.PhaseID); err != nil {
			return editModeGateResult(err)
		}
		return enrollmentNative.EditModeDirectEdit, nil
	case enrollmentNative.EditModeChangeRequest:
		if err := s.ensureChangeRequestDraftAvailable(ctx, req, children); err != nil {
			return editModeGateResult(err)
		}
		phase, err := s.loadPhaseForEditableRequest(ctx, req.PhaseID)
		if err != nil {
			return editModeGateResult(err)
		}
		if !phase.EnrollmentWindowOpen(time.Now()) {
			return enrollmentNative.EditModeNone, nil
		}
		return enrollmentNative.EditModeChangeRequest, nil
	default:
		return enrollmentNative.EditModeNone, nil
	}
}

// editModeGateResult turns a denied edit or closed phase gate into "no edit" and reports
// every other failure.
func editModeGateResult(err error) (string, error) {
	if errors.Is(err, enrollmentNative.ErrEditNotAllowed) || errors.Is(err, enrollmentNative.ErrEnrollmentDisabled) || errors.Is(err, enrollmentNative.ErrInvalidSubmission) {
		return enrollmentNative.EditModeNone, nil
	}
	return enrollmentNative.EditModeNone, err
}

func editModeForChildren(children []*RequestChild) string {
	if len(children) == 0 {
		return enrollmentNative.EditModeDirectEdit
	}
	for _, c := range children {
		if c.Status != enrollmentNative.ChildStatusSubmitted {
			return enrollmentNative.EditModeChangeRequest
		}
	}
	return enrollmentNative.EditModeDirectEdit
}

// ensureRequestEditable allows a direct edit while the request is not
// withdrawn, edits are allowed and every child is still submitted.
func (s *Intake) ensureRequestEditable(ctx context.Context, req *Request, children []*RequestChild) error {
	if req == nil || req.WithdrawnAt != nil || len(children) == 0 {
		return enrollmentNative.ErrEditNotAllowed
	}
	if err := s.requireSubmissionEdit(ctx); err != nil {
		return err
	}
	for _, c := range children {
		if c.Status != enrollmentNative.ChildStatusSubmitted {
			return enrollmentNative.ErrEditNotAllowed
		}
	}
	return nil
}

// ensureChangeRequestDraftAvailable allows a change request while the
// request is decided in part, nothing is withdrawn and at least one child is
// not yet taken over into care.
func (s *Intake) ensureChangeRequestDraftAvailable(ctx context.Context, req *Request, children []*RequestChild) error {
	if req == nil || req.WithdrawnAt != nil || len(children) == 0 {
		return enrollmentNative.ErrEditNotAllowed
	}
	if err := s.requireSubmissionEdit(ctx); err != nil {
		return err
	}
	if editModeForChildren(children) != enrollmentNative.EditModeChangeRequest {
		return enrollmentNative.ErrEditNotAllowed
	}
	for _, c := range children {
		if c.Status == enrollmentNative.ChildStatusWithdrawn {
			return enrollmentNative.ErrEditNotAllowed
		}
	}
	if allChildrenTakenOver(children) {
		return enrollmentNative.ErrEditNotAllowed
	}
	return nil
}

// GuardiansByStatusToken loads the co-guardians behind a status token. The
// caller runs it in an administrative transaction.
func (s *Intake) GuardiansByStatusToken(ctx context.Context, token string) ([]*enrollmentNative.RequestGuardian, error) {
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
// The tenant transaction re-reads the request and its children locked, so a
// staff decision that commits after the token lookup still blocks the edit.
func (s *Intake) Edit(ctx context.Context, token string, patch enrollmentNative.EditPatch) error {
	consentFlags, err := decodeJSONMap(patch.ConsentFlags, "consent flags")
	if err != nil {
		return err
	}
	customData, err := decodeJSONMap(patch.CustomData, "custom data")
	if err != nil {
		return err
	}
	req, err := s.requestByStatusToken(ctx, token, false)
	if err != nil {
		return err
	}
	if err := s.requireSubmissionEdit(s.deps.Runtime.WithTenant(ctx, req.TenantID)); err != nil {
		return err
	}
	if s.deps.Runtime.IsAdminTx(ctx) {
		// The token lookup runs in an administrative transaction; the write
		// opens a tenant transaction instead of reusing its unscoped role.
		ctx = s.deps.Runtime.DetachTransaction(ctx)
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	return s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		return s.editInTenant(txCtx, token, patch, consentFlags, customData)
	})
}

func (s *Intake) editInTenant(ctx context.Context, token string, patch enrollmentNative.EditPatch, consentFlags, customData map[string]any) error {
	lockedReq, children, err := s.lockedStatusByToken(ctx, token)
	if err != nil {
		return err
	}
	for _, c := range children {
		if c.Status != enrollmentNative.ChildStatusSubmitted {
			return enrollmentNative.ErrEditNotAllowed
		}
	}
	if err := applyEditPatch(lockedReq, patch, consentFlags, customData); err != nil {
		return err
	}
	if err := s.writeEditPatch(ctx, lockedReq, consentFlags != nil); err != nil {
		return err
	}
	return s.markParentChanged(ctx, lockedReq.ID)
}

// lockedStatusByToken re-reads the request behind a status token and its
// children locked inside the tenant transaction. A parent write checks the
// child statuses it reads here: a staff decision that committed after the
// token lookup is visible, and one still open waits for the write.
func (s *Intake) lockedStatusByToken(ctx context.Context, token string) (*Request, []*RequestChild, error) {
	req, err := s.requestByStatusToken(ctx, token, true)
	if err != nil {
		return nil, nil, err
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, req.ID, true)
	if err != nil {
		return nil, nil, fmt.Errorf("lock request children: %w", err)
	}
	return req, children, nil
}

func applyEditPatch(req *Request, patch enrollmentNative.EditPatch, consentFlags, customData map[string]any) error {
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
			return enrollmentNative.ErrInvalidGuardianPhone
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
func (s *Intake) writeEditPatch(ctx context.Context, req *Request, consentPatched bool) error {
	var schema *enrollmentNative.FormSchema
	if req.SchemaID != nil {
		loaded, err := s.intakeSchema(ctx, *req.SchemaID)
		if err != nil {
			return fmt.Errorf("edit: load pinned schema: %w", err)
		}
		schema = loaded
		// A signed form is edited through the full form, which signs the
		// new version again (#4786); the field patch carries no signature.
		if signatureRequired(schema, req.SubmissionSource) {
			return enrollmentNative.InvalidInput(enrollmentNative.CodeSignatureRequired, "signature", enrollmentNative.ErrSignatureRequired)
		}
		byKey := buildFieldsByKey(schema)
		req.CustomData = sanitizeVisibleAnswers(schema, false, req.CustomData,
			fieldVisibilityContext{guardianAnswers: req.CustomData, fieldsByKey: byKey})
		claims, err := s.resolveFileAnswers(ctx, req.PhaseID, schema, req.ID, req.CustomData)
		if err != nil {
			return err
		}
		if err := s.claimFileUploads(ctx, claims, req.ID); err != nil {
			return fmt.Errorf("edit: %w", err)
		}
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
	var req *Request
	if err := s.deps.Runtime.AdminTx(ctx, func(adminCtx context.Context) error {
		var loadErr error
		req, loadErr = s.requestByStatusToken(adminCtx, token, false)
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
	lockedReq, children, err := s.lockedStatusByToken(ctx, token)
	if err != nil {
		return err
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

// withdrawChildren withdraws the open children of the request, or only the
// one named by childID. A childID that is not a child of this request is
// refused as not found rather than answered as a successful no-op.
func (s *Intake) withdrawChildren(ctx context.Context, children []*RequestChild, childID int64) (bool, error) {
	if childID != 0 && !slices.ContainsFunc(children, func(child *RequestChild) bool { return child.ID == childID }) {
		return false, enrollmentNative.ErrRequestChildNotFound
	}
	anyWithdrawn := false
	for _, child := range children {
		if childID != 0 && child.ID != childID {
			continue
		}
		if child.Status == enrollmentNative.ChildStatusApproved {
			if childID != 0 {
				return false, enrollmentNative.ErrWithdrawNotAllowed
			}
			continue
		}
		if (&enrollmentNative.RequestChild{Status: child.Status}).IsTerminal() {
			continue
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, child.ID, enrollmentNative.ChildStatusWithdrawn, nil, 0); err != nil {
			return false, err
		}
		child.Status, child.StatusReason, anyWithdrawn = enrollmentNative.ChildStatusWithdrawn, nil, true
	}
	return anyWithdrawn, nil
}

func (s *Intake) enqueueWithdrawDecision(ctx context.Context, request *Request) error {
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
// error. The statuses come from the locked re-read in the tenant
// transaction, so a staff decision in between is never overwritten.
func (s *Intake) ConfirmRenewal(ctx context.Context, token string) (int, error) {
	req, err := s.requestByStatusToken(ctx, token, false)
	if err != nil {
		return 0, err
	}
	if s.deps.Runtime.IsAdminTx(ctx) {
		// Like Edit: the token lookup runs in an administrative transaction;
		// the write opens a tenant transaction instead of reusing it.
		ctx = s.deps.Runtime.DetachTransaction(ctx)
	}
	tenantCtx := s.deps.Runtime.WithTenant(ctx, req.TenantID)
	var confirmed int
	if err := s.deps.Runtime.TenantTx(tenantCtx, req.TenantID, func(txCtx context.Context) error {
		var confirmErr error
		confirmed, confirmErr = s.confirmRenewalInTenant(txCtx, token)
		return confirmErr
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

func (s *Intake) confirmRenewalInTenant(ctx context.Context, token string) (int, error) {
	lockedReq, children, err := s.lockedStatusByToken(ctx, token)
	if err != nil {
		return 0, err
	}
	confirmed := 0
	for _, c := range children {
		if c.Status != enrollmentNative.ChildStatusPendingRenewal {
			continue
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, c.ID, enrollmentNative.ChildStatusSubmitted, nil, 0); err != nil {
			return 0, err
		}
		confirmed++
	}
	if confirmed == 0 {
		return 0, nil
	}
	return confirmed, s.markParentChanged(ctx, lockedReq.ID)
}

// markParentChanged makes the request unread for the staff again (#3778).
func (s *Intake) markParentChanged(ctx context.Context, requestID int64) error {
	if s.deps.ParentChanges == nil {
		return nil
	}
	if err := s.deps.ParentChanges.MarkRequestParentChanged(ctx, requestID); err != nil {
		return fmt.Errorf("record parent change: %w", err)
	}
	return nil
}
