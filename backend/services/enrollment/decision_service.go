package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// DecisionService sentinel errors. Mapped to HTTP status codes by the
// admin handlers.
var (
	ErrDecisionRequestNotFound = errors.New("enrollment request not found")
	ErrDecisionChildNotFound   = errors.New("request child not found")
	ErrDecisionInvalidStatus   = errors.New("invalid decision status")
	ErrDecisionAlreadyTerminal = errors.New("child is already in a terminal status")
)

// DecisionStatus enumerates the per-child decisions an admin can apply.
// Mirrors the request_children.status CHECK constraint subset that
// admins are allowed to write (parent-initiated 'withdrawn' goes
// through a different path).
type DecisionStatus string

const (
	DecisionApproved    DecisionStatus = enrollmentModels.ChildStatusApproved
	DecisionWaitlisted  DecisionStatus = enrollmentModels.ChildStatusWaitlisted
	DecisionRejected    DecisionStatus = enrollmentModels.ChildStatusRejected
	DecisionUnderReview DecisionStatus = enrollmentModels.ChildStatusUnderReview
)

var validDecisionStatuses = map[DecisionStatus]bool{
	DecisionApproved:    true,
	DecisionWaitlisted:  true,
	DecisionRejected:    true,
	DecisionUnderReview: true,
}

// DecideInput carries the per-child decision the admin makes.
type DecideInput struct {
	RequestID  int64
	ChildID    int64
	Status     DecisionStatus
	Reason     string // optional; surfaced to parent only when phase.show_status_reason_to_parent
	ReviewedBy int64  // admin's auth account id
}

// RequestSummary is the admin-list shape: one row per request with
// per-child counts so the admin can scan the queue without expanding
// every detail page.
type RequestSummary struct {
	Request  *enrollmentModels.Request
	Phase    *enrollmentModels.Phase
	Children []*enrollmentModels.RequestChild
}

// RequestFilters narrows the admin list. Zero-value fields are
// ignored.
type RequestFilters struct {
	PhaseID     int64
	ChildStatus string // matches when ANY child carries this status
}

// DecisionService backs the admin review UI. Slice 1 ships per-child
// status mutations; the downstream creation of users.students,
// guardian_profiles, students_guardians, and activities.student_enrollments
// rides in slice 2 alongside parent + guardian-invite emails.
type DecisionService interface {
	List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error)
	Get(ctx context.Context, requestID int64) (*RequestSummary, error)
	Decide(ctx context.Context, input DecideInput) (*enrollmentModels.RequestChild, error)
}

// DecisionServiceConfig is the dep-injection bundle.
type DecisionServiceConfig struct {
	RequestRepo      enrollmentModels.RequestRepository
	RequestChildRepo enrollmentModels.RequestChildRepository
	PhaseRepo        enrollmentModels.PhaseRepository
	Logger           *slog.Logger
}

type decisionService struct {
	requestRepo      enrollmentModels.RequestRepository
	requestChildRepo enrollmentModels.RequestChildRepository
	phaseRepo        enrollmentModels.PhaseRepository
	logger           *slog.Logger
}

func NewDecisionService(cfg DecisionServiceConfig) DecisionService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &decisionService{
		requestRepo:      cfg.RequestRepo,
		requestChildRepo: cfg.RequestChildRepo,
		phaseRepo:        cfg.PhaseRepo,
		logger:           logger,
	}
}

func (s *decisionService) List(ctx context.Context, filters RequestFilters) ([]*RequestSummary, error) {
	requests, err := s.requestRepo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
		PhaseID:     filters.PhaseID,
		ChildStatus: filters.ChildStatus,
	})
	if err != nil {
		return nil, fmt.Errorf("decision: list requests: %w", err)
	}

	out := make([]*RequestSummary, 0, len(requests))
	for _, req := range requests {
		summary, err := s.assemble(ctx, req)
		if err != nil {
			return nil, err
		}
		out = append(out, summary)
	}
	return out, nil
}

