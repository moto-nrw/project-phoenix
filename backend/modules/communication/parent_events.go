package communication

import "context"

// ParentChildEvent is a parent-authored timeline pill. The referenced record
// remains its source of truth. Staff decision and delivery intents are separate.
type ParentChildEvent struct {
	EventType      string
	ActorKind      string
	ActorAccountID int64
	Body           string
	RequestType    string
	RequestStatus  string
	RefTable       string
	RefID          *int64
}

// ParentChildEvents appends parent-authored pills and invalidates live views.
// Callers schedule both operations after the originating transaction commits.
type ParentChildEvents interface {
	EmitParentChildEvent(ctx context.Context, tenantID, studentID, guardianAccountID int64, event ParentChildEvent) error
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}
