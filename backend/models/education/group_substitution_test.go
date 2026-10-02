package education

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Helper function to create int64 pointer
func ptr(i int64) *int64 {
	return &i
}

func TestGroupSubstitution_Validate(t *testing.T) {
	t.Parallel()

	currentTime := calendar.TodayDate()
	tomorrow := currentTime.AddDays(1)

	tests := []struct {
		name         string
		substitution GroupSubstitution
		wantErr      bool
		errorMessage string
	}{
		{
			name: "Valid substitution",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 2,
				StartDate:         currentTime,
				EndDate:           tomorrow,
				Reason:            "Vacation",
			},
			wantErr: false,
		},
		{
			name: "Missing group ID",
			substitution: GroupSubstitution{
				GroupID:           0, // Invalid
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 2,
				StartDate:         currentTime,
				EndDate:           tomorrow,
			},
			wantErr:      true,
			errorMessage: "group ID is required",
		},
		{
			name: "Valid substitution without regular staff (general coverage)",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    nil, // Optional - general coverage
				SubstituteStaffID: 2,
				StartDate:         currentTime,
				EndDate:           tomorrow,
			},
			wantErr: false, // This is now valid
		},
		{
			name: "Invalid regular staff ID when provided",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(0), // Invalid when provided
				SubstituteStaffID: 2,
				StartDate:         currentTime,
				EndDate:           tomorrow,
			},
			wantErr:      true,
			errorMessage: "regular staff ID must be positive if provided",
		},
		{
			name: "Missing substitute staff ID",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 0, // Invalid
				StartDate:         currentTime,
				EndDate:           tomorrow,
			},
			wantErr:      true,
			errorMessage: "substitute staff ID is required",
		},
		{
			name: "Missing start date",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 2,
				StartDate:         calendar.Date(""), // Zero date
				EndDate:           tomorrow,
			},
			wantErr:      true,
			errorMessage: "start date is required",
		},
		{
			name: "Missing end date",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 2,
				StartDate:         currentTime,
				EndDate:           calendar.Date(""), // Zero date
			},
			wantErr:      true,
			errorMessage: "end date is required",
		},
		{
			name: "End date before start date",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 2,
				StartDate:         tomorrow,
				EndDate:           currentTime, // Before start date
			},
			wantErr:      true,
			errorMessage: "end date cannot be before start date",
		},
		{
			name: "Same regular and substitute staff",
			substitution: GroupSubstitution{
				GroupID:           1,
				RegularStaffID:    ptr(1),
				SubstituteStaffID: 1, // Same as regular staff
				StartDate:         currentTime,
				EndDate:           tomorrow,
			},
			wantErr:      true,
			errorMessage: "regular staff and substitute staff cannot be the same",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.substitution.Validate()

			// Check if we expected an error
			if (err != nil) != tt.wantErr {
				t.Errorf("GroupSubstitution.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If we expected a specific error message, check it
			if tt.wantErr && err.Error() != tt.errorMessage {
				t.Errorf("GroupSubstitution.Validate() error message = %v, want %v", err.Error(), tt.errorMessage)
			}
		})
	}
}

func TestGroupSubstitution_Duration(t *testing.T) {
	t.Parallel()

	// Use a fixed date to avoid DST edge cases
	now := calendar.NewDate(2026, time.June, 15)

	tests := []struct {
		name         string
		substitution GroupSubstitution
		want         int
	}{
		{
			name: "One day substitution",
			substitution: GroupSubstitution{
				StartDate: now,
				EndDate:   now,
			},
			want: 1,
		},
		{
			name: "Two day substitution",
			substitution: GroupSubstitution{
				StartDate: now,
				EndDate:   now.AddDays(1),
			},
			want: 2,
		},
		{
			name: "Week-long substitution",
			substitution: GroupSubstitution{
				StartDate: now,
				EndDate:   now.AddDays(6),
			},
			want: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.substitution.Duration(); got != tt.want {
				t.Errorf("GroupSubstitution.Duration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGroupSubstitution_SetGroup(t *testing.T) {
	t.Parallel()

	t.Run("set group", func(t *testing.T) {
		gs := &GroupSubstitution{SubstituteStaffID: 1}
		group := &Group{
			Model: Model{ID: 42},
			Name:  "Test Group",
		}

		gs.SetGroup(group)

		if gs.Group != group {
			t.Error("GroupSubstitution.SetGroup() did not set Group reference")
		}

		if gs.GroupID != 42 {
			t.Errorf("GroupSubstitution.GroupID = %v, want 42", gs.GroupID)
		}
	})

	t.Run("set nil group", func(t *testing.T) {
		gs := &GroupSubstitution{
			GroupID:           42,
			SubstituteStaffID: 1,
		}

		gs.SetGroup(nil)

		if gs.Group != nil {
			t.Error("GroupSubstitution.SetGroup(nil) did not clear Group reference")
		}
	})
}
