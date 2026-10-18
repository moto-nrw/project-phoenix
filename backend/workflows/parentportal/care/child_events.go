package care

import "context"

// ChildEvent is a parent-authored timeline pill. The referenced row remains
// the source of truth; this durable event carries no write authority.
type ChildEvent struct {
	EventType      string
	ActorKind      string
	ActorAccountID int64
	Body           string
	RequestType    string
	RequestStatus  string
	RefTable       string
	RefID          *int64
}

// ChildEvents appends timeline pills in the caller's transaction. Only its
// realtime broadcasts run after commit. A nil port omits timeline integration.
type ChildEvents interface {
	EmitChildEvent(ctx context.Context, tenantID, studentID, guardianAccountID int64, event ChildEvent) error
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}
