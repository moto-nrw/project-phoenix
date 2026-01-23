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
	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			ctx = context.WithValue(ctx, tenant.CtxPermissions, tt.permissions)
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
	ctx = context.WithValue(ctx, tenant.CtxPermissions, permissions)
	req = req.WithContext(ctx)

	// Execute request
	protectedHandler.ServeHTTP(rr, req)

	// ASSERT: Should be forbidden even with permission if resource auth fails
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// TestCreateSubjectFromContext tests the different branches of context extraction.
// Tests coverage for nil TenantContext, nil AccountID, and empty Role.
func TestCreateSubjectFromContext(t *testing.T) {
	tests := []struct {
		name                string
		setupContext        func(ctx context.Context) context.Context
		expectedAccountID   int64
		expectedRoles       []string
		expectedPermissions []string
	}{
		{
			name: "nil tenant context returns zero values",
			setupContext: func(ctx context.Context) context.Context {
				// No tenant context set - tests tc == nil branch
				return ctx
			},
			expectedAccountID:   0,
			expectedRoles:       nil, // No roles when tc is nil
			expectedPermissions: []string{},
		},
		{
			name: "tenant context with nil AccountID",
			setupContext: func(ctx context.Context) context.Context {
				tc := &tenant.TenantContext{
					UserID: "user-123",
					Role:   "supervisor",
					// AccountID is nil - tests tc.AccountID == nil branch
				}
				return tenant.SetTenantContext(ctx, tc)
			},
			expectedAccountID:   0,
			expectedRoles:       []string{"supervisor"},
			expectedPermissions: nil, // Permissions from context are nil when not set in TenantContext
		},
		{
			name: "tenant context with empty Role",
			setupContext: func(ctx context.Context) context.Context {
				accountID := int64(42)
				tc := &tenant.TenantContext{
					UserID:    "user-123",
					AccountID: &accountID,
					Role:      "", // Empty role - tests tc.Role == "" branch
				}
				return tenant.SetTenantContext(ctx, tc)
			},
			expectedAccountID:   42,
			expectedRoles:       nil, // Empty role should not add to roles slice
			expectedPermissions: nil, // Permissions from context are nil when not set in TenantContext
		},
		{
			name: "tenant context with both AccountID and Role",
			setupContext: func(ctx context.Context) context.Context {
				accountID := int64(99)
				tc := &tenant.TenantContext{
					UserID:      "user-456",
					AccountID:   &accountID,
					Role:        "ogsAdmin",
					Permissions: []string{"student:read", "student:write"},
				}
				return tenant.SetTenantContext(ctx, tc)
			},
			expectedAccountID:   99,
			expectedRoles:       []string{"ogsAdmin"},
			expectedPermissions: []string{"student:read", "student:write"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE: Create auth service and policy for testing
			authService := authorize.NewAuthorizationService()
			testPolicy := &TestAllowPolicy{
				allowResult:  true,
				resourceType: "test",
			}
			err := authService.RegisterPolicy(testPolicy)
			require.NoError(t, err)

			authorizer := authorize.NewResourceAuthorizer(authService)

			// Capture the subject that gets created
			var capturedAccountID int64
			var capturedRoles []string
			var capturedPermissions []string

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// If we reach here, authorization passed
				w.WriteHeader(http.StatusOK)
			})

			// Use a custom policy that captures the subject values
			capturePolicy := &SubjectCapturePolicy{
				resourceType: "capture",
				onEvaluate: func(ctx *policy.Context) {
					capturedAccountID = ctx.Subject.AccountID
					capturedRoles = ctx.Subject.Roles
					capturedPermissions = ctx.Subject.Permissions
				},
			}
			err = authService.RegisterPolicy(capturePolicy)
			require.NoError(t, err)

			middleware := authorizer.RequiresResourceAccess("capture", policy.ActionView)
			protectedHandler := middleware(handler)

			// ACT
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()
			ctx := tt.setupContext(req.Context())
			req = req.WithContext(ctx)

			protectedHandler.ServeHTTP(rr, req)

			// ASSERT
			assert.Equal(t, tt.expectedAccountID, capturedAccountID)
			assert.Equal(t, tt.expectedRoles, capturedRoles)
			assert.Equal(t, tt.expectedPermissions, capturedPermissions)
		})
	}
}

// SubjectCapturePolicy captures subject values during evaluation for testing.
type SubjectCapturePolicy struct {
	resourceType string
	onEvaluate   func(ctx *policy.Context)
}

func (p *SubjectCapturePolicy) Name() string {
	return "subject_capture_" + p.resourceType
}

