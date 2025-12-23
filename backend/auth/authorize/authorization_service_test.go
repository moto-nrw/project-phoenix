package authorize

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock policy for testing
type mockPolicy struct {
	name         string
	resourceType string
	shouldAllow  bool
	shouldError  bool
}

func (m *mockPolicy) Name() string {
	return m.name
}

func (m *mockPolicy) ResourceType() string {
	return m.resourceType
}

func (m *mockPolicy) Evaluate(_ context.Context, _ *policy.Context) (bool, error) {
	if m.shouldError {
		return false, errors.New("mock policy error")
	}
	return m.shouldAllow, nil
}

func TestNewAuthorizationService(t *testing.T) {
	service := NewAuthorizationService()
	require.NotNil(t, service)

	// Should have a policy engine
	engine := service.GetPolicyEngine()
	require.NotNil(t, engine)
}

func TestAuthorizationService_RegisterPolicy(t *testing.T) {
	service := NewAuthorizationService()

	mockP := &mockPolicy{
		name:         "test-policy",
		resourceType: "test-resource",
		shouldAllow:  true,
	}

	err := service.RegisterPolicy(mockP)
	require.NoError(t, err)

	// Verify policy was registered
	engine := service.GetPolicyEngine()
	policies := engine.GetPoliciesForResource("test-resource")
	assert.Len(t, policies, 1)
	assert.Equal(t, "test-policy", policies[0].Name())
}

func TestAuthorizationService_AuthorizeResource(t *testing.T) {
	tests := []struct {
		name        string
		policy      *mockPolicy
		subject     policy.Subject
		resource    policy.Resource
		action      policy.Action
		extra       map[string]interface{}
		wantAllowed bool
		wantErr     bool
	}{
		{
			name: "allows when policy approves",
			policy: &mockPolicy{
				name:         "allow-policy",
				resourceType: "resource",
				shouldAllow:  true,
			},
			subject: policy.Subject{
				AccountID:   1,
				Roles:       []string{"user"},
				Permissions: []string{"resource:read"},
			},
			resource: policy.Resource{
				Type: "resource",
				ID:   "123",
			},
			action:      "read",
			extra:       nil,
			wantAllowed: true,
			wantErr:     false,
		},
		{
			name: "denies when policy denies",
			policy: &mockPolicy{
				name:         "deny-policy",
				resourceType: "resource",
				shouldAllow:  false,
			},
			subject: policy.Subject{
				AccountID:   1,
				Roles:       []string{"user"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "resource",
				ID:   "123",
			},
			action:      "delete",
			extra:       nil,
			wantAllowed: false,
			wantErr:     false,
		},
		{
			name: "returns error when policy errors",
			policy: &mockPolicy{
				name:         "error-policy",
				resourceType: "resource",
				shouldError:  true,
			},
			subject: policy.Subject{
				AccountID:   1,
				Roles:       []string{"user"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "resource",
				ID:   "123",
			},
			action:      "read",
			extra:       nil,
			wantAllowed: false,
			wantErr:     true,
		},
		{
			name: "passes extra context",
			policy: &mockPolicy{
				name:         "extra-policy",
				resourceType: "resource",
				shouldAllow:  true,
			},
			subject: policy.Subject{
				AccountID: 1,
				Roles:     []string{"user"},
			},
			resource: policy.Resource{
				Type: "resource",
				ID:   "123",
			},
			action: "update",
			extra: map[string]interface{}{
				"custom_key": "custom_value",
			},
			wantAllowed: true,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewAuthorizationService()

			// Register the mock policy
			err := service.RegisterPolicy(tt.policy)
			require.NoError(t, err)

			// Test authorization
			allowed, err := service.AuthorizeResource(
				context.Background(),
				tt.subject,
				tt.resource,
				tt.action,
				tt.extra,
			)

			if tt.wantErr {
				require.Error(t, err)
				assert.False(t, allowed)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.wantAllowed, allowed)
		})
	}
}

func TestAuthorizationService_GetPolicyEngine(t *testing.T) {
	service := NewAuthorizationService()

	engine1 := service.GetPolicyEngine()
	engine2 := service.GetPolicyEngine()

	// Should return the same instance
	assert.Same(t, engine1, engine2)
}

func TestAuthorizationService_DeniesWithNoPolicy(t *testing.T) {
	service := NewAuthorizationService()

	// Don't register any policies - should deny by default
	allowed, err := service.AuthorizeResource(
		context.Background(),
		policy.Subject{AccountID: 1},
		policy.Resource{Type: "unknown-resource", ID: "123"},
		"read",
		nil,
	)

	require.NoError(t, err)
	assert.False(t, allowed, "Should deny when no policies exist for resource")
}
