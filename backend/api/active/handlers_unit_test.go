package active

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
)

// =============================================================================
// CONVERTER TESTS - Testing model-to-response conversion functions
// =============================================================================

func TestNewActiveGroupResponse_BasicFields(t *testing.T) {
	now := time.Now()
	endTime := now.Add(time.Hour)

	group := &active.Group{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		GroupID:   100,
		RoomID:    200,
		StartTime: now,
		EndTime:   &endTime,
	}

	response := newActiveGroupResponse(group)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(100), response.GroupID)
	assert.Equal(t, int64(200), response.RoomID)
	assert.Equal(t, now, response.StartTime)
	assert.Equal(t, &endTime, response.EndTime)
	assert.False(t, response.IsActive) // Has end time so not active
	assert.Equal(t, 0, response.VisitCount)
	assert.Equal(t, 0, response.SupervisorCount)
	assert.Nil(t, response.Room)
}

func TestNewActiveGroupResponse_WithVisits(t *testing.T) {
	now := time.Now()

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   100,
		RoomID:    200,
		StartTime: now,
		EndTime:   nil, // Active group
		Visits: []*active.Visit{
			{Model: base.Model{ID: 1}},
			{Model: base.Model{ID: 2}},
			{Model: base.Model{ID: 3}},
		},
	}

	response := newActiveGroupResponse(group)

	assert.True(t, response.IsActive)
	assert.Equal(t, 3, response.VisitCount)
}

func TestNewActiveGroupResponse_WithActiveSupervisors(t *testing.T) {
	now := time.Now()

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   100,
		RoomID:    200,
		StartTime: now,
		EndTime:   nil,
		Supervisors: []*active.GroupSupervisor{
			{Model: base.Model{ID: 1}, StaffID: 10, Role: "Teacher", StartDate: now, EndDate: nil},   // Active
			{Model: base.Model{ID: 2}, StaffID: 20, Role: "Helper", StartDate: now, EndDate: &now},   // Inactive (has end date)
			{Model: base.Model{ID: 3}, StaffID: 30, Role: "Supervisor", StartDate: now, EndDate: nil}, // Active
		},
	}

	response := newActiveGroupResponse(group)

	assert.Equal(t, 2, response.SupervisorCount) // Only 2 active supervisors
	assert.Len(t, response.Supervisors, 2)
	assert.Equal(t, int64(10), response.Supervisors[0].StaffID)
	assert.Equal(t, "Teacher", response.Supervisors[0].Role)
	assert.Equal(t, int64(30), response.Supervisors[1].StaffID)
}

func TestNewActiveGroupResponse_WithRoom(t *testing.T) {
	now := time.Now()

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   100,
		RoomID:    200,
		StartTime: now,
		Room: &facilities.Room{
			Model: base.Model{ID: 200},
			Name:  "Test Room",
		},
	}

	response := newActiveGroupResponse(group)

	assert.NotNil(t, response.Room)
	assert.Equal(t, int64(200), response.Room.ID)
	assert.Equal(t, "Test Room", response.Room.Name)
}

func TestNewVisitResponse_BasicFields(t *testing.T) {
	now := time.Now()
	exitTime := now.Add(time.Hour)

	visit := &active.Visit{
		Model:         base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		StudentID:     100,
		ActiveGroupID: 200,
		EntryTime:     now,
		ExitTime:      &exitTime,
	}

	response := newVisitResponse(visit)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(100), response.StudentID)
	assert.Equal(t, int64(200), response.ActiveGroupID)
	assert.Equal(t, now, response.CheckInTime)
	assert.Equal(t, &exitTime, response.CheckOutTime)
	assert.False(t, response.IsActive) // Has exit time
	assert.Empty(t, response.StudentName)
	assert.Empty(t, response.ActiveGroupName)
}

func TestNewVisitResponse_ActiveVisit(t *testing.T) {
	now := time.Now()

	visit := &active.Visit{
		Model:         base.Model{ID: 1},
		StudentID:     100,
		ActiveGroupID: 200,
		EntryTime:     now,
		ExitTime:      nil, // Active visit
	}

	response := newVisitResponse(visit)

	assert.True(t, response.IsActive)
	assert.Nil(t, response.CheckOutTime)
}

