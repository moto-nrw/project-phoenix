package active

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/ptrtest"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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
	t.Parallel()

	now := time.Now()
	endTime := now.Add(time.Hour)

	group := &active.Group{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		GroupID:   ptrtest.Ptr(int64(100)),
		RoomID:    200,
		StartTime: now,
		EndTime:   &endTime,
	}

	response := newActiveGroupResponse(group)

	assert.Equal(t, int64(1), response.ID)
	if assert.NotNil(t, response.GroupID) {
		assert.Equal(t, int64(100), *response.GroupID)
	}
	assert.Equal(t, int64(200), response.RoomID)
	assert.Equal(t, now, response.StartTime)
	assert.Equal(t, &endTime, response.EndTime)
	assert.False(t, response.IsActive) // Has end time so not active
	assert.Equal(t, 0, response.VisitCount)
	assert.Equal(t, 0, response.SupervisorCount)
	assert.Nil(t, response.Room)
}

func TestNewActiveGroupResponse_WithVisits(t *testing.T) {
	t.Parallel()

	now := time.Now()

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   ptrtest.Ptr(int64(100)),
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
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   ptrtest.Ptr(int64(100)),
		RoomID:    200,
		StartTime: now,
		EndTime:   nil,
		Supervisors: []*active.GroupSupervisor{
			{Model: base.Model{ID: 1}, StaffID: 10, Role: "Teacher", StartDate: timezone.DateFromTime(now), EndDate: nil},                                   // Active
			{Model: base.Model{ID: 2}, StaffID: 20, Role: "Helper", StartDate: timezone.DateFromTime(now), EndDate: ptrDate(timezone.NewDate(2026, 8, 24))}, // Inactive (has end date)
			{Model: base.Model{ID: 3}, StaffID: 30, Role: "Supervisor", StartDate: timezone.DateFromTime(now), EndDate: nil},                                // Active
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
	t.Parallel()

	now := time.Now()

	group := &active.Group{
		Model:     base.Model{ID: 1},
		GroupID:   ptrtest.Ptr(int64(100)),
		RoomID:    200,
		StartTime: now,
		Room: &facilities.Room{
			ID: 200, Name: "Test Room",
		},
	}

	response := newActiveGroupResponse(group)

	assert.NotNil(t, response.Room)
	assert.Equal(t, int64(200), response.Room.ID)
	assert.Equal(t, "Test Room", response.Room.Name)
}

func TestNewVisitResponse_BasicFields(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	now := time.Now()

	visit := &active.Visit{
		Model:         base.Model{ID: 1},
		StudentID:     100,
		ActiveGroupID: 200,
		EntryTime:     now,
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 200},
			GroupID: ptrtest.Ptr(int64(300)),
		},
	}

	response := newVisitResponse(visit)

	assert.Equal(t, "Group #300", response.ActiveGroupName)
}

func TestNewSupervisorResponse_BasicFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endDate := timezone.TodayDate().AddDays(-1) // End date in the past = inactive

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		StaffID:   100,
		GroupID:   200,
		StartDate: timezone.TodayDate().AddDays(-1),
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
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: timezone.DateFromTime(now),
		EndDate:   nil, // Active
	}

	response := newSupervisorResponse(supervisor)

	assert.True(t, response.IsActive)
}

func TestNewSupervisorResponse_WithStaff(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: timezone.DateFromTime(now),
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
	t.Parallel()

	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)

	supervisor := &active.GroupSupervisor{
		Model:     base.Model{ID: 1},
		StaffID:   100,
		GroupID:   200,
		StartDate: timezone.DateFromTime(now),
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 200},
			GroupID: ptrtest.Ptr(int64(300)),
		},
	}

	response := newSupervisorResponse(supervisor)

	assert.Equal(t, "Group #300", response.ActiveGroupName)
}

func TestNewCombinedGroupResponse_BasicFields(t *testing.T) {
	t.Parallel()

	now := time.Now()
	endTime := now.Add(-time.Hour) // Past end time = inactive

	group := &active.CombinedGroup{
		Model:     base.Model{ID: 1, CreatedAt: now, UpdatedAt: now},
		StartTime: now.Add(-2 * time.Hour), // Started 2 hours ago
		EndTime:   &endTime,                // Ended 1 hour ago
	}

	response := newCombinedGroupResponse(group)

	assert.Equal(t, int64(1), response.ID)
	assert.Equal(t, "Combined Group #1", response.Name)
	assert.Empty(t, response.Description)
	assert.Equal(t, int64(0), response.RoomID)
	assert.Equal(t, group.StartTime, response.StartTime)
	assert.Equal(t, &endTime, response.EndTime)
	assert.False(t, response.IsActive) // End time is in the past
	assert.Equal(t, 0, response.GroupCount)
}

