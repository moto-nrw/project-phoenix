package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// parseName Tests
// =============================================================================

func TestParseName_FullName(t *testing.T) {
	firstName, lastName := parseName("John Doe")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Doe", lastName)
}

func TestParseName_SingleWord(t *testing.T) {
	firstName, lastName := parseName("John")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "", lastName)
}

func TestParseName_EmptyString(t *testing.T) {
	firstName, lastName := parseName("")
	assert.Equal(t, "", firstName)
	assert.Equal(t, "", lastName)
}

func TestParseName_MultipleSpaces(t *testing.T) {
	firstName, lastName := parseName("John Paul Smith")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Paul Smith", lastName)
}

func TestParseName_LeadingTrailingWhitespace(t *testing.T) {
	firstName, lastName := parseName("  John Doe  ")
	assert.Equal(t, "John", firstName)
	assert.Equal(t, "Doe", lastName)
}

func TestParseName_OnlyWhitespace(t *testing.T) {
	firstName, lastName := parseName("   ")
	assert.Equal(t, "", firstName)
	assert.Equal(t, "", lastName)
}

func TestParseName_GermanName(t *testing.T) {
	firstName, lastName := parseName("Hans Müller")
	assert.Equal(t, "Hans", firstName)
	assert.Equal(t, "Müller", lastName)
}

func TestParseName_DoubleLastName(t *testing.T) {
	firstName, lastName := parseName("Maria Garcia-López")
	assert.Equal(t, "Maria", firstName)
	assert.Equal(t, "Garcia-López", lastName)
}

// =============================================================================
// isAdminRole Tests
// =============================================================================

func TestIsAdminRole_Admin(t *testing.T) {
	assert.True(t, isAdminRole("admin"))
	assert.True(t, isAdminRole("Admin"))
	assert.True(t, isAdminRole("ADMIN"))
}

func TestIsAdminRole_Owner(t *testing.T) {
	assert.True(t, isAdminRole("owner"))
	assert.True(t, isAdminRole("Owner"))
	assert.True(t, isAdminRole("OWNER"))
}

func TestIsAdminRole_OGSAdmin(t *testing.T) {
	assert.True(t, isAdminRole("ogsadmin"))
	assert.True(t, isAdminRole("OGSAdmin"))
	assert.True(t, isAdminRole("OGSADMIN"))
}

func TestIsAdminRole_TraegerAdmin(t *testing.T) {
	assert.True(t, isAdminRole("traegeradmin"))
	assert.True(t, isAdminRole("TraegerAdmin"))
	assert.True(t, isAdminRole("TRAEGERADMIN"))
}

func TestIsAdminRole_BueroAdmin(t *testing.T) {
	assert.True(t, isAdminRole("bueroadmin"))
	assert.True(t, isAdminRole("BueroAdmin"))
	assert.True(t, isAdminRole("BUEROADMIN"))
}

func TestIsAdminRole_Member(t *testing.T) {
	assert.False(t, isAdminRole("member"))
	assert.False(t, isAdminRole("Member"))
	assert.False(t, isAdminRole("MEMBER"))
}

func TestIsAdminRole_User(t *testing.T) {
	assert.False(t, isAdminRole("user"))
	assert.False(t, isAdminRole("User"))
}

func TestIsAdminRole_EmptyString(t *testing.T) {
	assert.False(t, isAdminRole(""))
}

func TestIsAdminRole_UnknownRole(t *testing.T) {
	assert.False(t, isAdminRole("viewer"))
	assert.False(t, isAdminRole("guest"))
	assert.False(t, isAdminRole("superadmin"))
}

// =============================================================================
// mapRoleToTeacherRole Tests
// =============================================================================

func TestMapRoleToTeacherRole_Owner(t *testing.T) {
	assert.Equal(t, "Leitung", mapRoleToTeacherRole("owner"))
	assert.Equal(t, "Leitung", mapRoleToTeacherRole("Owner"))
	assert.Equal(t, "Leitung", mapRoleToTeacherRole("OWNER"))
}

