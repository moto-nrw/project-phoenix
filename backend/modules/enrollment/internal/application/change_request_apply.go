package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/selection"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/moto-nrw/project-phoenix/sharedkernel/textnorm"
)

// approval is the state one change-request approval works on.
type approval struct {
	row               *ChangeRequest
	input             enrollmentNative.ReviewChangeRequestInput
	req               *enrollmentNative.Request
	children          []*enrollmentNative.RequestChild
	prepared          *proposal
	capabilities      enrollmentNative.FormCapabilities
	previousGuardians []*enrollmentNative.RequestGuardian
	overrides         map[int]string
	// eligibilityEnforced is false for trusted-source and rollover-generated
	// requests, which bypassed the self-service gates at creation.
	eligibilityEnforced bool
	newlyWaitlisted     map[int64]struct{}
	// rosterImpacts collect what the booking changes did to the children's
	// Regeltermine (#5102).
	rosterImpacts []enrollmentNative.ChildRosterImpact
	// pins are the pins the approval leaves on the children it reconciles,
	// resolved before the phase locks; holds are the request identity's
	// grants over them and over the approved students, held behind the
	// parent guard until commit (#6638).
	pins  map[int]*int64
	holds renewalHolds
	reads *approvalAuthorityReads
}

// applyApprovedChange applies an approved proposal to the request, its
// children, co-guardians, bookings and the students an approval created, in
// the locked transaction.
func (s *ChangeRequests) applyApprovedChange(ctx context.Context, row *ChangeRequest, input enrollmentNative.ReviewChangeRequestInput) ([]enrollmentNative.ChildRosterImpact, error) {
	run, err := s.applyApprovedProposal(ctx, row, input)
	if err != nil {
		return nil, err
	}
	return run.rosterImpacts, nil
}

// applyApprovedProposal runs under the gates Approve took before the request
// row lock (lockApprovalGates): booking writers hold the recurrence gate while
// their FK checks touch request_children.
func (s *ChangeRequests) applyApprovedProposal(ctx context.Context, row *ChangeRequest, input enrollmentNative.ReviewChangeRequestInput) (*approval, error) {
	run, err := s.loadApproval(ctx, row, input)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, approvalAuthorityReadsKey{}, run.reads)
	if err := s.writeApprovedRequest(ctx, run); err != nil {
		return nil, err
	}
	// One approval is one edit of the whole family, applied child by child.
	// Defer the companion stranding verdicts and decide them once against
	// every child's final plan (#1694).
	ctx = s.beginCompanionStrandingBatch(ctx)
	for i, existing := range run.children {
		if err := s.applyApprovedChild(ctx, run, i, existing); err != nil {
			return nil, err
		}
	}
	if err := s.verifyCompanionStrandingBatch(ctx); err != nil {
		return nil, err
	}
	if err := s.reconcileApprovedPickups(ctx, run.children); err != nil {
		return nil, err
	}
	return run, s.notifyApprovedCapacityDecisions(ctx, run)
}

// loadApproval locks the request and its children, refuses a stale base
// snapshot and prepares the stored proposal with the capability pinned at
// creation.
func (s *ChangeRequests) loadApproval(ctx context.Context, row *ChangeRequest, input enrollmentNative.ReviewChangeRequestInput) (*approval, error) {
	req, children, err := s.lockApprovalRequest(ctx, row)
	if err != nil {
		return nil, err
	}
	proposed, err := snapshotToSubmitRequest(row.ProposedSnapshot)
	if err != nil {
		return nil, err
	}
	run := &approval{row: row, input: input, req: req, children: children, newlyWaitlisted: map[int64]struct{}{}}
	if run.capabilities, err = s.formCapabilities(ctx, &row.CareOfferingsEnabledAtCreation); err != nil {
		return nil, err
	}
	phase, err := s.lockedProposalPhase(ctx, req.PhaseID)
	if err != nil {
		return nil, err
	}
	if run.prepared, err = s.prepareProposed(ctx, phase, req, children, proposed, run.capabilities, true, false); err != nil {
		return nil, err
	}
	// Renewals hold their students before the email-scoped dedup lock. Hold
	// the parent's authority and lock the entire approved graph in that
	// order before taking it too.
	if err := s.holdApprovalAuthority(ctx, run); err != nil {
		return nil, err
	}
	if err := s.ensureNoActiveDuplicateForApproval(ctx, req, run.prepared.request); err != nil {
		return nil, err
	}
	if err := s.loadApprovalContext(ctx, run); err != nil {
		return nil, err
	}
	run.eligibilityEnforced = storedRequestEligibility(req.SubmissionSource, children) != eligibilityNone
	return run, nil
}

