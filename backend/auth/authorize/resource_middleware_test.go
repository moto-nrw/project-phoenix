package authorize_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
)

// TestAllowPolicy is a configurable policy for testing middleware behavior.
// Instead of mocking the AuthorizationService, this policy allows tests to
// control authorization outcomes through real policy evaluation.
type TestAllowPolicy struct {
	allowResult  bool
	shouldError  bool
	errorMsg     string
	resourceType string // Which resource type this policy handles
}

func (p *TestAllowPolicy) Name() string {
	return "test_policy_" + p.resourceType
}

func (p *TestAllowPolicy) ResourceType() string {
	return p.resourceType
}

func (p *TestAllowPolicy) Evaluate(_ context.Context, _ *policy.Context) (bool, error) {
	if p.shouldError {
		return false, errors.New(p.errorMsg)
	}
	return p.allowResult, nil
}

func TestResourceAuthorizer_RequiresResourceAccess(t *testing.T) {
	tests := []struct {
		name           string
		resourceType   string
		action         policy.Action
		claims         jwt.AppClaims
		permissions    []string
		extractorID    interface{}
		extraData      map[string]interface{}
		policyAllow    bool
		policyError    bool
		expectedStatus int
	}{
		{
			name:         "allows access when policy allows",
			resourceType: "student",
			action:       policy.ActionView,
			claims: jwt.AppClaims{
				ID:       1,
				Username: "teacher1",
				Roles:    []string{"teacher"},
			},
			permissions:    []string{"students:read"},
			extractorID:    int64(123),
			policyAllow:    true,
			policyError:    false,
			expectedStatus: http.StatusOK,
		},
		{
			name:         "denies access when policy denies",
			resourceType: "student",
			action:       policy.ActionView,
			claims: jwt.AppClaims{
				ID:       2,
				Username: "user2",
				Roles:    []string{"user"},
			},
			permissions:    []string{},
			extractorID:    int64(123),
			policyAllow:    false,
			policyError:    false,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:         "returns error when policy errors",
			resourceType: "student",
			action:       policy.ActionEdit,
			claims: jwt.AppClaims{
				ID:       1,
				Username: "user1",
				Roles:    []string{"user"},
			},
			permissions:    []string{"students:update"},
			extractorID:    int64(456),
			policyAllow:    false,
			policyError:    true,
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE: Create real AuthorizationService with test policy
			authService := authorize.NewAuthorizationService()
			testPolicy := &TestAllowPolicy{
				allowResult:  tt.policyAllow,
				shouldError:  tt.policyError,
				errorMsg:     "authorization service error",
				resourceType: tt.resourceType,
			}
			err := authService.RegisterPolicy(testPolicy)
			assert.NoError(t, err)

			authorizer := authorize.NewResourceAuthorizer(authService)

			// Handler that returns 200 OK on success
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Success"))
			})

			// Extractor that returns test ID
			extractor := func(_ *http.Request) (interface{}, map[string]interface{}) {
				return tt.extractorID, tt.extraData
			}

			// ACT: Create middleware and execute request
			middleware := authorizer.RequiresResourceAccess(tt.resourceType, tt.action, extractor)
			protectedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test/123", nil)
			rr := httptest.NewRecorder()

			// Setup context with claims and permissions
			ctx := context.WithValue(req.Context(), jwt.CtxClaims, tt.claims)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, tt.permissions)
			req = req.WithContext(ctx)

			protectedHandler.ServeHTTP(rr, req)

			// ASSERT
			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

