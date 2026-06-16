package authorize

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
)

// Moved from models/users/person_guardian_test.go (TestPersonGuardian_HasPermission)
// when the guardian permission check left the model in issue #586 (Rule 12).
func TestPersonGuardianHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions string
		permission  string
		expected    bool
	}{
		{"has permission - true", `{"view_grades": true}`, "view_grades", true},
		{"has permission - false", `{"view_grades": false}`, "view_grades", false},
		{"permission not found", `{"view_grades": true}`, "contact_teachers", false},
		{"empty permissions", "", "view_grades", false},
		{"invalid JSON returns false", "invalid json", "view_grades", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := &users.PersonGuardian{Permissions: tt.permissions}
			if got := PersonGuardianHasPermission(pg, tt.permission); got != tt.expected {
				t.Errorf("PersonGuardianHasPermission(%q) = %v, want %v", tt.permission, got, tt.expected)
			}
		})
	}

	if PersonGuardianHasPermission(nil, "view_grades") {
		t.Error("PersonGuardianHasPermission(nil, ...) should return false")
	}
}

// Moved from models/users/person_guardian_test.go (TestPersonGuardian_GrantPermission).
func TestPersonGuardianGrantPermission(t *testing.T) {
	t.Run("grant permission on empty", func(t *testing.T) {
		pg := &users.PersonGuardian{}
		if err := PersonGuardianGrantPermission(pg, "view_grades"); err != nil {
			t.Errorf("PersonGuardianGrantPermission() error = %v", err)
		}
		if !PersonGuardianHasPermission(pg, "view_grades") {
			t.Error("PersonGuardianGrantPermission() failed to grant permission")
		}
	})

	t.Run("grant permission on existing", func(t *testing.T) {
		pg := &users.PersonGuardian{Permissions: `{"contact_teachers": true}`}
		if err := PersonGuardianGrantPermission(pg, "view_grades"); err != nil {
			t.Errorf("PersonGuardianGrantPermission() error = %v", err)
		}
		if !PersonGuardianHasPermission(pg, "view_grades") {
			t.Error("PersonGuardianGrantPermission() failed to grant new permission")
		}
		if !PersonGuardianHasPermission(pg, "contact_teachers") {
			t.Error("PersonGuardianGrantPermission() removed existing permission")
		}
	})

	t.Run("grant permission on invalid JSON resets permissions", func(t *testing.T) {
		pg := &users.PersonGuardian{Permissions: "invalid json"}
		if err := PersonGuardianGrantPermission(pg, "view_grades"); err != nil {
			t.Errorf("PersonGuardianGrantPermission() error = %v", err)
		}
		if !PersonGuardianHasPermission(pg, "view_grades") {
			t.Error("PersonGuardianGrantPermission() failed to grant permission after resetting")
		}
	})
}

// Moved from models/users/person_guardian_test.go (TestPersonGuardian_RevokePermission).
func TestPersonGuardianRevokePermission(t *testing.T) {
	t.Run("revoke existing permission", func(t *testing.T) {
		pg := &users.PersonGuardian{Permissions: `{"view_grades": true, "contact_teachers": true}`}
		if err := PersonGuardianRevokePermission(pg, "view_grades"); err != nil {
			t.Errorf("PersonGuardianRevokePermission() error = %v", err)
		}
		if PersonGuardianHasPermission(pg, "view_grades") {
			t.Error("PersonGuardianRevokePermission() failed to revoke permission")
		}
		if !PersonGuardianHasPermission(pg, "contact_teachers") {
			t.Error("PersonGuardianRevokePermission() incorrectly revoked other permission")
		}
	})

	t.Run("revoke from empty permissions", func(t *testing.T) {
		pg := &users.PersonGuardian{}
		if err := PersonGuardianRevokePermission(pg, "view_grades"); err != nil {
			t.Errorf("PersonGuardianRevokePermission() error = %v", err)
		}
	})

	t.Run("revoke non-existent permission", func(t *testing.T) {
		pg := &users.PersonGuardian{Permissions: `{"view_grades": true}`}
		if err := PersonGuardianRevokePermission(pg, "contact_teachers"); err != nil {
			t.Errorf("PersonGuardianRevokePermission() error = %v", err)
		}
		if !PersonGuardianHasPermission(pg, "view_grades") {
			t.Error("PersonGuardianRevokePermission() incorrectly affected other permission")
		}
	})
}

// Moved from models/users/student_guardian_test.go (TestStudentGuardian_HasPermission).
func TestStudentGuardianHasPermission(t *testing.T) {
	sg1 := &users.StudentGuardian{
		StudentID:         1,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
		Permissions: map[string]interface{}{
			"can_view_grades":     true,
			"can_attend_meetings": false,
		},
	}

	if !StudentGuardianHasPermission(sg1, "can_view_grades") {
		t.Error("StudentGuardianHasPermission() expected true for can_view_grades")
	}
	if StudentGuardianHasPermission(sg1, "can_attend_meetings") {
		t.Error("StudentGuardianHasPermission() expected false for can_attend_meetings")
	}
	if StudentGuardianHasPermission(sg1, "non_existent_permission") {
		t.Error("StudentGuardianHasPermission() expected false for non-existent permission")
	}

	sg2 := &users.StudentGuardian{
		StudentID:         1,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
		Permissions:       map[string]interface{}{},
	}
	if StudentGuardianHasPermission(sg2, "any_permission") {
		t.Error("StudentGuardianHasPermission() expected false for empty permissions")
	}

	if StudentGuardianHasPermission(nil, "any") {
		t.Error("StudentGuardianHasPermission(nil, ...) should return false")
	}
}

