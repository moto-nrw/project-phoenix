package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
)

func TestStaff_Validate(t *testing.T) {
	tests := []struct {
		name    string
		staff   *Staff
		wantErr bool
	}{
		{
			name: "valid staff",
			staff: &Staff{
				PersonID: 1,
			},
			wantErr: false,
		},
		{
			name: "valid staff with notes",
			staff: &Staff{
				PersonID:   1,
				StaffNotes: "Some notes about the staff member",
			},
			wantErr: false,
		},
		{
			name: "zero person ID",
			staff: &Staff{
				PersonID: 0,
			},
			wantErr: true,
		},
		{
			name: "negative person ID",
			staff: &Staff{
				PersonID: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.staff.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Staff.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStaff_SetPerson(t *testing.T) {
	t.Run("set person", func(t *testing.T) {
		staff := &Staff{}

		person := &Person{
			Model:     base.Model{ID: 42},
			FirstName: "John",
			LastName:  "Doe",
		}

		staff.SetPerson(person)

		if staff.Person != person {
			t.Error("Staff.SetPerson() did not set Person reference")
		}

		if staff.PersonID != 42 {
			t.Errorf("Staff.SetPerson() did not set PersonID, got %v", staff.PersonID)
		}
	})

	t.Run("set nil person", func(t *testing.T) {
		staff := &Staff{
			PersonID: 42,
		}

		staff.SetPerson(nil)

		if staff.Person != nil {
			t.Error("Staff.SetPerson(nil) did not clear Person reference")
		}

		// PersonID is not cleared - this matches the implementation
	})
}

func TestStaff_GetFullName(t *testing.T) {
	tests := []struct {
		name     string
		staff    *Staff
		expected string
	}{
		{
			name: "with person",
			staff: &Staff{
				PersonID: 1,
				Person: &Person{
					FirstName: "John",
					LastName:  "Doe",
				},
			},
			expected: "John Doe",
		},
		{
			name: "without person",
			staff: &Staff{
				PersonID: 1,
				Person:   nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.staff.GetFullName()
			if got != tt.expected {
				t.Errorf("Staff.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestStaff_AddNotes(t *testing.T) {
	t.Run("add notes to empty", func(t *testing.T) {
		staff := &Staff{
			PersonID:   1,
			StaffNotes: "",
		}

		staff.AddNotes("First note")

		if staff.StaffNotes != "First note" {
			t.Errorf("Staff.StaffNotes = %q, want %q", staff.StaffNotes, "First note")
		}
	})

	t.Run("append notes", func(t *testing.T) {
		staff := &Staff{
			PersonID:   1,
			StaffNotes: "First note",
		}

		staff.AddNotes("Second note")

		expected := "First note\nSecond note"
		if staff.StaffNotes != expected {
			t.Errorf("Staff.StaffNotes = %q, want %q", staff.StaffNotes, expected)
		}
	})

	t.Run("multiple appends", func(t *testing.T) {
		staff := &Staff{
			PersonID:   1,
			StaffNotes: "",
		}

		staff.AddNotes("Note 1")
		staff.AddNotes("Note 2")
		staff.AddNotes("Note 3")

		expected := "Note 1\nNote 2\nNote 3"
		if staff.StaffNotes != expected {
			t.Errorf("Staff.StaffNotes = %q, want %q", staff.StaffNotes, expected)
		}
	})
}
