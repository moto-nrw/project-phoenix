package jwt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Helper Functions Tests
// =============================================================================

func TestGetRequiredInt(t *testing.T) {
	t.Run("valid float64 value", func(t *testing.T) {
		claims := map[string]any{"id": float64(42)}
		val, err := getRequiredInt(claims, "id")
		require.NoError(t, err)
		assert.Equal(t, 42, val)
	})

	t.Run("missing key", func(t *testing.T) {
		claims := map[string]any{}
		_, err := getRequiredInt(claims, "id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required claim")
	})

	t.Run("wrong type string", func(t *testing.T) {
		claims := map[string]any{"id": "not a number"}
		_, err := getRequiredInt(claims, "id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a number")
	})

	t.Run("wrong type bool", func(t *testing.T) {
		claims := map[string]any{"id": true}
		_, err := getRequiredInt(claims, "id")
		assert.Error(t, err)
	})

	t.Run("zero value", func(t *testing.T) {
		claims := map[string]any{"id": float64(0)}
		val, err := getRequiredInt(claims, "id")
		require.NoError(t, err)
		assert.Equal(t, 0, val)
	})

	t.Run("negative value", func(t *testing.T) {
		claims := map[string]any{"id": float64(-5)}
		val, err := getRequiredInt(claims, "id")
		require.NoError(t, err)
		assert.Equal(t, -5, val)
	})
}

func TestGetRequiredString(t *testing.T) {
	t.Run("valid string", func(t *testing.T) {
		claims := map[string]any{"sub": "user@example.com"}
		val, err := getRequiredString(claims, "sub")
		require.NoError(t, err)
		assert.Equal(t, "user@example.com", val)
	})

	t.Run("missing key", func(t *testing.T) {
		claims := map[string]any{}
		_, err := getRequiredString(claims, "sub")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required claim")
	})

	t.Run("wrong type int", func(t *testing.T) {
		claims := map[string]any{"sub": 123}
		_, err := getRequiredString(claims, "sub")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a string")
	})

	t.Run("empty string is valid", func(t *testing.T) {
		claims := map[string]any{"sub": ""}
		val, err := getRequiredString(claims, "sub")
		require.NoError(t, err)
		assert.Equal(t, "", val)
	})
}

func TestGetOptionalString(t *testing.T) {
	t.Run("present value", func(t *testing.T) {
		claims := map[string]any{"username": "john"}
		val := getOptionalString(claims, "username")
		assert.Equal(t, "john", val)
	})

	t.Run("missing key returns empty", func(t *testing.T) {
		claims := map[string]any{}
		val := getOptionalString(claims, "username")
		assert.Equal(t, "", val)
	})

	t.Run("nil value returns empty", func(t *testing.T) {
		claims := map[string]any{"username": nil}
		val := getOptionalString(claims, "username")
		assert.Equal(t, "", val)
	})

	t.Run("wrong type returns empty", func(t *testing.T) {
		claims := map[string]any{"username": 123}
		val := getOptionalString(claims, "username")
		assert.Equal(t, "", val)
	})
}

func TestGetOptionalBool(t *testing.T) {
	t.Run("true value", func(t *testing.T) {
		claims := map[string]any{"is_admin": true}
		val := getOptionalBool(claims, "is_admin")
		assert.True(t, val)
	})

	t.Run("false value", func(t *testing.T) {
		claims := map[string]any{"is_admin": false}
		val := getOptionalBool(claims, "is_admin")
		assert.False(t, val)
	})

	t.Run("missing key returns false", func(t *testing.T) {
		claims := map[string]any{}
		val := getOptionalBool(claims, "is_admin")
		assert.False(t, val)
	})

	t.Run("nil value returns false", func(t *testing.T) {
		claims := map[string]any{"is_admin": nil}
		val := getOptionalBool(claims, "is_admin")
		assert.False(t, val)
	})

	t.Run("wrong type returns false", func(t *testing.T) {
		claims := map[string]any{"is_admin": "true"}
		val := getOptionalBool(claims, "is_admin")
		assert.False(t, val)
	})
}

func TestGetRequiredStringSlice(t *testing.T) {
	t.Run("valid string array", func(t *testing.T) {
		claims := map[string]any{"roles": []any{"admin", "user"}}
		val, err := getRequiredStringSlice(claims, "roles")
		require.NoError(t, err)
		assert.Equal(t, []string{"admin", "user"}, val)
	})

	t.Run("empty array", func(t *testing.T) {
		claims := map[string]any{"roles": []any{}}
		val, err := getRequiredStringSlice(claims, "roles")
		require.NoError(t, err)
		assert.Equal(t, []string{}, val)
	})

	t.Run("missing key", func(t *testing.T) {
		claims := map[string]any{}
		_, err := getRequiredStringSlice(claims, "roles")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing required claim")
	})

	t.Run("non-array value", func(t *testing.T) {
		claims := map[string]any{"roles": "admin"}
		_, err := getRequiredStringSlice(claims, "roles")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not an array")
	})

	t.Run("array with non-string element", func(t *testing.T) {
		claims := map[string]any{"roles": []any{"admin", 123}}
		_, err := getRequiredStringSlice(claims, "roles")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a string")
	})
}

