package education

import (
	"testing"
	"time"
)

// Helper function to create int64 pointer
func ptr(i int64) *int64 {
	return &i
}

func TestGroupSubstitution_Validate(t *testing.T) {
	currentTime := time.Now()
	tomorrow := currentTime.AddDate(0, 0, 1)

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
				StartDate:         time.Time{}, // Zero time
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
				EndDate:           time.Time{}, // Zero time
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
	now := time.Now()

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
				EndDate:   now.AddDate(0, 0, 1),
			},
			want: 2,
		},
		{
			name: "Week-long substitution",
			substitution: GroupSubstitution{
				StartDate: now,
				EndDate:   now.AddDate(0, 0, 6),
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

func TestGroupSubstitution_IsActive(t *testing.T) {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)
	tomorrow := now.AddDate(0, 0, 1)
	dayAfterTomorrow := now.AddDate(0, 0, 2)
	lastWeek := now.AddDate(0, 0, -7)
	nextWeek := now.AddDate(0, 0, 7)

	tests := []struct {
		name         string
		substitution GroupSubstitution
		checkDate    time.Time
		want         bool
	}{
		{
			name: "Active on start date",
			substitution: GroupSubstitution{
				StartDate: now,
				EndDate:   tomorrow,
			},
			checkDate: now,
			want:      true,
		},
		{
			name: "Active on end date",
			substitution: GroupSubstitution{
				StartDate: yesterday,
				EndDate:   now,
			},
			checkDate: now,
			want:      true,
		},
		{
			name: "Active between start and end",
			substitution: GroupSubstitution{
				StartDate: yesterday,
				EndDate:   tomorrow,
			},
			checkDate: now,
			want:      true,
		},
		{
			name: "Not active before start date",
			substitution: GroupSubstitution{
				StartDate: tomorrow,
				EndDate:   dayAfterTomorrow,
			},
			checkDate: now,
			want:      false,
		},
		{
			name: "Not active after end date",
			substitution: GroupSubstitution{
				StartDate: lastWeek,
				EndDate:   yesterday,
			},
			checkDate: now,
			want:      false,
		},
		{
			name: "Not active for future date",
			substitution: GroupSubstitution{
				StartDate: yesterday,
				EndDate:   tomorrow,
			},
			checkDate: nextWeek,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.substitution.IsActive(tt.checkDate); got != tt.want {
				t.Errorf("GroupSubstitution.IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}
