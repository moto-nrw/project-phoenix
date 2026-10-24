package peopledirectory

import (
	"testing"
)

func TestTeacher_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		teacher *Teacher
		wantErr bool
	}{
		{
			name: "valid teacher",
			teacher: &Teacher{
				StaffID: 1,
			},
			wantErr: false,
		},
		{
			name: "valid teacher with specialization",
			teacher: &Teacher{
				StaffID:        1,
				Specialization: "Mathematics",
			},
			wantErr: false,
		},
		{
			name: "valid teacher with all fields",
			teacher: &Teacher{
				StaffID:        1,
				Specialization: "Physics",
				Role:           "Head of Department",
				Qualifications: "PhD in Physics",
			},
			wantErr: false,
		},
		{
			name: "zero staff ID",
			teacher: &Teacher{
				StaffID: 0,
			},
			wantErr: true,
		},
		{
			name: "negative staff ID",
			teacher: &Teacher{
				StaffID: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.teacher.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Teacher.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTeacher_Validate_Normalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		inputSpecialization    string
		inputRole              string
		expectedSpecialization string
		expectedRole           string
	}{
		{
			name:                   "trims specialization whitespace",
			inputSpecialization:    "  Mathematics  ",
			inputRole:              "Teacher",
			expectedSpecialization: "Mathematics",
			expectedRole:           "Teacher",
		},
		{
			name:                   "trims role whitespace",
			inputSpecialization:    "Physics",
			inputRole:              "  Head of Department  ",
			expectedSpecialization: "Physics",
			expectedRole:           "Head of Department",
		},
		{
			name:                   "handles empty specialization",
			inputSpecialization:    "",
			inputRole:              "Teacher",
			expectedSpecialization: "",
			expectedRole:           "Teacher",
		},
		{
			name:                   "handles whitespace-only specialization",
			inputSpecialization:    "   ",
			inputRole:              "Teacher",
			expectedSpecialization: "",
			expectedRole:           "Teacher",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			teacher := &Teacher{
				StaffID:        1,
				Specialization: tt.inputSpecialization,
				Role:           tt.inputRole,
			}

			err := teacher.Validate()
			if err != nil {
				t.Fatalf("Teacher.Validate() unexpected error = %v", err)
			}

			if teacher.Specialization != tt.expectedSpecialization {
				t.Errorf("Teacher.Specialization = %q, want %q", teacher.Specialization, tt.expectedSpecialization)
			}
			if teacher.Role != tt.expectedRole {
				t.Errorf("Teacher.Role = %q, want %q", teacher.Role, tt.expectedRole)
			}
		})
	}
}
