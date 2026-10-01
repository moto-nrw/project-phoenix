package services

import (
	"context"
	"errors"

	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// errParentRequestEventIncomplete refuses a ledger entry that cannot name its
// request.
var errParentRequestEventIncomplete = errors.New("parent request event: student, request and event type are required")

// parentRequestLedger is the one append-only history of every parent request
// (#2267), shared by all four queues so a request's history survives the
// edits, decisions and corrections that overwrite the request rows. People
// Directory owns users.parent_request_events; the queues record through
// their consumer-owned ledger ports, inside the ambient transaction of the
// change an entry describes, so an event can never survive a rolled-back
// write and a committed write can never miss its event.
//
// A nil ledger records nothing, which is what graphs without a ledger need.
type parentRequestLedger struct {
	repo usersModels.ParentRequestEventRepository
}

var _ carePlanCompose.RequestLedger = (*parentRequestLedger)(nil)

func newParentRequestLedger(repo usersModels.ParentRequestEventRepository) *parentRequestLedger {
	if repo == nil {
		return nil
	}
	return &parentRequestLedger{repo: repo}
}

// Record appends one entry. The entry's version is the request row's
// updated_at AFTER the event, so a reader can tell which version was acted
// on.
func (l *parentRequestLedger) Record(ctx context.Context, entry carePlanCompose.RequestLedgerEntry) error {
	if l == nil {
		return nil
	}
	if entry.StudentID <= 0 || entry.RequestID <= 0 || entry.RequestType == "" || entry.EventType == "" {
		return errParentRequestEventIncomplete
	}
	event := &usersModels.ParentRequestEvent{
		StudentID:   entry.StudentID,
		RequestType: entry.RequestType,
		RequestID:   entry.RequestID,
		EventType:   entry.EventType,
		Version:     careplan.ParentRequestVersion(entry.UpdatedAt),
		Payload:     entry.Payload,
	}
	if entry.ActorAccountID > 0 {
		actor := entry.ActorAccountID
		event.ActorAccountID = &actor
	}
	return l.repo.Create(ctx, event)
}

// ListForRequest returns one request's history oldest first.
func (l *parentRequestLedger) ListForRequest(ctx context.Context, requestType string, requestID int64) ([]*usersModels.ParentRequestEvent, error) {
	if l == nil {
		return nil, nil
	}
	return l.repo.ListForRequest(ctx, requestType, requestID)
}
