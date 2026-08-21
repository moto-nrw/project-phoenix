package users

import (
	"strings"
	"testing"
)

func TestDepartureDaysValidate(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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

// TestAllowedDepartureModesLegacyPickupStatus locks the safety-critical
// derivation that the exclusive DepartureDays() projection cannot express: a day
// allowing BOTH bus and accompanied must still report the accompanied status,
// because the projection ranks bus above accompanied and would otherwise bucket
// the child as a self-goer (#1694).
func TestAllowedDepartureModesLegacyPickupStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		modes AllowedDepartureModes
		want  string
	}{
		{name: "empty goes alone", modes: AllowedDepartureModes{}, want: PickupStatusGoesAlone},
		{name: "bus only goes alone", modes: AllowedDepartureModes{PickupDayMonday: {DepartureBus}}, want: PickupStatusGoesAlone},
		{name: "accompanied only", modes: AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied}}, want: PickupStatusAccompanied},
		{
			name:  "bus and accompanied same day keeps accompanied",
			modes: AllowedDepartureModes{PickupDayMonday: {DepartureBus, DepartureAccompanied}},
			want:  PickupStatusAccompanied,
		},
		{
			name:  "bus one day accompanied another keeps accompanied",
			modes: AllowedDepartureModes{PickupDayMonday: {DepartureBus}, PickupDayTuesday: {DepartureAccompanied}},
			want:  PickupStatusAccompanied,
		},
		{
			name:  "pickup outranks accompanied",
			modes: AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied, DeparturePickup}},
			want:  PickupStatusPickedUp,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.modes.LegacyPickupStatus(); got != tt.want {
				t.Fatalf("LegacyPickupStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestAllowedDepartureModesMerge covers the union used when one submission
// carries multiple fields targeting allowed_departure_modes: a later field never
// drops a mode an earlier field allowed (#1694).
func TestAllowedDepartureModesMerge(t *testing.T) {
	t.Parallel()

	t.Run("union preserves accompanied from the earlier field", func(t *testing.T) {
		first := AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied}}
		second := AllowedDepartureModes{PickupDayMonday: {DepartureBus}}
		got := first.Merge(second)
		if !allowedModesContain(got[PickupDayMonday], DepartureAccompanied) {
			t.Fatalf("merge dropped accompanied: %#v", got[PickupDayMonday])
		}
		if !allowedModesContain(got[PickupDayMonday], DepartureBus) {
			t.Fatalf("merge dropped bus: %#v", got[PickupDayMonday])
		}
	})

	t.Run("dedupes and normalizes", func(t *testing.T) {
		first := AllowedDepartureModes{PickupDayMonday: {DepartureBus, DepartureAccompanied}}
		second := AllowedDepartureModes{PickupDayMonday: {DepartureAccompanied}}
		got := first.Merge(second)
		if len(got[PickupDayMonday]) != 2 {
			t.Fatalf("merge mon = %#v, want 2 deduped modes", got[PickupDayMonday])
		}
		// Normalized canonical order: bus before accompanied.
		if got[PickupDayMonday][0] != DepartureBus || got[PickupDayMonday][1] != DepartureAccompanied {
			t.Fatalf("merge mon order = %#v, want [bus accompanied]", got[PickupDayMonday])
		}
	})
}

// TestDepartureDaysMerge covers merging two exclusive plans: the higher-stakes
// mode wins per day, and accompanied always beats bus so the merge never drops
// the "Mit anderem Kind" signal (#1694).
func TestDepartureDaysMerge(t *testing.T) {
	t.Parallel()

	t.Run("accompanied beats bus", func(t *testing.T) {
		got := DepartureDays{PickupDayMonday: DepartureAccompanied}.
			Merge(DepartureDays{PickupDayMonday: DepartureBus})
		if got.ModeFor(PickupDayMonday) != DepartureAccompanied {
			t.Fatalf("mon = %q, want accompanied", got.ModeFor(PickupDayMonday))
		}
	})

	t.Run("pickup beats accompanied", func(t *testing.T) {
		got := DepartureDays{PickupDayMonday: DepartureAccompanied}.
			Merge(DepartureDays{PickupDayMonday: DeparturePickup})
		if got.ModeFor(PickupDayMonday) != DeparturePickup {
			t.Fatalf("mon = %q, want pickup", got.ModeFor(PickupDayMonday))
		}
	})

	t.Run("disjoint days combine", func(t *testing.T) {
		got := DepartureDays{PickupDayMonday: DepartureBus}.
			Merge(DepartureDays{PickupDayTuesday: DepartureAccompanied})
		if got.ModeFor(PickupDayMonday) != DepartureBus {
			t.Fatalf("mon = %q, want bus", got.ModeFor(PickupDayMonday))
		}
		if got.ModeFor(PickupDayTuesday) != DepartureAccompanied {
			t.Fatalf("tue = %q, want accompanied", got.ModeFor(PickupDayTuesday))
		}
	})
}

// TestDepartureModeGermanLabel pins the single source of truth for the
// parent-facing German wording of each departure mode (used by the parent
// messaging diff via germanAllowedDepartureModes). The accompanied case is
// safety-sensitive: an "accompanied" child leaves WITH another child/person and
// must NEVER fall through to "Geht alleine" — that would mislabel the child as a
// self-goer on the staff confirm diff (#1694). An unknown mode falls back to the
// alone label, matching how an unset weekday is treated.
func TestDepartureModeGermanLabel(t *testing.T) {
	t.Parallel()

	cases := map[DepartureMode]string{
		DepartureBus:         "Fährt Bus",
		DeparturePickup:      "Wird abgeholt",
		DepartureAccompanied: "Geht mit anderem Kind/Person",
		DepartureAlone:       "Geht alleine",
		DepartureMode("xxx"): "Geht alleine", // unknown → safe default
	}
	for mode, want := range cases {
		if got := mode.GermanLabel(); got != want {
			t.Errorf("DepartureMode(%q).GermanLabel() = %q, want %q", mode, got, want)
		}
	}
	// The accompanied label must be distinct from the alone label — regression
	// guard against a future refactor letting accompanied fall through to default.
	if DepartureAccompanied.GermanLabel() == DepartureAlone.GermanLabel() {
		t.Fatal("accompanied must not render as the alone label")
	}
}
