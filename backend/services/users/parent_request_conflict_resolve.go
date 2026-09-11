package users

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Conflict resolution (#2267, stories 6-10). Two open requests of one child
// that would write the same thing cannot be decided one after the other: the
// second decision silently overwrites the first, and the family ends up with a
// result nobody chose. ResolveConflict is the single command that closes a
// whole conflict group at once — at most one request is approved, every other
// one in the group is rejected, and either all of it happens or none of it.
//
// Three outcomes, exactly one per call:
//   - ChosenRequestID: that request is approved, the rest rejected.
//   - StaffValue: every request is rejected and the staff member's own value is
//     written through the domain's normal staff write path.
//   - None: every request is rejected and nothing is written.
//
// A resolve therefore ALWAYS rejects at least one request, and a rejection
// always carries a reason whatever operations.parent_request_reason_policy
// says (see ReasonRequiredFor). So the reason is unconditionally required here
// and the coordinator needs no settings dependency of its own.

// The request kinds the resolver knows beyond the two bulk-approval kinds.
// care_schedule and pickup_change share one service but are separate kinds on
// the wire, because they conflict on different things (a weekday of the weekly
// plan vs. one calendar day's pickup time).
const (
	ParentRequestKindCareSchedule ParentRequestKind = "care_schedule"
	ParentRequestKindPickupChange ParentRequestKind = "pickup_change"
	ParentRequestKindOffering     ParentRequestKind = "offering"
)

var (
	// ErrInvalidConflictResolution means the command itself is malformed:
	// fewer than two requests, mismatched version list, no outcome or more
	// than one, a missing reason, or requests belonging to different children.
	ErrInvalidConflictResolution = errors.New("parent requests: invalid conflict resolution")
	// ErrConflictKindUnsupported means no domain port is wired for the kind.
	ErrConflictKindUnsupported = errors.New("parent requests: conflict resolution is not supported for this request kind")
	// ErrStaffValueUnsupported means the domain accepts no staff-entered
	// result for this conflict.
	ErrStaffValueUnsupported = errors.New("parent requests: this request kind accepts no staff-entered value")
)

// maxConflictRequests bounds one resolve command. A conflict group is small by
// construction (the requests of ONE child touching ONE key); the cap only
// stops a malicious client from locking half the queue in one transaction.
const maxConflictRequests = 20

// ParentRequestConflictCandidate is the minimum the coordinator needs to
// validate a group before touching it. The payload stays owned by the domain.
type ParentRequestConflictCandidate struct {
	StudentID int64
	UpdatedAt time.Time
}

// ParentRequestConflictDecision is one verdict inside a resolve command.
type ParentRequestConflictDecision struct {
	RequestID int64
	Approve   bool
	Reason    string
	// ReviewerID is the acting staff ACCOUNT id; ActorRole their roles as the
	// audit log records them. Domains that keep an actor role in their own
	// audit trail need both.
	ReviewerID      int64
	ActorRole       string
	ExpectedVersion string
}

// ParentRequestStaffValueWrite is the staff member's own result, written
// through the domain's existing staff write path so it passes exactly the
// validation, audit and notification a value typed on the child's own screen
// would pass. Value is the domain's payload shape, unread by the coordinator.
type ParentRequestStaffValueWrite struct {
	StudentID int64
	// RequestIDs are the requests just rejected, in the order the client sent
	// them. The domain reads the group's SCOPE from them — which field, which
	// days — so the staff value carries only the value itself and can never be
	// typed against a different scope than the wishes it replaces.
	RequestIDs []int64
	// ConflictKey is the group's key exactly as the list emitted it (see
	// ParentRequestConflictKeys). One request can occupy several keys — a
	// weekly plan touching three weekdays — so where the request alone does
	// not pin the scope, the key is what says WHICH part of it the group is
	// about. Empty where the domain does not need it.
	ConflictKey string
	ReviewerID  int64
	ActorRole   string
	Reason      string
	Value       map[string]any
}

// ParentRequestConflictPort is what one domain contributes to the resolve
// command. The four implementations differ only in payload, so they share one
// interface and the coordinator branches on nothing.
//
// WriteStaffValue returns ErrStaffValueUnsupported when the domain accepts no
// typed result; the coordinator turns that into a 400 rather than rejecting
// the group and then writing nothing.
type ParentRequestConflictPort interface {
	ConflictCandidate(ctx context.Context, requestID int64) (*ParentRequestConflictCandidate, error)
	LockConflictRequest(ctx context.Context, requestID int64) error
	DecideConflictRequest(ctx context.Context, decision ParentRequestConflictDecision) error
	WriteStaffValue(ctx context.Context, write ParentRequestStaffValueWrite) error
}

// ResolveConflictInput is one resolve command. RequestIDs and ExpectedVersions
// are positional pairs: the client sends back exactly what the list gave it,
// so a group edited in the meantime is refused as a whole.
type ResolveConflictInput struct {
	Kind             ParentRequestKind
	RequestIDs       []int64
	ExpectedVersions []string
	// ChosenRequestID is the request to approve. Zero means none is approved.
	ChosenRequestID int64
	// StaffValue is the staff member's own result. Nil means none was typed.
	StaffValue map[string]any
	// None records that the staff member deliberately chose "keine Änderung".
	None bool
	// ConflictKey is the group's key from the list. Required only where the
	// requests alone do not pin the scope a staff value writes against.
	ConflictKey string
	Reason      string
	ReviewerID  int64
	// ActorRole is the reviewer's roles, passed straight through to the audit
	// trails that record one.
	ActorRole string
}

// ParentRequestConflictService is the resolve command's port for the handler.
type ParentRequestConflictService interface {
	ResolveConflict(context.Context, ResolveConflictInput) error
}

// SetMasterDataConflictPort wires the Stammdaten domain into the resolver.
func (s *ParentRequestCoordinator) SetMasterDataConflictPort(port ParentRequestConflictPort) {
	s.setConflictPort(port, ParentRequestKindMasterData)
}

// SetExcusedConflictPort wires the Abwesenheiten domain into the resolver.
func (s *ParentRequestCoordinator) SetExcusedConflictPort(port ParentRequestConflictPort) {
	s.setConflictPort(port, ParentRequestKindExcused)
}

// SetCareConflictPort wires the Betreuungszeiten domain into the resolver. One
// service owns both kinds: the weekly plan and the single-day pickup change.
func (s *ParentRequestCoordinator) SetCareConflictPort(port ParentRequestConflictPort) {
	s.setConflictPort(port, ParentRequestKindCareSchedule, ParentRequestKindPickupChange)
}

// SetOfferingConflictPort wires the Angebote domain into the resolver.
func (s *ParentRequestCoordinator) SetOfferingConflictPort(port ParentRequestConflictPort) {
	s.setConflictPort(port, ParentRequestKindOffering)
}

// SetEventRecorder wires the parent-request ledger. The resolver records ONLY
// the staff-entered result: every verdict it takes on a request goes through
// that domain's own Decide, which writes its own `decided` event, and a second
// one from here would show the family two decisions where there was one.
func (s *ParentRequestCoordinator) SetEventRecorder(recorder ParentRequestEventRecorder) {
	s.events = recorder
}

func (s *ParentRequestCoordinator) setConflictPort(port ParentRequestConflictPort, kinds ...ParentRequestKind) {
	if port == nil {
		return
	}
	if s.conflictPorts == nil {
		s.conflictPorts = make(map[ParentRequestKind]ParentRequestConflictPort, len(kinds))
	}
	for _, kind := range kinds {
		s.conflictPorts[kind] = port
	}
}

// ResolveConflict closes one conflict group atomically. Every failure marks
// the ambient tenant transaction for rollback, so a half-resolved group — some
// requests rejected, the winner still pending — can never be committed.
func (s *ParentRequestCoordinator) ResolveConflict(ctx context.Context, input ResolveConflictInput) error {
	if err := validateConflictResolution(input); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	port, err := s.conflictPort(ctx, input.Kind)
	if err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	group, err := s.prepareConflictGroup(ctx, port, input)
	if err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	if err := s.applyConflictResolution(ctx, port, input, group); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	return nil
}

// lockedConflictGroup is the validated, locked group: the child it belongs to and
// the anchor the staff-value ledger entry is filed against.
type lockedConflictGroup struct {
	studentID int64
	// anchorID is the group's lowest request id, and anchorVersion that row's
	// version. A staff-entered result belongs to the group rather than to any
	// one request, so it is recorded once against a deterministic member
	// instead of once per request.
	anchorID      int64
	anchorVersion time.Time
}

// prepareConflictGroup loads, version-checks and locks the whole group, and
// returns the child every request in it belongs to. Locking happens in a fixed
// id order so two staff members resolving overlapping groups queue up instead
// of deadlocking.
func (s *ParentRequestCoordinator) prepareConflictGroup(
	ctx context.Context, port ParentRequestConflictPort, input ResolveConflictInput,
) (lockedConflictGroup, error) {
	candidates, studentID, err := s.loadConflictCandidates(ctx, port, input)
	if err != nil {
		return lockedConflictGroup{}, err
	}
	for i, requestID := range input.RequestIDs {
		if ParentRequestVersion(candidates[requestID].UpdatedAt) != input.ExpectedVersions[i] {
			return lockedConflictGroup{}, ErrParentRequestStale
		}
	}
	ordered := sortedConflictIDs(input.RequestIDs)
	for _, requestID := range ordered {
		if lockErr := port.LockConflictRequest(ctx, requestID); lockErr != nil {
			if isBulkRequestRace(lockErr) {
				return lockedConflictGroup{}, fmt.Errorf("%w: %s %d changed before locking", ErrParentRequestStale, input.Kind, requestID)
			}
			return lockedConflictGroup{}, fmt.Errorf("parent requests: lock %s %d: %w", input.Kind, requestID, lockErr)
		}
	}
	if s.masterData != nil {
		if err := s.masterData.LockBulkStudents(ctx, []int64{studentID}); err != nil {
			return lockedConflictGroup{}, fmt.Errorf("parent requests: lock conflict student: %w", err)
		}
	}
	return lockedConflictGroup{
		studentID:     studentID,
		anchorID:      ordered[0],
		anchorVersion: candidates[ordered[0]].UpdatedAt,
	}, nil
}

// loadConflictCandidates reads every request of the group and proves they all
// belong to ONE child. A group spanning two children is not a conflict, and
// resolving it would reject a request the staff member never saw.
func (s *ParentRequestCoordinator) loadConflictCandidates(
	ctx context.Context, port ParentRequestConflictPort, input ResolveConflictInput,
) (map[int64]*ParentRequestConflictCandidate, int64, error) {
	candidates := make(map[int64]*ParentRequestConflictCandidate, len(input.RequestIDs))
	var studentID int64
	for _, requestID := range input.RequestIDs {
		candidate, err := port.ConflictCandidate(ctx, requestID)
		if err != nil {
			if isBulkRequestRace(err) {
				return nil, 0, ErrParentRequestNotFound
			}
			return nil, 0, fmt.Errorf("parent requests: load %s candidate %d: %w", input.Kind, requestID, err)
		}
		if candidate == nil || candidate.StudentID <= 0 {
			return nil, 0, ErrParentRequestNotFound
		}
		if studentID == 0 {
			studentID = candidate.StudentID
		} else if candidate.StudentID != studentID {
			return nil, 0, ErrInvalidConflictResolution
		}
		candidates[requestID] = candidate
	}
	return candidates, studentID, nil
}

// applyConflictResolution writes the verdicts and, when the staff member typed
// their own result, that value. The winner is decided FIRST: if approving it
// fails on a domain guard, nothing has been rejected yet.
func (s *ParentRequestCoordinator) applyConflictResolution(
	ctx context.Context, port ParentRequestConflictPort, input ResolveConflictInput, group lockedConflictGroup,
) error {
	if input.ChosenRequestID > 0 {
		if err := s.decideConflictRequest(ctx, port, input, input.ChosenRequestID, true); err != nil {
			return err
		}
	}
	for _, requestID := range input.RequestIDs {
		if requestID == input.ChosenRequestID {
			continue
		}
		if err := s.decideConflictRequest(ctx, port, input, requestID, false); err != nil {
			return err
		}
	}
	if input.StaffValue == nil {
		return nil
	}
	err := port.WriteStaffValue(ctx, ParentRequestStaffValueWrite{
		StudentID: group.studentID, RequestIDs: input.RequestIDs, ConflictKey: input.ConflictKey,
		ReviewerID: input.ReviewerID, ActorRole: input.ActorRole,
		Reason: input.Reason, Value: input.StaffValue,
	})
	if errors.Is(err, ErrStaffValueUnsupported) {
		return ErrStaffValueUnsupported
	}
	if err != nil {
		return fmt.Errorf("parent requests: write staff value for %s: %w", input.Kind, err)
	}
	return s.recordStaffValueDecision(ctx, input, group)
}

// recordStaffValueDecision files the ONE ledger entry the domains cannot file
// for the resolver: the result the staff member typed themselves. The verdicts
// on the requests are recorded by each domain's own Decide, so nothing here
// repeats them.
func (s *ParentRequestCoordinator) recordStaffValueDecision(
	ctx context.Context, input ResolveConflictInput, group lockedConflictGroup,
) error {
	requestIDs := make([]int64, 0, len(input.RequestIDs))
	requestIDs = append(requestIDs, input.RequestIDs...)
	return RecordParentRequestEvent(ctx, s.events, ParentRequestEventInput{
		StudentID:      group.studentID,
		RequestType:    string(input.Kind),
		RequestID:      group.anchorID,
		EventType:      userModels.ParentRequestEventDecided,
		ActorAccountID: input.ReviewerID,
		UpdatedAt:      group.anchorVersion,
		Payload: map[string]any{
			"staff_value":  input.StaffValue,
			"conflict_key": input.ConflictKey,
			"request_ids":  requestIDs,
		},
	})
}

func (s *ParentRequestCoordinator) decideConflictRequest(
	ctx context.Context, port ParentRequestConflictPort,
	input ResolveConflictInput, requestID int64, approve bool,
) error {
	err := port.DecideConflictRequest(ctx, ParentRequestConflictDecision{
		RequestID: requestID, Approve: approve, Reason: input.Reason,
		ReviewerID: input.ReviewerID, ActorRole: input.ActorRole,
		ExpectedVersion: conflictVersionFor(input, requestID),
	})
	if err == nil {
		return nil
	}
	if isBulkRequestRace(err) {
		return fmt.Errorf("%w: %s %d changed during resolution", ErrParentRequestStale, input.Kind, requestID)
	}
	return fmt.Errorf("parent requests: resolve %s %d: %w", input.Kind, requestID, err)
}

func conflictVersionFor(input ResolveConflictInput, requestID int64) string {
	for i, id := range input.RequestIDs {
		if id == requestID {
			return input.ExpectedVersions[i]
		}
	}
	return ""
}

func sortedConflictIDs(ids []int64) []int64 {
	ordered := append([]int64(nil), ids...)
	slices.Sort(ordered)
	return ordered
}

// conflictPort resolves the kind to its domain and re-checks the caller's
// permission for it. The route gate only decides who may knock.
func (s *ParentRequestCoordinator) conflictPort(
	ctx context.Context, kind ParentRequestKind,
) (ParentRequestConflictPort, error) {
	if err := authorizeConflictKind(ctx, kind); err != nil {
		return nil, err
	}
	port := s.conflictPorts[kind]
	if port == nil {
		return nil, ErrConflictKindUnsupported
	}
	return port, nil
}

// authorizeConflictKind mirrors authorizeBulkParentRequestKinds: everything is
// a users:update write except an absence, which a users:absence holder may
// decide too (#2232).
func authorizeConflictKind(ctx context.Context, kind ParentRequestKind) error {
	granted := jwt.PermissionsFromCtx(ctx)
	if authorize.HasPermission(permissions.UsersUpdate, granted) {
		return nil
	}
	if kind == ParentRequestKindExcused && authorize.HasPermission(permissions.UsersAbsence, granted) {
		return nil
	}
	return ErrParentRequestForbidden
}

func validateConflictResolution(input ResolveConflictInput) error {
	if err := validateConflictRequestList(input); err != nil {
		return err
	}
	// A resolve always rejects at least one request, and a rejection always
	// states why — the reason is required whatever the school's reason policy
	// says.
	if strings.TrimSpace(input.Reason) == "" {
		return ErrParentRequestReasonRequired
	}
	if input.ReviewerID <= 0 {
		return ErrInvalidConflictResolution
	}
	outcomes := 0
	if input.ChosenRequestID > 0 {
		outcomes++
		if !slices.Contains(input.RequestIDs, input.ChosenRequestID) {
			return ErrInvalidConflictResolution
		}
	}
	if input.StaffValue != nil {
		outcomes++
	}
	if input.None {
		outcomes++
	}
	if outcomes != 1 {
		return ErrInvalidConflictResolution
	}
	return nil
}

func validateConflictRequestList(input ResolveConflictInput) error {
	if len(input.RequestIDs) < 2 || len(input.RequestIDs) > maxConflictRequests {
		return ErrInvalidConflictResolution
	}
	if len(input.ExpectedVersions) != len(input.RequestIDs) {
		return ErrInvalidConflictResolution
	}
	seen := make(map[int64]struct{}, len(input.RequestIDs))
	for i, requestID := range input.RequestIDs {
		if requestID <= 0 || strings.TrimSpace(input.ExpectedVersions[i]) == "" {
			return ErrInvalidConflictResolution
		}
		if _, duplicate := seen[requestID]; duplicate {
			return ErrInvalidConflictResolution
		}
		seen[requestID] = struct{}{}
	}
	return nil
}
