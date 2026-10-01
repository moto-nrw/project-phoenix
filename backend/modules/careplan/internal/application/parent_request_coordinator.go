package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// maxBulkParentRequests bounds one bulk approval.
const maxBulkParentRequests = 50

// ParentRequestCoordinatorDependencies wires the cross-kind commands. Rights
// and Rollback are required. A kind without its port is refused: a bulk
// approval naming it fails, and a resolve answers
// parentrequests.ErrConflictKindUnsupported.
type ParentRequestCoordinatorDependencies struct {
	Rights   ports.ParentRequestRightsResolver
	Rollback ports.RollbackMarker
	// MasterData contributes the Stammdaten requests to a bulk approval and
	// holds the student-lock frontier every command takes.
	MasterData ports.MasterDataBulkPort
	Excused    ports.ExcusedBulkPort
	// The conflict ports, one per queue. Care serves both the weekly plan and
	// the single-day pickup change.
	MasterDataConflicts ports.ConflictPort
	ExcusedConflicts    ports.ConflictPort
	CareConflicts       ports.ConflictPort
	OfferingConflicts   ports.ConflictPort
	// Ledger records the ONE thing the queues cannot record for the resolver:
	// the staff member's own result. Every verdict on a request goes through
	// that queue's own decision, which writes its own `decided` event.
	Ledger ports.RequestLedger
}

// ParentRequestCoordinator owns the invariants spanning request kinds (#2267):
// a bulk approval and a conflict resolution are all-or-nothing, lock in one
// canonical order, and re-check the caller's rights per kind. Each queue
// still loads, validates and applies its own payload.
type ParentRequestCoordinator struct {
	rights        ports.ParentRequestRightsResolver
	rollback      ports.RollbackMarker
	masterData    ports.MasterDataBulkPort
	excused       ports.ExcusedBulkPort
	conflictPorts map[parentrequests.Kind]ports.ConflictPort
	ledger        ports.RequestLedger
}

var _ parentrequests.Coordinator = (*ParentRequestCoordinator)(nil)

func NewParentRequestCoordinator(deps ParentRequestCoordinatorDependencies) (*ParentRequestCoordinator, error) {
	if deps.Rights == nil || deps.Rollback == nil {
		return nil, errors.New("care plan parent request coordinator: rights and rollback marker are required")
	}
	conflictPorts := make(map[parentrequests.Kind]ports.ConflictPort, 5)
	addConflictPort(conflictPorts, deps.MasterDataConflicts, parentrequests.KindMasterData)
	addConflictPort(conflictPorts, deps.ExcusedConflicts, parentrequests.KindExcused)
	addConflictPort(conflictPorts, deps.CareConflicts, parentrequests.KindCareSchedule, parentrequests.KindPickupChange)
	addConflictPort(conflictPorts, deps.OfferingConflicts, parentrequests.KindOffering)
	return &ParentRequestCoordinator{
		rights: deps.Rights, rollback: deps.Rollback, masterData: deps.MasterData, excused: deps.Excused,
		conflictPorts: conflictPorts, ledger: deps.Ledger,
	}, nil
}

func addConflictPort(conflictPorts map[parentrequests.Kind]ports.ConflictPort, port ports.ConflictPort, kinds ...parentrequests.Kind) {
	if port == nil {
		return
	}
	for _, kind := range kinds {
		conflictPorts[kind] = port
	}
}

// failed marks the ambient transaction for rollback, so a half-applied
// command can never commit.
func (c *ParentRequestCoordinator) failed(ctx context.Context, err error) error {
	c.rollback.MarkRollback(ctx)
	return err
}

// BulkApprove approves every request of the command or none of them.
func (c *ParentRequestCoordinator) BulkApprove(ctx context.Context, input parentrequests.BulkApproveInput) error {
	if err := validateBulkParentRequestInput(input); err != nil {
		return c.failed(ctx, err)
	}
	if err := authorizeBulkParentRequestKinds(c.rights(ctx), input.Requests); err != nil {
		return c.failed(ctx, err)
	}
	input.Requests = sortedParentRequestRefs(input.Requests)
	masters, excused, err := c.loadBulkParentRequests(ctx, input.Requests)
	if err != nil {
		return c.failed(ctx, err)
	}
	if err := validateBulkParentRequestVersions(input.Requests, masters, excused); err != nil {
		return c.failed(ctx, err)
	}
	if err := c.lockBulkParentRequests(ctx, input.Requests); err != nil {
		return c.failed(ctx, err)
	}
	input.Requests = sortedBulkRefsByStudent(input.Requests, masters, excused)
	if err := c.masterData.LockBulkStudents(ctx, bulkStudentIDs(input.Requests, masters, excused)); err != nil {
		return c.failed(ctx, fmt.Errorf("parent requests: lock bulk students: %w", err))
	}
	if err := c.applyBulkParentRequests(ctx, input); err != nil {
		return c.failed(ctx, err)
	}
	return nil
}

