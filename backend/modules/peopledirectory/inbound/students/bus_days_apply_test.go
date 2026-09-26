package students

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	"github.com/stretchr/testify/assert"
)

// TestApplyBusDays verifies the create/update bus_days resolution (#1582):
// bus_days is the source of truth and the legacy bus boolean is only an alias
// that must never flatten an existing per-day selection.
func TestApplyBusDays(t *testing.T) {
	t.Parallel()

	boolPtr := func(b bool) *bool { return &b }
	daysPtr := func(d departure.BusDays) *departure.BusDays { return &d }

	tests := []struct {
		name      string
		legacyBus *bool
		days      *departure.BusDays
		existing  departure.BusDays
		want      departure.BusDays
	}{
		{
			name: "bus_days provided wins and replaces existing",
			days: daysPtr(departure.BusDays{departure.BusDayTuesday: true}),
			existing: departure.BusDays{
				departure.BusDayMonday: true,
				departure.BusDayFriday: true,
			},
			want: departure.BusDays{departure.BusDayTuesday: true},
		},
		{
			name:      "bus_days wins over legacy bus when both are sent",
			legacyBus: boolPtr(true),
			days:      daysPtr(departure.BusDays{departure.BusDayWednesday: true}),
			want:      departure.BusDays{departure.BusDayWednesday: true},
		},
		{
			name:      "neither provided leaves bus_days untouched",
			legacyBus: nil,
			days:      nil,
			existing:  departure.BusDays{departure.BusDayThursday: true},
			want:      departure.BusDays{departure.BusDayThursday: true},
		},
		{
			name:      "legacy bus=false clears all days",
			legacyBus: boolPtr(false),
			existing: departure.BusDays{
				departure.BusDayMonday: true,
				departure.BusDayFriday: true,
			},
			want: departure.BusDays{},
		},
		{
			name:      "legacy bus=true with no existing days defaults to all weekdays",
			legacyBus: boolPtr(true),
			existing:  nil,
			want: departure.BusDays{
				departure.BusDayMonday:    true,
				departure.BusDayTuesday:   true,
				departure.BusDayWednesday: true,
				departure.BusDayThursday:  true,
				departure.BusDayFriday:    true,
			},
		},
		{
			name:      "legacy bus=true preserves an existing per-day selection",
			legacyBus: boolPtr(true),
			existing: departure.BusDays{
				departure.BusDayMonday: true,
				departure.BusDayFriday: true,
			},
			want: departure.BusDays{
				departure.BusDayMonday: true,
				departure.BusDayFriday: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			student := &Student{BusDays: tc.existing}
			applyBusDays(tc.legacyBus, tc.days, student)
			assert.Equal(t, tc.want, student.BusDays)
		})
	}
}
