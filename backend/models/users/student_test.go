package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestStudent_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		student *Student
		wantErr bool
	}{
		{
			name: "valid student with required fields",
			student: &Student{
				PersonID:    1,
				SchoolClass: "1a",
			},
			wantErr: false,
		},
		{
			name: "valid student with all optional fields",
			student: &Student{
				PersonID:    1,
				SchoolClass: "3b",
				GroupID:     ptrtest.Ptr(int64(5)),
			},
			wantErr: false,
		},
		{
			name: "missing person ID",
			student: &Student{
				PersonID:    0,
				SchoolClass: "1a",
			},
			wantErr: true,
		},
		{
			name: "negative person ID",
			student: &Student{
				PersonID:    -1,
				SchoolClass: "1a",
			},
			wantErr: true,
		},
		{
			name: "missing school class",
			student: &Student{
				PersonID:    1,
				SchoolClass: "",
			},
			wantErr: true,
		},
		{
			name: "whitespace only school class - passes then trimmed",
			student: &Student{
				PersonID:    1,
				SchoolClass: "   ",
			},
			wantErr: false, // Note: validation checks empty before trimming
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.student.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Student.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestStudent_Validate_DepartureCompanionNote(t *testing.T) {
	t.Parallel()

	t.Run("over-long note is rejected", func(t *testing.T) {
		long := make([]rune, MaxDepartureCompanionNoteLen+1)
		for i := range long {
			long[i] = 'ä' // multibyte, so byte length != rune length
		}
		note := string(long)
		student := &Student{PersonID: 1, SchoolClass: "1a", DepartureCompanionNote: &note}
		if err := student.Validate(); err == nil {
			t.Fatal("expected error for over-long companion note, got nil")
		}
	})

	t.Run("note at the cap is accepted and trimmed", func(t *testing.T) {
		note := "  Geschwisterkind Lena  "
		student := &Student{PersonID: 1, SchoolClass: "1a", DepartureCompanionNote: &note}
		if err := student.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if student.DepartureCompanionNote == nil || *student.DepartureCompanionNote != "Geschwisterkind Lena" {
			t.Errorf("expected trimmed note, got %v", student.DepartureCompanionNote)
		}
	})

	t.Run("blank note is dropped to nil", func(t *testing.T) {
		note := "   "
		student := &Student{PersonID: 1, SchoolClass: "1a", DepartureCompanionNote: &note}
		if err := student.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if student.DepartureCompanionNote != nil {
			t.Errorf("expected nil note, got %q", *student.DepartureCompanionNote)
		}
	})
}

// TestStudent_Validate_AccompaniedRequiresNote pins the coupling enforced at the
// model boundary (#1694): a day that allows the accompanied ("Mit anderem Kind")
// departure mode is incomplete without the free-text "mit wem" companion note,
// so a caller bypassing the React forms cannot persist one with a blank note.
func TestStudent_Validate_AccompaniedRequiresNote(t *testing.T) {
	t.Parallel()

	t.Run("allowed accompanied without note is rejected", func(t *testing.T) {
		student := &Student{
			PersonID:    1,
			SchoolClass: "1a",
			AllowedDepartureModes: AllowedDepartureModes{
				PickupDayMonday: []DepartureMode{DepartureAccompanied},
			},
		}
		if err := student.Validate(); err == nil {
			t.Fatal("expected error for accompanied day with no companion note, got nil")
		}
	})

	t.Run("blank-only note with accompanied is rejected", func(t *testing.T) {
		note := "   "
		student := &Student{
			PersonID:               1,
			SchoolClass:            "1a",
			DepartureCompanionNote: &note,
			AllowedDepartureModes: AllowedDepartureModes{
				PickupDayMonday: []DepartureMode{DepartureAccompanied},
			},
		}
		if err := student.Validate(); err == nil {
			t.Fatal("expected error: a whitespace note is dropped to nil and must not satisfy the coupling")
		}
	})

	t.Run("departure_days accompanied without note is rejected", func(t *testing.T) {
		student := &Student{
			PersonID:      1,
			SchoolClass:   "1a",
			DepartureDays: DepartureDays{PickupDayMonday: DepartureAccompanied},
		}
		if err := student.Validate(); err == nil {
			t.Fatal("expected error for accompanied departure_days with no companion note, got nil")
		}
	})

	t.Run("accompanied with note is accepted", func(t *testing.T) {
		note := "Geschwisterkind Lena"
		student := &Student{
			PersonID:               1,
			SchoolClass:            "1a",
			DepartureCompanionNote: &note,
			AllowedDepartureModes: AllowedDepartureModes{
				PickupDayMonday: []DepartureMode{DepartureAccompanied},
			},
		}
		if err := student.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("non-accompanied plan needs no note", func(t *testing.T) {
		student := &Student{
			PersonID:    1,
			SchoolClass: "1a",
			AllowedDepartureModes: AllowedDepartureModes{
				PickupDayMonday: []DepartureMode{DepartureBus, DeparturePickup},
			},
		}
		if err := student.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestStudent_Validate_TrimSchoolClass(t *testing.T) {
	t.Parallel()

	student := &Student{
		PersonID:    1,
		SchoolClass: "  3a  ",
	}

	err := student.Validate()
	if err != nil {
		t.Fatalf("Student.Validate() unexpected error = %v", err)
	}

	if student.SchoolClass != "3a" {
		t.Errorf("Student.Validate() did not trim SchoolClass, got %q", student.SchoolClass)
	}
}

func TestStudent_SetPerson(t *testing.T) {
	t.Parallel()

	t.Run("set person", func(t *testing.T) {
		student := &Student{
			SchoolClass: "1a",
		}

		person := &Person{
			Model:     base.Model{ID: 42},
			FirstName: "John",
			LastName:  "Doe",
		}

		student.SetPerson(person)

		if student.Person != person {
			t.Error("Student.SetPerson() did not set Person reference")
		}

		if student.PersonID != 42 {
			t.Errorf("Student.SetPerson() did not set PersonID, got %v", student.PersonID)
		}
	})

	t.Run("set nil person", func(t *testing.T) {
		student := &Student{
			PersonID:    42,
			SchoolClass: "1a",
		}

		student.SetPerson(nil)

		if student.Person != nil {
			t.Error("Student.SetPerson(nil) did not clear Person reference")
		}

		// PersonID is not cleared by SetPerson(nil) - only the reference
		// This is intentional based on the implementation
	})
}
