package active

import (
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// ===== Conversion Functions =====

func newPresenceLiveGroupResponse(group studentpresence.LiveGroup) ActiveGroupResponse {
	return ActiveGroupResponse{
		ID: group.ID, GroupID: group.ActivityGroupID, RoomID: group.RoomID,
		StartTime: group.StartTime, EndTime: group.EndTime, IsActive: group.IsOpen(),
		CreatedAt: group.CreatedAt, UpdatedAt: group.UpdatedAt,
	}
}

// newSessionRoomResponse projects the room summary a session response embeds.
func newSessionRoomResponse(room studentpresence.SessionRoomSummary) *RoomSimple {
	return &RoomSimple{ID: room.ID, Name: room.Name, Color: room.Color}
}

func newPresenceCombinationResponse(group studentpresence.CombinedGroup) CombinedGroupResponse {
	response := CombinedGroupResponse{
		ID:          group.ID,
		Name:        "Combined Group #" + strconv.FormatInt(group.ID, 10), // Using ID as name since the model doesn't have name
		Description: "",                                                   // Using empty description since the model doesn't have description
		RoomID:      0,                                                    // Using default value since the model doesn't have roomID
		StartTime:   group.StartTime,
		EndTime:     group.EndTime,
		IsActive:    group.EndTime == nil || time.Now().Before(*group.EndTime),
		CreatedAt:   group.CreatedAt,
		UpdatedAt:   group.UpdatedAt,
	}

	return response
}

// newCombinationWithGroupsResponse adds the mapped session count to the
// combination response.
func newCombinationWithGroupsResponse(group studentpresence.CombinedGroup, groups []studentpresence.LiveGroup) CombinedGroupResponse {
	response := newPresenceCombinationResponse(group)
	response.GroupCount = len(groups)
	return response
}

// claimedSupervisionResponse keeps the wire shape of the claimed supervision
// record: the persisted row with its calendar start day.
type claimedSupervisionResponse struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TenantID  int64     `json:"tenant_id"`
	StaffID   int64     `json:"staff_id"`
	GroupID   int64     `json:"group_id"`
	Role      string    `json:"role"`
	StartDate string    `json:"start_date"`
}

func newClaimedSupervisionResponse(row studentpresence.ClaimedSupervision) claimedSupervisionResponse {
	return claimedSupervisionResponse{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		StaffID: row.StaffID, GroupID: row.GroupID, Role: row.Role, StartDate: row.StartDate,
	}
}

// unclaimedSessionResponse keeps the wire shape of an unclaimed session: the
// session row with its room and template projections.
type unclaimedSessionResponse struct {
	ID             int64                      `json:"id"`
	CreatedAt      time.Time                  `json:"created_at"`
	UpdatedAt      time.Time                  `json:"updated_at"`
	TenantID       int64                      `json:"tenant_id"`
	StartTime      time.Time                  `json:"start_time"`
	EndTime        *time.Time                 `json:"end_time,omitempty"`
	LastActivity   time.Time                  `json:"last_activity"`
	TimeoutMinutes int                        `json:"timeout_minutes"`
	GroupID        *int64                     `json:"group_id"`
	DeviceID       *int64                     `json:"device_id,omitempty"`
	RoomID         int64                      `json:"room_id"`
	ActualGroup    *unclaimedActivityResponse `json:"actual_group,omitempty"`
	Room           *unclaimedRoomResponse     `json:"room,omitempty"`
}

type unclaimedActivityResponse struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type unclaimedRoomResponse struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Category *string `json:"category,omitempty"`
	Color    *string `json:"color,omitempty"`
}

func newUnclaimedSessionResponse(row studentpresence.UnclaimedSession) unclaimedSessionResponse {
	response := unclaimedSessionResponse{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		StartTime: row.StartTime, EndTime: row.EndTime, LastActivity: row.LastActivity,
		TimeoutMinutes: row.TimeoutMinutes, GroupID: row.GroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
	}
	if row.Activity != nil {
		response.ActualGroup = &unclaimedActivityResponse{ID: row.Activity.ID, Name: row.Activity.Name}
	}
	if row.Room != nil {
		response.Room = &unclaimedRoomResponse{ID: row.Room.ID, Name: row.Room.Name, Category: row.Room.Category, Color: row.Room.Color}
	}
	return response
}

// crossTenantStudentResponse keeps the wire shape of a hosted child.
type crossTenantStudentResponse struct {
	StudentID  int64  `json:"student_id"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	GroupName  string `json:"group_name"`
	HomeTenant string `json:"home_tenant"`
}

func newCrossTenantStudentResponses(rows []studentpresence.CrossTenantStudent) []crossTenantStudentResponse {
	responses := make([]crossTenantStudentResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, crossTenantStudentResponse{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			GroupName: row.GroupName, HomeTenant: row.HomeTenant,
		})
	}
	return responses
}
