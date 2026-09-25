package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// approval is the state one change-request approval works on.
type approval struct {
	row               *ChangeRequest
	input             enrollment.ReviewChangeRequestInput
	req               *enrollmentModels.Request
	children          []*RequestChild
	prepared          *proposal
	capabilities      enrollment.FormCapabilities
	previousGuardians []*enrollment.RequestGuardian
	overrides         map[int]string
	// eligibilityEnforced is false for trusted-source and rollover-generated
	// requests, which bypassed the self-service gates at creation.
	eligibilityEnforced bool
	newlyWaitlisted     map[int64]struct{}
}

// applyApprovedChange applies an approved proposal to the request, its
// children, co-guardians, bookings and the students an approval created, in
// the locked transaction.
func (s *ChangeRequests) applyApprovedChange(ctx context.Context, row *ChangeRequest, input enrollment.ReviewChangeRequestInput) error {
	// Take the global gates before the first request or student row lock:
	// booking writers hold the recurrence gate while their FK checks touch
	// request_children, so taking it later would invert the order.
	if s.deps.Decisions != nil && s.deps.BookingGates != nil {
		if err := s.deps.BookingGates.LockOfferingDerivedWrites(ctx); err != nil {
			return fmt.Errorf("change request approve: lock offering-derived writes: %w", err)
		}
	}
	run, err := s.loadApproval(ctx, row, input)
	if err != nil {
		return err
	}
	if err := s.writeApprovedRequest(ctx, run); err != nil {
		return err
	}
	if err := s.lockApprovedStudents(ctx, run.children); err != nil {
		return err
	}
	// One approval is one edit of the whole family, applied child by child.
	// Defer the companion stranding verdicts and decide them once against
	// every child's final plan (#1694).
	ctx = s.beginCompanionStrandingBatch(ctx)
	for i, existing := range run.children {
		if err := s.applyApprovedChild(ctx, run, i, existing); err != nil {
			return err
		}
	}
	if err := s.verifyCompanionStrandingBatch(ctx); err != nil {
		return err
	}
	if err := s.reconcileApprovedPickups(ctx, run.children); err != nil {
		return err
	}
	return s.notifyApprovedCapacityDecisions(ctx, run)
}

// loadApproval locks the request and its children, refuses a stale base
// snapshot and prepares the stored proposal with the capability pinned at
// creation.
func (s *ChangeRequests) loadApproval(ctx context.Context, row *ChangeRequest, input enrollment.ReviewChangeRequestInput) (*approval, error) {
	req, err := s.requestByID(ctx, row.RequestID, true)
	if err != nil {
		return nil, err
	}
	children, err := decodedChildrenOfRequest(ctx, s.deps.Children, req.ID, true)
	if err != nil {
		return nil, fmt.Errorf("change request approve: lock children: %w", err)
	}
	for _, child := range children {
		if child.Status == enrollmentModels.ChildStatusWithdrawn {
			return nil, enrollment.ErrChangeRequestNotAllowed
		}
	}
	current, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, err
	}
	if !jsonEqual(current, row.BaseSnapshot) {
		return nil, enrollment.ErrChangeRequestConflict
	}
	proposed, err := snapshotToSubmitRequest(row.ProposedSnapshot)
	if err != nil {
		return nil, err
	}
	run := &approval{row: row, input: input, req: req, children: children, newlyWaitlisted: map[int64]struct{}{}}
	if run.capabilities, err = s.formCapabilities(ctx, &row.CareOfferingsEnabledAtCreation); err != nil {
		return nil, err
	}
	if run.prepared, err = s.prepareProposed(ctx, req, children, proposed, run.capabilities, true, false); err != nil {
		return nil, err
	}
	if err := s.ensureNoActiveDuplicateForApproval(ctx, req, run.prepared.request); err != nil {
		return nil, err
	}
	if s.deps.Guardians != nil {
		if run.previousGuardians, err = s.deps.Guardians.RequestGuardians(ctx, []int64{req.ID}); err != nil {
			return nil, fmt.Errorf("change request approve: list previous guardians: %w", err)
		}
	}
	run.overrides = map[int]string{}
	if run.capabilities.CareOfferingsEnabled {
		if run.overrides, err = s.changeRequestCapacityOverrides(ctx, run); err != nil {
			return nil, err
		}
	}
	run.eligibilityEnforced = !isTrustedEnrollmentSource(req.SubmissionSource) && !hasRolloverGeneratedChild(children)
	return run, nil
}

