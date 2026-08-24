package students

import (
	"testing"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
)

// TestApplyBusDays verifies the create/update bus_days resolution (#1582):
// bus_days is the source of truth and the legacy bus boolean is only an alias
// that must never flatten an existing per-day selection.
func TestApplyBusDays(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }
	daysPtr := func(d usersModel.BusDays) *usersModel.BusDays { return &d }

	tests := []struct {
		name      string
		legacyBus *bool
		days      *usersModel.BusDays
		existing  usersModel.BusDays
		want      usersModel.BusDays
	}{
		{
			name: "bus_days provided wins and replaces existing",
			days: daysPtr(usersModel.BusDays{usersModel.BusDayTuesday: true}),
			existing: usersModel.BusDays{
				usersModel.BusDayMonday: true,
				usersModel.BusDayFriday: true,
			},
			want: usersModel.BusDays{usersModel.BusDayTuesday: true},
		},
		{
			name:      "bus_days wins over legacy bus when both are sent",
			legacyBus: boolPtr(true),
			days:      daysPtr(usersModel.BusDays{usersModel.BusDayWednesday: true}),
			want:      usersModel.BusDays{usersModel.BusDayWednesday: true},
		},
		{
			name:      "neither provided leaves bus_days untouched",
			legacyBus: nil,
			days:      nil,
			existing:  usersModel.BusDays{usersModel.BusDayThursday: true},
			want:      usersModel.BusDays{usersModel.BusDayThursday: true},
		},
		{
			name:      "legacy bus=false clears all days",
			legacyBus: boolPtr(false),
			existing: usersModel.BusDays{
				usersModel.BusDayMonday: true,
				usersModel.BusDayFriday: true,
			},
			want: usersModel.BusDays{},
		},
		{
			name:      "legacy bus=true with no existing days defaults to all weekdays",
			legacyBus: boolPtr(true),
			existing:  nil,
			want: usersModel.BusDays{
				usersModel.BusDayMonday:    true,
				usersModel.BusDayTuesday:   true,
				usersModel.BusDayWednesday: true,
				usersModel.BusDayThursday:  true,
				usersModel.BusDayFriday:    true,
			},
		},
		{
			name:      "legacy bus=true preserves an existing per-day selection",
			legacyBus: boolPtr(true),
			existing: usersModel.BusDays{
				usersModel.BusDayMonday: true,
				usersModel.BusDayFriday: true,
			},
			want: usersModel.BusDays{
				usersModel.BusDayMonday: true,
				usersModel.BusDayFriday: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			student := &usersModel.Student{BusDays: tc.existing}
			applyBusDays(tc.legacyBus, tc.days, student)
			assert.Equal(t, tc.want, student.BusDays)
		})
	}
}