func (s *decisionService) Get(ctx context.Context, requestID int64) (*RequestSummary, error) {
	req, err := s.requestRepo.FindByID(ctx, requestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	return s.assemble(ctx, req)
}

func (s *decisionService) assemble(ctx context.Context, req *enrollmentModels.Request) (*RequestSummary, error) {
	phase, err := s.phaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil {
		// Phase may have been deleted under us — surface as "phase
		// missing" but don't drop the row from the list.
		s.logger.Warn("decision: phase lookup failed",
			slog.Int64("request_id", req.ID),
			slog.Int64("phase_id", req.PhaseID),
			slog.String("error", err.Error()))
		phase = nil
	}
	children, err := s.requestChildRepo.ListByRequestID(ctx, req.ID)
	if err != nil {
		return nil, fmt.Errorf("decision: list children for request %d: %w", req.ID, err)
	}
	return &RequestSummary{Request: req, Phase: phase, Children: children}, nil
}

// Decide updates a single child's status. Slice 1 only mutates the
// row + audit fields; slice 2 will wrap this in a tx with downstream
// student/guardian/enrollment creation when status==approved.
//
// Idempotency: applying the same status twice is a no-op. Applying a
// new status to an already-terminal child (approved/rejected/
// withdrawn) returns ErrDecisionAlreadyTerminal — admin must use the
// dedicated "promote waitlisted" or "revoke approval" flows for those
// transitions (deferred to slice 2).
func (s *decisionService) Decide(ctx context.Context, input DecideInput) (*enrollmentModels.RequestChild, error) {
	if input.RequestID <= 0 {
		return nil, fmt.Errorf("%w: request_id required", ErrDecisionInvalidStatus)
	}
	if input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: child_id required", ErrDecisionInvalidStatus)
	}
	if !validDecisionStatuses[input.Status] {
		return nil, fmt.Errorf("%w: %s", ErrDecisionInvalidStatus, input.Status)
	}

	children, err := s.requestChildRepo.ListByRequestID(ctx, input.RequestID)
	if err != nil {
		return nil, fmt.Errorf("decision: load children: %w", err)
	}

	var target *enrollmentModels.RequestChild
	for _, c := range children {
		if c.ID == input.ChildID {
			target = c
			break
		}
	}
	if target == nil {
		return nil, ErrDecisionChildNotFound
	}

	// No-op: same status. Don't bump reviewed_at when nothing changes.
	if target.Status == string(input.Status) {
		return target, nil
	}

	// Block transitions out of a terminal status. Promotion flows
	// (waitlisted → approved, etc.) come in slice 2; for slice 1 the
	// admin can only move out of submitted / under_review / waitlisted.
	if target.IsTerminal() {
		return nil, ErrDecisionAlreadyTerminal
	}

	reason := strings.TrimSpace(input.Reason)
	var reasonPtr *string
	if reason != "" {
		reasonPtr = &reason
	}

	if err := s.requestChildRepo.UpdateStatus(ctx, target.ID, string(input.Status), reasonPtr, input.ReviewedBy); err != nil {
		return nil, fmt.Errorf("decision: update child status: %w", err)
	}

	s.logger.Info("enrollment decision applied",
		slog.Int64("request_id", input.RequestID),
		slog.Int64("child_id", input.ChildID),
		slog.String("status", string(input.Status)),
		slog.Int64("reviewed_by", input.ReviewedBy))

	// Refetch to surface DB-managed fields (reviewed_at, updated_at).
	refreshed, err := s.findChildByID(ctx, input.RequestID, input.ChildID)
	if err != nil {
		// Fall back to the in-memory copy with the new status applied.
		target.Status = string(input.Status)
		target.StatusReason = reasonPtr
		return target, nil
	}
	return refreshed, nil
}

func (s *decisionService) findChildByID(ctx context.Context, requestID, childID int64) (*enrollmentModels.RequestChild, error) {
	children, err := s.requestChildRepo.ListByRequestID(ctx, requestID)
	if err != nil {
		return nil, err
	}
	for _, c := range children {
		if c.ID == childID {
			return c, nil
		}
	}
	return nil, ErrDecisionChildNotFound
}