// writeApprovedRequest writes the proposal's guardian fields and replaces the
// co-guardians, carrying their stamped guardian profiles over.
func (s *ChangeRequests) writeApprovedRequest(ctx context.Context, run *approval) error {
	req, prepared := run.req, run.prepared.request
	req.GuardianFirstName = strings.TrimSpace(prepared.GuardianFirstName)
	req.GuardianLastName = strings.TrimSpace(prepared.GuardianLastName)
	req.GuardianEmail = lowerTrim(prepared.GuardianEmail)
	req.GuardianPhone = prepared.GuardianPhone
	req.ConsentFlags = prepared.ConsentFlags
	req.CustomData = prepared.CustomData
	if err := updateDecodedRequest(ctx, s.deps.Requests, req, true); err != nil {
		return err
	}
	if s.deps.Guardians == nil {
		return nil
	}
	if err := s.deps.Guardians.DeleteRequestGuardians(ctx, req.ID); err != nil {
		return err
	}
	return createRequestGuardians(ctx, s.deps.Guardians, req.ID, prepared.AdditionalGuardians, func(guardian SubmitGuardian) *int64 {
		return matchingPreviousGuardianProfileID(run.previousGuardians, guardian)
	}, "change request approve: create guardian %d: %w")
}

// applyApprovedChild writes one child's proposed data. A child the decision
// flow already approved goes through it; any other child gets its bookings
// replaced and its status adjusted.
func (s *ChangeRequests) applyApprovedChild(ctx context.Context, run *approval, i int, existing *RequestChild) error {
	next := run.prepared.request.Children[i]
	// Re-point the existing_students pin before the proposed identity
	// overwrites the row, and re-run the uniqueness guard for a child that
	// ends this approval active (#1663).
	if err := s.reconcileMatchedStudent(ctx, run, existing, next, i); err != nil {
		return err
	}
	existing.FirstName = strings.TrimSpace(next.FirstName)
	existing.LastName = strings.TrimSpace(next.LastName)
	existing.DateOfBirth = next.DateOfBirth
	existing.TargetGradeLevel = next.TargetGradeLevel
	existing.TargetSchoolClass = next.TargetSchoolClass
	existing.CustomData = next.CustomData
	existing.SortOrder = i
	if err := updateDecodedChild(ctx, s.deps.Children, existing); err != nil {
		return err
	}
	if s.approvedChildUsesDecisionSync(existing) {
		return s.syncApprovedChild(ctx, run, existing, run.prepared.selections[i])
	}
	return s.applyUndecidedChild(ctx, run, i, existing)
}

// syncApprovedChild applies the proposal to a child whose student exists.
// The offering capability is the one frozen at creation: a later setting
// change must not discard a reviewed offering change.
func (s *ChangeRequests) syncApprovedChild(ctx context.Context, run *approval, existing *RequestChild, selections []selection.Selection) error {
	if run.row.CareOfferingsEnabledAtCreation {
		offeringInput := enrollment.UpdateChildOfferingsInput{
			RequestID: run.req.ID, ChildID: existing.ID, Reason: run.input.Note,
			ActorAccountID: run.input.ActorAccountID, ActorRole: run.input.ActorRole,
		}
		for _, pick := range selections {
			offeringInput.Offerings = append(offeringInput.Offerings, enrollment.OfferingAdjustmentSelection{OfferingID: pick.OfferingID, SelectedDays: pick.SelectedDays})
		}
		if _, err := s.deps.Decisions.ApplyChangeRequestOfferings(ctx, offeringInput); err != nil {
			return err
		}
	}
	snapshot, err := json.Marshal(run.row.BaseSnapshot)
	if err != nil {
		return err
	}
	_, err = s.deps.Decisions.SyncApprovedChildData(ctx, enrollment.ApprovedChildSync{
		RequestID: run.req.ID, ChildID: existing.ID, ActorAccountID: run.input.ActorAccountID,
		ReplaceTargetedData: true, PreviousSnapshot: snapshot, PreviousRequestGuardians: run.previousGuardians,
	})
	return err
}

