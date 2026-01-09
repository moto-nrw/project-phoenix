package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestPersonGuardian_Validate(t *testing.T) {
	tests := []struct {
		name    string
		pg      *PersonGuardian
		wantErr bool
	}{
		{
			name: "valid parent relationship",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
			},
			wantErr: false,
		},
		{
			name: "valid guardian relationship",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipGuardian,
			},
			wantErr: false,
		},
		{
			name: "valid relative relationship",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipRelative,
			},
			wantErr: false,
		},
		{
			name: "valid other relationship",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipOther,
			},
			wantErr: false,
		},
		{
			name: "valid with permissions JSON",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
				Permissions:       `{"view_grades": true, "contact_teachers": true}`,
			},
			wantErr: false,
		},
		{
			name: "missing person ID",
			pg: &PersonGuardian{
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "zero person ID",
			pg: &PersonGuardian{
				PersonID:          0,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "negative person ID",
			pg: &PersonGuardian{
				PersonID:          -1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "missing guardian account ID",
			pg: &PersonGuardian{
				PersonID:         1,
				RelationshipType: RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "zero guardian account ID",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 0,
				RelationshipType:  RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "negative guardian account ID",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: -1,
				RelationshipType:  RelationshipParent,
			},
			wantErr: true,
		},
		{
			name: "missing relationship type",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
			},
			wantErr: true,
		},
		{
			name: "invalid relationship type",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  "invalid",
			},
			wantErr: true,
		},
		{
			name: "normalize relationship type to lowercase",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  "PARENT",
			},
			wantErr: false,
		},
		{
			name: "invalid permissions JSON",
			pg: &PersonGuardian{
				PersonID:          1,
				GuardianAccountID: 2,
				RelationshipType:  RelationshipParent,
				Permissions:       "not valid json",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("PersonGuardian.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check normalization of relationship type
			if tt.name == "normalize relationship type to lowercase" && tt.pg.RelationshipType != RelationshipParent {
				t.Errorf("PersonGuardian.Validate() failed to normalize relationship type, got %v", tt.pg.RelationshipType)
			}
		})
	}
}

func TestPersonGuardian_TableName(t *testing.T) {
	pg := &PersonGuardian{}
	expected := "users.persons_guardians"

	if got := pg.TableName(); got != expected {
		t.Errorf("PersonGuardian.TableName() = %q, want %q", got, expected)
	}
}

func TestPersonGuardian_SetPerson(t *testing.T) {
	t.Run("set with person", func(t *testing.T) {
		pg := &PersonGuardian{
			GuardianAccountID: 2,
			RelationshipType:  RelationshipParent,
		}

		person := &Person{
			Model: base.Model{ID: 42},
		}

		pg.SetPerson(person)

		if pg.Person != person {
			t.Error("SetPerson should set the Person field")
		}
		if pg.PersonID != 42 {
			t.Errorf("SetPerson should set PersonID = 42, got %d", pg.PersonID)
		}
	})

	t.Run("set with nil person", func(t *testing.T) {
		pg := &PersonGuardian{
			PersonID:          10,
			GuardianAccountID: 2,
			RelationshipType:  RelationshipParent,
		}

		pg.SetPerson(nil)

		if pg.Person != nil {
			t.Error("SetPerson(nil) should set Person to nil")
		}
		// PersonID should remain unchanged when setting nil
		if pg.PersonID != 10 {
			t.Errorf("SetPerson(nil) should not change PersonID, got %d", pg.PersonID)
		}
	})
}

func TestPersonGuardian_HasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions string
		permission  string
		expected    bool
	}{
		{
			name:        "has permission - true",
			permissions: `{"view_grades": true}`,
			permission:  "view_grades",
			expected:    true,
		},
		{
			name:        "has permission - false",
			permissions: `{"view_grades": false}`,
			permission:  "view_grades",
			expected:    false,
		},
		{
			name:        "permission not found",
			permissions: `{"view_grades": true}`,
			permission:  "contact_teachers",
			expected:    false,
		},
		{
			name:        "empty permissions",
			permissions: "",
			permission:  "view_grades",
			expected:    false,
		},
		{
			name:        "invalid JSON returns false",
			permissions: "invalid json",
			permission:  "view_grades",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pg := &PersonGuardian{
				Permissions: tt.permissions,
			}

			if got := pg.HasPermission(tt.permission); got != tt.expected {
				t.Errorf("PersonGuardian.HasPermission(%q) = %v, want %v", tt.permission, got, tt.expected)
			}
		})
	}
}

