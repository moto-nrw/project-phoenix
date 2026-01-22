package users

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

func TestGuest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		guest   *Guest
		wantErr bool
	}{
		{
			name: "valid guest with required fields only",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
			},
			wantErr: false,
		},
		{
			name: "valid guest with all fields",
			guest: &Guest{
				StaffID:           1,
				Organization:      "Sports Club",
				ContactEmail:      "guest@example.com",
				ContactPhone:      "+49 123 456789",
				ActivityExpertise: "Basketball",
				StartDate:         base.TimePtr(time.Now()),
				EndDate:           base.TimePtr(time.Now().Add(30 * 24 * time.Hour)),
			},
			wantErr: false,
		},
		{
			name: "missing staff ID",
			guest: &Guest{
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "zero staff ID",
			guest: &Guest{
				StaffID:           0,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "negative staff ID",
			guest: &Guest{
				StaffID:           -1,
				ActivityExpertise: "Soccer",
			},
			wantErr: true,
		},
		{
			name: "missing activity expertise",
			guest: &Guest{
				StaffID: 1,
			},
			wantErr: true,
		},
		{
			name: "empty activity expertise",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "",
			},
			wantErr: true,
		},
		{
			name: "invalid email format",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactEmail:      "not-an-email",
			},
			wantErr: true,
		},
		{
			name: "invalid phone format",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				ContactPhone:      "abc",
			},
			wantErr: true,
		},
		{
			name: "end date before start date",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "Soccer",
				StartDate:         base.TimePtr(time.Now().Add(24 * time.Hour)),
				EndDate:           base.TimePtr(time.Now()),
			},
			wantErr: true,
		},
		{
			name: "trimmed whitespace",
			guest: &Guest{
				StaffID:           1,
				ActivityExpertise: "  Soccer  ",
				Organization:      "  Sports Club  ",
				ContactEmail:      "  guest@example.com  ",
				ContactPhone:      "  +49 123 456789  ",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.guest.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Guest.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGuest_SetStaff(t *testing.T) {
	t.Run("set with staff", func(t *testing.T) {
		guest := &Guest{
			ActivityExpertise: "Soccer",
		}

		staff := &Staff{
			TenantModel: base.TenantModel{Model: base.Model{ID: 42}},
		}

		guest.SetStaff(staff)

		if guest.Staff != staff {
			t.Error("SetStaff should set the Staff field")
		}
		if guest.StaffID != 42 {
			t.Errorf("SetStaff should set StaffID = 42, got %d", guest.StaffID)
		}
	})

	t.Run("set with nil staff", func(t *testing.T) {
		guest := &Guest{
			StaffID:           10,
			ActivityExpertise: "Soccer",
		}

		guest.SetStaff(nil)

		if guest.Staff != nil {
			t.Error("SetStaff(nil) should set Staff to nil")
		}
		// StaffID should remain unchanged when setting nil
		if guest.StaffID != 10 {
			t.Errorf("SetStaff(nil) should not change StaffID, got %d", guest.StaffID)
		}
	})
}

func TestGuest_GetFullName(t *testing.T) {
	t.Run("with staff and person", func(t *testing.T) {
		guest := &Guest{
			Staff: &Staff{
				Person: &Person{
					FirstName: "John",
					LastName:  "Doe",
				},
			},
		}

		got := guest.GetFullName()
		if got != "John Doe" {
			t.Errorf("Guest.GetFullName() = %q, want %q", got, "John Doe")
		}
	})

	t.Run("with staff but no person", func(t *testing.T) {
		guest := &Guest{
			Staff: &Staff{},
		}

		got := guest.GetFullName()
		if got != "" {
			t.Errorf("Guest.GetFullName() = %q, want empty string", got)
		}
	})

	t.Run("without staff", func(t *testing.T) {
		guest := &Guest{}

		got := guest.GetFullName()
		if got != "" {
			t.Errorf("Guest.GetFullName() = %q, want empty string", got)
		}
	})
}

func TestGuest_IsActive(t *testing.T) {
	now := time.Now()
	pastDate := now.Add(-30 * 24 * time.Hour)
	futureDate := now.Add(30 * 24 * time.Hour)

	tests := []struct {
		name      string
		startDate *time.Time
		endDate   *time.Time
		expected  bool
	}{
		{
			name:      "no dates - always active",
			startDate: nil,
			endDate:   nil,
			expected:  true,
		},
		{
			name:      "only start date in past - active",
			startDate: &pastDate,
			endDate:   nil,
			expected:  true,
		},
		{
			name:      "only start date in future - inactive",
			startDate: &futureDate,
			endDate:   nil,
			expected:  false,
		},
		{
			name:      "only end date in future - active",
			startDate: nil,
			endDate:   &futureDate,
			expected:  true,
		},
		{
			name:      "only end date in past - inactive",
			startDate: nil,
			endDate:   &pastDate,
			expected:  false,
		},
		{
			name:      "both dates - currently within range",
			startDate: &pastDate,
			endDate:   &futureDate,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			guest := &Guest{
				StartDate: tt.startDate,
				EndDate:   tt.endDate,
			}

			if got := guest.IsActive(); got != tt.expected {
				t.Errorf("Guest.IsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuest_AddNotes(t *testing.T) {
	t.Run("add first note", func(t *testing.T) {
		guest := &Guest{}
		guest.AddNotes("First note")

		if guest.Notes != "First note" {
			t.Errorf("Guest.Notes = %q, want %q", guest.Notes, "First note")
		}
	})

	t.Run("add second note", func(t *testing.T) {
		guest := &Guest{Notes: "First note"}
		guest.AddNotes("Second note")

		expected := "First note\nSecond note"
		if guest.Notes != expected {
			t.Errorf("Guest.Notes = %q, want %q", guest.Notes, expected)
		}
	})
}

func TestGuest_BeforeAppendModel(t *testing.T) {
	t.Run("handles nil query", func(t *testing.T) {
		guest := &Guest{StaffID: 1, ActivityExpertise: "Soccer"}
		err := guest.BeforeAppendModel(nil)
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})

	t.Run("returns no error for unknown query type", func(t *testing.T) {
		guest := &Guest{StaffID: 1, ActivityExpertise: "Soccer"}
		err := guest.BeforeAppendModel("some string")
		if err != nil {
			t.Errorf("BeforeAppendModel() error = %v", err)
		}
	})
}

func TestGuest_TableName(t *testing.T) {
	guest := &Guest{}
	if got := guest.TableName(); got != "users.guests" {
		t.Errorf("TableName() = %v, want users.guests", got)
	}
}

func TestGuest_GetID(t *testing.T) {
	guest := &Guest{
		Model:             base.Model{ID: 42},
		StaffID:           1,
		ActivityExpertise: "Soccer",
	}

	if got, ok := guest.GetID().(int64); !ok || got != 42 {
		t.Errorf("GetID() = %v, want 42", guest.GetID())
	}
}

func TestGuest_GetCreatedAt(t *testing.T) {
	now := time.Now()
	guest := &Guest{
		Model:             base.Model{CreatedAt: now},
		StaffID:           1,
		ActivityExpertise: "Soccer",
	}

	if got := guest.GetCreatedAt(); !got.Equal(now) {
		t.Errorf("GetCreatedAt() = %v, want %v", got, now)
	}
}

func TestGuest_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	guest := &Guest{
		Model:             base.Model{UpdatedAt: now},
		StaffID:           1,
		ActivityExpertise: "Soccer",
	}

	if got := guest.GetUpdatedAt(); !got.Equal(now) {
		t.Errorf("GetUpdatedAt() = %v, want %v", got, now)
	}
}
