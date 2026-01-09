package users

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestStudent_Validate(t *testing.T) {
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
				PersonID:        1,
				SchoolClass:     "3b",
				GuardianName:    base.StringPtr("Jane Doe"),
				GuardianContact: base.StringPtr("123-456-7890"),
				GuardianEmail:   base.StringPtr("jane@example.com"),
				GuardianPhone:   base.StringPtr("+49 123 456789"),
				GroupID:         base.Int64Ptr(5),
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

func TestStudent_Validate_TrimSchoolClass(t *testing.T) {
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

func TestStudent_Validate_GuardianEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   *string
		wantErr bool
	}{
		{
			name:    "valid email",
			email:   base.StringPtr("parent@example.com"),
			wantErr: false,
		},
		{
			name:    "valid email with dots",
			email:   base.StringPtr("parent.name@example.co.uk"),
			wantErr: false,
		},
		{
			name:    "nil email is valid",
			email:   nil,
			wantErr: false,
		},
		{
			name:    "empty email is valid",
			email:   base.StringPtr(""),
			wantErr: false,
		},
		{
			name:    "invalid email - no at sign",
			email:   base.StringPtr("parentexample.com"),
			wantErr: true,
		},
		{
			name:    "invalid email - no domain",
			email:   base.StringPtr("parent@"),
			wantErr: true,
		},
		{
			name:    "invalid email - no TLD",
			email:   base.StringPtr("parent@example"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			student := &Student{
				PersonID:      1,
				SchoolClass:   "1a",
				GuardianEmail: tt.email,
			}

			err := student.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Student.Validate() with email %v, error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestStudent_Validate_GuardianPhone(t *testing.T) {
	tests := []struct {
		name    string
		phone   *string
		wantErr bool
	}{
		{
			name:    "valid phone - international format",
			phone:   base.StringPtr("+49 123 456789"),
			wantErr: false,
		},
		{
			name:    "valid phone - with dashes",
			phone:   base.StringPtr("123-456-7890"),
			wantErr: false,
		},
		{
			name:    "valid phone - simple digits",
			phone:   base.StringPtr("1234567890"),
			wantErr: false,
		},
		{
			name:    "nil phone is valid",
			phone:   nil,
			wantErr: false,
		},
		{
			name:    "empty phone is valid",
			phone:   base.StringPtr(""),
			wantErr: false,
		},
		{
			name:    "invalid phone - too short",
			phone:   base.StringPtr("123"),
			wantErr: true,
		},
		{
			name:    "invalid phone - contains letters",
			phone:   base.StringPtr("123-ABC-7890"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			student := &Student{
				PersonID:      1,
				SchoolClass:   "1a",
				GuardianPhone: tt.phone,
			}

			err := student.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Student.Validate() with phone %v, error = %v, wantErr %v", tt.phone, err, tt.wantErr)
			}
		})
	}
}

func TestStudent_SetPerson(t *testing.T) {
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