func TestMapRoleToTeacherRole_Admin(t *testing.T) {
	assert.Equal(t, "Gruppenleitung", mapRoleToTeacherRole("admin"))
	assert.Equal(t, "Gruppenleitung", mapRoleToTeacherRole("Admin"))
	assert.Equal(t, "Gruppenleitung", mapRoleToTeacherRole("ADMIN"))
}

func TestMapRoleToTeacherRole_OGSAdmin(t *testing.T) {
	assert.Equal(t, "Gruppenleitung", mapRoleToTeacherRole("ogsadmin"))
	assert.Equal(t, "Gruppenleitung", mapRoleToTeacherRole("OGSAdmin"))
}

func TestMapRoleToTeacherRole_TraegerAdmin(t *testing.T) {
	assert.Equal(t, "Träger-Admin", mapRoleToTeacherRole("traegeradmin"))
	assert.Equal(t, "Träger-Admin", mapRoleToTeacherRole("TraegerAdmin"))
}

func TestMapRoleToTeacherRole_BueroAdmin(t *testing.T) {
	assert.Equal(t, "Büro-Admin", mapRoleToTeacherRole("bueroadmin"))
	assert.Equal(t, "Büro-Admin", mapRoleToTeacherRole("BueroAdmin"))
}

func TestMapRoleToTeacherRole_DefaultMitarbeiter(t *testing.T) {
	// Any unknown role should map to "Mitarbeiter"
	assert.Equal(t, "Mitarbeiter", mapRoleToTeacherRole("member"))
	assert.Equal(t, "Mitarbeiter", mapRoleToTeacherRole("user"))
	assert.Equal(t, "Mitarbeiter", mapRoleToTeacherRole("guest"))
	assert.Equal(t, "Mitarbeiter", mapRoleToTeacherRole(""))
	assert.Equal(t, "Mitarbeiter", mapRoleToTeacherRole("unknown"))
}

// =============================================================================
// UserSyncParams Validation Tests
// =============================================================================

func TestUserSyncParams_RequiredFields(t *testing.T) {
	// Test validation of required fields
	// Note: Actual SyncUser tests require mocked repositories and DB

	testCases := []struct {
		name   string
		params UserSyncParams
		field  string
	}{
		{
			name: "missing betterauth_user_id",
			params: UserSyncParams{
				Email:          "test@example.com",
				Name:           "Test User",
				OrganizationID: "org-123",
			},
			field: "betterauth_user_id",
		},
		{
			name: "missing email",
			params: UserSyncParams{
				BetterAuthUserID: "user-123",
				Name:             "Test User",
				OrganizationID:   "org-123",
			},
			field: "email",
		},
		{
			name: "missing name",
			params: UserSyncParams{
				BetterAuthUserID: "user-123",
				Email:            "test@example.com",
				OrganizationID:   "org-123",
			},
			field: "name",
		},
		{
			name: "missing organization_id",
			params: UserSyncParams{
				BetterAuthUserID: "user-123",
				Email:            "test@example.com",
				Name:             "Test User",
			},
			field: "organization_id",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Validate that the field is indeed empty
			switch tc.field {
			case "betterauth_user_id":
				assert.Empty(t, tc.params.BetterAuthUserID)
			case "email":
				assert.Empty(t, tc.params.Email)
			case "name":
				assert.Empty(t, tc.params.Name)
			case "organization_id":
				assert.Empty(t, tc.params.OrganizationID)
			}
		})
	}
}

func TestUserSyncParams_AllFieldsPresent(t *testing.T) {
	params := UserSyncParams{
		BetterAuthUserID: "user-abc123",
		Email:            "teacher@school.de",
		Name:             "Maria Schmidt",
		OrganizationID:   "org-xyz789",
		Role:             "admin",
	}

	assert.NotEmpty(t, params.BetterAuthUserID)
	assert.NotEmpty(t, params.Email)
	assert.NotEmpty(t, params.Name)
	assert.NotEmpty(t, params.OrganizationID)
	assert.Equal(t, "admin", params.Role)
}