func TestGetOptionalStringSlice(t *testing.T) {
	t.Run("valid string array", func(t *testing.T) {
		claims := map[string]any{"permissions": []any{"read", "write"}}
		val := getOptionalStringSlice(claims, "permissions")
		assert.Equal(t, []string{"read", "write"}, val)
	})

	t.Run("missing key returns empty slice", func(t *testing.T) {
		claims := map[string]any{}
		val := getOptionalStringSlice(claims, "permissions")
		assert.Equal(t, []string{}, val)
	})

	t.Run("nil value returns empty slice", func(t *testing.T) {
		claims := map[string]any{"permissions": nil}
		val := getOptionalStringSlice(claims, "permissions")
		assert.Equal(t, []string{}, val)
	})

	t.Run("non-array returns empty slice", func(t *testing.T) {
		claims := map[string]any{"permissions": "read"}
		val := getOptionalStringSlice(claims, "permissions")
		assert.Equal(t, []string{}, val)
	})

	t.Run("array with mixed types skips non-strings", func(t *testing.T) {
		claims := map[string]any{"permissions": []any{"read", 123, "write", true}}
		val := getOptionalStringSlice(claims, "permissions")
		assert.Equal(t, []string{"read", "write"}, val)
	})
}

// =============================================================================
// toStringSlice Tests
// =============================================================================

func TestToStringSliceStrict(t *testing.T) {
	t.Run("valid slice", func(t *testing.T) {
		val, err := toStringSliceStrict([]any{"a", "b", "c"})
		require.NoError(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, val)
	})

	t.Run("empty slice", func(t *testing.T) {
		val, err := toStringSliceStrict([]any{})
		require.NoError(t, err)
		assert.Equal(t, []string{}, val)
	})

	t.Run("non-slice value", func(t *testing.T) {
		_, err := toStringSliceStrict("not a slice")
		assert.Error(t, err)
	})

	t.Run("mixed types fails", func(t *testing.T) {
		_, err := toStringSliceStrict([]any{"string", 123})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "element 1 is not a string")
	})
}

func TestToStringSliceLenient(t *testing.T) {
	t.Run("valid slice", func(t *testing.T) {
		val := toStringSliceLenient([]any{"a", "b", "c"})
		assert.Equal(t, []string{"a", "b", "c"}, val)
	})

	t.Run("mixed types skips non-strings", func(t *testing.T) {
		val := toStringSliceLenient([]any{"a", 123, "b", nil, "c"})
		assert.Equal(t, []string{"a", "b", "c"}, val)
	})

	t.Run("non-slice returns empty", func(t *testing.T) {
		val := toStringSliceLenient("not a slice")
		assert.Equal(t, []string{}, val)
	})

	t.Run("all non-strings returns empty", func(t *testing.T) {
		val := toStringSliceLenient([]any{1, 2, 3})
		assert.Equal(t, []string{}, val)
	})
}

// =============================================================================
// AppClaims.ParseClaims Tests
// =============================================================================

func TestAppClaims_ParseClaims_ValidClaims(t *testing.T) {
	claims := map[string]any{
		"id":          float64(42),
		"sub":         "user@example.com",
		"username":    "johndoe",
		"first_name":  "John",
		"last_name":   "Doe",
		"roles":       []any{"admin", "user"},
		"permissions": []any{"read", "write", "delete"},
		"is_admin":    true,
		"is_teacher":  false,
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)

	assert.Equal(t, 42, c.ID)
	assert.Equal(t, "user@example.com", c.Sub)
	assert.Equal(t, "johndoe", c.Username)
	assert.Equal(t, "John", c.FirstName)
	assert.Equal(t, "Doe", c.LastName)
	assert.Equal(t, []string{"admin", "user"}, c.Roles)
	assert.Equal(t, []string{"read", "write", "delete"}, c.Permissions)
	assert.True(t, c.IsAdmin)
}

func TestAppClaims_ParseClaims_MinimalClaims(t *testing.T) {
	// Only required fields
	claims := map[string]any{
		"id":    float64(1),
		"sub":   "test@test.com",
		"roles": []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)

	assert.Equal(t, 1, c.ID)
	assert.Equal(t, "test@test.com", c.Sub)
	assert.Equal(t, []string{"user"}, c.Roles)
	// Optional fields should be empty/default
	assert.Empty(t, c.Username)
	assert.Empty(t, c.FirstName)
	assert.Empty(t, c.LastName)
	assert.Empty(t, c.Permissions)
	assert.False(t, c.IsAdmin)
}

