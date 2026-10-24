package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// OpenRoomMover is the device-scan workflow's port to the open-room move
// (#3067), re-exported for the composition root.
type OpenRoomMover = devicescan.OpenRoomMover

// OpenRoomMoveReport is what the open-room move reported for one booking.
type OpenRoomMoveReport struct {
	RoomSessionID int64
	Moved         []int64
	Skipped       []OpenRoomSkip
}

// OpenRoomSkip names a child the move left where it was, and why.
type OpenRoomSkip struct {
	StudentID int64
	Reason    string
}

// OpenRoomMoveFunc runs the open-room move for one child with the kiosk's
// device trust.
type OpenRoomMoveFunc func(ctx context.Context, studentID, roomID int64) (OpenRoomMoveReport, error)

// OpenRoomRefusals names the move workflow's own refusals, which the
// device-scan module cannot import.
type OpenRoomRefusals struct {
	RoomNotFound    error
	RoomNotReleased error
}