// lockApprovalRequest locks the request and its children and refuses a
// withdrawn child or a base snapshot that no longer matches.
func (s *ChangeRequests) lockApprovalRequest(ctx context.Context, row *ChangeRequest) (*enrollmentNative.Request, []*enrollmentNative.RequestChild, error) {
	req, err := s.requestByID(ctx, row.RequestID, true)
	if err != nil {
		return nil, nil, err
	}
	children, err := s.deps.Children.ChildrenForRequest(ctx, req.ID, true)
	if err != nil {
		return nil, nil, fmt.Errorf("change request approve: lock children: %w", err)
	}
	for _, child := range children {
		if child.Status == enrollmentNative.ChildStatusWithdrawn {
			return nil, nil, enrollmentNative.ErrChangeRequestNotAllowed
		}
	}
	current, err := s.currentSnapshot(ctx, req, children)
	if err != nil {
		return nil, nil, err
	}
	if !reconcileRenewalConsent(row, req, children, current) {
		return nil, nil, enrollmentNative.ErrChangeRequestConflict
	}
	return req, children, nil
}

// loadApprovalContext reads the co-guardians the approval replaces and runs
// the capacity gate when the pinned capability books offerings.
func (s *ChangeRequests) loadApprovalContext(ctx context.Context, run *approval) error {
	var err error
	if s.deps.Guardians != nil {
		if run.previousGuardians, err = s.deps.Guardians.RequestGuardians(ctx, []int64{run.req.ID}); err != nil {
			return fmt.Errorf("change request approve: list previous guardians: %w", err)
		}
	}
	run.overrides = map[int]string{}
	if run.capabilities.CareOfferingsEnabled {
		if run.overrides, err = s.changeRequestCapacityOverrides(ctx, run); err != nil {
			return err
		}
	}
	return nil
}