func TestPersonGuardian_GrantPermission(t *testing.T) {
	t.Run("grant permission on empty", func(t *testing.T) {
		pg := &PersonGuardian{}

		err := pg.GrantPermission("view_grades")
		if err != nil {
			t.Errorf("PersonGuardian.GrantPermission() error = %v", err)
		}

		if !pg.HasPermission("view_grades") {
			t.Error("PersonGuardian.GrantPermission() failed to grant permission")
		}
	})

	t.Run("grant permission on existing", func(t *testing.T) {
		pg := &PersonGuardian{
			Permissions: `{"contact_teachers": true}`,
		}

		err := pg.GrantPermission("view_grades")
		if err != nil {
			t.Errorf("PersonGuardian.GrantPermission() error = %v", err)
		}

		if !pg.HasPermission("view_grades") {
			t.Error("PersonGuardian.GrantPermission() failed to grant new permission")
		}
		if !pg.HasPermission("contact_teachers") {
			t.Error("PersonGuardian.GrantPermission() removed existing permission")
		}
	})

	t.Run("grant permission on invalid JSON resets permissions", func(t *testing.T) {
		pg := &PersonGuardian{
			Permissions: "invalid json",
		}

		err := pg.GrantPermission("view_grades")
		if err != nil {
			t.Errorf("PersonGuardian.GrantPermission() error = %v", err)
		}

		if !pg.HasPermission("view_grades") {
			t.Error("PersonGuardian.GrantPermission() failed to grant permission after resetting")
		}
	})
}

func TestPersonGuardian_RevokePermission(t *testing.T) {
	t.Run("revoke existing permission", func(t *testing.T) {
		pg := &PersonGuardian{
			Permissions: `{"view_grades": true, "contact_teachers": true}`,
		}

		err := pg.RevokePermission("view_grades")
		if err != nil {
			t.Errorf("PersonGuardian.RevokePermission() error = %v", err)
		}

		if pg.HasPermission("view_grades") {
			t.Error("PersonGuardian.RevokePermission() failed to revoke permission")
		}
		if !pg.HasPermission("contact_teachers") {
			t.Error("PersonGuardian.RevokePermission() incorrectly revoked other permission")
		}
	})

	t.Run("revoke from empty permissions", func(t *testing.T) {
		pg := &PersonGuardian{}

		err := pg.RevokePermission("view_grades")
		if err != nil {
			t.Errorf("PersonGuardian.RevokePermission() error = %v", err)
		}
	})

	t.Run("revoke non-existent permission", func(t *testing.T) {
		pg := &PersonGuardian{
			Permissions: `{"view_grades": true}`,
		}

		err := pg.RevokePermission("contact_teachers")
		if err != nil {
			t.Errorf("PersonGuardian.RevokePermission() error = %v", err)
		}

		if !pg.HasPermission("view_grades") {
			t.Error("PersonGuardian.RevokePermission() incorrectly affected other permission")
		}
	})
}

func TestPersonGuardian_GetRelationshipName(t *testing.T) {
	tests := []struct {
		relationshipType RelationshipType
		expectedName     string
	}{
		{RelationshipParent, "Parent"},
		{RelationshipGuardian, "Guardian"},
		{RelationshipRelative, "Relative"},
		{RelationshipOther, "Other"},
		{"unknown", "Unknown"},
		{"", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(string(tt.relationshipType), func(t *testing.T) {
			pg := &PersonGuardian{
				RelationshipType: tt.relationshipType,
			}

			if got := pg.GetRelationshipName(); got != tt.expectedName {
				t.Errorf("PersonGuardian.GetRelationshipName() = %v, want %v", got, tt.expectedName)
			}
		})
	}
}