func TestNewVisitResponse_WithStudent(t *testing.T) {
	now := time.Now()

	visit := &active.Visit{
		Model:         base.Model{ID: 1},
		StudentID:     100,
		ActiveGroupID: 200,
		EntryTime:     now,
		Student: &users.Student{
			Model: base.Model{ID: 100},
			Person: &users.Person{
				Model:     base.Model{ID: 50},
				FirstName: "John",
				LastName:  "Doe",
			},
		},
	}

	response := newVisitResponse(visit)

	assert.Equal(t, "John Doe", response.StudentName)
}

func TestNewVisitResponse_WithActiveGroup(t *testing.T) {
	now := time.Now()

	visit := &active.Visit{
		Model:         base.Model{ID: 1},
		StudentID:     100,
		ActiveGroupID: 200,
		EntryTime:     now,
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 200},
			GroupID: 300,
		},
	}

	response := newVisitResponse(visit)

	assert.Equal(t, "Group #300", response.ActiveGroupName)
}

func TestNewSupervisorResponse_BasicFields(t *testing.T) {
	now := time.Now()
	endDate := now.Add(-time.Hour) // End date in the past = inactive

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		StaffID:   100,
		GroupID:   200,
		StartDate: now.Add(-2 * time.Hour),
		EndDate:   &endDate,
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(100), response.StaffID)
	assert.Equal(t, int64(200), response.ActiveGroupID)
	assert.NotNil(t, response.EndTime)
	assert.False(t, response.IsActive) // End date in the past = inactive
}

func TestNewSupervisorResponse_ActiveSupervisor(t *testing.T) {
	now := time.Now()

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: now,
		EndDate:   nil, // Active
	}

	response := newSupervisorResponse(supervisor)

	assert.True(t, response.IsActive)
}

func TestNewSupervisorResponse_WithStaff(t *testing.T) {
	now := time.Now()

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: now,
		Staff: &users.Staff{
			Model: base.Model{ID: 100},
			Person: &users.Person{
				Model:     base.Model{ID: 50},
				FirstName: "Jane",
				LastName:  "Smith",
			},
		},
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, "Jane Smith", response.StaffName)
}

func TestNewSupervisorResponse_WithActiveGroup(t *testing.T) {
	now := time.Now()

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: now,
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 200},
			GroupID: 300,
		},
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, "Group #300", response.ActiveGroupName)
}

func TestNewCombinedGroupResponse_BasicFields(t *testing.T) {
	now := time.Now()
	endTime := now.Add(time.Hour)

	group := &active.CombinedGroup{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		StartTime: now,
		EndTime:   &endTime,
	}

	response := newCombinedGroupResponse(group)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Combined Group #1", response.Name)
	assert.Empty(t, response.Description)
	assert.Equal(t, int64(0), response.RoomID)
	assert.Equal(t, now, response.StartTime)
	assert.Equal(t, &endTime, response.EndTime)
	assert.False(t, response.IsActive) // Has end time
	assert.Equal(t, 0, response.GroupCount)
}

func TestNewCombinedGroupResponse_ActiveWithGroups(t *testing.T) {
	now := time.Now()

	group := &active.CombinedGroup{
		Model:     base.Model{ID: 5},
		StartTime: now,
		EndTime:   nil, // Active
		ActiveGroups: []*active.Group{
			{Model: base.Model{ID: 1}},
			{Model: base.Model{ID: 2}},
		},
	}

	response := newCombinedGroupResponse(group)

	assert.True(t, response.IsActive)
	assert.Equal(t, 2, response.GroupCount)
	assert.Equal(t, "Combined Group #5", response.Name)
}

func TestNewGroupMappingResponse_BasicFields(t *testing.T) {
	mapping := &active.GroupMapping{
		Model:                 base.Model{ID: 1},
		ActiveGroupID:         100,
		ActiveCombinedGroupID: 200,
	}

	response := newGroupMappingResponse(mapping)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, int64(100), response.ActiveGroupID)
	assert.Equal(t, int64(200), response.CombinedGroupID)
	assert.Empty(t, response.GroupName)
	assert.Empty(t, response.CombinedName)
}