func (p *SubjectCapturePolicy) ResourceType() string {
	return p.resourceType
}

func (p *SubjectCapturePolicy) Evaluate(_ context.Context, ctx *policy.Context) (bool, error) {
	if p.onEvaluate != nil {
		p.onEvaluate(ctx)
	}
	return true, nil
}

// TestApplyExtractors tests the extractor application logic.
func TestApplyExtractors(t *testing.T) {
	t.Run("multiple extractors combine extra data", func(t *testing.T) {
		// ARRANGE
		authService := authorize.NewAuthorizationService()
		var capturedExtra map[string]interface{}

		capturePolicy := &ExtraDataCapturePolicy{
			resourceType: "multi",
			onEvaluate: func(ctx *policy.Context) {
				capturedExtra = ctx.Extra
			},
		}
		err := authService.RegisterPolicy(capturePolicy)
		require.NoError(t, err)

		authorizer := authorize.NewResourceAuthorizer(authService)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Create multiple extractors that each add different extra data
		extractor1 := func(_ *http.Request) (interface{}, map[string]interface{}) {
			return int64(100), map[string]interface{}{"key1": "value1"}
		}
		extractor2 := func(_ *http.Request) (interface{}, map[string]interface{}) {
			return nil, map[string]interface{}{"key2": "value2"} // nil ID - tests id == nil branch
		}
		extractor3 := func(_ *http.Request) (interface{}, map[string]interface{}) {
			return int64(200), map[string]interface{}{"key3": "value3"} // Second ID overwrites first
		}

		middleware := authorizer.RequiresResourceAccess("multi", policy.ActionView, extractor1, extractor2, extractor3)
		protectedHandler := middleware(handler)

		// ACT
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		// ASSERT: All extra data should be combined
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "value1", capturedExtra["key1"])
		assert.Equal(t, "value2", capturedExtra["key2"])
		assert.Equal(t, "value3", capturedExtra["key3"])
	})

	t.Run("empty extractors returns nil ID and empty extra", func(t *testing.T) {
		// ARRANGE
		authService := authorize.NewAuthorizationService()
		var capturedResourceID interface{}
		var capturedExtra map[string]interface{}

		capturePolicy := &ResourceCapturePolicy{
			resourceType: "empty",
			onEvaluate: func(ctx *policy.Context) {
				capturedResourceID = ctx.Resource.ID
				capturedExtra = ctx.Extra
			},
		}
		err := authService.RegisterPolicy(capturePolicy)
		require.NoError(t, err)

		authorizer := authorize.NewResourceAuthorizer(authService)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// No extractors
		middleware := authorizer.RequiresResourceAccess("empty", policy.ActionView)
		protectedHandler := middleware(handler)

		// ACT
		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		// ASSERT
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, capturedResourceID)
		assert.Empty(t, capturedExtra)
	})
}

// ExtraDataCapturePolicy captures extra data during evaluation.
type ExtraDataCapturePolicy struct {
	resourceType string
	onEvaluate   func(ctx *policy.Context)
}

func (p *ExtraDataCapturePolicy) Name() string {
	return "extra_capture_" + p.resourceType
}

func (p *ExtraDataCapturePolicy) ResourceType() string {
	return p.resourceType
}

func (p *ExtraDataCapturePolicy) Evaluate(_ context.Context, ctx *policy.Context) (bool, error) {
	if p.onEvaluate != nil {
		p.onEvaluate(ctx)
	}
	return true, nil
}

// ResourceCapturePolicy captures resource data during evaluation.
type ResourceCapturePolicy struct {
	resourceType string
	onEvaluate   func(ctx *policy.Context)
}

func (p *ResourceCapturePolicy) Name() string {
	return "resource_capture_" + p.resourceType
}

func (p *ResourceCapturePolicy) ResourceType() string {
	return p.resourceType
}

func (p *ResourceCapturePolicy) Evaluate(_ context.Context, ctx *policy.Context) (bool, error) {
	if p.onEvaluate != nil {
		p.onEvaluate(ctx)
	}
	return true, nil
}

// failingResponseWriter is a ResponseWriter that fails on Write.
// Used to test error handling paths in handleAuthorizationError and handleForbiddenResponse.
type failingResponseWriter struct {
	header     http.Header
	statusCode int
}

func newFailingResponseWriter() *failingResponseWriter {
	return &failingResponseWriter{
		header: make(http.Header),
	}
}

func (w *failingResponseWriter) Header() http.Header {
	return w.header
}

func (w *failingResponseWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("simulated write failure")
}

func (w *failingResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
}

