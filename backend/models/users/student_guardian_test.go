package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestStudentGuardian_Validate(t *testing.T) {
	tests := []struct {
		name    string
		sg      *StudentGuardian
		wantErr bool
	}{
		{
			name: "valid",
			sg: &StudentGuardian{
				StudentID:         1,
				GuardianProfileID: 2,
				RelationshipType:  "parent",
				IsPrimary:         true,
			},
			wantErr: false,
		},
		{
			name: "missing student ID",
			sg: &StudentGuardian{
				GuardianProfileID: 2,
				RelationshipType:  "parent",
			},
			wantErr: true,
		},
		{
			name: "missing guardian profile ID",
			sg: &StudentGuardian{
				StudentID:        1,
				RelationshipType: "parent",
			},
			wantErr: true,
		},
		{
			name: "missing relationship type",
			sg: &StudentGuardian{
				StudentID:         1,
				GuardianProfileID: 2,
			},
			wantErr: true,
		},
		{
			name: "invalid relationship type",
			sg: &StudentGuardian{
				StudentID:         1,
				GuardianProfileID: 2,
				RelationshipType:  "invalid",
			},
			wantErr: true,
		},
		{
			name: "normalize relationship type to lowercase",
			sg: &StudentGuardian{
				StudentID:         1,
				GuardianProfileID: 2,
				RelationshipType:  "PARENT",
			},
			wantErr: false,
		},
		{
			name: "permissions map",
			sg: &StudentGuardian{
				StudentID:         1,
				GuardianProfileID: 2,
				RelationshipType:  "parent",
				Permissions:       map[string]interface{}{"test": true},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.sg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("StudentGuardian.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check normalization of relationship type
			if tt.name == "normalize relationship type to lowercase" && tt.sg.RelationshipType != "parent" {
				t.Errorf("StudentGuardian.Validate() failed to normalize relationship type, got %v", tt.sg.RelationshipType)
			}
		})
	}
}

func TestStudentGuardian_SetStudent(t *testing.T) {
	student := &Student{
		Model: base.Model{
			ID: 123,
		},
		PersonID:    1,
		SchoolClass: "Class A",
	}

	sg := &StudentGuardian{
		StudentID:         0,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
	}

	sg.SetStudent(student)

	if sg.StudentID != 123 {
		t.Errorf("StudentGuardian.SetStudent() failed to set student ID, got %v", sg.StudentID)
	}

	if sg.Student != student {
		t.Errorf("StudentGuardian.SetStudent() failed to set student reference")
	}
}

func TestStudentGuardian_HasPermission(t *testing.T) {
	// Test with boolean permissions
	sg1 := &StudentGuardian{
		StudentID:         1,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
		Permissions: map[string]interface{}{
			"can_view_grades":     true,
			"can_attend_meetings": false,
		},
	}

	if !sg1.HasPermission("can_view_grades") {
		t.Errorf("StudentGuardian.HasPermission() failed, expected true for can_view_grades")
	}

	if sg1.HasPermission("can_attend_meetings") {
		t.Errorf("StudentGuardian.HasPermission() failed, expected false for can_attend_meetings")
	}

	if sg1.HasPermission("non_existent_permission") {
		t.Errorf("StudentGuardian.HasPermission() failed, expected false for non-existent permission")
	}

	// Test with empty permissions
	sg2 := &StudentGuardian{
		StudentID:         1,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
		Permissions:       map[string]interface{}{},
	}

	if sg2.HasPermission("any_permission") {
		t.Errorf("StudentGuardian.HasPermission() failed, expected false for any permission with empty permissions")
	}
}

func TestStudentGuardian_GetRelationshipName(t *testing.T) {
	tests := []struct {
		relationshipType string
		expectedName     string
	}{
		{"parent", "Parent"},
		{"guardian", "Guardian"},
		{"relative", "Relative"},
		{"other", "Other"},
		{"unknown", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.relationshipType, func(t *testing.T) {
			sg := &StudentGuardian{
				RelationshipType: tt.relationshipType,
			}

			if sg.GetRelationshipName() != tt.expectedName {
				t.Errorf("StudentGuardian.GetRelationshipName() = %v, want %v", sg.GetRelationshipName(), tt.expectedName)
			}
		})
	}
}

func TestStudentGuardian_UpdatePermissions(t *testing.T) {
	sg := &StudentGuardian{
		StudentID:         1,
		GuardianProfileID: 2,
		RelationshipType:  "parent",
		Permissions:       map[string]interface{}{},
	}

	// Update permissions
	newPermissions := map[string]interface{}{
		"can_view_grades":     true,
		"can_attend_meetings": true,
		"can_authorize_trips": false,
		"contact_preferences": map[string]interface{}{
			"email": true,
			"phone": false,
		},
	}

	err := sg.UpdatePermissions(newPermissions)
	if err != nil {
		t.Errorf("StudentGuardian.UpdatePermissions() error = %v", err)
	}

	// Check if permissions were updated
	if !sg.HasPermission("can_view_grades") {
		t.Errorf("StudentGuardian.UpdatePermissions() failed, expected true for can_view_grades")
	}

	if !sg.HasPermission("can_attend_meetings") {
		t.Errorf("StudentGuardian.UpdatePermissions() failed, expected true for can_attend_meetings")
	}

	if sg.HasPermission("can_authorize_trips") {
		t.Errorf("StudentGuardian.UpdatePermissions() failed, expected false for can_authorize_trips")
	}

	// Check if nested permissions are correctly handled
	permissions := sg.GetPermissions()
	contactPrefs, ok := permissions["contact_preferences"].(map[string]interface{})
	if !ok {
		t.Errorf("StudentGuardian.UpdatePermissions() failed to handle nested permissions")
	} else {
		email, ok := contactPrefs["email"].(bool)
		if !ok || !email {
			t.Errorf("StudentGuardian.UpdatePermissions() failed to correctly store nested email preference")
		}

		phone, ok := contactPrefs["phone"].(bool)
		if !ok || phone {
			t.Errorf("StudentGuardian.UpdatePermissions() failed to correctly store nested phone preference")
		}
	}
}

func TestStudentGuardian_TableName(t *testing.T) {
	sg := &StudentGuardian{}
	expected := "users.students_guardians"

	if got := sg.TableName(); got != expected {
		t.Errorf("StudentGuardian.TableName() = %q, want %q", got, expected)
	}
}

func TestStudentGuardian_EntityInterface(t *testing.T) {
	now := time.Now()
	sg := &StudentGuardian{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}

	// Test GetID
	if sg.GetID() != int64(123) {
		t.Errorf("StudentGuardian.GetID() = %v, want %v", sg.GetID(), int64(123))
	}

	// Test GetCreatedAt
	if sg.GetCreatedAt() != now {
		t.Errorf("StudentGuardian.GetCreatedAt() = %v, want %v", sg.GetCreatedAt(), now)
	}

	// Test GetUpdatedAt
	if sg.GetUpdatedAt() != now {
		t.Errorf("StudentGuardian.GetUpdatedAt() = %v, want %v", sg.GetUpdatedAt(), now)
	}
}
