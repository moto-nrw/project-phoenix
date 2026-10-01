package api

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
)

// newDeviceOpenRoomMover binds the device-scan workflow's open-room port to
// the open-room move workflow (#3067), so a destination chosen at a kiosk
// runs the same capability as the phone move. The device-scan composition
// translates the refusals; this binding only converts the workflow's types.
func newDeviceOpenRoomMover(command openroommove.Command) devicescanCompose.OpenRoomMover {
	return devicescanCompose.NewOpenRoomMover(
		func(ctx context.Context, studentID, roomID int64) (devicescanCompose.OpenRoomMoveReport, error) {
			result, err := command.MoveToOpenRoom(ctx, openroommove.Move{
				StudentIDs: []int64{studentID},
				RoomID:     roomID,
				// The kiosk is trusted like the binary attendance toggle: it
				// authenticated with its key and the school PIN, and the
				// child's card was scanned at it.
				Actor: openroommove.Actor{BypassResourceChecks: true},
			})
			if err != nil {
				return devicescanCompose.OpenRoomMoveReport{}, err
			}
			skipped := make([]devicescanCompose.OpenRoomSkip, 0, len(result.Skipped))
			for _, skip := range result.Skipped {
				skipped = append(skipped, devicescanCompose.OpenRoomSkip{StudentID: skip.StudentID, Reason: skip.Reason})
			}
			return devicescanCompose.OpenRoomMoveReport{RoomSessionID: result.RoomSessionID, Moved: result.Moved, Skipped: skipped}, nil
		},
		devicescanCompose.OpenRoomRefusals{RoomNotFound: openroommove.ErrRoomNotFound, RoomNotReleased: openroommove.ErrRoomNotReleased},
	)
}
