package application

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Activation modes of enrollmentNative.default_activation_mode.
const (
	activationModeImmediate = "immediate"
	activationModeScheduled = "scheduled"
	// defaultActivationModeKey names the setting in the fallback warning.
	defaultActivationModeKey = "enrollment.default_activation_mode"
)

// decisionTarget is the locked request of a decision with its locked
// children, raw and decoded, and the child the decision is about.
type decisionTarget struct {
	request  *Request
	children []*enrollmentNative.RequestChild
	decoded  []*RequestChild
	raw      *enrollmentNative.RequestChild
	child    *RequestChild
}

// Decide updates a single child's status. When status==approved the
// decision also creates the downstream records (Person + Student +
// GuardianProfile + StudentGuardian + StudentEnrollment[s]) inside the
// same tenant tx the handler provides - failure of any one rolls the
// whole approval back. Parent decision emails are enqueued via the
// outbox in the same tx; guardian invitation creation is returned as a
// PendingGuardianInvite for the handler to fire post-commit.
//
// Idempotency: applying the same status twice is a no-op. Re-applying
// any new status to an already-terminal child (approved/rejected/
// withdrawn) returns ErrDecisionAlreadyTerminal - admins must use
// dedicated revoke/promote flows for those (deferred).
func (d *Decisions) Decide(ctx context.Context, input enrollmentNative.DecideInput) (*enrollmentNative.DecideOutcome, error) {
	if err := validateDecideInput(input); err != nil {
		return nil, err
	}
	target, err := d.lockDecisionTarget(ctx, input)
	if err != nil {
		return nil, err
	}
	// No-op: same status. Don't bump reviewed_at when nothing changes.
	if target.child.Status == string(input.Status) {
		return &enrollmentNative.DecideOutcome{Child: target.raw}, nil
	}
	// Block transitions out of a terminal status before resolving settings or
	// loading phase data. A retry of an invalid transition must keep its stable
	// conflict contract even during an unrelated settings outage.
	if (&enrollmentNative.RequestChild{Status: target.child.Status}).IsTerminal() {
		return nil, enrollmentNative.ErrDecisionAlreadyTerminal
	}
	autoInviteEnabled, err := d.decisionSettingsPreflight(ctx, input.Status)
	if err != nil {
		return nil, err
	}
	phase, err := d.deps.Phases.Phase(ctx, target.request.PhaseID)
	if err != nil {
		return nil, fmt.Errorf("decision: load phase: %w", err)
	}
	if input.Status == enrollmentNative.DecisionApproved {
		if err := d.validateApprovalOfferingSelection(ctx, target.child, phase); err != nil {
			return nil, err
		}
	}
	outcome := &enrollmentNative.DecideOutcome{}
	invite, err := d.applyDecision(ctx, target, phase, input)
	if err != nil {
		return nil, err
	}
	if autoInviteEnabled && !input.SuppressGuardianInvitation {
		outcome.PendingInvite = invite
	}
	if err := d.refreshDecidedChild(ctx, target, input); err != nil {
		return nil, err
	}
	if err := d.afterDecision(ctx, target, phase, input); err != nil {
		return nil, err
	}
	outcome.Child = target.raw
	return outcome, nil
}

func validateDecideInput(input enrollmentNative.DecideInput) error {
	if input.RequestID <= 0 {
		return fmt.Errorf("%w: request_id required", enrollmentNative.ErrDecisionInvalidStatus)
	}
	if input.ChildID <= 0 {
		return fmt.Errorf("%w: child_id required", enrollmentNative.ErrDecisionInvalidStatus)
	}
	if !validDecisionStatuses[input.Status] {
		return fmt.Errorf("%w: %s", enrollmentNative.ErrDecisionInvalidStatus, input.Status)
	}
	return nil
}

