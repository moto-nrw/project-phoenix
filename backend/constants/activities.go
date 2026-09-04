package constants

// Activity names from seed data
// These constants ensure consistency across the codebase when referencing
// specific activities that have special meaning in the system.
const (
	// SchulhofActivityName is the name of the permanent Schulhof (playground) activity.
	// This activity is auto-created on first use if not found.
	SchulhofActivityName = "Schulhof Freispiel"

	// SchulhofCategoryName is the name of the Schulhof activity category.
	// Auto-created alongside the Schulhof activity if not found.
	SchulhofCategoryName = "Schulhof"

	// SchulhofCategoryDescription is the description for the Schulhof activity category.
	SchulhofCategoryDescription = "Outdoor playground activities"

	// SchulhofColor is the color of the auto-provisioned Schulhof activity
	// CATEGORY (green for outdoor/nature).
	//
	// Deliberately NOT stamped on the Schulhof room since #2405: the room's
	// color is admin-configurable, and leaving it unset is what makes the
	// orange Schulhof default apply.
	SchulhofColor = "#7ED321"

	// SchulhofRoomName is the name of the Schulhof room/outdoor area.
	// Auto-created alongside the Schulhof activity if not found.
	SchulhofRoomName = "Schulhof"

	// SchulhofRoomCapacity is the default capacity for the Schulhof room.
	SchulhofRoomCapacity = 300

	// SchulhofMaxParticipants is the default max participants for the Schulhof activity.
	SchulhofMaxParticipants = 300

	// WCActivityName is the name of the permanent WC (toilet) activity.
	// This activity is auto-created on first use if not found.
	WCActivityName = "WC"

	// WCCategoryName is the name of the WC activity category.
	// Auto-created alongside the WC activity if not found.
	WCCategoryName = "WC"

	// WCCategoryDescription is the description for the WC activity category.
	WCCategoryDescription = "Bathroom/toilet break"

	// WCColor is the default color for WC elements (blue).
	WCColor = "#60A5FA"

	// WCRoomName is the name of the WC room.
	// Auto-created alongside the WC activity if not found.
	WCRoomName = "WC"

	// WCRoomAliasName is an accepted room-only alias for the WC room.
	// The canonical auto-created room name remains WCRoomName.
	WCRoomAliasName = "Toilette"

	// WCRoomCapacity is the default capacity for the WC room.
	WCRoomCapacity = 20

	// WCMaxParticipants is the default max participants for the WC activity.
	WCMaxParticipants = 20
)

// IsWCRoomName returns true if the given room name is one of the accepted
// canonical toilet-room aliases ("WC" or "Toilette"). Matching is exact-case
// to match the existing system-room contract, which has always compared
// SchulhofRoomName and WCRoomName by `==`. See issue #1184.
func IsWCRoomName(name string) bool {
	return name == WCRoomName || name == WCRoomAliasName
}

// IsSystemActivityName returns true if the given activity name is a system activity
// that must not be deleted or renamed.
func IsSystemActivityName(name string) bool {
	return name == SchulhofActivityName || name == WCActivityName
}