func TestStudentGuardianPermissionSet(t *testing.T) {
	fullRoles := []string{
		GuardianRolePrimaryGuardian,
		GuardianRoleLegalGuardian,
		GuardianRoleCoGuardian,
	}
	for _, role := range fullRoles {
		t.Run(role, func(t *testing.T) {
			sg := &users.StudentGuardian{}
			ApplyStudentGuardianRole(sg, role)

			for _, permission := range []string{
				GuardianPermissionPortalAccess,
				GuardianPermissionSickNoteSubmit,
				GuardianPermissionNotesWrite,
				GuardianPermissionEnrollmentsView,
				GuardianPermissionEnrollmentSubmit,
			} {
				if !StudentGuardianHasPermission(sg, permission) {
					t.Fatalf("role %q missing permission %q", role, permission)
				}
			}
		})
	}

	noPortalRoles := []string{
		GuardianRoleEmergency,
		GuardianRolePickupOnly,
		GuardianRoleSocialWorker,
		GuardianRoleCustom,
		"unknown_role",
	}
	for _, role := range noPortalRoles {
		t.Run(role, func(t *testing.T) {
			perms := StudentGuardianPermissionSet(role)
			if len(perms) != 0 {
				t.Fatalf("role %q got portal permissions: %#v", role, perms)
			}
		})
	}
}

func TestStudentGuardianGrantPermissions(t *testing.T) {
	sg := &users.StudentGuardian{}

	StudentGuardianGrantPermissions(sg, GuardianPermissionPortalAccess, "", "  ")
	StudentGuardianGrantPermissions(nil, GuardianPermissionNotesWrite)

	if !StudentGuardianHasPermission(sg, GuardianPermissionPortalAccess) {
		t.Fatal("StudentGuardianGrantPermissions() failed to grant portal access")
	}
	if StudentGuardianHasPermission(sg, GuardianPermissionNotesWrite) {
		t.Fatal("StudentGuardianGrantPermissions(nil, ...) should not mutate another guardian")
	}
}

func TestApplyStudentGuardianRole_NilIsNoop(t *testing.T) {
	ApplyStudentGuardianRole(nil, GuardianRolePrimaryGuardian)
}

func TestApplyStudentGuardianRole_CustomPreservesPermissions(t *testing.T) {
	sg := &users.StudentGuardian{
		GuardianRole: GuardianRoleLegalGuardian,
		Permissions: map[string]interface{}{
			GuardianPermissionPortalAccess:     true,
			GuardianPermissionSickNoteSubmit:   true,
			GuardianPermissionEnrollmentSubmit: false,
		},
	}

	ApplyStudentGuardianRole(sg, GuardianRoleCustom)

	if sg.GuardianRole != GuardianRoleCustom {
		t.Fatalf("ApplyStudentGuardianRole() role = %q, want %q", sg.GuardianRole, GuardianRoleCustom)
	}
	if !StudentGuardianHasPermission(sg, GuardianPermissionPortalAccess) {
		t.Fatal("ApplyStudentGuardianRole(custom) should preserve existing portal access")
	}
	if !StudentGuardianHasPermission(sg, GuardianPermissionSickNoteSubmit) {
		t.Fatal("ApplyStudentGuardianRole(custom) should preserve existing sick-note permission")
	}
	if StudentGuardianHasPermission(sg, GuardianPermissionEnrollmentSubmit) {
		t.Fatal("false stored permissions must stay false")
	}
}

func TestDefaultStudentGuardianRole(t *testing.T) {
	tests := []struct {
		name               string
		relationshipType   string
		isPrimary          bool
		isEmergencyContact bool
		canPickup          bool
		want               string
	}{
		{
			name:             "primary wins",
			relationshipType: "relative",
			isPrimary:        true,
			canPickup:        true,
			want:             GuardianRolePrimaryGuardian,
		},
		{
			name:             "parent becomes legal guardian",
			relationshipType: "parent",
			want:             GuardianRoleLegalGuardian,
		},
		{
			name:             "guardian becomes legal guardian",
			relationshipType: "guardian",
			want:             GuardianRoleLegalGuardian,
		},
		{
			name:             "pickup relative is pickup only",
			relationshipType: "relative",
			canPickup:        true,
			want:             GuardianRolePickupOnly,
		},
		{
			name:               "emergency contact without stronger role",
			relationshipType:   "other",
			isEmergencyContact: true,
			want:               GuardianRoleEmergency,
		},
		{
			name:             "other without flags is custom",
			relationshipType: "other",
			want:             GuardianRoleCustom,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultStudentGuardianRole(
				tt.relationshipType,
				tt.isPrimary,
				tt.isEmergencyContact,
				tt.canPickup,
			)
			if got != tt.want {
				t.Fatalf("DefaultStudentGuardianRole() = %q, want %q", got, tt.want)
			}
		})
	}
}
