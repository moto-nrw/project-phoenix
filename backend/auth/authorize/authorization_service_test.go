package authorize

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NewAuthorizationService Tests
// =============================================================================

func TestNewAuthorizationService(t *testing.T) {
	svc := NewAuthorizationService()
	require.NotNil(t, svc)
}

func TestNewAuthorizationService_ImplementsInterface(t *testing.T) {
	svc := NewAuthorizationService()
	_, ok := any(svc).(AuthorizationService)
	assert.True(t, ok, "NewAuthorizationService should return an AuthorizationService")
}

func TestNewAuthorizationService_HasPolicyEngine(t *testing.T) {
	svc := NewAuthorizationService()
	engine := svc.GetPolicyEngine()
	assert.NotNil(t, engine)
}

// =============================================================================
// GetPolicyEngine Tests
// =============================================================================

func TestGetPolicyEngine_ReturnsNonNil(t *testing.T) {
	svc := NewAuthorizationService()
	engine := svc.GetPolicyEngine()
	require.NotNil(t, engine)

	// Engine should start with no policies for any resource type
	policies := engine.GetPoliciesForResource("nonexistent")
	assert.Empty(t, policies)
}

// =============================================================================
// RegisterPolicy Tests
// =============================================================================

func TestRegisterPolicy_Success(t *testing.T) {
	svc := NewAuthorizationService()

	p := &mockPolicy{
		name:         "test-policy",
		resourceType: "student",
		result:       true,
	}

	err := svc.RegisterPolicy(p)
	assert.NoError(t, err)

	// Verify the policy was registered
	engine := svc.GetPolicyEngine()
	policies := engine.GetPoliciesForResource("student")
	assert.Len(t, policies, 1)
	assert.Equal(t, "test-policy", policies[0].Name())
}

func TestRegisterPolicy_MultiplePolicies(t *testing.T) {
	svc := NewAuthorizationService()

	p1 := &mockPolicy{name: "policy-1", resourceType: "student", result: true}
	p2 := &mockPolicy{name: "policy-2", resourceType: "student", result: false}
	p3 := &mockPolicy{name: "policy-3", resourceType: "visit", result: true}

	require.NoError(t, svc.RegisterPolicy(p1))
	require.NoError(t, svc.RegisterPolicy(p2))
	require.NoError(t, svc.RegisterPolicy(p3))

	engine := svc.GetPolicyEngine()
	assert.Len(t, engine.GetPoliciesForResource("student"), 2)
	assert.Len(t, engine.GetPoliciesForResource("visit"), 1)
}

// =============================================================================
// AuthorizeResource Tests
// =============================================================================

func TestAuthorizeResource_Allowed(t *testing.T) {
	svc := NewAuthorizationService()

	p := &mockPolicy{
		name:         "allow-all",
		resourceType: "student",
		result:       true,
	}
	require.NoError(t, svc.RegisterPolicy(p))

	subject := policy.Subject{
		AccountID: 1,
		Roles:     []string{"teacher"},
	}
	resource := policy.Resource{
		Type: "student",
		ID:   int64(42),
	}

	allowed, err := svc.AuthorizeResource(context.Background(), subject, resource, policy.ActionView, nil)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestAuthorizeResource_Denied(t *testing.T) {
	svc := NewAuthorizationService()

	p := &mockPolicy{
		name:         "deny-all",
		resourceType: "student",
		result:       false,
	}
	require.NoError(t, svc.RegisterPolicy(p))

	subject := policy.Subject{
		AccountID: 1,
		Roles:     []string{"teacher"},
	}
	resource := policy.Resource{
		Type: "student",
		ID:   int64(42),
	}

	allowed, err := svc.AuthorizeResource(context.Background(), subject, resource, policy.ActionEdit, nil)
	require.NoError(t, err)
	assert.False(t, allowed)
}

func TestAuthorizeResource_WithExtra(t *testing.T) {
	svc := NewAuthorizationService()

	p := &mockPolicy{
		name:         "check-extra",
		resourceType: "visit",
		result:       true,
	}
	require.NoError(t, svc.RegisterPolicy(p))

	subject := policy.Subject{AccountID: 1}
	resource := policy.Resource{Type: "visit", ID: int64(10)}
	extra := map[string]any{"group_id": int64(55)}

	allowed, err := svc.AuthorizeResource(context.Background(), subject, resource, policy.ActionView, extra)
	require.NoError(t, err)
	assert.True(t, allowed)

	// Verify the extra data was passed through
	assert.Equal(t, extra, p.lastExtra)
}

func TestAuthorizeResource_WithError(t *testing.T) {
	svc := NewAuthorizationService()

	p := &mockPolicy{
		name:         "error-policy",
		resourceType: "student",
		result:       false,
		err:          assert.AnError,
	}
	require.NoError(t, svc.RegisterPolicy(p))

	subject := policy.Subject{AccountID: 1}
	resource := policy.Resource{Type: "student", ID: int64(42)}

	_, err := svc.AuthorizeResource(context.Background(), subject, resource, policy.ActionDelete, nil)
	assert.Error(t, err)
}

func TestAuthorizeResource_NoMatchingPolicy(t *testing.T) {
	svc := NewAuthorizationService()

	// Register policy for "student" but query for "visit"
	p := &mockPolicy{
		name:         "student-policy",
		resourceType: "student",
		result:       true,
	}
	require.NoError(t, svc.RegisterPolicy(p))

	subject := policy.Subject{AccountID: 1}
	resource := policy.Resource{Type: "visit", ID: int64(99)}

	allowed, err := svc.AuthorizeResource(context.Background(), subject, resource, policy.ActionView, nil)
	require.NoError(t, err)
	// No matching policy means denied
	assert.False(t, allowed)
}

// =============================================================================
// Mock Policy
// =============================================================================

type mockPolicy struct {
	name         string
	resourceType string
	result       bool
	err          error
	lastExtra    map[string]any
}

func (m *mockPolicy) Name() string {
	return m.name
}

func (m *mockPolicy) ResourceType() string {
	return m.resourceType
}

func (m *mockPolicy) Evaluate(_ context.Context, authCtx *policy.Context) (bool, error) {
	m.lastExtra = authCtx.Extra
	return m.result, m.err
}
