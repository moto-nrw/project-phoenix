package active

import (
	"errors"
	"fmt"
)

// ErrRoomCapacityExceeded identifies a presence admission beyond room capacity.
var ErrRoomCapacityExceeded = errors.New("room capacity exceeded")

type RoomCapacityError struct {
	RoomID           int64
	RoomName         string
	CurrentOccupancy int
	MaxCapacity      int
}

func (e *RoomCapacityError) Error() string {
	return fmt.Sprintf("room capacity exceeded: %s (%d/%d)", e.RoomName, e.CurrentOccupancy, e.MaxCapacity)
}

func (e *RoomCapacityError) Unwrap() error {
	return ErrRoomCapacityExceeded
}