func TestNewGroupMappingResponse_WithRelations(t *testing.T) {
	now := time.Now()

	mapping := &active.GroupMapping{
		Model:                 base.Model{ID: 1},
		ActiveGroupID:         100,
		ActiveCombinedGroupID: 200,
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 100},
			GroupID: 50,
		},
		CombinedGroup: &active.CombinedGroup{
			Model:     base.Model{ID: 200},
			StartTime: now,
		},
	}

	response := newGroupMappingResponse(mapping)

	assert.Equal(t, "Group #50", response.GroupName)
	assert.Equal(t, "Combined Group #200", response.CombinedName)
}

// =============================================================================
// REQUEST BINDING TESTS - Testing request validation
// =============================================================================

func TestActiveGroupRequest_Bind_Valid(t *testing.T) {
	req := &ActiveGroupRequest{
		GroupID:   1,
		RoomID:    2,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestActiveGroupRequest_Bind_MissingGroupID(t *testing.T) {
	req := &ActiveGroupRequest{
		GroupID:   0,
		RoomID:    2,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "group ID is required")
}

func TestActiveGroupRequest_Bind_MissingRoomID(t *testing.T) {
	req := &ActiveGroupRequest{
		GroupID:   1,
		RoomID:    0,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "room ID is required")
}

func TestActiveGroupRequest_Bind_MissingStartTime(t *testing.T) {
	req := &ActiveGroupRequest{
		GroupID: 1,
		RoomID:  2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestVisitRequest_Bind_Valid(t *testing.T) {
	req := &VisitRequest{
		StudentID:     1,
		ActiveGroupID: 2,
		CheckInTime:   time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestVisitRequest_Bind_MissingStudentID(t *testing.T) {
	req := &VisitRequest{
		StudentID:     0,
		ActiveGroupID: 2,
		CheckInTime:   time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "student ID is required")
}

func TestVisitRequest_Bind_MissingActiveGroupID(t *testing.T) {
	req := &VisitRequest{
		StudentID:     1,
		ActiveGroupID: 0,
		CheckInTime:   time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active group ID is required")
}

func TestVisitRequest_Bind_MissingCheckInTime(t *testing.T) {
	req := &VisitRequest{
		StudentID:     1,
		ActiveGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check-in time is required")
}

func TestSupervisorRequest_Bind_Valid(t *testing.T) {
	req := &SupervisorRequest{
		StaffID:       1,
		ActiveGroupID: 2,
		StartTime:     time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestSupervisorRequest_Bind_MissingStaffID(t *testing.T) {
	req := &SupervisorRequest{
		StaffID:       0,
		ActiveGroupID: 2,
		StartTime:     time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "staff ID is required")
}

func TestSupervisorRequest_Bind_MissingActiveGroupID(t *testing.T) {
	req := &SupervisorRequest{
		StaffID:       1,
		ActiveGroupID: 0,
		StartTime:     time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active group ID is required")
}

func TestSupervisorRequest_Bind_MissingStartTime(t *testing.T) {
	req := &SupervisorRequest{
		StaffID:       1,
		ActiveGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestCombinedGroupRequest_Bind_Valid(t *testing.T) {
	req := &CombinedGroupRequest{
		Name:      "Test Combined",
		RoomID:    1,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestCombinedGroupRequest_Bind_MissingName(t *testing.T) {
	req := &CombinedGroupRequest{
		Name:      "",
		RoomID:    1,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestCombinedGroupRequest_Bind_MissingRoomID(t *testing.T) {
	req := &CombinedGroupRequest{
		Name:      "Test",
		RoomID:    0,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "room ID is required")
}

func TestCombinedGroupRequest_Bind_MissingStartTime(t *testing.T) {
	req := &CombinedGroupRequest{
		Name:   "Test",
		RoomID: 1,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestGroupMappingRequest_Bind_Valid(t *testing.T) {
	req := &GroupMappingRequest{
		ActiveGroupID:   1,
		CombinedGroupID: 2,
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestGroupMappingRequest_Bind_MissingActiveGroupID(t *testing.T) {
	req := &GroupMappingRequest{
		ActiveGroupID:   0,
		CombinedGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active group ID is required")
}

func TestGroupMappingRequest_Bind_MissingCombinedGroupID(t *testing.T) {
	req := &GroupMappingRequest{
		ActiveGroupID:   1,
		CombinedGroupID: 0,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "combined group ID is required")
}

// =============================================================================
// EXPORTED HANDLER TESTS - Testing exported handler wrappers
// =============================================================================

func TestExportedHandlers_NotNil(t *testing.T) {
	rs := &Resource{}

	// Active Group Handlers
	assert.NotNil(t, rs.ListActiveGroupsHandler())
	assert.NotNil(t, rs.GetActiveGroupHandler())
	assert.NotNil(t, rs.CreateActiveGroupHandler())
	assert.NotNil(t, rs.UpdateActiveGroupHandler())
	assert.NotNil(t, rs.DeleteActiveGroupHandler())
	assert.NotNil(t, rs.EndActiveGroupHandler())

	// Visit Handlers
	assert.NotNil(t, rs.ListVisitsHandler())
	assert.NotNil(t, rs.GetVisitHandler())
	assert.NotNil(t, rs.CreateVisitHandler())
	assert.NotNil(t, rs.UpdateVisitHandler())
	assert.NotNil(t, rs.DeleteVisitHandler())
	assert.NotNil(t, rs.EndVisitHandler())
	assert.NotNil(t, rs.GetStudentVisitsHandler())
	assert.NotNil(t, rs.GetStudentCurrentVisitHandler())

	// Supervisor Handlers
	assert.NotNil(t, rs.ListSupervisorsHandler())
	assert.NotNil(t, rs.GetSupervisorHandler())
	assert.NotNil(t, rs.CreateSupervisorHandler())
	assert.NotNil(t, rs.UpdateSupervisorHandler())
	assert.NotNil(t, rs.DeleteSupervisorHandler())
	assert.NotNil(t, rs.EndSupervisionHandler())
	assert.NotNil(t, rs.GetStaffSupervisionsHandler())
	assert.NotNil(t, rs.GetStaffActiveSupervisionsHandler())

	// Analytics Handlers
	assert.NotNil(t, rs.GetCountsHandler())
	assert.NotNil(t, rs.GetDashboardAnalyticsHandler())
	assert.NotNil(t, rs.GetRoomUtilizationHandler())
	assert.NotNil(t, rs.GetStudentAttendanceHandler())

	// Combined Group Handlers
	assert.NotNil(t, rs.ListCombinedGroupsHandler())
	assert.NotNil(t, rs.GetCombinedGroupHandler())
	assert.NotNil(t, rs.CreateCombinedGroupHandler())
	assert.NotNil(t, rs.UpdateCombinedGroupHandler())
	assert.NotNil(t, rs.DeleteCombinedGroupHandler())
	assert.NotNil(t, rs.EndCombinedGroupHandler())
	assert.NotNil(t, rs.GetActiveCombinedGroupsHandler())

	// Group by filters Handlers
	assert.NotNil(t, rs.GetActiveGroupsByRoomHandler())
	assert.NotNil(t, rs.GetActiveGroupsByGroupHandler())
	assert.NotNil(t, rs.GetActiveGroupVisitsHandler())
	assert.NotNil(t, rs.GetActiveGroupVisitsWithDisplayHandler())
	assert.NotNil(t, rs.GetActiveGroupSupervisorsHandler())
	assert.NotNil(t, rs.GetVisitsByGroupHandler())
	assert.NotNil(t, rs.GetSupervisorsByGroupHandler())

	// Group Mapping Handlers
	assert.NotNil(t, rs.GetGroupMappingsHandler())
	assert.NotNil(t, rs.GetCombinedGroupMappingsHandler())
	assert.NotNil(t, rs.AddGroupToCombinationHandler())
	assert.NotNil(t, rs.RemoveGroupFromCombinationHandler())

	// Unclaimed Group Handlers
	assert.NotNil(t, rs.ListUnclaimedGroupsHandler())
	assert.NotNil(t, rs.ClaimGroupHandler())

	// Checkout Handler
	assert.NotNil(t, rs.CheckoutStudentHandler())

	// Checkin Handler
	assert.NotNil(t, rs.CheckinStudentHandler())
}

// =============================================================================
// ERROR RENDERER TESTS - Testing error response functions
// =============================================================================

func TestErrorRenderer_ActiveGroupNotFound(t *testing.T) {
	err := activeSvc.ErrActiveGroupNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Active Group Not Found", errResp.StatusText)
}

func TestErrorRenderer_VisitNotFound(t *testing.T) {
	err := activeSvc.ErrVisitNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Visit Not Found", errResp.StatusText)
}

func TestErrorRenderer_GroupSupervisorNotFound(t *testing.T) {
	err := activeSvc.ErrGroupSupervisorNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Supervisor Not Found", errResp.StatusText)
}

func TestErrorRenderer_CombinedGroupNotFound(t *testing.T) {
	err := activeSvc.ErrCombinedGroupNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Combined Group Not Found", errResp.StatusText)
}

func TestErrorRenderer_GroupMappingNotFound(t *testing.T) {
	err := activeSvc.ErrGroupMappingNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Mapping Not Found", errResp.StatusText)
}

func TestErrorRenderer_InvalidData(t *testing.T) {
	err := activeSvc.ErrInvalidData
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Data", errResp.StatusText)
}

func TestErrorRenderer_ActiveGroupAlreadyEnded(t *testing.T) {
	err := activeSvc.ErrActiveGroupAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Active Group Already Ended", errResp.StatusText)
}

func TestErrorRenderer_VisitAlreadyEnded(t *testing.T) {
	err := activeSvc.ErrVisitAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Visit Already Ended", errResp.StatusText)
}

func TestErrorRenderer_SupervisionAlreadyEnded(t *testing.T) {
	err := activeSvc.ErrSupervisionAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Supervision Already Ended", errResp.StatusText)
}

func TestErrorRenderer_CombinedGroupAlreadyEnded(t *testing.T) {
	err := activeSvc.ErrCombinedGroupAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Combined Group Already Ended", errResp.StatusText)
}

func TestErrorRenderer_GroupAlreadyInCombination(t *testing.T) {
	err := activeSvc.ErrGroupAlreadyInCombination
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Already In Combination", errResp.StatusText)
}

func TestErrorRenderer_StudentAlreadyInGroup(t *testing.T) {
	err := activeSvc.ErrStudentAlreadyInGroup
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Student Already In Group", errResp.StatusText)
}

func TestErrorRenderer_StudentAlreadyActive(t *testing.T) {
	err := activeSvc.ErrStudentAlreadyActive
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Student Already Has Active Visit", errResp.StatusText)
}

func TestErrorRenderer_StaffAlreadySupervising(t *testing.T) {
	err := activeSvc.ErrStaffAlreadySupervising
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Staff Already Supervising This Group", errResp.StatusText)
}

func TestErrorRenderer_CannotDeleteActiveGroup(t *testing.T) {
	err := activeSvc.ErrCannotDeleteActiveGroup
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Cannot Delete Active Group With Active Visits", errResp.StatusText)
}

func TestErrorRenderer_InvalidTimeRange(t *testing.T) {
	err := activeSvc.ErrInvalidTimeRange
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Time Range", errResp.StatusText)
}

func TestErrorRenderer_RoomConflict(t *testing.T) {
	err := activeSvc.ErrRoomConflict
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 409, errResp.HTTPStatusCode)
	assert.Equal(t, "Room Conflict", errResp.StatusText)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	err := errors.New("unknown error")
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 500, errResp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", errResp.StatusText)
	assert.Equal(t, "unknown error", errResp.ErrorText)
}

func TestErrorForbidden(t *testing.T) {
	err := errors.New("access denied")
	renderer := ErrorForbidden(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 403, errResp.HTTPStatusCode)
	assert.Equal(t, "Forbidden", errResp.StatusText)
	assert.Equal(t, "access denied", errResp.ErrorText)
}

func TestErrorUnauthorized(t *testing.T) {
	err := errors.New("not authenticated")
	renderer := ErrorUnauthorized(err)

	errResp, ok := renderer.(*ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 401, errResp.HTTPStatusCode)
	assert.Equal(t, "Unauthorized", errResp.StatusText)
	assert.Equal(t, "not authenticated", errResp.ErrorText)
}

func TestErrResponse_Render(t *testing.T) {
	errResp := &ErrResponse{
		HTTPStatusCode: 400,
		StatusText:     "Bad Request",
		ErrorText:      "test error",
	}

	req, _ := http.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	err := errResp.Render(w, req)

	assert.NoError(t, err)
}
