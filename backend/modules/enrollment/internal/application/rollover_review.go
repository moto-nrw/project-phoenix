package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// allChildStatuses is every status the child model knows. Kept explicit so
// a future status shows up as a compile-visible gap here rather than a
// silently mis-counted preview.
var allChildStatuses = []string{
	enrollmentModels.ChildStatusSubmitted,
	enrollmentModels.ChildStatusUnderReview,
	enrollmentModels.ChildStatusApproved,
	enrollmentModels.ChildStatusWaitlisted,
	enrollmentModels.ChildStatusRejected,
	enrollmentModels.ChildStatusWithdrawn,
	enrollmentModels.ChildStatusPendingRenewal,
	enrollmentModels.ChildStatusAutoRenewed,
	enrollmentModels.ChildStatusPendingAdminReview,
}

// PreviewPhaseFromSource classifies the source phase's children without
// writing anything. Shares classifyRolloverGrade with the create path,
// so the numbers the admin confirms are the numbers the rollover
// produces.
func (s *Rollovers) PreviewPhaseFromSource(ctx context.Context, sourcePhaseID int64, bumpsGrade bool) (*enrollment.RolloverPreview, error) {
	tenantID := s.deps.Runtime.TenantID(ctx)
	if tenantID == 0 {
		return nil, fmt.Errorf("rollover preview: tenant not in context")
	}
	if _, err := s.loadRolloverSourcePhase(ctx, tenantID, sourcePhaseID); err != nil {
		return nil, err
	}
	maxGrade, err := s.resolveMaxGrade(ctx)
	if err != nil {
		return nil, fmt.Errorf("rollover preview: %w", err)
	}
	collectGradeLevel, err := s.deps.Settings.CollectGradeLevel(ctx)
	if err != nil {
		return nil, fmt.Errorf("rollover preview: resolve collect grade level: %w", err)
	}
	children, err := s.childrenByStatuses(ctx, sourcePhaseID, allChildStatuses)
	if err != nil {
		return nil, fmt.Errorf("rollover preview: list source children: %w", err)
	}
	preview := &enrollment.RolloverPreview{
		ReviewByReason:   make(map[string]int),
		ExcludedByStatus: make(map[string]int),
	}
	candidateRequests := make(map[int64]struct{})
	for _, child := range children {
		if child.Status != enrollmentModels.ChildStatusApproved {
			preview.ExcludedCount++
			preview.ExcludedByStatus[child.Status]++
			continue
		}
		preview.CarryCandidateCount++
		candidateRequests[child.RequestID] = struct{}{}
		_, reviewReason := classifyRolloverGrade(child.TargetGradeLevel, bumpsGrade, collectGradeLevel, maxGrade)
		if reviewReason == "" {
			preview.CarriedCount++
		} else {
			preview.ReviewCount++
			preview.ReviewByReason[reviewReason]++
		}
	}
	preview.RequestCount = len(candidateRequests)
	return preview, nil
}

// ListReviewQueue loads admin-review rows + their parent request + the
// source child for context. Tenant RLS scopes the read.
func (s *Rollovers) ListReviewQueue(ctx context.Context, phaseID int64) ([]*enrollment.RolloverReviewItem, error) {
	if phaseID <= 0 {
		return nil, fmt.Errorf("%w: phase_id is required", enrollment.ErrRolloverInvalidRequest)
	}
	children, err := s.deps.Children.ChildrenByPhaseStatuses(ctx, phaseID, []string{enrollmentModels.ChildStatusPendingAdminReview})
	if err == nil {
		_, err = childValues(children)
	}
	if err != nil {
		return nil, fmt.Errorf("rollover: list review queue: %w", err)
	}
	requestIDs := make([]int64, 0, len(children))
	sourceChildIDs := make([]int64, 0, len(children))
	for _, child := range children {
		requestIDs = append(requestIDs, child.RequestID)
		if child.RolloverSourceChildID != nil {
			sourceChildIDs = append(sourceChildIDs, *child.RolloverSourceChildID)
		}
	}
	requestsByID, sourceChildrenByID, err := s.reviewQueueContext(ctx, requestIDs, sourceChildIDs)
	if err != nil {
		return nil, err
	}
	out := make([]*enrollment.RolloverReviewItem, 0, len(children))
	for _, c := range children {
		req := requestsByID[c.RequestID]
		if req == nil {
			s.deps.Logger.Warn("rollover: review queue request lookup failed",
				slog.Int64("request_child_id", c.ID),
				slog.Int64("request_id", c.RequestID))
			continue
		}
		item := &enrollment.RolloverReviewItem{Child: c, Request: req}
		if c.RolloverSourceChildID != nil {
			// The source child is in the previous phase of the same tenant.
			// If it cannot be read, the admin still gets a useful row — just
			// without the prior-year context.
			item.SourceChild = sourceChildrenByID[*c.RolloverSourceChildID]
		}
		out = append(out, item)
	}
	return out, nil
}

