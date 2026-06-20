package users

import (
	"strings"
	"testing"
)

func TestDepartureDaysValidate(t *testing.T) {
	tests := []struct {
		name    string
		days    DepartureDays
		wantErr string
	}{
		{name: "accepts empty days", days: DepartureDays{}},
		{
			name: "accepts valid modes",
			days: DepartureDays{
				PickupDayMonday:  DepartureBus,
				PickupDayFriday:  DeparturePickup,
				PickupDayTuesday: DepartureAlone,
			},
		},
		{
			name:    "rejects unknown day",
			days:    DepartureDays{"sat": DepartureBus},
			wantErr: `weekday "sat"`,
		},
		{
			name:    "rejects unknown mode",
			days:    DepartureDays{PickupDayMonday: DepartureMode("walks")},
			wantErr: `mode "walks"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.days.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("DepartureDays.Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("DepartureDays.Validate() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("DepartureDays.Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestDepartureDaysNormalize(t *testing.T) {
	days := DepartureDays{
		PickupDayMonday:    DepartureBus,
		PickupDayTuesday:   DepartureAlone, // dropped (default)
		PickupDayWednesday: DeparturePickup,
		"sat":              DepartureBus, // dropped (unknown)
	}

	got := days.Normalize()
	want := DepartureDays{
		PickupDayMonday:    DepartureBus,
		PickupDayWednesday: DeparturePickup,
	}

	if len(got) != len(want) {
		t.Fatalf("Normalize() length = %d, want %d: %#v", len(got), len(want), got)
	}
	for day, mode := range want {
		if got[day] != mode {
			t.Fatalf("Normalize()[%q] = %q, want %q", day, got[day], mode)
		}
	}
}

func TestDepartureDaysModeForAndHasAny(t *testing.T) {
	days := DepartureDays{PickupDayMonday: DepartureBus}
	if got := days.ModeFor(PickupDayMonday); got != DepartureBus {
		t.Fatalf("ModeFor(mon) = %q, want bus", got)
	}
	if got := days.ModeFor(PickupDayTuesday); got != DepartureAlone {
		t.Fatalf("ModeFor(tue) = %q, want alone (default)", got)
	}
	if !days.HasAny() {
		t.Fatal("HasAny() = false, want true")
	}
	if (DepartureDays{}).HasAny() {
		t.Fatal("empty HasAny() = true, want false")
	}
}

func TestDepartureDaysDerivations(t *testing.T) {
	days := DepartureDays{
		PickupDayMonday:    DepartureBus,
		PickupDayWednesday: DeparturePickup,
		PickupDayFriday:    DepartureBus,
	}

	bus := days.BusDays()
	if !bus[PickupDayMonday] || !bus[PickupDayFriday] || bus[PickupDayWednesday] {
		t.Fatalf("BusDays() = %#v, want mon+fri only", bus)
	}

	pickup := days.PickupDays()
	if !pickup[PickupDayWednesday] || pickup[PickupDayMonday] {
		t.Fatalf("PickupDays() = %#v, want wed only", pickup)
	}

	if got := days.LegacyPickupStatus(); got != PickupStatusPickedUp {
		t.Fatalf("LegacyPickupStatus() = %q, want %q", got, PickupStatusPickedUp)
	}

	// Bus-only must NOT count as picked up for the legacy string.
	busOnly := DepartureDays{PickupDayMonday: DepartureBus}
	if got := busOnly.LegacyPickupStatus(); got != PickupStatusGoesAlone {
		t.Fatalf("bus-only LegacyPickupStatus() = %q, want %q", got, PickupStatusGoesAlone)
	}
}

func TestDepartureDaysFromLegacy(t *testing.T) {
	t.Run("merges disjoint maps", func(t *testing.T) {
		got := DepartureDaysFromLegacy(
			BusDays{PickupDayMonday: true},
			PickupDays{PickupDayWednesday: true},
		)
		if got.ModeFor(PickupDayMonday) != DepartureBus {
			t.Fatalf("mon = %q, want bus", got.ModeFor(PickupDayMonday))
		}
		if got.ModeFor(PickupDayWednesday) != DeparturePickup {
			t.Fatalf("wed = %q, want pickup", got.ModeFor(PickupDayWednesday))
		}
		if got.ModeFor(PickupDayFriday) != DepartureAlone {
			t.Fatalf("fri = %q, want alone", got.ModeFor(PickupDayFriday))
		}
	})

	t.Run("pickup wins on contradiction", func(t *testing.T) {
		got := DepartureDaysFromLegacy(
			BusDays{PickupDayMonday: true},
			PickupDays{PickupDayMonday: true},
		)
		if got.ModeFor(PickupDayMonday) != DeparturePickup {
			t.Fatalf("contradictory mon = %q, want pickup (pickup wins)", got.ModeFor(PickupDayMonday))
		}
	})
}

// TestDepartureAccompaniedMode covers the fourth departure mode (#1694): it is
// a valid, normalizable mode that survives the DepartureDays <-> allowed-modes
// round-trip but never leaks into the legacy bus/pickup mirrors.
func TestDepartureAccompaniedMode(t *testing.T) {
	t.Run("validates and normalizes", func(t *testing.T) {
		days := DepartureDays{PickupDayMonday: DepartureAccompanied}
		if err := days.Validate(); err != nil {
			t.Fatalf("Validate() unexpected error = %v", err)
		}
		if got := days.Normalize(); got[PickupDayMonday] != DepartureAccompanied {
			t.Fatalf("Normalize() dropped accompanied: %#v", got)
		}
		if got := days.ModeFor(PickupDayMonday); got != DepartureAccompanied {
			t.Fatalf("ModeFor(mon) = %q, want accompanied", got)
		}
		if !days.HasAny() {
			t.Fatal("HasAny() = false for accompanied day, want true")
		}
	})

	t.Run("does not leak into legacy bus/pickup mirrors", func(t *testing.T) {
		days := DepartureDays{PickupDayMonday: DepartureAccompanied}
		if days.BusDays().HasAny() {
			t.Fatal("accompanied counted as a bus day")
		}
		if days.PickupDays().HasAny() {
			t.Fatal("accompanied counted as a pickup day")
		}
	})

	t.Run("derives a dedicated non-self legacy pickup status", func(t *testing.T) {
		// An accompanied child leaves WITH a companion and must never collapse onto
		// the self-goer status, which would let legacy search/list/admin-filter
		// consumers bucket safety-sensitive accompanied children as going home alone
		// (#1694). It is also not a pickup, so it gets its own value rather than
		// "Wird abgeholt".
		days := DepartureDays{PickupDayMonday: DepartureAccompanied}
		if got := days.LegacyPickupStatus(); got != PickupStatusAccompanied {
			t.Fatalf("LegacyPickupStatus() = %q, want %q", got, PickupStatusAccompanied)
		}

		// A pickup day still outranks accompanied for the legacy string.
		mixed := DepartureDays{PickupDayMonday: DepartureAccompanied, PickupDayTuesday: DeparturePickup}
		if got := mixed.LegacyPickupStatus(); got != PickupStatusPickedUp {
			t.Fatalf("accompanied+pickup LegacyPickupStatus() = %q, want %q", got, PickupStatusPickedUp)
		}
	})

	t.Run("survives allowed-modes round-trip", func(t *testing.T) {
		allowed := AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied}}
		if err := allowed.Validate(); err != nil {
			t.Fatalf("AllowedDepartureModes.Validate() unexpected error = %v", err)
		}
		if got := allowed.DepartureDays().ModeFor(PickupDayMonday); got != DepartureAccompanied {
			t.Fatalf("allowed -> DepartureDays mon = %q, want accompanied", got)
		}
		back := AllowedDepartureModesFromDeparture(DepartureDays{PickupDayMonday: DepartureAccompanied})
		if len(back[PickupDayMonday]) != 1 || back[PickupDayMonday][0] != DepartureAccompanied {
			t.Fatalf("DepartureDays -> allowed mon = %#v, want [accompanied]", back[PickupDayMonday])
		}
	})

	t.Run("pickup outranks accompanied in exclusive derivation", func(t *testing.T) {
		allowed := AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied, DeparturePickup}}
		if got := allowed.DepartureDays().ModeFor(PickupDayMonday); got != DeparturePickup {
			t.Fatalf("mixed mon = %q, want pickup (higher stakes)", got)
		}
	})
}
