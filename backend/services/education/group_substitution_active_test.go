package education

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/education"
)

func TestSubstitutionIsActive(t *testing.T) {
	t.Parallel()

	now := timezone.NewDate(2026, 6, 9)
	yesterday := now.AddDays(-1)
	tomorrow := now.AddDays(1)
	dayAfterTomorrow := now.AddDays(2)
	lastWeek := now.AddDays(-7)
	nextWeek := now.AddDays(7)

	tests := []struct {
		name         string
		substitution education.GroupSubstitution
		checkDate    timezone.Date
		want         bool
	}{
		{
			name:         "Active on start date",
			substitution: education.GroupSubstitution{StartDate: now, EndDate: tomorrow},
			checkDate:    now,
			want:         true,
		},
		{
			name:         "Active on end date",
			substitution: education.GroupSubstitution{StartDate: yesterday, EndDate: now},
			checkDate:    now,
			want:         true,
		},
		{
			name:         "Active between start and end",
			substitution: education.GroupSubstitution{StartDate: yesterday, EndDate: tomorrow},
			checkDate:    now,
			want:         true,
		},
		{
			name:         "Not active before start date",
			substitution: education.GroupSubstitution{StartDate: tomorrow, EndDate: dayAfterTomorrow},
			checkDate:    now,
			want:         false,
		},
		{
			name:         "Not active after end date",
			substitution: education.GroupSubstitution{StartDate: lastWeek, EndDate: yesterday},
			checkDate:    now,
			want:         false,
		},
		{
			name:         "Not active for future date",
			substitution: education.GroupSubstitution{StartDate: yesterday, EndDate: tomorrow},
			checkDate:    nextWeek,
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := tt.substitution
			if got := SubstitutionIsActive(&sub, tt.checkDate); got != tt.want {
				t.Errorf("SubstitutionIsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubstitutionIsActiveNow(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 6, 9, 12, 0, 0, 0, time.UTC)
	today := timezone.DateFromTime(now)
	yesterday := today.AddDays(-1)
	tomorrow := today.AddDays(1)
	lastWeek := today.AddDays(-7)

	tests := []struct {
		name string
		gs   *education.GroupSubstitution
		want bool
	}{
		{
			name: "currently active",
			gs:   &education.GroupSubstitution{StartDate: yesterday, EndDate: tomorrow},
			want: true,
		},
		{
			name: "already ended",
			gs:   &education.GroupSubstitution{StartDate: lastWeek, EndDate: yesterday},
			want: false,
		},
		{
			name: "not yet started",
			gs:   &education.GroupSubstitution{StartDate: tomorrow, EndDate: tomorrow.AddDays(1)},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SubstitutionIsActiveNow(tt.gs, now); got != tt.want {
				t.Errorf("SubstitutionIsActiveNow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubstitutionIsActive_Nil(t *testing.T) {
	t.Parallel()

	if SubstitutionIsActive(nil, timezone.TodayDate()) {
		t.Error("SubstitutionIsActive(nil) = true, want false")
	}
}
