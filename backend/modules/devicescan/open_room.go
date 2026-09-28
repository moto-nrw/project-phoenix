package devicescan

import (
	"context"
	"errors"
)

// Wire strings of the open-room destination booking (#3067). PyrePortal maps
// the codes; the guard test in api/iot pins the messages.
const (
	MessageOpenRoomIDRequired  = "room_id is required"
	MessageOpenRoomNotFound    = "room not found"
	CodeOpenRoomNotFound       = "room_not_found"
	MessageOpenRoomNotReleased = "room is not released as an open room"
	CodeOpenRoomNotReleased    = "room_not_released"
	MessageOpenRoomBinaryMode  = "open rooms need detailed presence mode"
	CodeOpenRoomBinaryMode     = "open_room_binary_mode"
	MessageStudentNotPresent   = "student is not checked in"
	CodeStudentNotPresent      = "student_not_present"
	MessageOpenRoomUnavailable = "open room booking is not available"

	// ScanActionOpenRoomStay is the action of a booked independent stay.
	ScanActionOpenRoomStay = "open_room_stay"
)

// Sentinels the open-room mover reports. The composition root translates
// the open-room move workflow's and Student Presence's errors into these, so
// the device-scan workflow classifies them without importing either owner.
var (
	ErrOpenRoomNotFound          = errors.New("open room not found")
	ErrOpenRoomNotReleased       = errors.New("room is not released as an open room")
	ErrOpenRoomStudentNotPresent = errors.New("student is not checked in")
)

// OpenRoomCommand books the child behind a card into a released room from
// the device the child leaves. RoomID is the destination; the destination
// needs no device, no second scan and no supervision.
type OpenRoomCommand struct {
	RFIDTag string
	RoomID  int64
}

// OpenRoomResult is the kiosk projection of a booked independent stay.
// Moved is false when the child already stayed in that room: a repeated
// booking records no second stay.
type OpenRoomResult struct {
	StudentID     int64
	StudentName   string
	RoomID        int64
	RoomName      string
	RoomSessionID int64
	Moved         bool
	Action        string
	Message       string
}

// OpenRoomBooking is the kiosk contract for choosing a released destination
// room at the device a child leaves (#3067).
type OpenRoomBooking interface {
	DeviceIdentity
	// BookOpenRoom records an independent stay in the released room: never
	// participation in an activity running there, never supervision. It
	// requires an existing tenant transaction; the HTTP middleware supplies it.
	BookOpenRoom(ctx context.Context, command OpenRoomCommand) (*OpenRoomResult, error)
}

// OpenRoomMove is one child the device-scan workflow asks the open-room move
// to relocate. The device is trusted like the binary attendance toggle: it
// authenticated with its key and the school PIN, and the child's card was
// scanned at it.
type OpenRoomMove struct {
	StudentID int64
	RoomID    int64
}

// OpenRoomMoveOutcome reports where the child now stays.
type OpenRoomMoveOutcome struct {
	RoomSessionID int64
	Moved         bool
}

// OpenRoomMover is the consumer-owned port the composition root binds to the
// open-room move workflow, the same capability the phone move uses.
type OpenRoomMover interface {
	MoveToOpenRoom(ctx context.Context, move OpenRoomMove) (OpenRoomMoveOutcome, error)
}