func TestNewCombinedGroupResponse_ActiveWithGroups(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	now := time.Now()

	mapping := &active.GroupMapping{
		Model:                 base.Model{ID: 1},
		ActiveGroupID:         100,
		ActiveCombinedGroupID: 200,
		ActiveGroup: &active.Group{
			Model:   base.Model{ID: 100},
			GroupID: ptrtest.Ptr(int64(50)),
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
	t.Parallel()

	req := &ActiveGroupRequest{
		GroupID:   1,
		RoomID:    2,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestActiveGroupRequest_Bind_MissingGroupID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	req := &ActiveGroupRequest{
		GroupID: 1,
		RoomID:  2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestVisitRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := &VisitRequest{
		StudentID:     1,
		ActiveGroupID: 2,
		CheckInTime:   time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestVisitRequest_Bind_MissingStudentID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	req := &VisitRequest{
		StudentID:     1,
		ActiveGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "check-in time is required")
}

func TestSupervisorRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := &SupervisorRequest{
		StaffID:       1,
		ActiveGroupID: 2,
		StartTime:     time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestSupervisorRequest_Bind_MissingStaffID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	req := &SupervisorRequest{
		StaffID:       1,
		ActiveGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestCombinedGroupRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := &CombinedGroupRequest{
		Name:      "Test Combined",
		RoomID:    1,
		StartTime: time.Now(),
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestCombinedGroupRequest_Bind_MissingName(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	req := &CombinedGroupRequest{
		Name:   "Test",
		RoomID: 1,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "start time is required")
}

func TestGroupMappingRequest_Bind_Valid(t *testing.T) {
	t.Parallel()

	req := &GroupMappingRequest{
		ActiveGroupID:   1,
		CombinedGroupID: 2,
	}

	err := req.Bind(nil)

	assert.NoError(t, err)
}

func TestGroupMappingRequest_Bind_MissingActiveGroupID(t *testing.T) {
	t.Parallel()

	req := &GroupMappingRequest{
		ActiveGroupID:   0,
		CombinedGroupID: 2,
	}

	err := req.Bind(nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "active group ID is required")
}

func TestGroupMappingRequest_Bind_MissingCombinedGroupID(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	rs := &Resource{}

	// Active Group Handlers
	assert.NotNil(t, rs.listActiveGroups)
	assert.NotNil(t, rs.getActiveGroup)
	assert.NotNil(t, rs.createActiveGroup)
	assert.NotNil(t, rs.updateActiveGroup)
	assert.NotNil(t, rs.deleteActiveGroup)
	assert.NotNil(t, rs.endActiveGroup)

	// Visit Handlers
	assert.NotNil(t, rs.listVisits)
	assert.NotNil(t, rs.getVisit)
	assert.NotNil(t, rs.createVisit)
	assert.NotNil(t, rs.updateVisit)
	assert.NotNil(t, rs.deleteVisit)
	assert.NotNil(t, rs.endVisit)
	assert.NotNil(t, rs.getStudentVisits)
	assert.NotNil(t, rs.getStudentCurrentVisit)

	// Supervisor Handlers
	assert.NotNil(t, rs.listSupervisors)
	assert.NotNil(t, rs.getSupervisor)
	assert.NotNil(t, rs.createSupervisor)
	assert.NotNil(t, rs.updateSupervisor)
	assert.NotNil(t, rs.deleteSupervisor)
	assert.NotNil(t, rs.endSupervision)
	assert.NotNil(t, rs.getStaffSupervisions)
	assert.NotNil(t, rs.getStaffActiveSupervisions)

	// Analytics Handlers
	assert.NotNil(t, rs.getDashboardAnalytics)
	// Combined Group Handlers
	assert.NotNil(t, rs.listCombinedGroups)
	assert.NotNil(t, rs.getCombinedGroup)
	assert.NotNil(t, rs.createCombinedGroup)
	assert.NotNil(t, rs.updateCombinedGroup)
	assert.NotNil(t, rs.deleteCombinedGroup)
	assert.NotNil(t, rs.endCombinedGroup)
	assert.NotNil(t, rs.getActiveCombinedGroups)

	// Group by filters Handlers
	assert.NotNil(t, rs.getActiveGroupsByRoom)
	assert.NotNil(t, rs.getActiveGroupsByGroup)
	assert.NotNil(t, rs.getActiveGroupVisits)
	assert.NotNil(t, rs.getActiveGroupVisitsWithDisplay)
	assert.NotNil(t, rs.getActiveGroupSupervisors)
	assert.NotNil(t, rs.getVisitsByGroup)
	assert.NotNil(t, rs.getSupervisorsByGroup)

	// Group Mapping Handlers
	assert.NotNil(t, rs.getGroupMappings)
	assert.NotNil(t, rs.getCombinedGroupMappings)
	assert.NotNil(t, rs.addGroupToCombination)
	assert.NotNil(t, rs.removeGroupFromCombination)

	// Unclaimed Group Handlers
	assert.NotNil(t, rs.listUnclaimedGroups)
	assert.NotNil(t, rs.claimGroup)

	// Checkout Handler
	assert.NotNil(t, rs.checkoutStudent)

	// Checkin Handler
	assert.NotNil(t, rs.checkinStudent)
}

// =============================================================================
// ERROR RENDERER TESTS - Testing error response functions
// =============================================================================

func TestErrorRenderer_ActiveGroupNotFound(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrActiveGroupNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Active Group Not Found", errResp.Status)
}

func TestErrorRenderer_VisitNotFound(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrVisitNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Visit Not Found", errResp.Status)
}

func TestErrorRenderer_GroupSupervisorNotFound(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrGroupSupervisorNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Supervisor Not Found", errResp.Status)
}

func TestErrorRenderer_CombinedGroupNotFound(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrCombinedGroupNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Combined Group Not Found", errResp.Status)
}

func TestErrorRenderer_GroupMappingNotFound(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrGroupMappingNotFound
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 404, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Mapping Not Found", errResp.Status)
}

func TestErrorRenderer_InvalidData(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrInvalidData
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Data", errResp.Status)
}

func TestErrorRenderer_ActiveGroupAlreadyEnded(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrActiveGroupAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Active Group Already Ended", errResp.Status)
}

func TestErrorRenderer_VisitAlreadyEnded(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrVisitAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Visit Already Ended", errResp.Status)
}

func TestErrorRenderer_SupervisionAlreadyEnded(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrSupervisionAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Supervision Already Ended", errResp.Status)
}

func TestErrorRenderer_CombinedGroupAlreadyEnded(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrCombinedGroupAlreadyEnded
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Combined Group Already Ended", errResp.Status)
}

func TestErrorRenderer_GroupAlreadyInCombination(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrGroupAlreadyInCombination
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Group Already In Combination", errResp.Status)
}

func TestErrorRenderer_StudentAlreadyInGroup(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrStudentAlreadyInGroup
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Student Already In Group", errResp.Status)
}

func TestErrorRenderer_StudentAlreadyActive(t *testing.T) {
	t.Parallel()

	// 409 Conflict, not 400 Bad Request: the duplicate-active-visit
	// translation introduced for Issue #844 means this error now
	// surfaces from the DB-level unique index on every CreateVisit
	// caller, not just the IoT checkin flow. Mapping it to 400 would
	// contradict both the IoT path (which already returns 409) and
	// the HTTP semantics — a duplicate visit is a state conflict,
	// not a malformed request.
	err := activeSvc.ErrStudentAlreadyActive
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "Student Already Has Active Visit", errResp.Status)
}

func TestErrorRenderer_StaffAlreadySupervising(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrStaffAlreadySupervising
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Staff Already Supervising This Group", errResp.Status)
}

func TestErrorRenderer_CannotDeleteActiveGroup(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrCannotDeleteActiveGroup
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Cannot Delete Active Group With Active Visits", errResp.Status)
}

func TestErrorRenderer_InvalidTimeRange(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrInvalidTimeRange
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 400, errResp.HTTPStatusCode)
	assert.Equal(t, "Invalid Time Range", errResp.Status)
}

func TestErrorRenderer_RoomConflict(t *testing.T) {
	t.Parallel()

	err := activeSvc.ErrRoomConflict
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 409, errResp.HTTPStatusCode)
	assert.Equal(t, "Room Conflict", errResp.Status)
}

func TestErrorRenderer_UnknownError(t *testing.T) {
	t.Parallel()

	err := errors.New("unknown error")
	renderer := ErrorRenderer(err)

	errResp, ok := renderer.(*common.ErrResponse)
	assert.True(t, ok)
	assert.Equal(t, 500, errResp.HTTPStatusCode)
	assert.Equal(t, "Internal Server Error", errResp.Status)
	assert.Equal(t, "unknown error", errResp.ErrorText)
}

func ptrDate(d timezone.Date) *timezone.Date { return &d }
