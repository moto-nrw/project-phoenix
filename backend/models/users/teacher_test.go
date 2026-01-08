package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestTeacher_Validate(t *testing.T) {
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

func TestTeacher_SetStaff(t *testing.T) {
	t.Run("set staff", func(t *testing.T) {
		teacher := &Teacher{}

		person := &Person{
			Model:     base.Model{ID: 10},
			FirstName: "John",
			LastName:  "Doe",
		}
		staff := &Staff{
			Model:    base.Model{ID: 42},
			PersonID: 10,
			Person:   person,
		}

		teacher.SetStaff(staff)

		if teacher.Staff != staff {
			t.Error("Teacher.SetStaff() did not set Staff reference")
		}

		if teacher.StaffID != 42 {
			t.Errorf("Teacher.SetStaff() did not set StaffID, got %v", teacher.StaffID)
		}
	})

	t.Run("set nil staff", func(t *testing.T) {
		teacher := &Teacher{
			StaffID: 42,
		}

		teacher.SetStaff(nil)

		if teacher.Staff != nil {
			t.Error("Teacher.SetStaff(nil) did not clear Staff reference")
		}

		// StaffID is not cleared - this matches the implementation
	})
}

func TestTeacher_GetFullName(t *testing.T) {
	tests := []struct {
		name     string
		teacher  *Teacher
		expected string
	}{
		{
			name: "with staff and person",
			teacher: &Teacher{
				StaffID: 1,
				Staff: &Staff{
					PersonID: 1,
					Person: &Person{
						FirstName: "John",
						LastName:  "Doe",
					},
				},
			},
			expected: "John Doe",
		},
		{
			name: "with staff but nil person",
			teacher: &Teacher{
				StaffID: 1,
				Staff: &Staff{
					PersonID: 1,
					Person:   nil,
				},
			},
			expected: "",
		},
		{
			name: "without staff",
			teacher: &Teacher{
				StaffID: 1,
				Staff:   nil,
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.teacher.GetFullName()
			if got != tt.expected {
				t.Errorf("Teacher.GetFullName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTeacher_GetTitle(t *testing.T) {
	tests := []struct {
		name     string
		teacher  *Teacher
		expected string
	}{
		{
			name: "role takes precedence",
			teacher: &Teacher{
				StaffID:        1,
				Role:           "Head of Department",
				Specialization: "Mathematics",
			},
			expected: "Head of Department",
		},
		{
			name: "falls back to specialization",
			teacher: &Teacher{
				StaffID:        1,
				Role:           "",
				Specialization: "Physics",
			},
			expected: "Physics",
		},
		{
			name: "empty when both are empty",
			teacher: &Teacher{
				StaffID:        1,
				Role:           "",
				Specialization: "",
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.teacher.GetTitle()
			if got != tt.expected {
				t.Errorf("Teacher.GetTitle() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTeacher_HasQualifications(t *testing.T) {
	tests := []struct {
		name     string
		teacher  *Teacher
		expected bool
	}{
		{
			name: "has qualifications",
			teacher: &Teacher{
				StaffID:        1,
				Qualifications: "PhD in Computer Science",
			},
			expected: true,
		},
		{
			name: "no qualifications",
			teacher: &Teacher{
				StaffID:        1,
				Qualifications: "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.teacher.HasQualifications()
			if got != tt.expected {
				t.Errorf("Teacher.HasQualifications() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestTeacher_TableName(t *testing.T) {
	teacher := &Teacher{}
	expected := "users.teachers"

	got := teacher.TableName()
	if got != expected {
		t.Errorf("Teacher.TableName() = %q, want %q", got, expected)
	}
}

func TestTeacher_EntityInterface(t *testing.T) {
	now := time.Now()
	teacher := &Teacher{
		Model: base.Model{
			ID:        123,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Hour),
		},
		StaffID: 1,
	}

	t.Run("GetID", func(t *testing.T) {
		got := teacher.GetID()
		if got != int64(123) {
			t.Errorf("Teacher.GetID() = %v, want %v", got, int64(123))
		}
	})

	t.Run("GetCreatedAt", func(t *testing.T) {
		got := teacher.GetCreatedAt()
		if !got.Equal(now) {
			t.Errorf("Teacher.GetCreatedAt() = %v, want %v", got, now)
		}
	})

	t.Run("GetUpdatedAt", func(t *testing.T) {
		expected := now.Add(time.Hour)
		got := teacher.GetUpdatedAt()
		if !got.Equal(expected) {
			t.Errorf("Teacher.GetUpdatedAt() = %v, want %v", got, expected)
		}
	})
}
