package location

// State represents a canonical student location state emitted by the backend.
type State string

const (
	// StatePresentInRoom indicates the student is currently in a specific room.
	StatePresentInRoom State = "PRESENT_IN_ROOM"
	// StateTransit indicates the student is between rooms while remaining checked in.
	StateTransit State = "TRANSIT"
	// StateSchoolyard indicates the student is on the schoolyard.
	StateSchoolyard State = "SCHOOLYARD"
	// StateHome indicates the student is not checked in today (home/off site).
	StateHome State = "HOME"
)

// RoomOwnerType describes who owns the room a student is in.
type RoomOwnerType string

const (
	// RoomOwnerGroup indicates the room belongs to the student's educational group.
	RoomOwnerGroup RoomOwnerType = "GROUP"
	// RoomOwnerActivity indicates the room belongs to an activity or other context.
	RoomOwnerActivity RoomOwnerType = "ACTIVITY"
)

// Room contains metadata about the room associated with a student's location.
type Room struct {
	ID          int64         `json:"id"`
	Name        string        `json:"name"`
	IsGroupRoom bool          `json:"is_group_room"`
	OwnerType   RoomOwnerType `json:"owner_type"`
}

// Status is the structured representation of a student's current location.
type Status struct {
	State State `json:"state"`
	Room  *Room `json:"room,omitempty"`
}

// NewStatus creates a status with the provided state and optional room metadata.
func NewStatus(state State, room *Room) *Status {
	return &Status{
		State: state,
		Room:  room,
	}
}
