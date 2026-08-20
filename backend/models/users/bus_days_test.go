package users

import (
	"strings"
	"testing"
)

func TestBusDaysValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		days    BusDays
		wantErr string
	}{
		{
			name: "accepts empty days",
			days: BusDays{},
		},
		{
			name: "accepts weekdays",
			days: BusDays{
				BusDayMonday:    true,
				BusDayWednesday: false,
				BusDayFriday:    true,
			},
		},
		{
			name: "rejects unknown day",
			days: BusDays{
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
					t.Fatalf("BusDays.Validate() unexpected error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("BusDays.Validate() expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("BusDays.Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestBusDaysHasAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		days BusDays
		want bool
	}{
		{name: "empty days", days: BusDays{}, want: false},
		{name: "all false weekdays", days: BusDays{BusDayMonday: false, BusDayFriday: false}, want: false},
		{name: "known true weekday", days: BusDays{BusDayTuesday: true}, want: true},
		{name: "unknown true day is ignored", days: BusDays{"sat": true}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.days.HasAny(); got != tt.want {
				t.Fatalf("BusDays.HasAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBusDaysNormalize(t *testing.T) {
	t.Parallel()

	days := BusDays{
		BusDayMonday:    true,
		BusDayTuesday:   false,
		BusDayWednesday: true,
		"sat":           true,
	}

	got := days.Normalize()
	want := BusDays{
		BusDayMonday:    true,
		BusDayWednesday: true,
	}

	if len(got) != len(want) {
		t.Fatalf("BusDays.Normalize() length = %d, want %d: %#v", len(got), len(want), got)
	}
	for day, wantEnabled := range want {
		if got[day] != wantEnabled {
			t.Fatalf("BusDays.Normalize()[%q] = %v, want %v", day, got[day], wantEnabled)
		}
	}
	if got["sat"] {
		t.Fatal("BusDays.Normalize() kept unknown day sat")
	}
}

func TestBusDaysFromLegacyFlag(t *testing.T) {
	t.Parallel()

	disabled := BusDaysFromLegacyFlag(false)
	if disabled.HasAny() {
		t.Fatalf("BusDaysFromLegacyFlag(false) = %#v, want no enabled days", disabled)
	}

	enabled := BusDaysFromLegacyFlag(true)
	for _, day := range BusDayOrder {
		if !enabled[day] {
			t.Fatalf("BusDaysFromLegacyFlag(true)[%q] = false, want true", day)
		}
	}
	if len(enabled) != len(BusDayOrder) {
		t.Fatalf("BusDaysFromLegacyFlag(true) length = %d, want %d", len(enabled), len(BusDayOrder))
	}
}
