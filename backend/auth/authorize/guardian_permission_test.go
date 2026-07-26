package authorize

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
)

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

func TestApplyStudentGuardianRole_NilIsNoop(t *testing.T) {
	ApplyStudentGuardianRole(nil, GuardianRolePrimaryGuardian)
}

func TestApplyStudentGuardianRole_CustomClearsPermissions(t *testing.T) {
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
	if StudentGuardianHasPermission(sg, GuardianPermissionPortalAccess) {
		t.Fatal("ApplyStudentGuardianRole(custom) should clear existing portal access")
	}
	if StudentGuardianHasPermission(sg, GuardianPermissionSickNoteSubmit) {
		t.Fatal("ApplyStudentGuardianRole(custom) should clear existing sick-note permission")
	}
	if StudentGuardianHasPermission(sg, GuardianPermissionEnrollmentSubmit) {
		t.Fatal("ApplyStudentGuardianRole(custom) should not grant enrollment submit")
	}
}

func TestApplyDefaultStudentGuardianRole(t *testing.T) {
	sg := &users.StudentGuardian{
		RelationshipType: "parent",
	}

	ApplyDefaultStudentGuardianRole(sg)

	if sg.GuardianRole != GuardianRoleLegalGuardian {
		t.Fatalf("ApplyDefaultStudentGuardianRole() role = %q, want %q", sg.GuardianRole, GuardianRoleLegalGuardian)
	}
	if !StudentGuardianHasPermission(sg, GuardianPermissionPortalAccess) {
		t.Fatal("parent default should grant portal access")
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
