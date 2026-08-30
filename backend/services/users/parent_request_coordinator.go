package users

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type ParentRequestKind string

const (
	ParentRequestKindMasterData ParentRequestKind = "master_data"
	ParentRequestKindExcused    ParentRequestKind = "excused"
)

var (
	ErrInvalidBulkRequest        = errors.New("parent requests: invalid bulk approval")
	ErrBulkIneligible            = errors.New("parent requests: request is not eligible for bulk approval")
	ErrParentRequestNotFound     = errors.New("parent requests: pending request not found")
	ErrParentRequestStale        = errors.New("parent requests: request version is stale")
	ErrParentRequestDecisionRace = errors.New("parent requests: request changed during approval")
	ErrParentRequestForbidden    = errors.New("parent requests: request kind is not permitted")
)

const maxBulkParentRequests = 50

type ParentRequestRef struct {
	Kind            ParentRequestKind `json:"kind"`
	ID              int64             `json:"id"`
	ExpectedVersion string            `json:"expected_version"`
}

type BulkApproveParentRequestsInput struct {
	Requests []ParentRequestRef
	Reason   string
	// ReasonRequired says the school's reason policy
	// (operations.parent_request_reason_policy) asks the deciding staff member
	// for a reason. A bulk command only ever approves, so this is the only
	// gate on its reason (#2267, story 28).
	ReasonRequired bool
	ReviewerID     int64
}

type ParentRequestBulkService interface {
	BulkApprove(context.Context, BulkApproveParentRequestsInput) error
}

// ExcusedBulkCandidate is the absence domain's minimal contribution to the
// cross-request command. Its payload stays owned by the absence service.
type ExcusedBulkCandidate struct {
	ID        int64
	StudentID int64
	UpdatedAt time.Time
	Eligible  bool
}

type ExcusedBulkReviewPort interface {
	GetExcusedBulkCandidate(context.Context, int64) (*ExcusedBulkCandidate, error)
	LockExcusedBulkRequest(context.Context, int64) error
	ApproveExcusedBulk(context.Context, int64, string, int64, string) error
}

type MasterDataBulkReviewPort interface {
	GetBulkCandidate(context.Context, int64) (*MasterDataReviewItem, error)
	LockBulkRequest(context.Context, int64) error
	LockBulkStudents(context.Context, []int64) error
	Decide(context.Context, MasterDataReviewDecideInput) (*MasterDataReviewItem, error)
}

// ParentRequestCoordinator owns invariants spanning request kinds. Each
// domain service still loads and applies its own payload.
type ParentRequestCoordinator struct {
	masterData MasterDataBulkReviewPort
	excused    ExcusedBulkReviewPort
	// conflictPorts carry the resolve command (#2267, stories 6-10). They are
	// injected by setter rather than by constructor parameter so that adding a
	// domain to the conflict resolver never rewrites the bulk-approval
	// constructor. See parent_request_conflict_resolve.go.
	conflictPorts map[ParentRequestKind]ParentRequestConflictPort
	// events records the ONE thing the domain services cannot record for the
	// resolver: the staff member's own result. Every verdict on a request goes
	// through a domain Decide, which writes its own `decided` event. Injected
	// by setter for the same reason as the ports.
	events ParentRequestEventRecorder
}

func NewParentRequestCoordinator(
	masterData MasterDataBulkReviewPort,
	excused ExcusedBulkReviewPort,
) *ParentRequestCoordinator {
	return &ParentRequestCoordinator{masterData: masterData, excused: excused}
}

func ParentRequestVersion(updatedAt time.Time) string {
	if updatedAt.IsZero() {
		return ""
	}
	return updatedAt.UTC().Format(time.RFC3339Nano)
}

func AbsenceBulkEligible(dates []timezone.Date, today timezone.Date) bool {
	if len(dates) == 0 {
		return false
	}
	for _, date := range dates {
		if date.Before(today) {
			return false
		}
	}
	return true
}

func (s *ParentRequestCoordinator) BulkApprove(ctx context.Context, input BulkApproveParentRequestsInput) error {
	if err := validateBulkParentRequestInput(input); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	if err := authorizeBulkParentRequestKinds(ctx, input.Requests); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	input.Requests = sortedParentRequestRefs(input.Requests)
	masters, excused, err := s.loadBulkParentRequests(ctx, input.Requests)
	if err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	if err := validateBulkParentRequestVersions(input.Requests, masters, excused); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	if err := s.lockBulkParentRequests(ctx, input.Requests); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	input.Requests = sortedBulkRefsByStudent(input.Requests, masters, excused)
	if err := s.masterData.LockBulkStudents(ctx, bulkStudentIDs(input.Requests, masters, excused)); err != nil {
		tenant.MarkRollback(ctx)
		return fmt.Errorf("parent requests: lock bulk students: %w", err)
	}
	if err := s.applyBulkParentRequests(ctx, input); err != nil {
		tenant.MarkRollback(ctx)
		return err
	}
	return nil
}