func (c *ParentRequestCoordinator) lockBulkParentRequests(ctx context.Context, refs []parentrequests.Ref) error {
	for _, ref := range refs {
		var err error
		if ref.Kind == parentrequests.KindMasterData {
			err = c.masterData.LockBulkRequest(ctx, ref.ID)
		} else {
			err = c.excused.LockExcusedBulkRequest(ctx, ref.ID)
		}
		if err != nil {
			if isParentRequestRace(err) {
				return fmt.Errorf("%w: %s %d changed before locking", parentrequests.ErrStale, ref.Kind, ref.ID)
			}
			return fmt.Errorf("parent requests: lock %s %d: %w", ref.Kind, ref.ID, err)
		}
	}
	return nil
}

// isParentRequestRace matches the queues' "this request is gone or already
// decided" answers: the list the client decided on is out of date.
func isParentRequestRace(err error) bool {
	return errors.Is(err, masterdatarequests.ErrReviewNotFound) ||
		errors.Is(err, masterdatarequests.ErrReviewNotPending) ||
		errors.Is(err, careplan.ErrStudentDataRequestNotFound) ||
		errors.Is(err, careplan.ErrStudentDataRequestNotPending) ||
		errors.Is(err, absencerecords.ErrExcusedRequestNotFound) ||
		errors.Is(err, absencerecords.ErrExcusedRequestNotPending)
}

func bulkStudentIDs(refs []parentrequests.Ref, masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate) []int64 {
	ids := make([]int64, 0, len(refs))
	var previous int64
	for _, ref := range refs {
		studentID := bulkRefStudentID(ref, masters, excused)
		if studentID != previous {
			ids = append(ids, studentID)
			previous = studentID
		}
	}
	return ids
}

func sortedBulkRefsByStudent(refs []parentrequests.Ref, masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate) []parentrequests.Ref {
	ordered := append([]parentrequests.Ref(nil), refs...)
	slices.SortFunc(ordered, func(left, right parentrequests.Ref) int {
		if byStudent := compareOrdered(bulkRefStudentID(left, masters, excused), bulkRefStudentID(right, masters, excused)); byStudent != 0 {
			return byStudent
		}
		if byKind := compareOrdered(left.Kind, right.Kind); byKind != 0 {
			return byKind
		}
		return compareOrdered(left.ID, right.ID)
	})
	return ordered
}

func bulkRefStudentID(ref parentrequests.Ref, masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate) int64 {
	if ref.Kind == parentrequests.KindMasterData {
		return masters[ref.ID].Request.StudentID
	}
	return excused[ref.ID].StudentID
}

// compareOrdered orders two values the way slices.SortFunc expects.
func compareOrdered[T int64 | parentrequests.Kind](left, right T) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func sortedParentRequestRefs(refs []parentrequests.Ref) []parentrequests.Ref {
	ordered := append([]parentrequests.Ref(nil), refs...)
	slices.SortFunc(ordered, func(left, right parentrequests.Ref) int {
		if byKind := compareOrdered(left.Kind, right.Kind); byKind != 0 {
			return byKind
		}
		return compareOrdered(left.ID, right.ID)
	})
	return ordered
}

func (c *ParentRequestCoordinator) applyBulkParentRequests(ctx context.Context, input parentrequests.BulkApproveInput) error {
	for _, ref := range input.Requests {
		if err := c.applyBulkParentRequest(ctx, input, ref); err != nil {
			if errors.Is(err, parentrequests.ErrDecisionRace) {
				return fmt.Errorf("%w: %s %d changed during approval", parentrequests.ErrStale, ref.Kind, ref.ID)
			}
			return fmt.Errorf("parent requests: bulk apply %s %d: %w", ref.Kind, ref.ID, err)
		}
	}
	return nil
}

func (c *ParentRequestCoordinator) applyBulkParentRequest(ctx context.Context, input parentrequests.BulkApproveInput, ref parentrequests.Ref) error {
	if ref.Kind == parentrequests.KindMasterData {
		_, err := c.masterData.Decide(ctx, masterdatarequests.DecideInput{
			RequestID: ref.ID, Approve: true, Reason: input.Reason, ReviewedBy: input.ReviewerID,
			ExpectedVersion: ref.ExpectedVersion,
		})
		if errors.Is(err, masterdatarequests.ErrReviewNotPending) || errors.Is(err, careplan.ErrStudentDataRequestNotPending) {
			return parentrequests.ErrDecisionRace
		}
		return err
	}
	err := c.excused.ApproveExcusedBulk(ctx, ref.ID, input.Reason, input.ReviewerID, ref.ExpectedVersion)
	if errors.Is(err, absencerecords.ErrExcusedRequestNotPending) {
		return parentrequests.ErrDecisionRace
	}
	return err
}

