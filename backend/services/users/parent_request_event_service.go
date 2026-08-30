package users

import (
	"context"
	"fmt"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// ParentRequestEventInput is one ledger entry. Everything except Payload and
// ActorAccountID is required; a caller that cannot name the request has
// nothing to record.
type ParentRequestEventInput struct {
	StudentID   int64
	RequestType string
	RequestID   int64
	EventType   string
	// ActorAccountID is the guardian on submit/edit and the reviewer on a
	// decision. Zero records a system event.
	ActorAccountID int64
	// UpdatedAt is the request row's updated_at AFTER the event; it becomes
	// the recorded version so a reader can tell which version was acted on.
	UpdatedAt time.Time
	Payload   map[string]any
}

// ParentRequestEventRecorder is the ONE way a parent request's history is
// written. Every domain records through it, inside the ambient transaction of
// the change it describes, so an event can never survive a rolled-back write
// and a committed write can never be missing its event.
type ParentRequestEventRecorder interface {
	Record(ctx context.Context, input ParentRequestEventInput) error
	ListForRequest(ctx context.Context, requestType string, requestID int64) ([]*userModels.ParentRequestEvent, error)
}

type parentRequestEventService struct {
	repo userModels.ParentRequestEventRepository
}

// NewParentRequestEventRecorder wires the recorder. A nil repository is a
// programming error, not a runtime state: callers hold the interface and skip
// recording by holding nil themselves.
func NewParentRequestEventRecorder(repo userModels.ParentRequestEventRepository) ParentRequestEventRecorder {
	return &parentRequestEventService{repo: repo}
}

func (s *parentRequestEventService) Record(ctx context.Context, input ParentRequestEventInput) error {
	if s == nil || s.repo == nil {
		return nil
	}
	if input.StudentID <= 0 || input.RequestID <= 0 || input.RequestType == "" || input.EventType == "" {
		return fmt.Errorf("parent request event: student, request and event type are required")
	}
	event := &userModels.ParentRequestEvent{
		StudentID:   input.StudentID,
		RequestType: input.RequestType,
		RequestID:   input.RequestID,
		EventType:   input.EventType,
		Version:     ParentRequestVersion(input.UpdatedAt),
		Payload:     input.Payload,
	}
	if input.ActorAccountID > 0 {
		actor := input.ActorAccountID
		event.ActorAccountID = &actor
	}
	return s.repo.Create(ctx, event)
}

func (s *parentRequestEventService) ListForRequest(
	ctx context.Context,
	requestType string,
	requestID int64,
) ([]*userModels.ParentRequestEvent, error) {
	if s == nil || s.repo == nil {
		return nil, nil
	}
	return s.repo.ListForRequest(ctx, requestType, requestID)
}

// RecordParentRequestEvent writes one ledger entry through a recorder that may
// be absent. Tests and older wiring hold a nil recorder; a missing ledger must
// never fail the write it describes, but a recorder that IS wired must, so its
// error propagates.
func RecordParentRequestEvent(ctx context.Context, recorder ParentRequestEventRecorder, input ParentRequestEventInput) error {
	if recorder == nil {
		return nil
	}
	return recorder.Record(ctx, input)
}
