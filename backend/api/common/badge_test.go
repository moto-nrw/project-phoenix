package common

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/constants"
)

func TestResolveRoomBadgeColor(t *testing.T) {
	t.Run("returns stammraum green when in own room", func(t *testing.T) {
		roomColor := "#06B6D4"

		resolved := ResolveRoomBadgeColor("Gruppenraum", &roomColor, true)

		if resolved == nil || *resolved != BadgeStammraumGreen {
			t.Fatalf("expected stammraum green %q, got %v", BadgeStammraumGreen, resolved)
		}
	})

	t.Run("returns nil for system rooms (not part of customisable colour feature)", func(t *testing.T) {
		// System rooms keep their existing frontend status-based rendering.
		// Even if the DB has a colour stored on them, the resolver ignores it.
		schulhofColor := "#123456"
		if got := ResolveRoomBadgeColor(constants.SchulhofRoomName, &schulhofColor, false); got != nil {
			t.Errorf("expected nil for Schulhof, got %v", got)
		}

		wcColor := "#654321"
		if got := ResolveRoomBadgeColor(constants.WCRoomName, &wcColor, false); got != nil {
			t.Errorf("expected nil for WC, got %v", got)
		}
	})

	t.Run("falls back to room color for regular rooms", func(t *testing.T) {
		roomColor := "#06B6D4"

		resolved := ResolveRoomBadgeColor("Musikraum", &roomColor, false)

		if resolved == nil || *resolved != roomColor {
			t.Fatalf("expected room color %q, got %v", roomColor, resolved)
		}
	})
}

// TestBadgeStammraumGreenLiteral pins the stammraum green to the exact hex
// shared with the frontend (LOCATION_COLORS.GROUP_ROOM). The matching TS
// assertion lives in frontend/src/lib/location-helper.test.ts. If this test
// fails, the two sides have drifted and badge colors will silently disagree.
func TestBadgeStammraumGreenLiteral(t *testing.T) {
	const expected = "#83CD2D"
	if BadgeStammraumGreen != expected {
		t.Fatalf("BadgeStammraumGreen drift: got %q, want %q — frontend LOCATION_COLORS.GROUP_ROOM must change in lock-step",
			BadgeStammraumGreen, expected)
	}
}
