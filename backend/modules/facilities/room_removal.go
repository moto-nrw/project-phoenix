package facilities

import (
	"errors"
	"fmt"
)

var (
	// ErrRoomStillPlanned rejects removing a room that activities, upcoming
	// blocks, education groups or devices still use (#6845). Removing it
	// would leave them without a room, so the school moves them first.
	ErrRoomStillPlanned = errors.New("room is still planned or assigned")
	// ErrRoomArchived rejects changing an archived room. Archived rooms only
	// keep the history that names them.
	ErrRoomArchived = errors.New("room is archived")
)

// RoomRemovalOutcome is what removing a room does or did.
type RoomRemovalOutcome string

const (
	// RoomRemovalDelete deletes a room nothing references.
	RoomRemovalDelete RoomRemovalOutcome = "delete"
	// RoomRemovalArchive keeps a room that past blocks or sessions name, but
	// takes it out of every list and selection.
	RoomRemovalArchive RoomRemovalOutcome = "archive"
	// RoomRemovalBlocked means the room is still in use and stays.
	RoomRemovalBlocked RoomRemovalOutcome = "blocked"
)

// RoomReferences counts what other owners keep pointing at a room. The live
// counts block a removal; the history counts turn it into an archive.
type RoomReferences struct {
	ActivityGroups  int
	UpcomingBlocks  int
	EducationGroups int
	Devices         int

	PastBlocks             int
	ArchivedActivityGroups int
	HasSessionHistory      bool
}

// Live reports whether anything still plans or uses the room.
func (r RoomReferences) Live() bool {
	return r.ActivityGroups > 0 || r.UpcomingBlocks > 0 || r.EducationGroups > 0 || r.Devices > 0
}

// History reports whether past records name the room.
func (r RoomReferences) History() bool {
	return r.PastBlocks > 0 || r.ArchivedActivityGroups > 0 || r.HasSessionHistory
}

// RoomRemovalImpact is the one answer to "what happens if this room is
// removed": the running session, a care offering that needs the room, and
// every reference other owners hold (#6845).
type RoomRemovalImpact struct {
	Outcome       RoomRemovalOutcome
	ActiveSession bool
	CareOffering  bool
	References    RoomReferences
}

// NewRoomRemovalImpact derives the outcome from the facts.
func NewRoomRemovalImpact(activeSession, careOffering bool, references RoomReferences) RoomRemovalImpact {
	impact := RoomRemovalImpact{ActiveSession: activeSession, CareOffering: careOffering, References: references}
	switch {
	case activeSession || careOffering || references.Live():
		impact.Outcome = RoomRemovalBlocked
	case references.History():
		impact.Outcome = RoomRemovalArchive
	default:
		impact.Outcome = RoomRemovalDelete
	}
	return impact
}

// RoomStillPlannedError carries the references that block a room removal.
type RoomStillPlannedError struct{ References RoomReferences }

func (e *RoomStillPlannedError) Error() string {
	r := e.References
	return fmt.Sprintf("room is still used by %d activities, %d upcoming blocks, %d education groups and %d devices",
		r.ActivityGroups, r.UpcomingBlocks, r.EducationGroups, r.Devices)
}

func (e *RoomStillPlannedError) Unwrap() error { return ErrRoomStillPlanned }

// RoomRemoval is the result of a room removal.
type RoomRemoval struct {
	Outcome RoomRemovalOutcome
}