// authorizeBulkParentRequestKinds lets an absence reviewer approve absences
// only; everything else is a users:update write (#2232).
func authorizeBulkParentRequestKinds(rights ports.ParentRequestRights, refs []parentrequests.Ref) error {
	for _, ref := range refs {
		if ref.Kind == parentrequests.KindMasterData && !rights.WriteQueues {
			return parentrequests.ErrForbidden
		}
		if ref.Kind == parentrequests.KindExcused && !rights.Absences {
			return parentrequests.ErrForbidden
		}
	}
	return nil
}

func validateBulkParentRequestInput(input parentrequests.BulkApproveInput) error {
	if len(input.Requests) < 2 || len(input.Requests) > maxBulkParentRequests || input.ReviewerID <= 0 {
		return parentrequests.ErrInvalidBulkRequest
	}
	// The shared reason is mandatory only while the school's policy asks staff
	// for one (#2267, story 28).
	if input.ReasonRequired && strings.TrimSpace(input.Reason) == "" {
		return parentrequests.ErrReasonRequired
	}
	seen := make(map[parentrequests.Ref]struct{}, len(input.Requests))
	for _, ref := range input.Requests {
		if ref.ID <= 0 || ref.ExpectedVersion == "" {
			return parentrequests.ErrInvalidBulkRequest
		}
		if ref.Kind != parentrequests.KindMasterData && ref.Kind != parentrequests.KindExcused {
			return parentrequests.ErrBulkIneligible
		}
		key := parentrequests.Ref{Kind: ref.Kind, ID: ref.ID}
		if _, duplicate := seen[key]; duplicate {
			return parentrequests.ErrInvalidBulkRequest
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (c *ParentRequestCoordinator) loadBulkParentRequests(
	ctx context.Context, refs []parentrequests.Ref,
) (map[int64]*careplan.MasterDataReviewItem, map[int64]ports.ExcusedBulkCandidate, error) {
	masters := make(map[int64]*careplan.MasterDataReviewItem)
	excused := make(map[int64]ports.ExcusedBulkCandidate)
	for _, ref := range refs {
		if err := c.loadBulkParentRequest(ctx, ref, masters, excused); err != nil {
			return nil, nil, err
		}
	}
	return masters, excused, nil
}

func (c *ParentRequestCoordinator) loadBulkParentRequest(
	ctx context.Context, ref parentrequests.Ref,
	masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate,
) error {
	if ref.Kind == parentrequests.KindMasterData {
		row, err := c.masterData.GetBulkCandidate(ctx, ref.ID)
		if errors.Is(err, masterdatarequests.ErrReviewNotFound) {
			return parentrequests.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("parent requests: load master data candidate: %w", err)
		}
		masters[ref.ID] = row
		return nil
	}
	row, err := c.excused.GetExcusedBulkCandidate(ctx, ref.ID)
	if err != nil {
		return fmt.Errorf("parent requests: load absence candidate: %w", err)
	}
	if row != nil {
		excused[ref.ID] = *row
	}
	return nil
}

func validateBulkParentRequestVersions(refs []parentrequests.Ref, masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate) error {
	for _, ref := range refs {
		version, err := bulkCandidateVersion(ref, masters, excused)
		if err != nil {
			return err
		}
		if version != ref.ExpectedVersion {
			return parentrequests.ErrStale
		}
	}
	return nil
}

func bulkCandidateVersion(ref parentrequests.Ref, masters map[int64]*careplan.MasterDataReviewItem, excused map[int64]ports.ExcusedBulkCandidate) (string, error) {
	if ref.Kind == parentrequests.KindMasterData {
		row := masters[ref.ID]
		if row == nil {
			return "", parentrequests.ErrNotFound
		}
		if !row.BulkEligible {
			return "", parentrequests.ErrBulkIneligible
		}
		return careplan.ParentRequestVersion(row.Request.UpdatedAt), nil
	}
	row, found := excused[ref.ID]
	if !found {
		return "", parentrequests.ErrNotFound
	}
	if !row.Eligible {
		return "", parentrequests.ErrBulkIneligible
	}
	return careplan.ParentRequestVersion(row.UpdatedAt), nil
}