func TestAppClaims_ParseClaims_MissingID(t *testing.T) {
	claims := map[string]any{
		"sub":   "test@test.com",
		"roles": []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestAppClaims_ParseClaims_MissingSub(t *testing.T) {
	claims := map[string]any{
		"id":    float64(1),
		"roles": []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sub")
}

func TestAppClaims_ParseClaims_MissingRoles(t *testing.T) {
	claims := map[string]any{
		"id":  float64(1),
		"sub": "test@test.com",
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "roles")
}

func TestAppClaims_ParseClaims_InvalidIDType(t *testing.T) {
	claims := map[string]any{
		"id":    "not-a-number",
		"sub":   "test@test.com",
		"roles": []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
}

func TestAppClaims_ParseClaims_InvalidRolesType(t *testing.T) {
	claims := map[string]any{
		"id":    float64(1),
		"sub":   "test@test.com",
		"roles": "admin", // Should be array
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
}

func TestAppClaims_ParseClaims_PermissionsWithInvalidElements(t *testing.T) {
	// Permissions are optional and use lenient parsing
	claims := map[string]any{
		"id":          float64(1),
		"sub":         "test@test.com",
		"roles":       []any{"user"},
		"permissions": []any{"read", 123, "write"}, // Mixed types
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)
	// Should only include valid strings
	assert.Equal(t, []string{"read", "write"}, c.Permissions)
}

// =============================================================================
// RefreshClaims.ParseClaims Tests
// =============================================================================

func TestRefreshClaims_ParseClaims_ValidClaims(t *testing.T) {
	claims := map[string]any{
		"id":    float64(42),
		"token": "refresh-token-value",
	}

	var c RefreshClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)

	assert.Equal(t, 42, c.ID)
	assert.Equal(t, "refresh-token-value", c.Token)
}

func TestRefreshClaims_ParseClaims_IntTypes(t *testing.T) {
	// Test different numeric types for ID
	testCases := []struct {
		name string
		id   any
	}{
		{"float64", float64(42)},
		{"int", int(42)},
		{"int64", int64(42)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			claims := map[string]any{
				"id":    tc.id,
				"token": "token",
			}

			var c RefreshClaims
			err := c.ParseClaims(claims)
			require.NoError(t, err)
			assert.Equal(t, 42, c.ID)
		})
	}
}

func TestRefreshClaims_ParseClaims_MissingID(t *testing.T) {
	claims := map[string]any{
		"token": "token",
	}

	var c RefreshClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "id")
}

func TestRefreshClaims_ParseClaims_MissingToken(t *testing.T) {
	claims := map[string]any{
		"id": float64(1),
	}

	var c RefreshClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token")
}

func TestRefreshClaims_ParseClaims_InvalidIDType(t *testing.T) {
	claims := map[string]any{
		"id":    "not-a-number",
		"token": "token",
	}

	var c RefreshClaims
	err := c.ParseClaims(claims)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid type")
}

// =============================================================================
// Edge Cases and Security Tests
// =============================================================================

func TestAppClaims_ParseClaims_EmptyRolesArray(t *testing.T) {
	claims := map[string]any{
		"id":    float64(1),
		"sub":   "test@test.com",
		"roles": []any{}, // Empty but present
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)
	assert.Empty(t, c.Roles)
}

func TestAppClaims_ParseClaims_LargeID(t *testing.T) {
	// Test with large ID values
	claims := map[string]any{
		"id":    float64(9223372036854775807), // Max int64 as float
		"sub":   "test@test.com",
		"roles": []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)
	// Note: precision may be lost for very large floats
}

func TestAppClaims_ParseClaims_SpecialCharactersInStrings(t *testing.T) {
	claims := map[string]any{
		"id":         float64(1),
		"sub":        "user+test@example.com",
		"username":   "user_with-special.chars",
		"first_name": "José",
		"last_name":  "O'Connor",
		"roles":      []any{"user"},
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)

	assert.Equal(t, "user+test@example.com", c.Sub)
	assert.Equal(t, "user_with-special.chars", c.Username)
	assert.Equal(t, "José", c.FirstName)
	assert.Equal(t, "O'Connor", c.LastName)
}

func TestAppClaims_ParseClaims_NilValues(t *testing.T) {
	claims := map[string]any{
		"id":          float64(1),
		"sub":         "test@test.com",
		"roles":       []any{"user"},
		"username":    nil, // nil optional field
		"permissions": nil, // nil optional array
		"is_admin":    nil, // nil optional bool
	}

	var c AppClaims
	err := c.ParseClaims(claims)
	require.NoError(t, err)

	assert.Empty(t, c.Username)
	assert.Empty(t, c.Permissions)
	assert.False(t, c.IsAdmin)
}

// =============================================================================
// IsPlatformScope Tests
// =============================================================================

func TestAppClaims_IsPlatformScope_True(t *testing.T) {
	c := &AppClaims{Scope: "platform"}
	assert.True(t, c.IsPlatformScope())
}

func TestAppClaims_IsPlatformScope_False_EmptyString(t *testing.T) {
	c := &AppClaims{Scope: ""}
	assert.False(t, c.IsPlatformScope())
}

func TestAppClaims_IsPlatformScope_False_Tenant(t *testing.T) {
	c := &AppClaims{Scope: "tenant"}
	assert.False(t, c.IsPlatformScope())
}

func TestAppClaims_IsPlatformScope_False_Other(t *testing.T) {
	c := &AppClaims{Scope: "other"}
	assert.False(t, c.IsPlatformScope())
}
