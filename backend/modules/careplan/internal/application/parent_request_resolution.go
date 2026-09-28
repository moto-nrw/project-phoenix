package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
)

// Conflict resolution (#2267, stories 6-10). Two open requests of one child
// that would write the same thing cannot be decided one after the other: the
// second decision silently overwrites the first, and the family ends up with a
// result nobody chose. ResolveConflict closes a whole conflict group at once —
// at most one request is approved, every other one in the group is rejected,
// and either all of it happens or none of it.
//
// Three outcomes, exactly one per call:
//   - ChosenRequestID: that request is approved, the rest rejected.
//   - StaffValue: every request is rejected and the staff member's own value is
//     written through the queue's normal staff write path.
//   - None: every request is rejected and nothing is written.
//
// A resolve therefore ALWAYS rejects at least one request, and a rejection
// always carries a reason whatever operations.parent_request_reason_policy
// says. So the reason is unconditionally required here and the coordinator
// needs no settings of its own.

// maxConflictRequests bounds one resolve command. A conflict group is small by
// construction (the requests of ONE child touching ONE key); the cap only
// stops a malicious client from locking half the queue in one transaction.
const maxConflictRequests = 20

// lockedConflictGroup is the validated, locked group: the child it belongs to
// and the anchor the staff-value ledger entry is filed against.
type lockedConflictGroup struct {
	studentID int64
	// anchorID is the group's lowest request id, and anchorVersion that row's
	// version. A staff-entered result belongs to the group rather than to any
	// one request, so it is recorded once against a deterministic member.
	anchorID      int64
	anchorVersion time.Time
}

// ResolveConflict closes one conflict group atomically. Every failure marks
// the ambient tenant transaction for rollback, so a half-resolved group — some
// requests rejected, the winner still pending — can never be committed.
func (c *ParentRequestCoordinator) ResolveConflict(ctx context.Context, input parentrequests.ResolveConflictInput) error {
	if err := validateConflictResolution(input); err != nil {
		return c.failed(ctx, err)
	}
	port, err := c.conflictPort(ctx, input.Kind)
	if err != nil {
		return c.failed(ctx, err)
	}
	group, err := c.prepareConflictGroup(ctx, port, input)
	if err != nil {
		return c.failed(ctx, err)
	}
	if err := c.applyConflictResolution(ctx, port, input, group); err != nil {
		return c.failed(ctx, err)
	}
	return nil
}

// prepareConflictGroup loads, version-checks and locks the whole group, and
// returns the child every request in it belongs to. Locking happens in a fixed
// id order so two staff members resolving overlapping groups queue up instead
// of deadlocking.
func (c *ParentRequestCoordinator) prepareConflictGroup(
	ctx context.Context, port ports.ConflictPort, input parentrequests.ResolveConflictInput,
) (lockedConflictGroup, error) {
	candidates, studentID, err := loadConflictCandidates(ctx, port, input)
	if err != nil {
		return lockedConflictGroup{}, err
	}
	for i, requestID := range input.RequestIDs {
		if careplan.ParentRequestVersion(candidates[requestID].UpdatedAt) != input.ExpectedVersions[i] {
			return lockedConflictGroup{}, parentrequests.ErrStale
		}
	}
	ordered := append([]int64(nil), input.RequestIDs...)
	slices.Sort(ordered)
	for _, requestID := range ordered {
		if lockErr := port.LockConflictRequest(ctx, requestID); lockErr != nil {
			if isParentRequestRace(lockErr) {
				return lockedConflictGroup{}, fmt.Errorf("%w: %s %d changed before locking", parentrequests.ErrStale, input.Kind, requestID)
			}
			return lockedConflictGroup{}, fmt.Errorf("parent requests: lock %s %d: %w", input.Kind, requestID, lockErr)
		}
	}
	if c.masterData != nil {
		if err := c.masterData.LockBulkStudents(ctx, []int64{studentID}); err != nil {
			return lockedConflictGroup{}, fmt.Errorf("parent requests: lock conflict student: %w", err)
		}
	}
	return lockedConflictGroup{studentID: studentID, anchorID: ordered[0], anchorVersion: candidates[ordered[0]].UpdatedAt}, nil
}

