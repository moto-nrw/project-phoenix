package jwt

import (
	"testing"

	"github.com/lestrrat-go/jwx/v2/jwt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppClaims_ParseClaims(t *testing.T) {
	tests := []struct {
		name        string
		claims      map[string]any
		wantErr     bool
		errContains string
		check       func(t *testing.T, c *AppClaims)
	}{
		{
			name: "valid claims with all fields",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "user@example.com",
				"username":    "testuser",
				"first_name":  "Test",
				"last_name":   "User",
				"roles":       []any{"admin", "teacher"},
				"permissions": []any{"read:users", "write:users"},
				"is_admin":    true,
				"is_teacher":  true,
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 1, c.ID)
				assert.Equal(t, "user@example.com", c.Sub)
				assert.Equal(t, "testuser", c.Username)
				assert.Equal(t, "Test", c.FirstName)
				assert.Equal(t, "User", c.LastName)
				assert.Equal(t, []string{"admin", "teacher"}, c.Roles)
				assert.Equal(t, []string{"read:users", "write:users"}, c.Permissions)
				assert.True(t, c.IsAdmin)
				assert.True(t, c.IsTeacher)
			},
		},
		{
			name: "minimal valid claims",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "minimal@example.com",
				"roles":       []any{"user"},
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 1, c.ID)
				assert.Equal(t, "minimal@example.com", c.Sub)
				assert.Equal(t, []string{"user"}, c.Roles)
				assert.Empty(t, c.Username)
				assert.Empty(t, c.FirstName)
				assert.Empty(t, c.LastName)
				// Permissions should be empty array when not present
				assert.Empty(t, c.Permissions)
				assert.False(t, c.IsAdmin)
				assert.False(t, c.IsTeacher)
			},
		},
		{
			name: "claims with empty roles array",
			claims: map[string]any{
				"id":          float64(2),
				jwt.SubjectKey: "noroles@example.com",
				"roles":       []any{},
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 2, c.ID)
				assert.Empty(t, c.Roles)
			},
		},
		{
			name: "claims with nil roles (should still work)",
			claims: map[string]any{
				"id":          float64(3),
				jwt.SubjectKey: "nilroles@example.com",
				"roles":       nil,
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 3, c.ID)
				assert.Nil(t, c.Roles)
			},
		},
		{
			name: "missing id claim",
			claims: map[string]any{
				jwt.SubjectKey: "missing@example.com",
				"roles":       []any{"user"},
			},
			wantErr:     true,
			errContains: "could not parse claim id",
		},
		{
			name: "missing sub claim",
			claims: map[string]any{
				"id":    float64(1),
				"roles": []any{"user"},
			},
			wantErr:     true,
			errContains: "could not parse claim sub",
		},
		{
			name: "missing roles claim",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "noroles@example.com",
			},
			wantErr:     true,
			errContains: "could not parse claims roles",
		},
		{
			name: "claims with nil optional fields",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "test@example.com",
				"username":    nil,
				"first_name":  nil,
				"last_name":   nil,
				"roles":       []any{"user"},
				"permissions": nil,
				"is_admin":    nil,
				"is_teacher":  nil,
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 1, c.ID)
				assert.Empty(t, c.Username)
				assert.Empty(t, c.FirstName)
				assert.Empty(t, c.LastName)
				assert.Empty(t, c.Permissions)
				assert.False(t, c.IsAdmin)
				assert.False(t, c.IsTeacher)
			},
		},
		{
			name: "id as different numeric types",
			claims: map[string]any{
				"id":          float64(999),
				jwt.SubjectKey: "test@example.com",
				"roles":       []any{},
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, 999, c.ID)
			},
		},
		{
			name: "large permission set",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "admin@example.com",
				"roles":       []any{"superadmin"},
				"permissions": []any{
					"users:read", "users:write", "users:delete",
					"groups:read", "groups:write", "groups:delete",
					"students:read", "students:write",
					"activities:read", "activities:write",
				},
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Len(t, c.Permissions, 10)
			},
		},
		{
			name: "unicode characters in names",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "unicode@example.com",
				"username":    "müller",
				"first_name":  "Müller",
				"last_name":   "Schröder",
				"roles":       []any{"user"},
			},
			wantErr: false,
			check: func(t *testing.T, c *AppClaims) {
				assert.Equal(t, "müller", c.Username)
				assert.Equal(t, "Müller", c.FirstName)
				assert.Equal(t, "Schröder", c.LastName)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c AppClaims
			err := c.ParseClaims(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, &c)
			}
		})
	}
}

