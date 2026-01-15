package policy_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/policy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockPolicy implements the Policy interface for testing
type MockPolicy struct {
	name         string
	resourceType string
	evaluateFunc func(ctx context.Context, authCtx *policy.Context) (bool, error)
}

func (p *MockPolicy) Name() string {
	return p.name
}

func (p *MockPolicy) ResourceType() string {
	return p.resourceType
}

func (p *MockPolicy) Evaluate(ctx context.Context, authCtx *policy.Context) (bool, error) {
	if p.evaluateFunc != nil {
		return p.evaluateFunc(ctx, authCtx)
	}
	return false, nil
}

func TestPolicyEngine_RegisterPolicy(t *testing.T) {
	engine := policy.NewPolicyEngine()

	// Test successful registration
	mockPolicy := &MockPolicy{
		name:         "test_policy",
		resourceType: "test_resource",
	}

	err := engine.RegisterPolicy(mockPolicy)
	assert.NoError(t, err)

	// Test duplicate registration
	err = engine.RegisterPolicy(mockPolicy)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
}

func TestPolicyEngine_Authorize(t *testing.T) {
	tests := []struct {
		name           string
		setupPolicies  func(engine policy.PolicyEngine)
		authContext    *policy.Context
		expectedResult bool
		expectError    bool
	}{
		{
			name: "allows access when policy approves",
			setupPolicies: func(engine policy.PolicyEngine) {
				_ = engine.RegisterPolicy(&MockPolicy{
					name:         "allow_policy",
					resourceType: "student",
					evaluateFunc: func(ctx context.Context, authCtx *policy.Context) (bool, error) {
						return true, nil
					},
				})
			},
			authContext: &policy.Context{
				Resource: policy.Resource{Type: "student", ID: 123},
				Action:   policy.ActionView,
				Subject: policy.Subject{
					AccountID: 1,
					Roles:     []string{"teacher"},
				},
			},
			expectedResult: true,
			expectError:    false,
		},
		{
			name: "denies access when policy denies",
			setupPolicies: func(engine policy.PolicyEngine) {
				_ = engine.RegisterPolicy(&MockPolicy{
					name:         "deny_policy",
					resourceType: "student",
					evaluateFunc: func(ctx context.Context, authCtx *policy.Context) (bool, error) {
						return false, nil
					},
				})
			},
			authContext: &policy.Context{
				Resource: policy.Resource{Type: "student", ID: 123},
				Action:   policy.ActionView,
				Subject: policy.Subject{
					AccountID: 1,
					Roles:     []string{"teacher"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "denies access when all policies must pass",
			setupPolicies: func(engine policy.PolicyEngine) {
				_ = engine.RegisterPolicy(&MockPolicy{
					name:         "allow_policy",
					resourceType: "student",
					evaluateFunc: func(ctx context.Context, authCtx *policy.Context) (bool, error) {
						return true, nil
					},
				})
				_ = engine.RegisterPolicy(&MockPolicy{
					name:         "deny_policy",
					resourceType: "student",
					evaluateFunc: func(ctx context.Context, authCtx *policy.Context) (bool, error) {
						return false, nil
					},
				})
			},
			authContext: &policy.Context{
				Resource: policy.Resource{Type: "student", ID: 123},
				Action:   policy.ActionView,
				Subject: policy.Subject{
					AccountID: 1,
					Roles:     []string{"teacher"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name:          "denies access when no policies exist for resource",
			setupPolicies: func(engine policy.PolicyEngine) {},
			authContext: &policy.Context{
				Resource: policy.Resource{Type: "unknown", ID: 123},
				Action:   policy.ActionView,
				Subject: policy.Subject{
					AccountID: 1,
					Roles:     []string{"teacher"},
				},
			},
			expectedResult: false,
			expectError:    false,
		},
		{
			name: "returns error when policy evaluation fails",
			setupPolicies: func(engine policy.PolicyEngine) {
				_ = engine.RegisterPolicy(&MockPolicy{
					name:         "error_policy",
					resourceType: "student",
					evaluateFunc: func(ctx context.Context, authCtx *policy.Context) (bool, error) {
						return false, assert.AnError
					},
				})
			},
			authContext: &policy.Context{
				Resource: policy.Resource{Type: "student", ID: 123},
				Action:   policy.ActionView,
				Subject: policy.Subject{
					AccountID: 1,
					Roles:     []string{"teacher"},
				},
			},
			expectedResult: false,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := policy.NewPolicyEngine()
			tt.setupPolicies(engine)

			result, err := engine.Authorize(context.Background(), tt.authContext)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPolicyEngine_GetPoliciesForResource(t *testing.T) {
	engine := policy.NewPolicyEngine()

	// Register multiple policies for different resources
	studentPolicy1 := &MockPolicy{
		name:         "student_policy_1",
		resourceType: "student",
	}
	studentPolicy2 := &MockPolicy{
		name:         "student_policy_2",
		resourceType: "student",
	}
	teacherPolicy := &MockPolicy{
		name:         "teacher_policy",
		resourceType: "teacher",
	}

	require.NoError(t, engine.RegisterPolicy(studentPolicy1))
	require.NoError(t, engine.RegisterPolicy(studentPolicy2))
	require.NoError(t, engine.RegisterPolicy(teacherPolicy))

	// Test retrieving policies for specific resource
	studentPolicies := engine.GetPoliciesForResource("student")
	assert.Len(t, studentPolicies, 2)

	teacherPolicies := engine.GetPoliciesForResource("teacher")
	assert.Len(t, teacherPolicies, 1)

	unknownPolicies := engine.GetPoliciesForResource("unknown")
	assert.Len(t, unknownPolicies, 0)
}