// lockDecisionTarget locks the parent before its children. Cleanup, editing,
// and change-request paths use the same order; the notification-mode pin
// updates the parent and must not introduce a parent/child lock inversion.
// Every sibling is locked, in the owner's stable sort_order/id order, before
// any status is inspected or changed: decisions for two children in the same
// request must serialize so the second transaction sees the first one's
// committed state when deciding whether the digest is complete.
func (d *Decisions) lockDecisionTarget(ctx context.Context, input enrollmentNative.DecideInput) (*decisionTarget, error) {
	request, err := decodedRequestByID(ctx, d.deps.Requests, input.RequestID, true)
	if err != nil {
		if d.deps.Runtime.NotFound(err) {
			return nil, careplan.ErrBookingRequestNotFound
		}
		return nil, fmt.Errorf("decision: load request: %w", err)
	}
	target := &decisionTarget{request: request}
	if err := d.loadDecisionChildren(ctx, target, input.ChildID, true); err != nil {
		return nil, fmt.Errorf("decision: load children: %w", err)
	}
	if target.child == nil {
		return nil, careplan.ErrBookingChildNotFound
	}
	return target, nil
}

// loadDecisionChildren reads the request's children and points the target at
// the decided child, or at nothing when it is gone.
func (d *Decisions) loadDecisionChildren(ctx context.Context, target *decisionTarget, childID int64, forUpdate bool) error {
	children, err := d.deps.Children.ChildrenForRequest(ctx, target.request.ID, forUpdate)
	if err != nil {
		return err
	}
	decoded, err := childValues(children)
	if err != nil {
		return err
	}
	target.children, target.decoded, target.raw, target.child = children, decoded, nil, nil
	for i, child := range decoded {
		if child.ID == childID {
			target.raw, target.child = children[i], child
			break
		}
	}
	return nil
}

// decisionSettingsPreflight resolves the settings a decision depends on and
// reports whether an approval invites the guardian.
func (d *Decisions) decisionSettingsPreflight(ctx context.Context, status enrollmentNative.DecisionStatus) (bool, error) {
	if status == enrollmentNative.DecisionWaitlisted {
		enabled, err := d.waitlistEnabled(ctx)
		if err != nil {
			return false, fmt.Errorf("decision: resolve waitlist setting: %w", err)
		}
		if !enabled {
			return false, enrollmentNative.ErrWaitlistDisabled
		}
	}
	if status != enrollmentNative.DecisionApproved {
		return true, nil
	}
	enabled, err := d.autoInviteGuardianOnApprove(ctx)
	if err != nil {
		return false, fmt.Errorf("decision: resolve guardian invitation setting: %w", err)
	}
	return enabled, nil
}

// applyDecision writes the decision. Approval is the heavy path: it creates
// the downstream records first so any failure rolls back BEFORE the status
// flips. The status update closes the loop after the records exist; if it
// fails the records are still rolled back via the surrounding tenant tx.
func (d *Decisions) applyDecision(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, input enrollmentNative.DecideInput) (*enrollmentNative.PendingGuardianInvite, error) {
	var invite *enrollmentNative.PendingGuardianInvite
	approved := input.Status == enrollmentNative.DecisionApproved
	if approved {
		var err error
		invite, err = d.applyApproval(ctx, target.request, target.child, phase, input.ReviewedBy)
		if err != nil {
			return nil, err
		}
	}
	reason := strings.TrimSpace(input.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}
	if err := d.deps.Children.UpdateChildStatus(ctx, target.child.ID, string(input.Status), reasonPtr, input.ReviewedBy); err != nil {
		return nil, fmt.Errorf("decision: update child status: %w", err)
	}
	if !approved {
		return invite, nil
	}
	if target.child.MatchedStudentID != nil {
		if err := d.reconcileExistingStudentCareRenewal(ctx, target.child.ID, *target.child.MatchedStudentID, phase); err != nil {
			return nil, err
		}
	}
	// Multi-source templates are reconciled through the union resync, which
	// resolves children by their APPROVED status — applyApproval deliberately
	// runs before the status flip, so its in-flight resync cannot see this
	// child yet. Re-run it now that the status is committed in this tx.
	if err := d.resyncMultiSourceTemplatesForChild(ctx, target.child.ID, phase); err != nil {
		return nil, fmt.Errorf("decision: resync multi-source templates after approval: %w", err)
	}
	return invite, nil
}