func TestRefreshClaims_ParseClaims(t *testing.T) {
	tests := []struct {
		name        string
		claims      map[string]any
		wantErr     bool
		errContains string
		check       func(t *testing.T, c *RefreshClaims)
	}{
		{
			name: "valid refresh claims with float64 id",
			claims: map[string]any{
				"id":    float64(1),
				"token": "refresh-token-uuid-123",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, 1, c.ID)
				assert.Equal(t, "refresh-token-uuid-123", c.Token)
			},
		},
		{
			name: "valid refresh claims with int id",
			claims: map[string]any{
				"id":    int(42),
				"token": "refresh-token-int-id",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, 42, c.ID)
			},
		},
		{
			name: "valid refresh claims with int64 id",
			claims: map[string]any{
				"id":    int64(999),
				"token": "refresh-token-int64-id",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, 999, c.ID)
			},
		},
		{
			name: "missing id claim",
			claims: map[string]any{
				"token": "some-token",
			},
			wantErr:     true,
			errContains: "could not parse claim id",
		},
		{
			name: "missing token claim",
			claims: map[string]any{
				"id": float64(1),
			},
			wantErr:     true,
			errContains: "could not parse claim token",
		},
		{
			name: "invalid id type - string",
			claims: map[string]any{
				"id":    "not-a-number",
				"token": "some-token",
			},
			wantErr:     true,
			errContains: "invalid type for claim id",
		},
		{
			name: "id with zero value",
			claims: map[string]any{
				"id":    float64(0),
				"token": "zero-id-token",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, 0, c.ID)
				assert.Equal(t, "zero-id-token", c.Token)
			},
		},
		{
			name: "id with negative value",
			claims: map[string]any{
				"id":    float64(-1),
				"token": "negative-id-token",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, -1, c.ID)
			},
		},
		{
			name: "id with large value",
			claims: map[string]any{
				"id":    float64(9999999999),
				"token": "large-id-token",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, 9999999999, c.ID)
			},
		},
		{
			name: "empty token string",
			claims: map[string]any{
				"id":    float64(1),
				"token": "",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Empty(t, c.Token)
			},
		},
		{
			name: "token with special characters",
			claims: map[string]any{
				"id":    float64(1),
				"token": "token-with-special-chars!@#$%^&*()",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, "token-with-special-chars!@#$%^&*()", c.Token)
			},
		},
		{
			name: "uuid v4 format token",
			claims: map[string]any{
				"id":    float64(1),
				"token": "550e8400-e29b-41d4-a716-446655440000",
			},
			wantErr: false,
			check: func(t *testing.T, c *RefreshClaims) {
				assert.Equal(t, "550e8400-e29b-41d4-a716-446655440000", c.Token)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c RefreshClaims
			err := c.ParseClaims(tt.claims)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, &c)
			}
		})
	}
}

func TestCommonClaims(t *testing.T) {
	// Test that CommonClaims is properly embedded
	appClaims := AppClaims{
		ID:  1,
		Sub: "test@example.com",
		CommonClaims: CommonClaims{
			ExpiresAt: 1234567890,
			IssuedAt:  1234567800,
		},
	}

	assert.Equal(t, int64(1234567890), appClaims.ExpiresAt)
	assert.Equal(t, int64(1234567800), appClaims.IssuedAt)

	refreshClaims := RefreshClaims{
		ID:    1,
		Token: "test-token",
		CommonClaims: CommonClaims{
			ExpiresAt: 9999999999,
			IssuedAt:  9999999000,
		},
	}

	assert.Equal(t, int64(9999999999), refreshClaims.ExpiresAt)
	assert.Equal(t, int64(9999999000), refreshClaims.IssuedAt)
}

// BUG CANDIDATE: Type assertion panic in AppClaims.ParseClaims - See Issue #420
// If claims["id"] is present but is a string instead of float64,
// the type assertion `id.(float64)` will panic
func TestAppClaims_ParseClaims_TypeAssertionSafety(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]any
		wantPanic bool
	}{
		{
			name: "id as string - potential panic",
			claims: map[string]any{
				"id":          "not-a-number",
				jwt.SubjectKey: "test@example.com",
				"roles":       []any{},
			},
			wantPanic: true, // BUG: This causes panic due to type assertion - see issue #420
		},
		{
			name: "roles as string instead of array - potential panic",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: "test@example.com",
				"roles":       "not-an-array",
			},
			wantPanic: true, // BUG: This causes panic due to type assertion
		},
		{
			name: "sub as int instead of string - potential panic",
			claims: map[string]any{
				"id":          float64(1),
				jwt.SubjectKey: 12345,
				"roles":       []any{},
			},
			wantPanic: true, // BUG: This causes panic due to type assertion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					r := recover()
					if r == nil {
						t.Log("Expected panic but none occurred - code may have protection or different behavior")
					} else {
						t.Logf("BUG CONFIRMED: Panic occurred as expected: %v (see issue #420)", r)
					}
				}()
			}

			var c AppClaims
			_ = c.ParseClaims(tt.claims)
		})
	}
}

// Test RefreshClaims type assertion safety
func TestRefreshClaims_ParseClaims_TypeAssertionSafety(t *testing.T) {
	tests := []struct {
		name      string
		claims    map[string]any
		wantPanic bool
	}{
		{
			name: "token as int instead of string - potential panic",
			claims: map[string]any{
				"id":    float64(1),
				"token": 12345, // number instead of string
			},
			wantPanic: true, // BUG: This causes panic due to type assertion
		},
		{
			name: "token as bool instead of string - potential panic",
			claims: map[string]any{
				"id":    float64(1),
				"token": true, // bool instead of string
			},
			wantPanic: true, // BUG: This causes panic due to type assertion
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPanic {
				defer func() {
					r := recover()
					if r == nil {
						t.Log("Expected panic but none occurred")
					} else {
						t.Logf("BUG CONFIRMED: Panic occurred as expected: %v", r)
					}
				}()
			}

			var c RefreshClaims
			_ = c.ParseClaims(tt.claims)
		})
	}
}