func (s *ParentRequestCoordinator) lockBulkParentRequests(ctx context.Context, refs []ParentRequestRef) error {
	for _, ref := range refs {
		var err error
		if ref.Kind == ParentRequestKindMasterData {
			err = s.masterData.LockBulkRequest(ctx, ref.ID)
		} else {
			err = s.excused.LockExcusedBulkRequest(ctx, ref.ID)
		}
		if err != nil {
			if isBulkRequestRace(err) {
				return fmt.Errorf("%w: %s %d changed before locking", ErrParentRequestStale, ref.Kind, ref.ID)
			}
			return fmt.Errorf("parent requests: lock %s %d: %w", ref.Kind, ref.ID, err)
		}
	}
	return nil
}

func isBulkRequestRace(err error) bool {
	return errors.Is(err, ErrReviewNotFound) ||
		errors.Is(err, ErrReviewNotPending) ||
		errors.Is(err, userModels.ErrChangeRequestNotFound) ||
		errors.Is(err, userModels.ErrChangeRequestNotPending) ||
		errors.Is(err, activeModels.ErrExcusedRequestNotFound) ||
		errors.Is(err, activeModels.ErrExcusedRequestNotPending)
}

func bulkStudentIDs(
	refs []ParentRequestRef,
	masters map[int64]*MasterDataReviewItem,
	excused map[int64]ExcusedBulkCandidate,
) []int64 {
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

func sortedBulkRefsByStudent(
	refs []ParentRequestRef,
	masters map[int64]*MasterDataReviewItem,
	excused map[int64]ExcusedBulkCandidate,
) []ParentRequestRef {
	ordered := append([]ParentRequestRef(nil), refs...)
	slices.SortFunc(ordered, func(left, right ParentRequestRef) int {
		if byStudent := cmp.Compare(bulkRefStudentID(left, masters, excused), bulkRefStudentID(right, masters, excused)); byStudent != 0 {
			return byStudent
		}
		if byKind := cmp.Compare(left.Kind, right.Kind); byKind != 0 {
			return byKind
		}
		return cmp.Compare(left.ID, right.ID)
	})
	return ordered
}

func bulkRefStudentID(
	ref ParentRequestRef,
	masters map[int64]*MasterDataReviewItem,
	excused map[int64]ExcusedBulkCandidate,
) int64 {
	if ref.Kind == ParentRequestKindMasterData {
		return masters[ref.ID].Request.StudentID
	}
	return excused[ref.ID].StudentID
}

func sortedParentRequestRefs(refs []ParentRequestRef) []ParentRequestRef {
	ordered := append([]ParentRequestRef(nil), refs...)
	slices.SortFunc(ordered, func(left, right ParentRequestRef) int {
		if byKind := cmp.Compare(left.Kind, right.Kind); byKind != 0 {
			return byKind
		}
		return cmp.Compare(left.ID, right.ID)
	})
	return ordered
}

func (s *ParentRequestCoordinator) applyBulkParentRequests(ctx context.Context, input BulkApproveParentRequestsInput) error {
	for _, ref := range input.Requests {
		err := s.applyBulkParentRequest(ctx, input, ref)
		if err != nil {
			if errors.Is(err, ErrParentRequestDecisionRace) {
				return fmt.Errorf("%w: %s %d changed during approval", ErrParentRequestStale, ref.Kind, ref.ID)
			}
			return fmt.Errorf("parent requests: bulk apply %s %d: %w", ref.Kind, ref.ID, err)
		}
	}
	return nil
}

func (s *ParentRequestCoordinator) applyBulkParentRequest(ctx context.Context, input BulkApproveParentRequestsInput, ref ParentRequestRef) error {
	if ref.Kind == ParentRequestKindMasterData {
		_, err := s.masterData.Decide(ctx, MasterDataReviewDecideInput{
			RequestID: ref.ID, Approve: true, Reason: input.Reason, ReviewedBy: input.ReviewerID,
			ExpectedVersion: ref.ExpectedVersion,
		})
		if errors.Is(err, ErrReviewNotPending) || errors.Is(err, userModels.ErrChangeRequestNotPending) {
			return ErrParentRequestDecisionRace
		}
		return err
	}
	err := s.excused.ApproveExcusedBulk(ctx, ref.ID, input.Reason, input.ReviewerID, ref.ExpectedVersion)
	if errors.Is(err, activeModels.ErrExcusedRequestNotPending) {
		return ErrParentRequestDecisionRace
	}
	return err
}

func authorizeBulkParentRequestKinds(ctx context.Context, refs []ParentRequestRef) error {
	granted := jwt.PermissionsFromCtx(ctx)
	canUpdate := authorize.HasPermission(permissions.UsersUpdate, granted)
	canReviewAbsence := canUpdate || authorize.HasPermission(permissions.UsersAbsence, granted)
	for _, ref := range refs {
		if ref.Kind == ParentRequestKindMasterData && !canUpdate {
			return ErrParentRequestForbidden
		}
		if ref.Kind == ParentRequestKindExcused && !canReviewAbsence {
			return ErrParentRequestForbidden
		}
	}
	return nil
}

func validateBulkParentRequestInput(input BulkApproveParentRequestsInput) error {
	if len(input.Requests) < 2 || len(input.Requests) > maxBulkParentRequests || input.ReviewerID <= 0 {
		return ErrInvalidBulkRequest
	}
	// The shared reason is mandatory only while the school's policy asks staff
	// for one (#2267, story 28).
	if input.ReasonRequired && strings.TrimSpace(input.Reason) == "" {
		return ErrParentRequestReasonRequired
	}
	type requestKey struct {
		kind ParentRequestKind
		id   int64
	}
	seen := make(map[requestKey]struct{}, len(input.Requests))
	for _, ref := range input.Requests {
		if ref.ID <= 0 || ref.ExpectedVersion == "" {
			return ErrInvalidBulkRequest
		}
		if ref.Kind != ParentRequestKindMasterData && ref.Kind != ParentRequestKindExcused {
			return ErrBulkIneligible
		}
		key := requestKey{kind: ref.Kind, id: ref.ID}
		if _, duplicate := seen[key]; duplicate {
			return ErrInvalidBulkRequest
		}
		seen[key] = struct{}{}
	}
	return nil
}

func (s *ParentRequestCoordinator) loadBulkParentRequests(
	ctx context.Context,
	refs []ParentRequestRef,
) (map[int64]*MasterDataReviewItem, map[int64]ExcusedBulkCandidate, error) {
	masters := make(map[int64]*MasterDataReviewItem)
	excused := make(map[int64]ExcusedBulkCandidate)
	for _, ref := range refs {
		if err := s.loadBulkParentRequest(ctx, ref, masters, excused); err != nil {
			return nil, nil, err
		}
	}
	return masters, excused, nil
}

func (s *ParentRequestCoordinator) loadBulkParentRequest(
	ctx context.Context, ref ParentRequestRef,
	masters map[int64]*MasterDataReviewItem, excused map[int64]ExcusedBulkCandidate,
) error {
	if ref.Kind == ParentRequestKindMasterData {
		row, err := s.masterData.GetBulkCandidate(ctx, ref.ID)
		if errors.Is(err, ErrReviewNotFound) {
			return ErrParentRequestNotFound
		}
		if err != nil {
			return fmt.Errorf("parent requests: load master data candidate: %w", err)
		}
		masters[ref.ID] = row
		return nil
	}
	row, err := s.excused.GetExcusedBulkCandidate(ctx, ref.ID)
	if err != nil {
		return fmt.Errorf("parent requests: load absence candidate: %w", err)
	}
	if row != nil {
		excused[ref.ID] = *row
	}
	return nil
}

func validateBulkParentRequestVersions(
	refs []ParentRequestRef,
	masters map[int64]*MasterDataReviewItem,
	excused map[int64]ExcusedBulkCandidate,
) error {
	for _, ref := range refs {
		version, err := bulkCandidateVersion(ref, masters, excused)
		if err != nil {
			return err
		}
		if version != ref.ExpectedVersion {
			return ErrParentRequestStale
		}
	}
	return nil
}

func bulkCandidateVersion(
	ref ParentRequestRef, masters map[int64]*MasterDataReviewItem, excused map[int64]ExcusedBulkCandidate,
) (string, error) {
	if ref.Kind == ParentRequestKindMasterData {
		row := masters[ref.ID]
		if row == nil {
			return "", ErrParentRequestNotFound
		}
		if !row.BulkEligible {
			return "", ErrBulkIneligible
		}
		return ParentRequestVersion(row.Request.UpdatedAt), nil
	}
	row, found := excused[ref.ID]
	if !found {
		return "", ErrParentRequestNotFound
	}
	if !row.Eligible {
		return "", ErrBulkIneligible
	}
	return ParentRequestVersion(row.UpdatedAt), nil
}