// TestHandleAuthorizationError tests the error handling path when render fails.
func TestHandleAuthorizationError(t *testing.T) {
	t.Run("falls back to http.Error when render fails", func(t *testing.T) {
		// ARRANGE
		authService := authorize.NewAuthorizationService()
		errorPolicy := &TestAllowPolicy{
			allowResult:  false,
			shouldError:  true,
			errorMsg:     "test authorization error",
			resourceType: "error_test",
		}
		err := authService.RegisterPolicy(errorPolicy)
		require.NoError(t, err)

		authorizer := authorize.NewResourceAuthorizer(authService)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := authorizer.RequiresResourceAccess("error_test", policy.ActionView)
		protectedHandler := middleware(handler)

		// ACT: Use failing response writer to trigger fallback
		req := httptest.NewRequest("GET", "/test", nil)
		fw := newFailingResponseWriter()

		protectedHandler.ServeHTTP(fw, req)

		// ASSERT: Should have set 500 status (from fallback http.Error)
		assert.Equal(t, http.StatusInternalServerError, fw.statusCode)
	})
}

// TestHandleForbiddenResponse tests the forbidden handling path when render fails.
func TestHandleForbiddenResponse(t *testing.T) {
	t.Run("falls back to http.Error when render fails", func(t *testing.T) {
		// ARRANGE
		authService := authorize.NewAuthorizationService()
		denyPolicy := &TestAllowPolicy{
			allowResult:  false,
			shouldError:  false,
			resourceType: "forbidden_test",
		}
		err := authService.RegisterPolicy(denyPolicy)
		require.NoError(t, err)

		authorizer := authorize.NewResourceAuthorizer(authService)

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := authorizer.RequiresResourceAccess("forbidden_test", policy.ActionView)
		protectedHandler := middleware(handler)

		// ACT: Use failing response writer to trigger fallback
		req := httptest.NewRequest("GET", "/test", nil)
		fw := newFailingResponseWriter()

		protectedHandler.ServeHTTP(fw, req)

		// ASSERT: Should have set 403 status (from fallback http.Error)
		assert.Equal(t, http.StatusForbidden, fw.statusCode)
	})
}

// TestGetResourceAuthorizer tests the global authorizer getter.
func TestGetResourceAuthorizer(t *testing.T) {
	t.Run("creates default authorizer when nil", func(t *testing.T) {
		// ARRANGE: Reset global state
		authorize.SetResourceAuthorizer(nil)

		// ACT: Get authorizer when none is set
		authorizer := authorize.GetResourceAuthorizer()

		// ASSERT: Should create a default authorizer
		assert.NotNil(t, authorizer)

		// Verify it works by using it
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		// Create a simple policy to test with
		authService := authorize.NewAuthorizationService()
		allowPolicy := &TestAllowPolicy{
			allowResult:  true,
			resourceType: "default_test",
		}
		err := authService.RegisterPolicy(allowPolicy)
		require.NoError(t, err)

		// Set the custom authorizer for verification
		customAuthorizer := authorize.NewResourceAuthorizer(authService)
		authorize.SetResourceAuthorizer(customAuthorizer)

		// Get again - should return the one we just set
		authorizer2 := authorize.GetResourceAuthorizer()
		assert.Equal(t, customAuthorizer, authorizer2)

		middleware := authorizer2.RequiresResourceAccess("default_test", policy.ActionView)
		protectedHandler := middleware(handler)

		req := httptest.NewRequest("GET", "/test", nil)
		rr := httptest.NewRecorder()
		protectedHandler.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// TestStudentIDFromURL_EmptyParam tests the empty parameter case.
func TestStudentIDFromURL_EmptyParam(t *testing.T) {
	t.Run("handles route without id parameter", func(t *testing.T) {
		// This tests the chi.URLParam returning empty string case
		r := chi.NewRouter()
		r.Get("/students", func(w http.ResponseWriter, r *http.Request) {
			extractor := authorize.StudentIDFromURL()
			id, extra := extractor(r)

			// Both should be nil when parameter doesn't exist
			assert.Nil(t, id)
			assert.Nil(t, extra)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/students", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

// TestURLParamExtractor_MissingParam tests extraction from non-existent parameter.
func TestURLParamExtractor_MissingParam(t *testing.T) {
	t.Run("returns nil for non-existent parameter", func(t *testing.T) {
		r := chi.NewRouter()
		r.Get("/items", func(w http.ResponseWriter, r *http.Request) {
			// Try to extract "id" but route doesn't have it
			extractor := authorize.URLParamExtractor("id")
			id, extra := extractor(r)

			assert.Nil(t, id)
			assert.Nil(t, extra)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest("GET", "/items", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
	})
}
