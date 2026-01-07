package authorize_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
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

// TestResourceExtractors tests URL parameter extraction logic.
// This is a pure unit test - no mocks or database needed.
func TestResourceExtractors(t *testing.T) {
	t.Run("URLParamExtractor", func(t *testing.T) {
		tests := []struct {
			name          string
			paramName     string
			paramValue    string
			expectedID    interface{}
			expectedExtra map[string]interface{}
		}{
			{
				name:          "extracts numeric ID",
				paramName:     "id",
				paramValue:    "123",
				expectedID:    int64(123),
				expectedExtra: nil,
			},
			{
				name:          "extracts string ID",
				paramName:     "id",
				paramValue:    "abc-def",
				expectedID:    "abc-def",
				expectedExtra: nil,
			},
			{
				name:          "returns nil for empty param",
				paramName:     "id",
				paramValue:    "",
				expectedID:    nil,
				expectedExtra: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				// Create router with parameter
				r := chi.NewRouter()
				r.Get("/{"+tt.paramName+"}", func(w http.ResponseWriter, r *http.Request) {
					extractor := authorize.URLParamExtractor(tt.paramName)
					id, extra := extractor(r)

					assert.Equal(t, tt.expectedID, id)
					assert.Equal(t, tt.expectedExtra, extra)
				})

				// Create test request
				req := httptest.NewRequest("GET", "/"+tt.paramValue, nil)
				rr := httptest.NewRecorder()

				// Execute request
				r.ServeHTTP(rr, req)
			})
		}
	})

	t.Run("StudentIDFromURL", func(t *testing.T) {
		tests := []struct {
			name          string
			urlParam      string
			expectedExtra map[string]interface{}
		}{
			{
				name:     "extracts student ID as extra data",
				urlParam: "123",
				expectedExtra: map[string]interface{}{
					"student_id": int64(123),
				},
			},
			{
				name:          "returns nil for non-numeric ID",
				urlParam:      "abc",
				expectedExtra: nil,
			},
			{
				name:          "returns nil for empty ID",
				urlParam:      "",
				expectedExtra: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := chi.NewRouter()
				r.Get("/{id}", func(w http.ResponseWriter, r *http.Request) {
					extractor := authorize.StudentIDFromURL()
					id, extra := extractor(r)

					assert.Nil(t, id)
					assert.Equal(t, tt.expectedExtra, extra)
				})

				req := httptest.NewRequest("GET", "/"+tt.urlParam, nil)
				rr := httptest.NewRecorder()

				r.ServeHTTP(rr, req)
			})
		}
	})
}

func TestCombinePermissionAndResource(t *testing.T) {
	// ARRANGE: Create real AuthorizationService with deny policy
	authService := authorize.NewAuthorizationService()
	denyPolicy := &TestAllowPolicy{
		allowResult:  false,
		resourceType: "student",
	}
	err := authService.RegisterPolicy(denyPolicy)
	assert.NoError(t, err)

	authorizer := authorize.NewResourceAuthorizer(authService)
	authorize.SetResourceAuthorizer(authorizer)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("Success"))
	})

	// Create combined middleware
	middleware := authorize.CombinePermissionAndResource(
		"students:read",
		"student",
		policy.ActionView,
		authorize.URLParamExtractor("id"),
	)
	protectedHandler := middleware(handler)

	// ACT: Create test with permission but no resource access
	req := httptest.NewRequest("GET", "/test/123", nil)
	rr := httptest.NewRecorder()

	// Setup context with claims and permissions
	claims := jwt.AppClaims{
		ID:       1,
		Username: "user1",
		Roles:    []string{"user"},
	}
	permissions := []string{"students:read"}

	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)

	// Execute request
	protectedHandler.ServeHTTP(rr, req)

	// ASSERT: Should be forbidden even with permission if resource auth fails
	assert.Equal(t, http.StatusForbidden, rr.Code)
}