// applyUndecidedChild replaces the bookings of a child without a student and
// applies its capacity override, or reopens a rejected child whose own data
// changed.
func (s *ChangeRequests) applyUndecidedChild(ctx context.Context, run *approval, i int, existing *RequestChild) error {
	replacement := make([]*enrollment.RequestChildOfferingRecord, 0, len(run.prepared.selections[i]))
	for _, pick := range run.prepared.selections[i] {
		replacement = append(replacement, &enrollment.RequestChildOfferingRecord{
			RequestChildID: existing.ID, CareOfferingID: pick.OfferingID, SelectedDays: pick.SelectedDays,
			ManualSelectedDays: pick.ManualSelectedDays, AutomaticSelectedDays: pick.AutomaticSelectedDays,
		})
	}
	if err := changeCareBookings(ctx, s.deps.Bookings, existing.ID, run.prepared.phase, replacement); err != nil {
		return err
	}
	if status, ok := run.overrides[i]; ok {
		if existing.Status == status {
			return nil
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, existing.ID, status, nil, run.input.ActorAccountID); err != nil {
			return err
		}
		if status == enrollmentModels.ChildStatusWaitlisted {
			run.newlyWaitlisted[existing.ID] = struct{}{}
		}
		return nil
	}
	if existing.Status == enrollmentModels.ChildStatusRejected && childSnapshotChanged(run.row.BaseSnapshot, run.row.ProposedSnapshot, existing.ID) {
		return s.deps.Children.UpdateChildStatus(ctx, existing.ID, enrollmentModels.ChildStatusUnderReview, nil, run.input.ActorAccountID)
	}
	return nil
}

// changeCareBookings replaces a request child's Care Plan bookings with the
// approved selection over the phase window.
func changeCareBookings(ctx context.Context, commands CareBookingChanges, childID int64, phase *enrollment.Phase, selections []*enrollment.RequestChildOfferingRecord) error {
	if commands == nil {
		return errors.New("offering adjustment requires Care Plan booking commands")
	}
	start, until := calendar.Date(phase.ServiceStartDate), calendar.Date(phase.ServiceEndDate).AddDays(1)
	bookings := make([]enrollment.CareBookingInput, 0, len(selections))
	for _, pick := range selections {
		if pick == nil {
			return errors.New("care booking cannot be nil")
		}
		from, end := start, until
		if pick.ValidFrom != nil {
			from = *pick.ValidFrom
		}
		if pick.ValidUntil != nil {
			end = *pick.ValidUntil
		}
		manual := pick.ManualSelectedDays
		if len(manual) == 0 && len(pick.AutomaticSelectedDays) == 0 {
			manual = pick.SelectedDays
		}
		bookings = append(bookings, enrollment.CareBookingInput{
			CareOfferingID: pick.CareOfferingID, ManualSelectedDays: manual, AutomaticSelectedDays: pick.AutomaticSelectedDays,
			ValidFrom: &from, ValidUntil: &end,
		})
		pick.ValidFrom, pick.ValidUntil = &from, &end
	}
	return commands.ReplaceCareBookings(ctx, childID, bookings)
}