// loadConflictCandidates reads every request of the group and proves they all
// belong to ONE child. A group spanning two children is not a conflict, and
// resolving it would reject a request the staff member never saw.
func loadConflictCandidates(
	ctx context.Context, port ports.ConflictPort, input parentrequests.ResolveConflictInput,
) (map[int64]*ports.ConflictCandidate, int64, error) {
	candidates := make(map[int64]*ports.ConflictCandidate, len(input.RequestIDs))
	var studentID int64
	for _, requestID := range input.RequestIDs {
		candidate, err := port.ConflictCandidate(ctx, requestID)
		if err != nil {
			if isParentRequestRace(err) {
				return nil, 0, parentrequests.ErrNotFound
			}
			return nil, 0, fmt.Errorf("parent requests: load %s candidate %d: %w", input.Kind, requestID, err)
		}
		if candidate == nil || candidate.StudentID <= 0 {
			return nil, 0, parentrequests.ErrNotFound
		}
		if studentID == 0 {
			studentID = candidate.StudentID
		} else if candidate.StudentID != studentID {
			return nil, 0, parentrequests.ErrInvalidConflictResolution
		}
		candidates[requestID] = candidate
	}
	return candidates, studentID, nil
}

// applyConflictResolution writes the verdicts and, when the staff member typed
// their own result, that value. The winner is decided FIRST: if approving it
// fails on a queue guard, nothing has been rejected yet.
func (c *ParentRequestCoordinator) applyConflictResolution(
	ctx context.Context, port ports.ConflictPort, input parentrequests.ResolveConflictInput, group lockedConflictGroup,
) error {
	if input.ChosenRequestID > 0 {
		if err := decideConflictRequest(ctx, port, input, input.ChosenRequestID, true); err != nil {
			return err
		}
	}
	for _, requestID := range input.RequestIDs {
		if requestID == input.ChosenRequestID {
			continue
		}
		if err := decideConflictRequest(ctx, port, input, requestID, false); err != nil {
			return err
		}
	}
	if input.StaffValue == nil {
		return nil
	}
	value, err := decodeStaffValue(input.StaffValue)
	if err != nil {
		return err
	}
	err = port.WriteStaffValue(ctx, ports.StaffValueWrite{
		StudentID: group.studentID, RequestIDs: input.RequestIDs, ConflictKey: input.ConflictKey,
		ReviewerID: input.ReviewerID, ActorRole: input.ActorRole,
		Reason: input.Reason, Value: value,
	})
	if errors.Is(err, parentrequests.ErrStaffValueUnsupported) {
		return parentrequests.ErrStaffValueUnsupported
	}
	if err != nil {
		return fmt.Errorf("parent requests: write staff value for %s: %w", input.Kind, err)
	}
	return c.recordStaffValueDecision(ctx, input, value, group)
}

// decodeStaffValue reads the typed result as the JSON object every kind's
// payload is. Anything else is a malformed command.
func decodeStaffValue(raw json.RawMessage) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil || value == nil {
		return nil, parentrequests.ErrInvalidConflictResolution
	}
	return value, nil
}

// recordStaffValueDecision files the ONE ledger entry the queues cannot file
// for the resolver: the result the staff member typed themselves. The verdicts
// on the requests are recorded by each queue's own decision.
func (c *ParentRequestCoordinator) recordStaffValueDecision(
	ctx context.Context, input parentrequests.ResolveConflictInput, value map[string]any, group lockedConflictGroup,
) error {
	if c.ledger == nil {
		return nil
	}
	requestIDs := append(make([]int64, 0, len(input.RequestIDs)), input.RequestIDs...)
	return c.ledger.Record(ctx, ports.RequestLedgerEntry{
		StudentID:      group.studentID,
		RequestType:    string(input.Kind),
		RequestID:      group.anchorID,
		EventType:      careplan.ParentRequestEventDecided,
		ActorAccountID: input.ReviewerID,
		UpdatedAt:      group.anchorVersion,
		Payload: map[string]any{
			"staff_value":  value,
			"conflict_key": input.ConflictKey,
			"request_ids":  requestIDs,
		},
	})
}

