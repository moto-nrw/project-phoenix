package tenant

import "context"

// actorKey carries the actor type through the Go context alongside the tenant ID.
const actorKey contextKey = "actor_type"

// Actor types. The actor answers "who is acting inside this tenant", which is a
// different question from the JWT scope ("which portal issued the token") and
// from the tenant ID ("which school"). It exists because staff and guardians
// share the phoenix_tenant database role and the same tenant_id, so RLS needs a
// second dimension to keep the two apart.
//
// ActorStaff is the default everywhere: an unset actor resolves to staff, both
// in Go (ActorFromContext) and in SQL (the COALESCE in the RLS policy). That
// keeps every pre-existing code path on the staff board.
const (
	ActorStaff  = "staff"
	ActorParent = "parent"
)

// WithActor returns a new context with the actor type set.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, actorKey, actor)
}

// ActorFromContext returns the actor type from context, defaulting to ActorStaff.
func ActorFromContext(ctx context.Context) string {
	if ctx == nil {
		return ActorStaff
	}
	actor, _ := ctx.Value(actorKey).(string)
	if actor == "" {
		return ActorStaff
	}
	return actor
}

// IsParentActor reports whether the context belongs to a parent-portal request.
func IsParentActor(ctx context.Context) bool {
	return ActorFromContext(ctx) == ActorParent
}