// lockApprovedStudents takes the row locks of every student this approval can
// write, and of their "läuft mit" companions, up front in the global
// ascending-id order; the per-child writes would otherwise lock in
// enrollment order and deadlock against a companion edit.
func (s *ChangeRequests) lockApprovedStudents(ctx context.Context, children []*RequestChild) error {
	if s.deps.Companions == nil {
		return nil
	}
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		if child != nil && child.CreatedStudentID != nil && *child.CreatedStudentID > 0 {
			ids = append(ids, *child.CreatedStudentID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if err := s.deps.Companions.LockCompanionGraph(ctx, ids, nil); err != nil {
		// Care Plan reports a linked child held elsewhere with its own value;
		// the approval answers the student write's retriable conflict.
		if errors.Is(err, careplan.ErrCompanionLockBusy) && s.deps.CompanionLockBusy != nil {
			err = s.deps.CompanionLockBusy
		}
		return fmt.Errorf("change request approve: lock students: %w", err)
	}
	return nil
}

// beginCompanionStrandingBatch opens the deferred-verdict scope of the
// per-child departure writes. Without a coordinator nothing could decide the
// deferred verdicts, so every write keeps deciding its own.
func (s *ChangeRequests) beginCompanionStrandingBatch(ctx context.Context) context.Context {
	if s.deps.Companions == nil {
		return ctx
	}
	batchCtx, _ := departure.ContextWithStrandingBatch(ctx)
	return batchCtx
}

func (s *ChangeRequests) verifyCompanionStrandingBatch(ctx context.Context) error {
	if s.deps.Companions == nil {
		return nil
	}
	if err := s.deps.Companions.VerifyCompanionStrandingBatch(ctx); err != nil {
		return fmt.Errorf("change request approve: verify companion links: %w", err)
	}
	return nil
}

// reconcileApprovedPickups refreshes the students' booking-derived pickup
// times the replaced bookings may have changed.
func (s *ChangeRequests) reconcileApprovedPickups(ctx context.Context, children []*RequestChild) error {
	if s.deps.Decisions == nil || s.deps.BookingGates == nil {
		return nil
	}
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child.CreatedStudentID != nil && *child.CreatedStudentID > 0 {
			studentIDs = append(studentIDs, *child.CreatedStudentID)
		}
	}
	if len(studentIDs) == 0 {
		return nil
	}
	if err := s.deps.BookingGates.ReconcileOfferingPickupForStudents(ctx, studentIDs); err != nil {
		return fmt.Errorf("change request approve: reconcile offering pickup times: %w", err)
	}
	return nil
}

func (s *ChangeRequests) notifyApprovedCapacityDecisions(ctx context.Context, run *approval) error {
	if len(run.newlyWaitlisted) == 0 {
		return nil
	}
	refreshed, err := decodedChildrenOfRequest(ctx, s.deps.Children, run.req.ID, false)
	if err != nil {
		return fmt.Errorf("change request approve: refresh capacity decisions: %w", err)
	}
	if err := notifyDecisions(ctx, s.deps.Notifications, decisionGeneration{
		Request: run.req, Children: refreshed, Phase: run.prepared.phase,
		ImmediateChildIDs: run.newlyWaitlisted, ParentsURL: s.deps.ParentsURL,
	}); err != nil {
		return fmt.Errorf("change request approve: notify capacity decisions: %w", err)
	}
	return nil
}

// changeRequestCapacityOverrides runs the capacity gate for the children the
// approval keeps or puts in competition again, excluding their own current
// claims.
func (s *ChangeRequests) changeRequestCapacityOverrides(ctx context.Context, run *approval) (map[int]string, error) {
	overrides := make(map[int]string)
	if s.deps.Children == nil || len(run.children) == 0 {
		return overrides, nil
	}
	candidates := make([]SubmitChild, 0, len(run.children))
	candidateIndexes := make([]int, 0, len(run.children))
	preservedChildIDs := make([]int64, 0, len(run.children))
	for i, child := range run.children {
		if s.approvedChildUsesDecisionSync(child) {
			continue
		}
		reopensRejected := child.Status == enrollmentModels.ChildStatusRejected &&
			childSnapshotChanged(run.row.BaseSnapshot, run.row.ProposedSnapshot, child.ID)
		if !childStatusCountsForCapacity(child.Status) && !reopensRejected {
			continue
		}
		candidates = append(candidates, run.prepared.request.Children[i])
		candidateIndexes = append(candidateIndexes, i)
		if childStatusCountsForCapacity(child.Status) {
			preservedChildIDs = append(preservedChildIDs, child.ID)
		}
	}
	if len(candidates) == 0 {
		return overrides, nil
	}
	candidateOverrides, err := s.intake.applyCapacityOverflow(ctx, run.prepared.phase, candidates, nil, preservedChildIDs)
	if err != nil {
		return nil, fmt.Errorf("change request approve: capacity overflow: %w", err)
	}
	for candidateIdx, status := range candidateOverrides {
		if candidateIdx >= 0 && candidateIdx < len(candidateIndexes) {
			overrides[candidateIndexes[candidateIdx]] = status
		}
	}
	return overrides, nil
}

func (s *ChangeRequests) approvedChildUsesDecisionSync(child *RequestChild) bool {
	return child != nil &&
		child.Status == enrollmentModels.ChildStatusApproved &&
		child.CreatedStudentID != nil &&
		s.deps.Decisions != nil
}

// postApprovalChildStatus predicts the status a child ends the approval with:
// a capacity override wins, otherwise a rejected child whose data changed is
// reopened, otherwise the status stays.
func postApprovalChildStatus(row *ChangeRequest, existing *RequestChild, overrides map[int]string, index int) string {
	if status, ok := overrides[index]; ok {
		return status
	}
	if existing.Status == enrollmentModels.ChildStatusRejected && childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, existing.ID) {
		return enrollmentModels.ChildStatusUnderReview
	}
	return existing.Status
}

