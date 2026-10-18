package care

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
)

// errRequestEventIncomplete refuses a ledger entry that cannot name its
// request.
var errRequestEventIncomplete = errors.New("parent request event: student, request and event type are required")

// RequestLedgerEntry is one line of a parent request's history (#2267): the
// guardian's submit, edit or share. Everything except Payload and
// ActorAccountID is required.
type RequestLedgerEntry struct {
	StudentID   int64
	RequestType string
	RequestID   int64
	EventType   string
	// ActorAccountID is the acting guardian. Zero records a system event.
	ActorAccountID int64
	// UpdatedAt is the request row's updated_at AFTER the event; it becomes
	// the recorded version, so a reader can tell which version was acted on.
	UpdatedAt time.Time
	Payload   json.RawMessage
}

type RequestEvent struct {
	ID             int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	TenantID       int64
	StudentID      int64
	RequestType    string
	RequestID      int64
	EventType      string
	ActorAccountID *int64
	Version        string
	Payload        json.RawMessage
}

// RequestEventWriter appends a request-history entry in the caller's tenant
// unit of work. The child flows do not read or replace the event ledger.
type RequestEventWriter interface {
	Create(context.Context, *RequestEvent) error
}

// RecordRequestEvent appends one entry to the People Directory ledger inside
// the ambient transaction of the write it describes, so an event can never
// survive a rolled-back write. A flow composed without a ledger records
// nothing; a ledger that IS composed must not fail silently, so its error
// propagates.
func RecordRequestEvent(ctx context.Context, ledger RequestEventWriter, entry RequestLedgerEntry) error {
	if ledger == nil {
		return nil
	}
	if entry.StudentID <= 0 || entry.RequestID <= 0 || entry.RequestType == "" || entry.EventType == "" {
		return errRequestEventIncomplete
	}
	event := &RequestEvent{
		StudentID:   entry.StudentID,
		RequestType: entry.RequestType,
		RequestID:   entry.RequestID,
		EventType:   entry.EventType,
		Version:     excusedrequests.ParentRequestVersion(entry.UpdatedAt),
		Payload:     entry.Payload,
	}
	if entry.ActorAccountID > 0 {
		actor := entry.ActorAccountID
		event.ActorAccountID = &actor
	}
	return ledger.Create(ctx, event)
}

// dataRequestEventPayload contains only protocol strings, so encoding cannot
// fail and produces the same sorted JSON object as the retained ledger.
func dataRequestEventPayload(target, field string) json.RawMessage {
	raw, _ := json.Marshal(map[string]string{"target": target, "field": field})
	return raw
}
