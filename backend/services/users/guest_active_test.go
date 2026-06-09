package users

import (
	"testing"
	"time"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestGuestIsActive(t *testing.T) {
	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
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
			guest := &userModels.Guest{
				StartDate: tt.startDate,
				EndDate:   tt.endDate,
			}

			if got := GuestIsActive(guest, now); got != tt.expected {
				t.Errorf("GuestIsActive() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGuestIsActive_NilGuest(t *testing.T) {
	if GuestIsActive(nil, time.Now()) {
		t.Error("GuestIsActive(nil) = true, want false")
	}
}
