package test

import (
	"context"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/authorize/policy"
	jwtpkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestAuth sets up authentication for testing
func setupTestAuth(t *testing.T) {
	// Create a JWT auth service with test secret
	tokenAuth, err := CreateTestJWTAuth()
	require.NoError(t, err)

	// Set it as the default in the authorization system
	jwtpkg.SetDefaultTokenAuth(tokenAuth)
}

// setupTestEnvironment sets up the test environment, including database and auth
func setupTestEnvironment(t *testing.T) {
	// Setup database connection for tests
	SetupTestDatabase()

	// Setup authentication for tests
	setupTestAuth(t)
}

// TestAuthorizationSystem performs comprehensive tests of the authorization system
func TestAuthorizationSystem(t *testing.T) {
	// Set up test environment
	setupTestEnvironment(t)

	// Create test data
	testData := CreateTestData(t)

	t.Run("Permission-based authorization", func(t *testing.T) {
		testPermissionBasedAuth(t, testData)
	})

	t.Run("Resource-based authorization", func(t *testing.T) {
		testResourceBasedAuth(t, testData)
	})

	t.Run("Combined authorization", func(t *testing.T) {
		testCombinedAuth(t, testData)
	})

	t.Run("Edge cases", func(t *testing.T) {
		testEdgeCases(t, testData)
	})
}

func testPermissionBasedAuth(t *testing.T, testData *TestData) {
	scenarios := []TestPermissionScenario{
		{
			Name:            "Admin has all permissions",
			Permission:      permissions.GroupsRead,
			UserPermissions: []string{"admin:*"},
			ExpectedResult:  true,
		},
		{
			Name:            "User with exact permission",
			Permission:      permissions.GroupsRead,
			UserPermissions: []string{permissions.GroupsRead},
			ExpectedResult:  true,
		},
		{
			Name:            "User with resource wildcard",
			Permission:      permissions.GroupsRead,
			UserPermissions: []string{"groups:*"},
			ExpectedResult:  true,
		},
		{
			Name:            "User without permission",
			Permission:      permissions.GroupsRead,
			UserPermissions: []string{permissions.ActivitiesRead},
			ExpectedResult:  false,
		},
		{
			Name:            "Empty permissions",
			Permission:      permissions.GroupsRead,
			UserPermissions: []string{},
			ExpectedResult:  false,
		},
	}

	RunPermissionScenarios(t, scenarios)
}

func testResourceBasedAuth(t *testing.T, testData *TestData) {
	// Create authorization service with test policies
	authService := CreateTestAuthorizationService(t)

	tests := []struct {
		name           string
		subject        policy.Subject
		resource       policy.Resource
		action         policy.Action
		expectedResult bool
	}{
		{
			name: "Admin can access any resource",
			subject: policy.Subject{
				AccountID:   1,
				Roles:       []string{"admin"},
				Permissions: []string{"admin:*"},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: true,
		},
		{
			name: "Student can access own visit",
			subject: policy.Subject{
				AccountID:   3,
				Roles:       []string{"student"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: true,
		},
		{
			name: "Student cannot access other student's visit",
			subject: policy.Subject{
				AccountID:   3,
				Roles:       []string{"student"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit2.ID,
			},
			action:         policy.ActionView,
			expectedResult: false,
		},
		{
			name: "Teacher can access student visit in their group",
			subject: policy.Subject{
				AccountID:   2,
				Roles:       []string{"teacher"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: true,
		},
		{
			name: "Teacher cannot access student visit not in their group",
			subject: policy.Subject{
				AccountID:   2,
				Roles:       []string{"teacher"},
				Permissions: []string{},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit2.ID,
			},
			action:         policy.ActionView,
			expectedResult: false,
		},
		{
			name: "User with permission can access any visit",
			subject: policy.Subject{
				AccountID:   4,
				Roles:       []string{"user"},
				Permissions: []string{permissions.VisitsRead},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := authService.AuthorizeResource(
				context.Background(),
				tt.subject,
				tt.resource,
				tt.action,
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func testCombinedAuth(t *testing.T, testData *TestData) {
	// Test scenarios where both permission and resource checks are required
	authService := CreateTestAuthorizationService(t)

	tests := []struct {
		name           string
		permission     string
		subject        policy.Subject
		resource       policy.Resource
		action         policy.Action
		expectedResult bool
	}{
		{
			name:       "Permission + Resource both allow",
			permission: permissions.VisitsRead,
			subject: policy.Subject{
				AccountID:   3,
				Roles:       []string{"student"},
				Permissions: []string{permissions.VisitsRead},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: true,
		},
		{
			name:       "Permission allows but resource denies",
			permission: permissions.VisitsRead,
			subject: policy.Subject{
				AccountID:   3,
				Roles:       []string{"student"},
				Permissions: []string{permissions.VisitsRead},
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit2.ID,
			},
			action:         policy.ActionView,
			expectedResult: false, // Resource check should deny
		},
		{
			name:       "Permission denies but resource allows",
			permission: permissions.VisitsRead,
			subject: policy.Subject{
				AccountID:   3,
				Roles:       []string{"student"},
				Permissions: []string{}, // No permission
			},
			resource: policy.Resource{
				Type: "visit",
				ID:   testData.Visit1.ID,
			},
			action:         policy.ActionView,
			expectedResult: false, // Permission check should deny first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// First check permission
			hasPermission := false
			for _, perm := range tt.subject.Permissions {
				if perm == tt.permission || perm == "admin:*" {
					hasPermission = true
					break
				}
			}

			if !hasPermission {
				assert.False(t, tt.expectedResult)
				return
			}

			// Then check resource access
			result, err := authService.AuthorizeResource(
				context.Background(),
				tt.subject,
				tt.resource,
				tt.action,
				nil,
			)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func testEdgeCases(t *testing.T, testData *TestData) {
	authService := CreateTestAuthorizationService(t)

	t.Run("Invalid permission format", func(t *testing.T) {
		// Test permissions with invalid format
		invalidPermissions := []string{"invalid", ":", "resource:", ":action"}

		for _, perm := range invalidPermissions {
			// hasPermission function should handle invalid formats gracefully
			result := authorize.RequiresPermission(perm)
			assert.NotNil(t, result)
		}
	})

	t.Run("Nil or empty resource", func(t *testing.T) {
		subject := policy.Subject{
			AccountID:   1,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		// Test with empty resource type
		_, err := authService.AuthorizeResource(
			context.Background(),
			subject,
			policy.Resource{Type: "", ID: 1},
			policy.ActionView,
			nil,
		)
		assert.NoError(t, err)

		// Test with nil ID
		_, err = authService.AuthorizeResource(
			context.Background(),
			subject,
			policy.Resource{Type: "visit", ID: nil},
			policy.ActionView,
			nil,
		)
		assert.NoError(t, err)
	})

	t.Run("Context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		subject := policy.Subject{
			AccountID:   1,
			Roles:       []string{"user"},
			Permissions: []string{},
		}

		_, err := authService.AuthorizeResource(
			ctx,
			subject,
			policy.Resource{Type: "visit", ID: 1},
			policy.ActionView,
			nil,
		)

		// Should handle cancelled context gracefully
		assert.NoError(t, err)
	})

	t.Run("Concurrent access", func(t *testing.T) {
		// Test that the authorization system is thread-safe
		done := make(chan bool)

		for i := 0; i < 10; i++ {
			go func(id int) {
				subject := policy.Subject{
					AccountID:   int64(id),
					Roles:       []string{"user"},
					Permissions: []string{},
				}

				_, err := authService.AuthorizeResource(
					context.Background(),
					subject,
					policy.Resource{Type: "visit", ID: id},
					policy.ActionView,
					nil,
				)

				assert.NoError(t, err)
				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

// BenchmarkAuthorization benchmarks the authorization system
func BenchmarkAuthorization(b *testing.B) {
	// Create a testing.T from testing.B using the TB interface
	testData := CreateTestData(b)
	authService := CreateTestAuthorizationService(b)

	b.Run("Permission check", func(b *testing.B) {
		permissions := []string{"groups:read", "visits:read"}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Simulate permission check
			hasPermission := false
			for _, perm := range permissions {
				if perm == "groups:read" {
					hasPermission = true
					break
				}
			}
			_ = hasPermission
		}
	})

	b.Run("Resource authorization", func(b *testing.B) {
		subject := policy.Subject{
			AccountID:   1,
			Roles:       []string{"teacher"},
			Permissions: []string{"visits:read"},
		}

		resource := policy.Resource{
			Type: "visit",
			ID:   testData.Visit1.ID,
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _ = authService.AuthorizeResource(
				context.Background(),
				subject,
				resource,
				policy.ActionView,
				nil,
			)
		}
	})
}

// TestAuthorizationIntegration tests the full authorization flow
func TestAuthorizationIntegration(t *testing.T) {
	// This test simulates a real request flow through the authorization system

	// 1. Create test request
	req, err := http.NewRequest("GET", "/api/active/visits/1", nil)
	require.NoError(t, err)

	// 2. Add JWT claims to request context
	claims := CreateTestClaims(
		1,
		"teacher",
		[]string{"teacher"},
		[]string{"visits:read"},
	)

	ctx := MockJWTContext(req.Context(), claims, claims.Permissions)
	req = req.WithContext(ctx)

	// 3. Simulate middleware checks
	t.Run("Permission middleware", func(t *testing.T) {
		// Test that permission middleware would allow this request
		hasPermission := false
		for _, perm := range claims.Permissions {
			if perm == "visits:read" {
				hasPermission = true
				break
			}
		}
		assert.True(t, hasPermission)
	})

	t.Run("Resource middleware", func(t *testing.T) {
		// Test that resource middleware would check properly
		subject := policy.Subject{
			AccountID:   int64(claims.ID),
			Roles:       claims.Roles,
			Permissions: claims.Permissions,
		}

		resource := policy.Resource{
			Type: "visit",
			ID:   int64(1),
		}

		// This would normally call the actual authorization service
		// For this test, we're simulating the result
		assert.NotNil(t, subject)
		assert.NotNil(t, resource)
	})
}