// reviewQueueContext loads the parent requests and the source children of
// the review rows.
func (s *Rollovers) reviewQueueContext(ctx context.Context, requestIDs, sourceChildIDs []int64) (map[int64]*enrollment.Request, map[int64]*enrollment.RequestChild, error) {
	requests, err := s.deps.Requests.RequestsByID(ctx, requestIDs)
	if err == nil {
		_, err = requestValues(requests)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("rollover: load review queue requests: %w", err)
	}
	sourceChildren, err := s.deps.Children.ChildrenByID(ctx, sourceChildIDs)
	if err == nil {
		_, err = childValues(sourceChildren)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("rollover: load review queue source children: %w", err)
	}
	requestsByID := make(map[int64]*enrollment.Request, len(requests))
	for _, row := range requests {
		requestsByID[row.ID] = row
	}
	sourceChildrenByID := make(map[int64]*enrollment.RequestChild, len(sourceChildren))
	for _, row := range sourceChildren {
		sourceChildrenByID[row.ID] = row
	}
	return requestsByID, sourceChildrenByID, nil
}

// DecideReview applies the admin's keep/drop/defer action.
//
// "Keep" promotes the row out of review and into the active renewal flow and
// always lands in auto_renewed: the admin has already implicitly confirmed
// they want this student carried forward. The deadline worker promotes
// auto_renewed → submitted, then the admin's decision queue handles final
// approval. The class override (if any) is applied at the same time.
// "Defer" means "I'll come back to it" — the row stays as-is, but reviewed_at
// is still stamped so the admin sees their last touch.
func (s *Rollovers) DecideReview(ctx context.Context, req enrollment.DecideReviewRequest) error {
	if req.RequestChildID <= 0 {
		return fmt.Errorf("%w: request_child_id is required", enrollment.ErrRolloverReviewInvalid)
	}
	switch req.Decision {
	case enrollment.ReviewDecisionKeep:
		return s.deps.Children.ReviewRolloverChild(ctx, req.RequestChildID, enrollmentModels.ChildStatusAutoRenewed, nil, req.NewGradeLevel, req.AdminAccountID)
	case enrollment.ReviewDecisionDrop:
		reason := "rollover_drop"
		return s.deps.Children.ReviewRolloverChild(ctx, req.RequestChildID, enrollmentModels.ChildStatusWithdrawn, &reason, nil, req.AdminAccountID)
	case enrollment.ReviewDecisionDefer:
		return s.deps.Children.ReviewRolloverChild(ctx, req.RequestChildID, enrollmentModels.ChildStatusPendingAdminReview, nil, nil, req.AdminAccountID)
	default:
		return fmt.Errorf("%w: decision must be keep/drop/defer, got %q",
			enrollment.ErrRolloverReviewInvalid, req.Decision)
	}
}

// classifyRolloverGrade is the ONE classification rule shared by the
// rollover create path (rolloverAttributesForSource) and its preview
// (PreviewPhaseFromSource, #2251). Keeping both on this helper is what
// guarantees the numbers the admin confirms are the numbers the
// rollover produces. With grade collection disabled every approved
// child carries without review.
func classifyRolloverGrade(sourceGrade *int16, bumpsGrade, collectGradeLevel bool, maxGrade int) (*int16, string) {
	if !collectGradeLevel {
		return nil, ""
	}
	return computeNewGrade(sourceGrade, bumpsGrade, maxGrade)
}

