package tenant_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/tenant"
)

// The actor defaults to staff in every direction. That default is what keeps
// every pre-existing code path on the school's board after #1678 introduced the
// second dimension, so it is asserted explicitly rather than assumed.
func TestActorFromContext_DefaultsToStaff(t *testing.T) {
	assert.Equal(t, tenant.ActorStaff, tenant.ActorFromContext(context.Background()),
		"a context that never saw WithActor must read as staff")
}

func TestActorFromContext_NilContextDefaultsToStaff(t *testing.T) {
	//nolint:staticcheck // deliberately passing a nil context: callers reach the
	// helper from non-HTTP paths where the context may be unset.
	assert.Equal(t, tenant.ActorStaff, tenant.ActorFromContext(nil))
}

func TestActorFromContext_EmptyActorDefaultsToStaff(t *testing.T) {
	ctx := tenant.WithActor(context.Background(), "")

	assert.Equal(t, tenant.ActorStaff, tenant.ActorFromContext(ctx),
		"an explicitly empty actor must not resolve to the parent board")
}

func TestWithActor_RoundTrip(t *testing.T) {
	ctx := tenant.WithActor(context.Background(), tenant.ActorParent)

	assert.Equal(t, tenant.ActorParent, tenant.ActorFromContext(ctx))
}

func TestIsParentActor(t *testing.T) {
	assert.True(t, tenant.IsParentActor(tenant.WithActor(context.Background(), tenant.ActorParent)))
	assert.False(t, tenant.IsParentActor(tenant.WithActor(context.Background(), tenant.ActorStaff)))
	assert.False(t, tenant.IsParentActor(context.Background()))
}

// The actor is carried independently of the tenant ID: both have to survive the
// same context, otherwise a parent transaction would lose one of the two values
// the RLS policy reads.
func TestActorAndTenantIDCoexist(t *testing.T) {
	ctx := tenant.WithActor(tenant.WithTenantID(context.Background(), 4711), tenant.ActorParent)

	assert.Equal(t, tenant.ActorParent, tenant.ActorFromContext(ctx))
	assert.Equal(t, int64(4711), tenant.FromContext(ctx))
}
