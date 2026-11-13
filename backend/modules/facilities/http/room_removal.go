package httpadapter

import (
	"errors"
	"net/http"

	"github.com/moto-ogs/project-phoenix/modules/facilities"
)

// RoomRemovalResponse tells the client whether DELETE deleted the room or
// kept it archived for its history (#6845).
type RoomRemovalResponse struct {
	Outcome facilities.RoomRemovalOutcome `json:"outcome"`
}

// RoomReferencesResponse counts what still points at a room.
type RoomReferencesResponse struct {
	ActivityGroups         int  `json:"activity_groups"`
	UpcomingBlocks         int  `json:"upcoming_blocks"`
	EducationGroups        int  `json:"education_groups"`
	Devices                int  `json:"devices"`
	PastBlocks             int  `json:"past_blocks"`
	ArchivedActivityGroups int  `json:"archived_activity_groups"`
	HasSessionHistory      bool `json:"has_session_history"`
}

// RoomRemovalImpactResponse previews DELETE /rooms/{id}.
type RoomRemovalImpactResponse struct {
	Outcome       facilities.RoomRemovalOutcome `json:"outcome"`
	ActiveSession bool                          `json:"active_session"`
	CareOffering  bool                          `json:"care_offering"`
	References    RoomReferencesResponse        `json:"references"`
}

func newRoomReferencesResponse(references facilities.RoomReferences) RoomReferencesResponse {
	return RoomReferencesResponse{
		ActivityGroups: references.ActivityGroups, UpcomingBlocks: references.UpcomingBlocks,
		EducationGroups: references.EducationGroups, Devices: references.Devices,
		PastBlocks: references.PastBlocks, ArchivedActivityGroups: references.ArchivedActivityGroups,
		HasSessionHistory: references.HasSessionHistory,
	}
}

func (rs *Resource) deleteRoom(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	removal, err := rs.rooms.DeleteRoom(r.Context(), id)
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	message := "Room deleted successfully"
	if removal.Outcome == facilities.RoomRemovalArchive {
		message = "Room archived"
	}
	rs.runtime.Success(w, r, http.StatusOK, RoomRemovalResponse{Outcome: removal.Outcome}, message)
}

func (rs *Resource) roomRemovalImpact(w http.ResponseWriter, r *http.Request) {
	id, err := rs.runtime.ParseID(r)
	if err != nil {
		rs.runtime.Failure(w, r, FailureInvalid, errors.New("invalid room ID"), "invalid_request")
		return
	}
	impact, err := rs.rooms.RoomRemovalImpact(r.Context(), id)
	if err != nil {
		rs.failure(w, r, err)
		return
	}
	rs.runtime.Success(w, r, http.StatusOK, RoomRemovalImpactResponse{
		Outcome: impact.Outcome, ActiveSession: impact.ActiveSession, CareOffering: impact.CareOffering,
		References: newRoomReferencesResponse(impact.References),
	}, "Room removal impact retrieved")
}
