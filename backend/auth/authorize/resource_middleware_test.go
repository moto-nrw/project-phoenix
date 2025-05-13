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
	"github.com/stretchr/testify/mock"
)

// MockAuthorizationService is a mock of the authorization service
type MockAuthorizationService struct {
	mock.Mock
}

func (m *MockAuthorizationService) AuthorizeResource(ctx context.Context, subject policy.Subject, resource policy.Resource, action policy.Action, extra map[string]interface{}) (bool, error) {
	args := m.Called(ctx, subject, resource, action, extra)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizationService) RegisterPolicy(p policy.Policy) error {
	args := m.Called(p)
	return args.Error(0)
}

func (m *MockAuthorizationService) GetPolicyEngine() policy.PolicyEngine {
	args := m.Called()
	return args.Get(0).(policy.PolicyEngine)
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
		authResult     bool
		authError      error
		expectedStatus int
	}{
		{
			name:         "allows access when authorized",
			resourceType: "student",
			action:       policy.ActionView,
			claims: jwt.AppClaims{
				ID:       1,
				Username: "teacher1",
				Roles:    []string{"teacher"},
			},
			permissions:    []string{"students:read"},
			extractorID:    int64(123),
			authResult:     true,
			authError:      nil,
			expectedStatus: http.StatusOK,
		},
		{
			name:         "denies access when not authorized",
			resourceType: "student",
			action:       policy.ActionView,
			claims: jwt.AppClaims{
				ID:       2,
				Username: "user2",
				Roles:    []string{"user"},
			},
			permissions:    []string{},
			extractorID:    int64(123),
			authResult:     false,
			authError:      nil,
			expectedStatus: http.StatusForbidden,
		},
		{
			name:         "returns error when authorization fails",
			resourceType: "student",
			action:       policy.ActionEdit,
			claims: jwt.AppClaims{
				ID:       1,
				Username: "user1",
				Roles:    []string{"user"},
			},
			permissions:    []string{"students:update"},
			extractorID:    int64(456),
			authResult:     false,
			authError:      errors.New("authorization service error"),
			expectedStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock authorization service
			mockAuthService := new(MockAuthorizationService)

			// Create the resource authorizer
			authorizer := authorize.NewResourceAuthorizer(mockAuthService)

			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success"))
			})

			// Create extractor that returns test data
			extractor := func(r *http.Request) (interface{}, map[string]interface{}) {
				return tt.extractorID, tt.extraData
			}

			// Create middleware
			middleware := authorizer.RequiresResourceAccess(tt.resourceType, tt.action, extractor)
			protectedHandler := middleware(handler)

			// Create test request
			req := httptest.NewRequest("GET", "/test/123", nil)
			rr := httptest.NewRecorder()

			// Setup context with claims and permissions
			ctx := context.WithValue(req.Context(), ctxKeyClaims, tt.claims)
			// Use JWT context keys for the actual implementation
			ctx = context.WithValue(ctx, jwt.CtxClaims, tt.claims)
			ctx = context.WithValue(ctx, jwt.CtxPermissions, tt.permissions)
			req = req.WithContext(ctx)

			// Setup mock expectations
			expectedSubject := policy.Subject{
				AccountID:   int64(tt.claims.ID),
				Roles:       tt.claims.Roles,
				Permissions: tt.permissions,
			}
			expectedResource := policy.Resource{
				Type: tt.resourceType,
				ID:   tt.extractorID,
			}

			if tt.extraData == nil {
				tt.extraData = make(map[string]interface{})
			}

			mockAuthService.On("AuthorizeResource",
				mock.Anything,
				expectedSubject,
				expectedResource,
				tt.action,
				tt.extraData,
			).Return(tt.authResult, tt.authError)

			// Execute request
			protectedHandler.ServeHTTP(rr, req)

			// Assert results
			assert.Equal(t, tt.expectedStatus, rr.Code)
			mockAuthService.AssertExpectations(t)
		})
	}
}

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
	// Test that combining permission and resource checks works correctly
	mockAuthService := new(MockAuthorizationService)
	authorizer := authorize.NewResourceAuthorizer(mockAuthService)
	authorize.SetResourceAuthorizer(authorizer)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Success"))
	})

	// Create combined middleware
	middleware := authorize.CombinePermissionAndResource(
		"students:read",
		"student",
		policy.ActionView,
		authorize.URLParamExtractor("id"),
	)
	protectedHandler := middleware(handler)

	// Create test with permission but no resource access
	req := httptest.NewRequest("GET", "/test/123", nil)
	rr := httptest.NewRecorder()

	// Setup context
	claims := jwt.AppClaims{
		ID:       1,
		Username: "user1",
		Roles:    []string{"user"},
	}
	permissions := []string{"students:read"}

	ctx := context.WithValue(req.Context(), ctxKeyClaims, claims)
	// Use JWT context keys for the actual implementation
	ctx = context.WithValue(ctx, jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)

	// Mock authorization service to deny access
	mockAuthService.On("AuthorizeResource",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		policy.ActionView,
		mock.Anything,
	).Return(false, nil)

	// Execute request
	protectedHandler.ServeHTTP(rr, req)

	// Should be forbidden even with permission if resource auth fails
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// Context keys for testing with a different name to avoid conflicts
type rmCtxKey int

const (
	ctxKeyClaims rmCtxKey = iota + 10
)
