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

	// SchulhofColor is the default color for Schulhof elements (green for outdoor/nature).
	SchulhofColor = "#7ED321"

	// SchulhofRoomName is the name of the Schulhof room/outdoor area.
	// Auto-created alongside the Schulhof activity if not found.
	SchulhofRoomName = "Schulhof"

	// SchulhofRoomCapacity is the default capacity for the Schulhof room.
	SchulhofRoomCapacity = 100

	// SchulhofMaxParticipants is the default max participants for the Schulhof activity.
	SchulhofMaxParticipants = 100
)
