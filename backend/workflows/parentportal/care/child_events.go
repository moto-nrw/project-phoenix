package care

// ChildEvent is a parent-authored timeline pill. The referenced row remains
// the source of truth; this best-effort event carries no write authority.
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

// ChildEvents runs after commit, in the event owner's detached tenant scope.
// A nil port omits these best-effort notifications.
type ChildEvents interface {
	EmitChildEvent(tenantID, studentID, guardianAccountID int64, event ChildEvent)
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}