func decideConflictRequest(ctx context.Context, port ports.ConflictPort, input parentrequests.ResolveConflictInput, requestID int64, approve bool) error {
	err := port.DecideConflictRequest(ctx, ports.ConflictDecision{
		RequestID: requestID, Approve: approve, Reason: input.Reason,
		ReviewerID: input.ReviewerID, ActorRole: input.ActorRole,
		ExpectedVersion: conflictVersionFor(input, requestID),
	})
	if err == nil {
		return nil
	}
	if isParentRequestRace(err) {
		return fmt.Errorf("%w: %s %d changed during resolution", parentrequests.ErrStale, input.Kind, requestID)
	}
	return fmt.Errorf("parent requests: resolve %s %d: %w", input.Kind, requestID, err)
}

func conflictVersionFor(input parentrequests.ResolveConflictInput, requestID int64) string {
	for i, id := range input.RequestIDs {
		if id == requestID {
			return input.ExpectedVersions[i]
		}
	}
	return ""
}

// conflictPort resolves the kind to its queue and re-checks the caller's
// rights for it. The route gate only decides who may knock: everything is a
// users:update write except an absence, which a users:absence holder may
// decide too (#2232).
func (c *ParentRequestCoordinator) conflictPort(ctx context.Context, kind parentrequests.Kind) (ports.ConflictPort, error) {
	rights := c.rights(ctx)
	if !rights.WriteQueues && (kind != parentrequests.KindExcused || !rights.Absences) {
		return nil, parentrequests.ErrForbidden
	}
	port := c.conflictPorts[kind]
	if port == nil {
		return nil, parentrequests.ErrConflictKindUnsupported
	}
	return port, nil
}

func validateConflictResolution(input parentrequests.ResolveConflictInput) error {
	if err := validateConflictRequestList(input); err != nil {
		return err
	}
	// A resolve always rejects at least one request, and a rejection always
	// states why — the reason is required whatever the school's policy says.
	if strings.TrimSpace(input.Reason) == "" {
		return parentrequests.ErrReasonRequired
	}
	if input.ReviewerID <= 0 {
		return parentrequests.ErrInvalidConflictResolution
	}
	outcomes := 0
	if input.ChosenRequestID > 0 {
		outcomes++
		if !slices.Contains(input.RequestIDs, input.ChosenRequestID) {
			return parentrequests.ErrInvalidConflictResolution
		}
	}
	if input.StaffValue != nil {
		outcomes++
		if _, err := decodeStaffValue(input.StaffValue); err != nil {
			return err
		}
	}
	if input.None {
		outcomes++
	}
	if outcomes != 1 {
		return parentrequests.ErrInvalidConflictResolution
	}
	return nil
}

func validateConflictRequestList(input parentrequests.ResolveConflictInput) error {
	if len(input.RequestIDs) < 2 || len(input.RequestIDs) > maxConflictRequests {
		return parentrequests.ErrInvalidConflictResolution
	}
	if len(input.ExpectedVersions) != len(input.RequestIDs) {
		return parentrequests.ErrInvalidConflictResolution
	}
	seen := make(map[int64]struct{}, len(input.RequestIDs))
	for i, requestID := range input.RequestIDs {
		if requestID <= 0 || strings.TrimSpace(input.ExpectedVersions[i]) == "" {
			return parentrequests.ErrInvalidConflictResolution
		}
		if _, duplicate := seen[requestID]; duplicate {
			return parentrequests.ErrInvalidConflictResolution
		}
		seen[requestID] = struct{}{}
	}
	return nil
}