// computeNewGrade returns the next grade level and (if positive) the
// review reason explaining why this row needs admin attention.
//
//   - source nil          → no grade on file, needs review
//   - bumpsGrade=false    → keep grade, no review (half-year cadence)
//   - newGrade > maxGrade → above the cap, needs review
//   - otherwise           → bumped grade, no review
func computeNewGrade(sourceGrade *int16, bumpsGrade bool, maxGrade int) (*int16, string) {
	if sourceGrade == nil {
		return nil, enrollmentModels.ReviewReasonNoGradeLevel
	}
	bump := int16(0)
	if bumpsGrade {
		bump = 1
	}
	newGrade := *sourceGrade + bump
	if int(newGrade) > maxGrade {
		// Surface as review with the same grade carried through so
		// the admin can see "this student would be in grade 5".
		return &newGrade, enrollmentModels.ReviewReasonGradeAboveMax
	}
	return &newGrade, ""
}

func renewalInitialStatus(mode string) string {
	if mode == enrollment.PhaseRolloverModeOptIn {
		return enrollmentModels.ChildStatusPendingRenewal
	}
	return enrollmentModels.ChildStatusAutoRenewed
}

// RunDeadlineWorker is the scheduled resolver. The caller (scheduler tick
// or CLI) wraps it in a tenant transaction so the bulk updates run as
// phoenix_tenant with RLS scoping to the current tenant.
func (s *Rollovers) RunDeadlineWorker(ctx context.Context, asOf time.Time) (*enrollment.DeadlineWorkerSummary, error) {
	if s.deps.Phases == nil || s.deps.Children == nil {
		return nil, fmt.Errorf("rollover deadline: required repos not wired")
	}
	summary := &enrollment.DeadlineWorkerSummary{}
	values, err := s.deps.Phases.PhasesWithExpiredRolloverDeadline(ctx, asOf)
	if err != nil {
		return summary, fmt.Errorf("rollover deadline: list expired phases: %w", err)
	}
	for _, phase := range values {
		summary.PhasesProcessed++
		if err := s.resolveExpiredPhase(ctx, phase, summary); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

// resolveExpiredPhase resolves the renewal rows of one expired phase. The
// opt-out side moves auto_renewed rows to approved (rollover_auto_approve,
// through the decision flow so the existing student gets updated) or to
// submitted (the admin still approves manually through the existing queue).
// The opt-in side lets pending_renewal rows lapse to withdrawn: the parent
// didn't act before the deadline.
func (s *Rollovers) resolveExpiredPhase(ctx context.Context, phase *enrollment.Phase, summary *enrollment.DeadlineWorkerSummary) error {
	autoApproved, autoSubmitted, autoErrs, autoFatalErr := s.resolveAutoRenewed(ctx, phase)
	summary.AutoRenewedToApproved += autoApproved
	summary.AutoRenewedToSubmitted += autoSubmitted
	summary.AutoApproveErrors += autoErrs
	if autoFatalErr != nil {
		return autoFatalErr
	}
	pendingCount, err := s.deps.Children.TransitionPhaseChildren(
		ctx, phase.ID,
		enrollmentModels.ChildStatusPendingRenewal,
		enrollmentModels.ChildStatusWithdrawn,
	)
	if err != nil {
		s.deps.Logger.Error("rollover deadline: demote pending_renewal failed",
			slog.Int64("phase_id", phase.ID),
			slog.String("error", err.Error()),
		)
		return nil
	}
	summary.PendingRenewalToWithdrawn += pendingCount
	if autoApproved > 0 || autoSubmitted > 0 || pendingCount > 0 {
		s.deps.Logger.Info("rollover deadline: resolved phase",
			slog.Int64("phase_id", phase.ID),
			slog.Int("auto_to_approved", autoApproved),
			slog.Int("auto_to_submitted", autoSubmitted),
			slog.Int("pending_to_withdrawn", pendingCount),
			slog.Int("auto_approve_errors", autoErrs),
		)
	}
	return nil
}

// resolveAutoRenewed handles the auto_renewed cohort for one phase.
// Returns the counts split by destination status (approved when the
// phase opts in to auto-approve and the decision flow is wired,
// otherwise submitted) plus how many per-row Decide() calls errored. A
// savepoint-control error is returned separately because the surrounding
// transaction is no longer safe to continue or commit.
//
// When rollover_auto_approve is on but the decision flow is not wired
// (test environments that don't wire the full approval pipeline), we fall
// back to the bulk-promotion-to-submitted path so the worker still
// completes — logs a warning so the gap is visible.
func (s *Rollovers) resolveAutoRenewed(ctx context.Context, phase *enrollment.Phase) (approved, submitted, errs int, fatalErr error) {
	if !phase.RolloverAutoApprove || s.deps.Decisions == nil {
		return 0, s.promoteAutoRenewed(ctx, phase), 0, nil
	}
	// Auto-approve path: pull each auto_renewed row and Decide it so the
	// rollover approval runs (updates the existing student, fires the
	// approval email, etc.).
	rows, err := s.childrenByStatuses(ctx, phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	if err != nil {
		s.deps.Logger.Error("rollover deadline: list auto_renewed failed",
			slog.Int64("phase_id", phase.ID),
			slog.String("error", err.Error()))
		return 0, 0, 0, nil
	}
	for _, row := range rows {
		decideErr := s.decideAutoRenewedRow(ctx, row)
		if decideErr == nil {
			approved++
			continue
		}
		if s.deps.Runtime.IsSavepointControl(decideErr) {
			return approved, 0, errs, fmt.Errorf(
				"rollover deadline: auto-approve savepoint failed for request_child %d: %w",
				row.ID,
				decideErr,
			)
		}
		errs++
		s.deps.Logger.Error("rollover deadline: auto-approve decide failed",
			slog.Int64("phase_id", phase.ID),
			slog.Int64("request_child_id", row.ID),
			slog.String("error", decideErr.Error()))
	}
	return approved, 0, errs, nil
}

// promoteAutoRenewed moves a phase's auto_renewed rows to submitted.
func (s *Rollovers) promoteAutoRenewed(ctx context.Context, phase *enrollment.Phase) int {
	if phase.RolloverAutoApprove && s.deps.Decisions == nil {
		s.deps.Logger.Warn("rollover deadline: auto_approve=true but DecisionService not wired, falling back to submitted",
			slog.Int64("phase_id", phase.ID))
	}
	count, err := s.deps.Children.TransitionPhaseChildren(
		ctx, phase.ID,
		enrollmentModels.ChildStatusAutoRenewed,
		enrollmentModels.ChildStatusSubmitted,
	)
	if err != nil {
		s.deps.Logger.Error("rollover deadline: promote auto_renewed failed",
			slog.Int64("phase_id", phase.ID),
			slog.String("error", err.Error()))
		return 0
	}
	return count
}

// decideAutoRenewedRow gives one approval row atomicity without sacrificing
// the worker's best-effort batch semantics. Production callers always provide
// an ambient tenant transaction, so a failed Decide is rolled back to the
// savepoint and later rows can still proceed. Direct calls without a
// transaction are retained for lightweight tests; they do not claim the
// production atomicity contract documented on RunDeadlineWorker.
func (s *Rollovers) decideAutoRenewedRow(ctx context.Context, row *RequestChild) error {
	decide := func(decideCtx context.Context) error {
		_, err := s.deps.Decisions.Decide(decideCtx, enrollment.DecideInput{
			RequestID: row.RequestID,
			ChildID:   row.ID,
			Status:    enrollment.DecisionApproved,
		})
		return err
	}
	if !s.deps.Runtime.InTransaction(ctx) {
		return decide(ctx)
	}
	return s.deps.Runtime.Savepoint(ctx, decide)
}
