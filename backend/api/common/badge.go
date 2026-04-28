package common

import "github.com/moto-nrw/project-phoenix/constants"

// BadgeStammraumGreen is the badge color reserved for students who are
// currently in the room assigned to their education group (Stammraum).
//
// Cross-stack parity (must stay in lock-step):
//   - frontend/src/lib/location-helper.ts → LOCATION_COLORS.GROUP_ROOM
//   - TestBadgeStammraumGreenLiteral pins this Go constant
//   - location-helper.test.ts pins the matching TS constant
//
// If you change the hex here, change it on the frontend in the same commit.
const BadgeStammraumGreen = "#83CD2D"

// ResolveRoomBadgeColor returns the badge color for a student's currently
// visited room. Stammraum (own group's room) wins with reserved green.
// System rooms (Schulhof, WC) are not part of the customisable-colour
// feature — they return nil so the frontend keeps its existing status-based
// rendering. All other rooms return their stored custom colour (or nil).
func ResolveRoomBadgeColor(roomName string, roomColor *string, isStammraum bool) *string {
	if isStammraum {
		green := BadgeStammraumGreen
		return &green
	}

	if constants.IsSystemRoomName(roomName) {
		return nil
	}

	return roomColor
}