// reconcileMatchedStudent keeps an existing_students child's pin honest
// across an approval (#1663): an edited identity drops the pin and
// re-resolves by the new identity under the per-student permission, and the
// uniqueness guard runs again for a child that ends the approval active — a
// reopened rejection competes again. Children with a student or a rollover
// source are skipped. Call it before the proposed identity overwrites the
// row.
func (s *ChangeRequests) reconcileMatchedStudent(ctx context.Context, run *approval, existing *RequestChild, next SubmitChild, index int) error {
	phase := run.prepared.phase
	if existing.CreatedStudentID != nil || existing.RolloverSourceChildID != nil {
		return nil
	}
	if phase.Audience != enrollment.PhaseAudienceExistingStudents && existing.MatchedStudentID == nil {
		return nil
	}
	pin, err := s.reconciledPin(ctx, run, existing, next, index)
	if err != nil {
		return err
	}
	if childStatusCountsForCapacity(postApprovalChildStatus(run.row, existing, run.overrides, index)) {
		if err := s.intake.guardMatchedStudentUnique(ctx, phase.ID, pin, existing.ID, index); err != nil {
			return err
		}
	}
	if ptrInt64Equal(pin, existing.MatchedStudentID) {
		return nil
	}
	if err := s.deps.Children.UpdateMatchedStudent(ctx, existing.ID, pin); err != nil {
		return fmt.Errorf("change request approve: update matched student for child %d: %w", index, err)
	}
	s.deps.Logger.Info("change request approve: re-pinned existing-student match",
		slog.Int64("request_child_id", existing.ID),
		slog.Bool("had_pin", existing.MatchedStudentID != nil),
		slog.Bool("has_pin", pin != nil),
	)
	existing.MatchedStudentID = pin
	return nil
}

