// Package openroommove is the application workflow that records an
// Ortswechsel into a released room ("offener Raum", #3062). Staff on a phone
// (#3066) and, later, a child at an NFC device (#3067) invoke the same
// command. It coordinates Facilities (the release), Timetable & Activities
// (the room's system activity) and Student Presence (the room session and the
// move) in one UnitOfWork.
//
// The move records an independent room stay ("angebotsunabhängiger
// Raumaufenthalt"): a visit in the room's own session, never participation in
// an activity that happens to run in the same room, and never supervision for
// the person recording it.
package openroommove

import (
	"context"
	"errors"
)

var (
	// ErrInvalidMove reports a request without children or without a room.
	ErrInvalidMove = errors.New("student_ids and target_room_id are required")
	// ErrRoomNotFound reports that the tenant has no such room.
	ErrRoomNotFound = errors.New("room not found")
	// ErrRoomNotReleased reports a room the administration has not released,
	// or no longer releases. Seeing a room never authorizes a move into it.
	ErrRoomNotReleased = errors.New("room is not released as an open room")
)

// Actor is the authenticated caller whose rights the move checks. The inbound
// adapter authenticates it; the workflow never derives rights from room
// visibility.
type Actor struct {
	StaffID int64
	// BypassResourceChecks is set only for administrators and for the kiosk
	// the child leaves (#3067): the device authenticated with its key and the
	// school PIN and the child's card was scanned at it, the same trust the
	// binary attendance toggle relies on.
	BypassResourceChecks bool
	// SchoolWideAttendanceEligible marks a verified OGS staff actor holding
	// the move permission; the tenant settings still decide whether the
	// school-wide attendance scope applies.
	SchoolWideAttendanceEligible bool
}

// Move asks to record that the children now use the released room.
type Move struct {
	StudentIDs []int64
	RoomID     int64
	Actor      Actor
}

// Skipped names a child the move left where it was, and why.
type Skipped struct {
	StudentID int64
	Reason    string
}

// Result describes what one move changed.
type Result struct {
	RoomID int64
	// RoomSessionID is the room session the children now stay in.
	RoomSessionID int64
	Moved         []int64
	Unchanged     []int64
	Skipped       []Skipped
}

// Command records a move into a released room. It joins the caller's tenant
// transaction when one is open and otherwise runs its own; every write of the
// move belongs to that single unit. When the caller owns the transaction, a
// returned error means the caller must roll back.
type Command interface {
	MoveToOpenRoom(ctx context.Context, move Move) (Result, error)
}