// refreshDecidedChild re-reads the children after the status update. It
// reads the DB-authored review generation before either immediate or digest
// idempotency keys are built. Status alone is insufficient: a supported
// rejected -> under_review -> rejected cycle is a new decision even though it
// ends at the same status vector.
func (d *Decisions) refreshDecidedChild(ctx context.Context, target *decisionTarget, input enrollmentNative.DecideInput) error {
	if err := d.loadDecisionChildren(ctx, target, input.ChildID, false); err != nil {
		return fmt.Errorf("decision: refresh children after status update: %w", err)
	}
	if target.child == nil {
		return careplan.ErrBookingChildNotFound
	}
	return nil
}

// afterDecision refreshes the projected-pickup consumers of an approval once
// the re-read carries the student id it stamped, logs the decision, and
// enqueues the parent decision email in the same transaction. An enqueue
// failure rolls back the decision so a retry can safely enqueue it; the
// tenant-scoped idempotency key prevents duplicate rows after retries.
func (d *Decisions) afterDecision(ctx context.Context, target *decisionTarget, phase *enrollmentNative.Phase, input enrollmentNative.DecideInput) error {
	if input.Status == enrollmentNative.DecisionApproved && !calendar.Date(phase.ServiceStartDate).After(d.todayDate()) {
		if err := d.syncOfferingPickupAfterApproval(ctx, target.child); err != nil {
			return fmt.Errorf("decision: refresh offering pickup projection: %w", err)
		}
	}
	d.logger().Info("enrollment decision applied",
		slog.Int64("request_id", input.RequestID),
		slog.Int64("child_id", input.ChildID),
		slog.String("status", string(input.Status)),
		slog.Int64("reviewed_by", input.ReviewedBy),
		slog.Bool("created_records", input.Status == enrollmentNative.DecisionApproved),
	)
	if input.SuppressParentEmail || !isParentVisibleDecision(input.Status) {
		return nil
	}
	return notifyDecisions(ctx, d.deps.Notifications, decisionGeneration{
		Request: target.request, Children: target.decoded, Phase: phase,
		ImmediateChildIDs: map[int64]struct{}{target.child.ID: {}}, ParentsURL: d.deps.ParentsURL,
	})
}

func isParentVisibleDecision(status enrollmentNative.DecisionStatus) bool {
	return status == enrollmentNative.DecisionApproved || status == enrollmentNative.DecisionRejected || status == enrollmentNative.DecisionWaitlisted
}

func (d *Decisions) validateApprovalOfferingSelection(ctx context.Context, child *RequestChild, phase *enrollmentNative.Phase) error {
	careOfferingsEnabled, err := d.careOfferingsEnabled(ctx)
	if err != nil {
		return fmt.Errorf("decision: resolve care offerings setting: %w", err)
	}
	if !careOfferingsEnabled {
		return nil
	}
	if phase.CareOfferingSelectionMode == "" ||
		phase.CareOfferingSelectionMode == enrollmentNative.PhaseCareOfferingSelectionOptional {
		return nil
	}
	links, err := enrollmentNative.OfferingSelectionRecordsAt(ctx, d.deps.Children, child.ID, calendar.Date(phase.ServiceStartDate))
	if err != nil {
		return fmt.Errorf("decision: validate child offerings: %w", err)
	}
	offerings, err := d.deps.Offerings.ListByIDs(ctx, uniqueCareOfferingIDs(links))
	if err != nil {
		return fmt.Errorf("decision: list child offerings: %w", err)
	}
	choosableCount := 0
	for _, offering := range offerings {
		if offering != nil && offering.PhaseID == phase.ID && !offering.IsRequired {
			choosableCount++
		}
	}
	switch phase.CareOfferingSelectionMode {
	case enrollmentNative.PhaseCareOfferingSelectionAtLeastOne:
		if choosableCount == 0 {
			return selection.ErrCareOfferingMissing
		}
	case enrollmentNative.PhaseCareOfferingSelectionExactlyOne:
		if choosableCount != 1 {
			return selection.ErrCareOfferingExactlyOneRequired
		}
	}
	return nil
}