// writeApprovedRequest writes the proposal's guardian fields and replaces the
// co-guardians, carrying their stamped guardian profiles over.
func (s *ChangeRequests) writeApprovedRequest(ctx context.Context, run *approval) error {
	req, prepared := run.req, run.prepared.request
	req.GuardianFirstName = textnorm.Name(prepared.GuardianFirstName)
	req.GuardianLastName = textnorm.Name(prepared.GuardianLastName)
	req.GuardianEmail = guardianEmailKey(prepared.GuardianEmail)
	req.GuardianPhone = prepared.GuardianPhone
	req.ConsentFlags = formValues(prepared.ConsentFlags)
	req.CustomData = formValues(prepared.CustomData)
	if err := s.deps.Requests.UpdateRequestGuardian(ctx, req, true); err != nil {
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
func (s *ChangeRequests) applyApprovedChild(ctx context.Context, run *approval, i int, existing *enrollmentNative.RequestChild) error {
	next := run.prepared.request.Children[i]
	// Re-point the existing_students pin before the proposed identity
	// overwrites the row, and re-run the uniqueness guard for a child that
	// ends this approval active (#1663).
	if err := s.reconcileMatchedStudent(ctx, run, existing, next, i); err != nil {
		return err
	}
	existing.FirstName = textnorm.Name(next.FirstName)
	existing.LastName = textnorm.Name(next.LastName)
	existing.DateOfBirth = next.DateOfBirth
	existing.TargetGradeLevel = next.TargetGradeLevel
	existing.TargetSchoolClass = next.TargetSchoolClass
	// A form that no longer asks keeps the answer given before (#4703).
	if next.HolidayCareWish != nil {
		existing.HolidayCareWish = next.HolidayCareWish
	}
	existing.CustomData = formValues(next.CustomData)
	existing.SortOrder = i
	if err := s.deps.Children.UpdateChildData(ctx, existing); err != nil {
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
func (s *ChangeRequests) syncApprovedChild(ctx context.Context, run *approval, existing *enrollmentNative.RequestChild, selections []selection.Selection) error {
	if run.row.CareOfferingsEnabledAtCreation {
		offeringInput := enrollmentNative.UpdateChildOfferingsInput{
			RequestID: run.req.ID, ChildID: existing.ID, Reason: run.input.Note,
			ActorAccountID: run.input.ActorAccountID, ActorRole: run.input.ActorRole,
		}
		for _, pick := range selections {
			offeringInput.Offerings = append(offeringInput.Offerings, enrollmentNative.OfferingAdjustmentSelection{OfferingID: pick.OfferingID, SelectedDays: pick.SelectedDays})
		}
		update, err := s.deps.Decisions.ApplyChangeRequestOfferings(ctx, offeringInput)
		if err != nil {
			return err
		}
		if existing.CreatedStudentID != nil {
			run.rosterImpacts = append(run.rosterImpacts, enrollmentNative.ChildRosterImpact{
				ChildID: existing.ID, StudentID: *existing.CreatedStudentID,
				FirstName: existing.FirstName, Impact: update.RosterImpact,
			})
		}
	}
	snapshot, err := json.Marshal(run.row.BaseSnapshot)
	if err != nil {
		return err
	}
	_, err = s.deps.Decisions.SyncApprovedChildData(ctx, enrollmentNative.ApprovedChildSync{
		RequestID: run.req.ID, ChildID: existing.ID, ActorAccountID: run.input.ActorAccountID,
		ReplaceTargetedData: true, PreviousSnapshot: snapshot, PreviousRequestGuardians: run.previousGuardians,
		ChildDataChanged: childSnapshotChanged(run.row.BaseSnapshot, run.row.ProposedSnapshot, existing.ID),
	})
	return err
}

// applyUndecidedChild replaces the bookings of a child without a student and
// applies its capacity override, or reopens a rejected child whose own data
// changed.
func (s *ChangeRequests) applyUndecidedChild(ctx context.Context, run *approval, i int, existing *enrollmentNative.RequestChild) error {
	replacement := make([]*enrollmentNative.RequestChildOfferingRecord, 0, len(run.prepared.selections[i]))
	for _, pick := range run.prepared.selections[i] {
		replacement = append(replacement, &enrollmentNative.RequestChildOfferingRecord{
			RequestChildID: existing.ID, CareOfferingID: pick.OfferingID, SelectedDays: pick.SelectedDays,
			ManualSelectedDays: pick.ManualSelectedDays, AutomaticSelectedDays: pick.AutomaticSelectedDays,
		})
	}
	if err := changeCareBookings(ctx, s.deps.Bookings, existing.ID, run.prepared.locked, replacement); err != nil {
		return err
	}
	if status, ok := run.overrides[i]; ok {
		if existing.Status == status {
			return nil
		}
		if err := s.deps.Children.UpdateChildStatus(ctx, existing.ID, status, nil, run.input.ActorAccountID); err != nil {
			return err
		}
		if status == enrollmentNative.ChildStatusWaitlisted {
			run.newlyWaitlisted[existing.ID] = struct{}{}
		}
		return nil
	}
	if existing.Status == enrollmentNative.ChildStatusRejected && childSnapshotChanged(run.row.BaseSnapshot, run.row.ProposedSnapshot, existing.ID) {
		return s.deps.Children.UpdateChildStatus(ctx, existing.ID, enrollmentNative.ChildStatusUnderReview, nil, run.input.ActorAccountID)
	}
	return nil
}

// changeCareBookings replaces a request child's Care Plan bookings with the
// approved selection over the window of the phase read under the recurrence
// gate (#6657).
func changeCareBookings(ctx context.Context, commands CareBookingChanges, childID int64, phase enrollmentNative.LockedPhase, selections []*enrollmentNative.RequestChildOfferingRecord) error {
	if commands == nil {
		return errors.New("offering adjustment requires Care Plan booking commands")
	}
	start, until, err := phase.BookingWindow()
	if err != nil {
		return err
	}
	bookings := make([]enrollmentNative.CareBookingInput, 0, len(selections))
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
		bookings = append(bookings, enrollmentNative.CareBookingInput{
			CareOfferingID: pick.CareOfferingID, ManualSelectedDays: manual, AutomaticSelectedDays: pick.AutomaticSelectedDays,
			ValidFrom: &from, ValidUntil: &end,
		})
		pick.ValidFrom, pick.ValidUntil = &from, &end
	}
	return commands.ReplaceCareBookings(ctx, childID, bookings)
}

// approvalAuthority groups the students of one approval by the identity
// whose authority over them the approval asks.
type approvalAuthority struct {
	s      *ChangeRequests
	run    *approval
	graph  []int64
	groups map[string]*authorityGroup
	// stored and edited cache the identities of the stored request and of
	// the request after the edit.
	stored, edited *reEnrollmentSubmitter
}

type authorityGroup struct {
	submitter reEnrollmentSubmitter
	ids       []int64
	pins      bool
}

func newApprovalAuthority(s *ChangeRequests, run *approval) *approvalAuthority {
	run.pins = map[int]*int64{}
	return &approvalAuthority{s: s, run: run, groups: map[string]*authorityGroup{}}
}

// collect records the child's pin and, for an approved child the decision
// flow syncs, its student.
func (a *approvalAuthority) collect(ctx context.Context, index int, child *enrollmentNative.RequestChild) error {
	if reconcilesPin(a.run.prepared.phase, child) {
		pin, err := a.s.approvalPin(ctx, a.run, child, a.run.prepared.request.Children[index], index)
		if err != nil {
			return err
		}
		a.run.pins[index] = pin
		if pin != nil {
			if err := a.add(ctx, a.editedRequest(), &a.edited, *pin, true); err != nil {
				return err
			}
		}
	}
	if child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil
	}
	a.graph = append(a.graph, *child.CreatedStudentID)
	if !a.s.approvedChildUsesDecisionSync(child) || child.MatchedByOffice {
		return nil
	}
	return a.addSynced(ctx, child)
}

// addSynced asks for the approved student with the identity the sync asks:
// the account, the late invite a rollover chain started with, or the stored
// request.
func (a *approvalAuthority) addSynced(ctx context.Context, child *enrollmentNative.RequestChild) error {
	if a.run.req.GuardianAccountID == nil && child.RolloverSourceChildID != nil {
		origin, err := rolloverOrigin(ctx, a.s.deps.Children, a.s.deps.Requests, a.s.deps.Runtime.NotFound, child)
		if err != nil {
			return err
		}
		if origin != nil && enrollmentNative.NormalizedSubmissionSource(origin.SubmissionSource) == enrollmentNative.RequestSourceLateInvite {
			var submitter *reEnrollmentSubmitter
			return a.add(ctx, origin, &submitter, *child.CreatedStudentID, false)
		}
	}
	return a.add(ctx, a.run.req, &a.stored, *child.CreatedStudentID, false)
}

// editedRequest is the request with the contact the edit writes
// (writeApprovedRequest): the pin is checked against the identity the
// approval links.
func (a *approvalAuthority) editedRequest() *enrollmentNative.Request {
	edited := *a.run.req
	edited.GuardianEmail = guardianEmailKey(a.run.prepared.request.GuardianEmail)
	return &edited
}

func (a *approvalAuthority) add(ctx context.Context, identity *enrollmentNative.Request, cached **reEnrollmentSubmitter, studentID int64, pin bool) error {
	if *cached == nil {
		submitter, err := reEnrollmentSubmitterForPersistedRequest(ctx, a.s.deps.LateInvites, identity)
		if err != nil {
			return fmt.Errorf("change request approve: resolve re-enrollment identity: %w", err)
		}
		*cached = &submitter
	}
	key := submitterKey(**cached)
	group := a.groups[key]
	if group == nil {
		group = &authorityGroup{submitter: **cached}
		a.groups[key] = group
	}
	group.ids = append(group.ids, studentID)
	group.pins = group.pins || pin
	return nil
}

// hold holds every identity's authority over all students of the approval,
// so the first hold locks the whole graph in its children slot; the grants
// of the pins' identity decide the pins (reconciledPin).
func (a *approvalAuthority) hold(ctx context.Context) error {
	a.run.holds = renewalHolds{}
	graph := a.graph
	if a.s.deps.Companions == nil {
		graph = nil
	}
	students := a.students()
	for _, key := range slices.Sorted(maps.Keys(a.groups)) {
		group := a.groups[key]
		holds, err := a.s.intake.holdReEnrollment(ctx, group.submitter, students, graph, a.run.req.TenantID)
		if err != nil {
			return fmt.Errorf("change request approve: %w", companionLockConflict(err, a.s.deps.CompanionLockBusy))
		}
		graph = nil
		if group.pins {
			for _, id := range group.ids {
				a.run.holds[id] = holds[id]
			}
		}
	}
	return nil
}

// students are the held students, ascending and unique.
func (a *approvalAuthority) students() []int64 {
	var ids []int64
	for _, group := range a.groups {
		ids = append(ids, group.ids...)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

// submitterKey names an identity; staff manual enrollment holds nothing.
func submitterKey(submitter reEnrollmentSubmitter) string {
	switch {
	case submitter.AdminManaged:
		return "admin"
	case submitter.GuardianAccountID != nil:
		return fmt.Sprintf("account:%d", *submitter.GuardianAccountID)
	default:
		return "email:" + submitter.GuardianEmail
	}
}

// companionLockConflict answers Care Plan's linked child held elsewhere with
// the student write's retriable conflict.
func companionLockConflict(err, busy error) error {
	if errors.Is(err, careplan.ErrCompanionLockBusy) && busy != nil {
		return busy
	}
	return err
}

// reconcilesPin reports whether the approval re-resolves the child's
// existing_students pin (reconcileMatchedStudent).
func reconcilesPin(phase *enrollmentNative.Phase, child *enrollmentNative.RequestChild) bool {
	if child.CreatedStudentID != nil || child.RolloverSourceChildID != nil {
		return false
	}
	return phase.Audience == enrollmentNative.PhaseAudienceExistingStudents || child.MatchedStudentID != nil
}

// lockApprovedStudents takes the row locks of every student this approval can
// write, and of their "läuft mit" companions, up front in the global
// ascending-id order; the per-child writes would otherwise lock in
// enrollment order and deadlock against a companion edit. The pinned
// students are locked in the same pass.
func (s *ChangeRequests) lockApprovedStudents(ctx context.Context, children []*enrollmentNative.RequestChild, pinned []int64) error {
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
	if err := s.deps.Companions.LockCompanionGraph(ctx, ids, pinned); err != nil {
		return fmt.Errorf("change request approve: lock students: %w", companionLockConflict(err, s.deps.CompanionLockBusy))
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
func (s *ChangeRequests) reconcileApprovedPickups(ctx context.Context, children []*enrollmentNative.RequestChild) error {
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
	refreshed, err := s.deps.Children.ChildrenForRequest(ctx, run.req.ID, false)
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
	candidates, candidateIndexes, preservedChildIDs := s.capacityCandidates(run)
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

// capacityCandidates selects the proposed children that compete for a slot:
// those holding one and rejected ones the proposal changes. Children the
// decision flow syncs are left out. It returns the candidates, their child
// indexes and the IDs whose own current claims the gate excludes.
func (s *ChangeRequests) capacityCandidates(run *approval) ([]SubmitChild, []int, []int64) {
	candidates := make([]SubmitChild, 0, len(run.children))
	candidateIndexes := make([]int, 0, len(run.children))
	preservedChildIDs := make([]int64, 0, len(run.children))
	for i, child := range run.children {
		if s.approvedChildUsesDecisionSync(child) {
			continue
		}
		reopensRejected := child.Status == enrollmentNative.ChildStatusRejected &&
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
	return candidates, candidateIndexes, preservedChildIDs
}

func (s *ChangeRequests) approvedChildUsesDecisionSync(child *enrollmentNative.RequestChild) bool {
	return child != nil &&
		child.Status == enrollmentNative.ChildStatusApproved &&
		child.CreatedStudentID != nil &&
		s.deps.Decisions != nil
}

// postApprovalChildStatus predicts the status a child ends the approval with:
// a capacity override wins, otherwise a rejected child whose data changed is
// reopened, otherwise the status stays.
func postApprovalChildStatus(row *ChangeRequest, existing *enrollmentNative.RequestChild, overrides map[int]string, index int) string {
	if status, ok := overrides[index]; ok {
		return status
	}
	if existing.Status == enrollmentNative.ChildStatusRejected && childSnapshotChanged(row.BaseSnapshot, row.ProposedSnapshot, existing.ID) {
		return enrollmentNative.ChildStatusUnderReview
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
func (s *ChangeRequests) reconcileMatchedStudent(ctx context.Context, run *approval, existing *enrollmentNative.RequestChild, next SubmitChild, index int) error {
	phase := run.prepared.phase
	if !reconcilesPin(phase, existing) {
		return nil
	}
	pin, err := s.reconciledPin(run, index)
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

// approvalPin resolves the pin the child carries after the approval: an
// edited identity drops the stored pin, and an existing_students phase
// re-resolves it by the proposed identity.
func (s *ChangeRequests) approvalPin(ctx context.Context, run *approval, existing *enrollmentNative.RequestChild, next SubmitChild, index int) (*int64, error) {
	phase := run.prepared.phase
	pin := existing.MatchedStudentID
	if pin != nil && !sameSubmittedIdentity(existing, next) {
		pin = nil
	}
	if phase.Audience == enrollmentNative.PhaseAudienceExistingStudents {
		resolved, err := s.intake.resolveMatchedStudentID(ctx, run.req.TenantID, phase, index, next)
		if err != nil {
			return nil, err
		}
		if resolved != nil {
			pin = resolved
		}
	}
	return pin, nil
}

// reconciledPin is the child's pin after the approval, authorized by the
// request identity's grant the approval holds (holdApprovalAuthority).
func (s *ChangeRequests) reconciledPin(run *approval, index int) (*int64, error) {
	pin := run.pins[index]
	if err := assertExistingStudentMatchResolved(run.prepared.phase, pin, run.eligibilityEnforced, index); err != nil {
		return nil, err
	}
	if err := s.intake.holdGuardianReEnrollment(run.holds, pin, index); err != nil {
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
	return status != enrollmentNative.ChildStatusRejected && status != enrollmentNative.ChildStatusWithdrawn
}

// ensureNoActiveDuplicateForApproval applies the duplicate policy to the
// approved identities under the submission's dedup lock.
func (s *ChangeRequests) ensureNoActiveDuplicateForApproval(ctx context.Context, req *enrollmentNative.Request, prepared SubmitRequest) error {
	emailLC := guardianEmailKey(req.GuardianEmail)
	if err := s.deps.Requests.AcquireSubmissionDedupLock(ctx, req.PhaseID, enrollmentNative.SubmissionDedupLockKey(emailLC)); err != nil {
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
		return enrollmentNative.ErrDuplicateEnrollment
	case duplicateHandlingWarn:
		s.deps.Logger.WarnContext(ctx, "change request approved with active duplicate", slog.Int64("request_id", req.ID))
		return nil
	case duplicateHandlingIgnore:
		return nil
	default:
		return fmt.Errorf("change request approve: unsupported duplicate handling %q", policy)
	}
}

func matchingPreviousGuardianProfileID(previous []*enrollmentNative.RequestGuardian, guardian SubmitGuardian) *int64 {
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
		lowerTrim(firstName), lowerTrim(lastName), guardianEmailKey(optionalString(email)), lowerTrim(optionalString(phone)),
	}, "\x00")
}
