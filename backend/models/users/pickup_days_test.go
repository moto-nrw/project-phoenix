package users

import (
	"strings"
	"testing"
)

func TestPickupDaysValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		days    PickupDays
		wantErr string
	}{
		{
			name: "accepts empty days",
			days: PickupDays{},
		},
		{
			name: "accepts weekdays",
			days: PickupDays{
				PickupDayMonday:    true,
				PickupDayWednesday: false,
				PickupDayFriday:    true,
			},
		},
		{
			name: "rejects unknown day",
			days: PickupDays{
				"sat": true,
			},
			wantErr: `weekday "sat"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.days.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("PickupDays.Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("PickupDays.Validate() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("PickupDays.Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestPickupDaysHasAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		days PickupDays
		want bool
	}{
		{name: "empty days", days: PickupDays{}, want: false},
		{name: "all false weekdays", days: PickupDays{PickupDayMonday: false, PickupDayFriday: false}, want: false},
		{name: "known true weekday", days: PickupDays{PickupDayTuesday: true}, want: true},
		{name: "unknown true day is ignored", days: PickupDays{"sat": true}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.days.HasAny(); got != tt.want {
				t.Fatalf("PickupDays.HasAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPickupDaysNormalize(t *testing.T) {
	t.Parallel()

	days := PickupDays{
		PickupDayMonday:    true,
		PickupDayTuesday:   false,
		PickupDayWednesday: true,
		"sat":              true,
	}

	got := days.Normalize()
	want := PickupDays{
		PickupDayMonday:    true,
		PickupDayWednesday: true,
	}

	if len(got) != len(want) {
		t.Fatalf("PickupDays.Normalize() length = %d, want %d: %#v", len(got), len(want), got)
	}
	for day, wantEnabled := range want {
		if got[day] != wantEnabled {
			t.Fatalf("PickupDays.Normalize()[%q] = %v, want %v", day, got[day], wantEnabled)
		}
	}
	if got["sat"] {
		t.Fatal("PickupDays.Normalize() kept unknown day sat")
	}
}

func TestPickupDaysFromLegacyStatus(t *testing.T) {
	t.Parallel()

	alone := PickupDaysFromLegacyStatus(PickupStatusGoesAlone)
	if alone.HasAny() {
		t.Fatalf("PickupDaysFromLegacyStatus(%q) = %#v, want no enabled days", PickupStatusGoesAlone, alone)
	}

	unset := PickupDaysFromLegacyStatus("")
	if unset.HasAny() {
		t.Fatalf("PickupDaysFromLegacyStatus(\"\") = %#v, want no enabled days", unset)
	}

	pickedUp := PickupDaysFromLegacyStatus(PickupStatusPickedUp)
	for _, day := range PickupDayOrder {
		if !pickedUp[day] {
			t.Fatalf("PickupDaysFromLegacyStatus(%q)[%q] = false, want true", PickupStatusPickedUp, day)
		}
	}
	if len(pickedUp) != len(PickupDayOrder) {
		t.Fatalf("PickupDaysFromLegacyStatus(%q) length = %d, want %d", PickupStatusPickedUp, len(pickedUp), len(PickupDayOrder))
	}
}

func TestPickupDaysLegacyPickupStatus(t *testing.T) {
	t.Parallel()

	if got := (PickupDays{}).LegacyPickupStatus(); got != PickupStatusGoesAlone {
		t.Fatalf("empty PickupDays.LegacyPickupStatus() = %q, want %q", got, PickupStatusGoesAlone)
	}
	if got := (PickupDays{PickupDayMonday: true}).LegacyPickupStatus(); got != PickupStatusPickedUp {
		t.Fatalf("non-empty PickupDays.LegacyPickupStatus() = %q, want %q", got, PickupStatusPickedUp)
	}
}