// waitlistEnabled, autoInviteGuardianOnApprove and careOfferingsEnabled
// answer the registry default (on) when no settings are bound.
func (d *Decisions) waitlistEnabled(ctx context.Context) (bool, error) {
	if d.deps.Settings == nil {
		return true, nil
	}
	return d.deps.Settings.WaitlistEnabled(ctx)
}

func (d *Decisions) autoInviteGuardianOnApprove(ctx context.Context) (bool, error) {
	if d.deps.Settings == nil {
		return true, nil
	}
	return d.deps.Settings.AutoInviteGuardianOnApprove(ctx)
}

func (d *Decisions) careOfferingsEnabled(ctx context.Context) (bool, error) {
	if d.deps.Settings == nil {
		return true, nil
	}
	return d.deps.Settings.CareOfferingsEnabled(ctx)
}

// resolveActivationMode reads enrollmentNative.default_activation_mode for the
// tenant in context. The setting was registered from the start with a
// registry default of "scheduled", so a plain resolve (tenant override →
// registry default) is correct — there is no env-var fallback.
//
// Any resolve error or empty/unknown value falls back to the safe
// "scheduled" default rather than failing the approval: an unconfigured
// or momentarily-unreadable setting must never block a school from
// approving a child.
func (d *Decisions) resolveActivationMode(ctx context.Context) string {
	if d.deps.Settings == nil {
		return activationModeScheduled
	}
	mode, err := d.deps.Settings.DefaultActivationMode(ctx)
	if err != nil {
		d.logger().Warn("decision: resolve activation mode failed, defaulting to scheduled",
			slog.String("key", defaultActivationModeKey),
			slog.String("error", err.Error()),
		)
		return activationModeScheduled
	}
	if mode == activationModeImmediate {
		return activationModeImmediate
	}
	// Empty (unconfigured) or any unknown value normalizes to scheduled.
	return activationModeScheduled
}

type approvalActivationPlan struct {
	Mode          string
	ActivateOn    *calendar.Date
	StudentStatus string
}

func (d *Decisions) approvalActivationPlan(ctx context.Context, phase *enrollmentNative.Phase) approvalActivationPlan {
	if d.resolveActivationMode(ctx) == activationModeImmediate {
		return approvalActivationPlan{
			Mode:          enrollmentNative.ChildActivationImmediate,
			StudentStatus: studentStatusActive,
		}
	}
	activateOn := calendar.Date(phase.ServiceStartDate)
	status := studentStatusPending
	if !activateOn.After(d.todayDate()) {
		status = studentStatusActive
	}
	return approvalActivationPlan{
		Mode:          enrollmentNative.ChildActivationScheduled,
		ActivateOn:    &activateOn,
		StudentStatus: status,
	}
}

func (d *Decisions) stampActivationPlan(ctx context.Context, requestChildID int64, plan approvalActivationPlan) error {
	var date *enrollmentNative.Date
	if plan.ActivateOn != nil {
		value := enrollmentNative.Date(plan.ActivateOn.String())
		date = &value
	}
	if err := d.deps.Children.UpdateChildActivationPlan(ctx, requestChildID, plan.Mode, date); err != nil {
		return fmt.Errorf("decision: stamp activation plan: %w", err)
	}
	return nil
}

// uniqueCareOfferingIDs returns the distinct positive offering ids of the
// links in their first-seen order.
func uniqueCareOfferingIDs(links []*enrollmentNative.RequestChildOfferingRecord) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}