// reconciledPin resolves the pin the child carries after the approval and
// authorizes it against the stored request's identity.
func (s *ChangeRequests) reconciledPin(ctx context.Context, run *approval, existing *RequestChild, next SubmitChild, index int) (*int64, error) {
	phase, req := run.prepared.phase, run.req
	pin := existing.MatchedStudentID
	if pin != nil && !sameSubmittedIdentity(existing, next) {
		pin = nil
	}
	if phase.Audience == enrollment.PhaseAudienceExistingStudents {
		resolved, err := s.intake.resolveMatchedStudentID(ctx, req.TenantID, phase, index, next)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			pin = resolved
		}
	}
	if err := assertExistingStudentMatchResolved(phase, pin, run.eligibilityEnforced, index); err != nil {
		return nil, err
	}
	submitter := reEnrollmentSubmitterFor(req.SubmissionSource, req.GuardianAccountID, req.GuardianEmail)
	if pin != nil {
		resolvedSubmitter, err := reEnrollmentSubmitterForPersistedRequest(ctx, s.deps.LateInvites, req)
		if err != nil {
			return nil, fmt.Errorf("change request approve: resolve re-enrollment identity: %w", err)
		}
		submitter = resolvedSubmitter
	}
	if err := s.intake.assertGuardianMayReEnrollStudent(ctx, submitter, pin, req.TenantID, index); err != nil {
		return nil, err
	}
	return pin, nil
}

func ptrInt64Equal(a, b *int64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func childStatusCountsForCapacity(status string) bool {
	return status != enrollmentModels.ChildStatusRejected && status != enrollmentModels.ChildStatusWithdrawn
}

// ensureNoActiveDuplicateForApproval applies the duplicate policy to the
// approved identities under the submission's dedup lock.
func (s *ChangeRequests) ensureNoActiveDuplicateForApproval(ctx context.Context, req *enrollmentModels.Request, prepared SubmitRequest) error {
	emailLC := lowerTrim(req.GuardianEmail)
	if err := s.deps.Requests.AcquireSubmissionDedupLock(ctx, req.PhaseID, enrollment.SubmissionDedupLockKey(emailLC)); err != nil {
		return fmt.Errorf("change request approve: acquire duplicate lock: %w", err)
	}
	dupes, err := s.deps.Requests.ActiveDuplicateChildren(ctx, req.PhaseID, req.GuardianEmail, duplicateChildKeys(prepared.Children), req.ID)
	if err != nil {
		return fmt.Errorf("change request approve: duplicate check: %w", err)
	}
	if len(dupes) == 0 {
		return nil
	}
	if s.deps.Settings == nil {
		return errSettingsNotConfigured
	}
	policy, err := s.deps.Settings.DuplicateHandling(ctx)
	if err != nil {
		return fmt.Errorf("change request approve: resolve duplicate handling: %w", err)
	}
	switch policy {
	case duplicateHandlingBlock:
		return enrollment.ErrDuplicateEnrollment
	case duplicateHandlingWarn:
		s.deps.Logger.WarnContext(ctx, "change request approved with active duplicate", slog.Int64("request_id", req.ID))
		return nil
	case duplicateHandlingIgnore:
		return nil
	default:
		return fmt.Errorf("change request approve: unsupported duplicate handling %q", policy)
	}
}

func matchingPreviousGuardianProfileID(previous []*enrollment.RequestGuardian, guardian SubmitGuardian) *int64 {
	key := guardianProfileCarryKey(guardian.FirstName, guardian.LastName, guardian.Email, guardian.Phone)
	for _, row := range previous {
		if row == nil || row.GuardianProfileID == nil || *row.GuardianProfileID <= 0 {
			continue
		}
		if guardianProfileCarryKey(row.FirstName, row.LastName, row.Email, row.Phone) == key {
			return row.GuardianProfileID
		}
	}
	return nil
}

func guardianProfileCarryKey(firstName, lastName string, email, phone *string) string {
	return strings.Join([]string{
		lowerTrim(firstName), lowerTrim(lastName), lowerTrim(optionalString(email)), lowerTrim(optionalString(phone)),
	}, "\x00")
}
