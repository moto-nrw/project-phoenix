package users

import (
	"testing"
	"time"

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

// Note: the guardian permission check and grant/revoke mutations (formerly
// PersonGuardian.HasPermission / GrantPermission / RevokePermission) moved out
// of the model in issue #586 (Rule 12). Their tests follow the logic to:
//   - auth/authorize/guardian_permission_test.go

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

func TestPersonGuardian_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		pg := &PersonGuardian{
			PersonID:          1,
			GuardianAccountID: 2,
			RelationshipType:  RelationshipParent,
		}
		err := pg.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		pg := &PersonGuardian{
			PersonID:          1,
			GuardianAccountID: 2,
			RelationshipType:  RelationshipParent,
		}
		err := pg.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestPersonGuardian_GetID(t *testing.T) {
	pg := &PersonGuardian{
		Model:             base.Model{ID: 42},
		PersonID:          1,
		GuardianAccountID: 2,
		RelationshipType:  RelationshipParent,
	}

	if got, ok := pg.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", pg.GetID())
	}
}

func TestPersonGuardian_GetCreatedAt(t *testing.T) {
	now := time.Now()
	pg := &PersonGuardian{
		Model:             base.Model{CreatedAt: now},
		PersonID:          1,
		GuardianAccountID: 2,
		RelationshipType:  RelationshipParent,
	}

	if got := pg.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestPersonGuardian_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	pg := &PersonGuardian{
		Model:             base.Model{UpdatedAt: now},
		PersonID:          1,
		GuardianAccountID: 2,
		RelationshipType:  RelationshipParent,
	}

	if got := pg.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
