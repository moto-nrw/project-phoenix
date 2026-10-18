package ports

import "context"

// OperationCaller is the consumer-owned port to the caller's session and the
// shared authorization rules the operational day applies: the session token
// of the request, the operational-overview gate and the school-wide action
// scope, and the People Directory's not-found result.
type OperationCaller interface {
	// AssignmentBoundPortal reports a school-scoped (assignment-bound)
	// session.
	AssignmentBoundPortal(ctx context.Context) bool
	// AdministrativeAccess reports an admin session or an admin wildcard
	// permission.
	AdministrativeAccess(ctx context.Context) bool
	// OGSActorToken reports a tenant or organisation session of accountID in
	// the request's tenant that may read schedules.
	OGSActorToken(ctx context.Context, accountID int64) bool
	// SchoolWideActionScope resolves the school-wide scope of an action.
	SchoolWideActionScope(ctx context.Context, settings OperationSettings, scopeKey string) (bool, error)
	// OperationalOverview applies the operational-overview gate.
	OperationalOverview(ctx context.Context, settings OperationSettings, assignmentBound, administrative, hasStaff bool) (bool, error)
	// IsPersonNotFound reports the People Directory's missing-person result.
	IsPersonNotFound(err error) bool
}
