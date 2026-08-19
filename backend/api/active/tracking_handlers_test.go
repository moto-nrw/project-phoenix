package active

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Mock SettingsService ---

// newTrackingMockSettingsService builds a configtest.Mock wired the same way
// as the previous hand-rolled stub: ResolveBool/ResolveString delegate to the
// given funcs (nil → zero value), and the ForTenant variants delegate to the
// same funcs.
func newTrackingMockSettingsService(resolveBoolFunc func(ctx context.Context, key string) (bool, error), resolveStringFunc func(ctx context.Context, key string) (string, error)) *configtest.Mock {
	resolveBool := func(ctx context.Context, key string) (bool, error) {
		if resolveBoolFunc != nil {
			return resolveBoolFunc(ctx, key)
		}
		return false, nil
	}
	resolveString := func(ctx context.Context, key string) (string, error) {
		if resolveStringFunc != nil {
			return resolveStringFunc(ctx, key)
		}
		return "", nil
	}
	return &configtest.Mock{
		ResolveBoolFn:   resolveBool,
		ResolveStringFn: resolveString,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
	}
}

// --- Mock ActiveService (stub all methods, only GetTrackingIndicators functional) ---

type trackingMockActiveService struct {
	getTrackingIndicatorsFunc               func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error)
	assignTransitStudentsToActiveGroupFunc  func(ctx context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.TransitAssignResult, error)
	moveStudentsToActiveGroupFunc           func(ctx context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.StudentMoveResult, error)
	moveStudentsToActiveGroupAuthorizedFunc func(ctx context.Context, studentIDs []int64, activeGroupID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
	moveStudentsToTransitFunc               func(ctx context.Context, studentIDs []int64) (*activeSvc.StudentMoveResult, error)
	moveStudentsToTransitAuthorizedFunc     func(ctx context.Context, studentIDs []int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error)
	getActiveGroupFunc                      func(ctx context.Context, id int64) (*activeModel.Group, error)
	getStudentCurrentVisitFunc              func(ctx context.Context, studentID int64) (*activeModel.Visit, error)
	getStudentsAttendanceStatusesFunc       func(ctx context.Context, studentIDs []int64) (map[int64]*activeSvc.AttendanceStatus, error)
	getStaffActiveSupervisionsFunc          func(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error)
	checkTeacherStudentAccessFunc           func(ctx context.Context, teacherID, studentID int64) (bool, error)
}

// The only method used by the tracking handler:
func (m *trackingMockActiveService) GetRoomsByIDs(_ context.Context, _ []int64) ([]*facilityModels.Room, error) {
	return nil, nil
}

func (m *trackingMockActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, _ int64) ([]*activeModel.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (m *trackingMockActiveService) HasOpenAttendanceOn(_ context.Context, _ timezone.Date) (bool, error) {
	return false, nil
}

func (m *trackingMockActiveService) ConfirmDailyCheckout(_ context.Context, _, _ int64, _ string) (*activeSvc.DailyCheckoutResult, error) {
	return nil, nil
}

func (m *trackingMockActiveService) GetTrackingIndicators(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
	if m.getTrackingIndicatorsFunc != nil {
		return m.getTrackingIndicatorsFunc(ctx, studentIDs, labels)
	}
	return nil, nil
}
func (m *trackingMockActiveService) SetSettingsService(_ activeSvc.SettingsResolver) {}
func (m *trackingMockActiveService) GetPresenceMode(_ context.Context) string {
	return "detailed"
}

// Stub out the rest of the Service interface:
func (m *trackingMockActiveService) GetActiveGroup(ctx context.Context, id int64) (*activeModel.Group, error) {
	if m.getActiveGroupFunc != nil {
		return m.getActiveGroupFunc(ctx, id)
	}
	return nil, nil
}
func (m *trackingMockActiveService) CreateActiveGroup(ctx context.Context, group *activeModel.Group) error {
	return nil
}
func (m *trackingMockActiveService) UpdateActiveGroup(ctx context.Context, group *activeModel.Group) error {
	return nil
}
func (m *trackingMockActiveService) DeleteActiveGroup(ctx context.Context, id int64) error {
	return nil
}
func (m *trackingMockActiveService) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindActiveGroupsByRoomID(ctx context.Context, roomID int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindDeviceActiveGroupInRoom(ctx context.Context, roomID int64, deviceID int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindActiveGroupsByGroupID(ctx context.Context, groupID int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) EndActiveGroupSession(ctx context.Context, id int64) error {
	return nil
}
func (m *trackingMockActiveService) GetActiveGroupWithVisits(ctx context.Context, id int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetActiveGroupWithSupervisors(ctx context.Context, id int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetVisit(ctx context.Context, id int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CreateVisit(ctx context.Context, visit *activeModel.Visit) error {
	return nil
}
func (m *trackingMockActiveService) UpdateVisit(ctx context.Context, visit *activeModel.Visit) error {
	return nil
}
func (m *trackingMockActiveService) DeleteVisit(ctx context.Context, id int64) error { return nil }
func (m *trackingMockActiveService) ListVisits(ctx context.Context, options *base.QueryOptions) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindVisitsByStudentID(ctx context.Context, studentID int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) EndVisit(ctx context.Context, id int64) error { return nil }
func (m *trackingMockActiveService) GetStudentCurrentVisit(ctx context.Context, studentID int64) (*activeModel.Visit, error) {
	if m.getStudentCurrentVisitFunc != nil {
		return m.getStudentCurrentVisitFunc(ctx, studentID)
	}
	return nil, nil
}
func (m *trackingMockActiveService) GetStudentCurrentVisitWithRoom(ctx context.Context, studentID int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetStudentsCurrentVisits(ctx context.Context, studentIDs []int64) (map[int64]*activeModel.Visit, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CountActiveVisitsByRoomID(ctx context.Context, roomID int64) (int, error) {
	return 0, nil
}
func (m *trackingMockActiveService) CountActiveVisitsByActiveGroupID(ctx context.Context, activeGroupID int64) (int, error) {
	return 0, nil
}
func (m *trackingMockActiveService) ListStudentsPresentInRoom(ctx context.Context, roomID int64) ([]int64, error) {
	return nil, nil
}
func (m *trackingMockActiveService) ListStudentsInTransit(ctx context.Context) ([]int64, error) {
	return nil, nil
}
func (m *trackingMockActiveService) ListStudentsPresentToday(ctx context.Context) ([]int64, error) {
	return nil, nil
}
func (m *trackingMockActiveService) AssignTransitStudentsToActiveGroup(ctx context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.TransitAssignResult, error) {
	if m.assignTransitStudentsToActiveGroupFunc != nil {
		return m.assignTransitStudentsToActiveGroupFunc(ctx, studentIDs, activeGroupID)
	}
	return nil, nil
}
func (m *trackingMockActiveService) MoveStudentsToActiveGroupAuthorized(ctx context.Context, studentIDs []int64, activeGroupID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	if m.moveStudentsToActiveGroupAuthorizedFunc != nil {
		return m.moveStudentsToActiveGroupAuthorizedFunc(ctx, studentIDs, activeGroupID, auth)
	}
	if m.moveStudentsToActiveGroupFunc != nil {
		return m.moveStudentsToActiveGroupFunc(ctx, studentIDs, activeGroupID)
	}
	return nil, nil
}
func (m *trackingMockActiveService) MoveStudentsToTransitAuthorized(ctx context.Context, studentIDs []int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	if m.moveStudentsToTransitAuthorizedFunc != nil {
		return m.moveStudentsToTransitAuthorizedFunc(ctx, studentIDs, auth)
	}
	if m.moveStudentsToTransitFunc != nil {
		return m.moveStudentsToTransitFunc(ctx, studentIDs)
	}
	return nil, nil
}
func (m *trackingMockActiveService) GetGroupSupervisor(ctx context.Context, id int64) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CreateGroupSupervisor(ctx context.Context, supervisor *activeModel.GroupSupervisor) error {
	return nil
}
func (m *trackingMockActiveService) UpdateGroupSupervisor(ctx context.Context, supervisor *activeModel.GroupSupervisor) error {
	return nil
}
func (m *trackingMockActiveService) DeleteGroupSupervisor(ctx context.Context, id int64) error {
	return nil
}
func (m *trackingMockActiveService) ListGroupSupervisors(ctx context.Context, options *base.QueryOptions) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindSupervisorsByStaffID(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindSupervisorsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindSupervisorsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) EndSupervision(ctx context.Context, id int64) error { return nil }
func (m *trackingMockActiveService) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
	if m.getStaffActiveSupervisionsFunc != nil {
		return m.getStaffActiveSupervisionsFunc(ctx, staffID)
	}
	return nil, nil
}
func (m *trackingMockActiveService) GetCombinedGroup(ctx context.Context, id int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CreateCombinedGroup(ctx context.Context, group *activeModel.CombinedGroup) error {
	return nil
}
func (m *trackingMockActiveService) UpdateCombinedGroup(ctx context.Context, group *activeModel.CombinedGroup) error {
	return nil
}
func (m *trackingMockActiveService) DeleteCombinedGroup(ctx context.Context, id int64) error {
	return nil
}
func (m *trackingMockActiveService) ListCombinedGroups(ctx context.Context, options *base.QueryOptions) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindActiveCombinedGroups(ctx context.Context) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *trackingMockActiveService) FindCombinedGroupsByTimeRange(ctx context.Context, start, end time.Time) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *trackingMockActiveService) EndCombinedGroup(ctx context.Context, id int64) error {
	return nil
}
func (m *trackingMockActiveService) GetCombinedGroupWithGroups(ctx context.Context, id int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CreateCombinedGroupWithGroups(ctx context.Context, group *activeModel.CombinedGroup, groupIDs []int64) error {
	return nil
}
func (m *trackingMockActiveService) AddGroupToCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	return nil
}
func (m *trackingMockActiveService) RemoveGroupFromCombination(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	return nil
}
func (m *trackingMockActiveService) GetGroupMappingsByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetGroupMappingsByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}
func (m *trackingMockActiveService) StartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CheckActivityConflict(ctx context.Context, activityID, deviceID int64) (*activeSvc.ActivityConflictInfo, error) {
	return nil, nil
}
func (m *trackingMockActiveService) EndActivitySession(ctx context.Context, activeGroupID int64) error {
	return nil
}
func (m *trackingMockActiveService) ForceStartActivitySessionWithSupervisors(ctx context.Context, activityID, deviceID int64, supervisorIDs []int64, roomID *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetDeviceCurrentSession(ctx context.Context, deviceID int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) UpdateActiveGroupSupervisors(ctx context.Context, activeGroupID int64, supervisorIDs []int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) ProcessSessionTimeout(ctx context.Context, deviceID int64) (*activeSvc.TimeoutResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) UpdateSessionActivity(ctx context.Context, activeGroupID int64) error {
	return nil
}
func (m *trackingMockActiveService) ValidateSessionTimeout(ctx context.Context, deviceID int64, timeoutMinutes int) error {
	return nil
}
func (m *trackingMockActiveService) GetSessionTimeoutInfo(ctx context.Context, deviceID int64) (*activeSvc.SessionTimeoutInfo, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CleanupAbandonedSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	return 0, nil
}
func (m *trackingMockActiveService) EndDailySessions(ctx context.Context) (*activeSvc.DailySessionCleanupResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetDashboardAnalytics(ctx context.Context) (*activeSvc.DashboardAnalytics, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetActiveGroupsByIDs(ctx context.Context, groupIDs []int64) (map[int64]*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetStudentAttendanceStatus(ctx context.Context, studentID int64) (*activeSvc.AttendanceStatus, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetStudentsAttendanceStatuses(ctx context.Context, studentIDs []int64) (map[int64]*activeSvc.AttendanceStatus, error) {
	if m.getStudentsAttendanceStatusesFunc != nil {
		return m.getStudentsAttendanceStatusesFunc(ctx, studentIDs)
	}
	return nil, nil
}
func (m *trackingMockActiveService) ToggleStudentAttendance(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CheckInStudent(ctx context.Context, studentID, staffID, deviceID int64, skipAuthCheck bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CheckOutStudent(ctx context.Context, studentID, staffID int64, skipAuthCheck bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CheckOutStudentFromDevice(ctx context.Context, studentID, deviceID int64) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) ProcessSchoolCheckinBatch(ctx context.Context, studentIDs []int64, staffID int64, action string) (*activeSvc.SchoolCheckinBatchResult, error) {
	return nil, nil
}
func (m *trackingMockActiveService) CheckTeacherStudentAccess(ctx context.Context, teacherID, studentID int64) (bool, error) {
	if m.checkTeacherStudentAccessFunc != nil {
		return m.checkTeacherStudentAccessFunc(ctx, teacherID, studentID)
	}
	return false, nil
}
func (m *trackingMockActiveService) GetUnclaimedActiveGroups(ctx context.Context) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *trackingMockActiveService) ClaimActiveGroup(ctx context.Context, groupID, staffID int64, role string) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *trackingMockActiveService) GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]activeModel.CrossTenantStudent, error) {
	return nil, nil
}

// --- Test Helpers ---

func createTrackingRequest(t *testing.T, body TrackingIndicatorsRequest) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	return httptest.NewRequest(http.MethodPost, "/tracking-indicators", bytes.NewReader(b))
}

// --- Tests ---

func TestGetTrackingIndicators_InvalidBody(t *testing.T) {
	rs := &Resource{}

	req := httptest.NewRequest(http.MethodPost, "/tracking-indicators", bytes.NewReader([]byte("not json")))
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_EmptyStudentIDs(t *testing.T) {
	rs := &Resource{}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
	assert.Empty(t, resp.Data.Results)
}

func TestGetTrackingIndicators_InvalidStudentID(t *testing.T) {
	rs := &Resource{}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, 0}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_NegativeStudentID(t *testing.T) {
	rs := &Resource{}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, -5}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetTrackingIndicators_SettingsError(t *testing.T) {
	settings := newTrackingMockSettingsService(func(ctx context.Context, key string) (bool, error) {
		return false, errors.New("settings service error")
	}, nil)

	rs := &Resource{SettingsService: settings}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	// Returns 200 with empty data on settings error (graceful degradation)
	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_Disabled(t *testing.T) {
	settings := newTrackingMockSettingsService(func(ctx context.Context, key string) (bool, error) {
		return false, nil
	}, nil)

	rs := &Resource{SettingsService: settings}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_NoLabelsConfigured(t *testing.T) {
	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "", nil
		},
	)

	rs := &Resource{SettingsService: settings}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_LabelResolutionError(t *testing.T) {
	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "", errors.New("label read error")
		},
	)

	rs := &Resource{SettingsService: settings}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_Success(t *testing.T) {
	labelValues := map[string]string{
		config.KeyTrackingIndicator1: "Hausaufgaben",
		config.KeyTrackingIndicator2: "Mensa",
		config.KeyTrackingIndicator3: "",
	}

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return labelValues[key], nil
		},
	)

	mockSvc := &trackingMockActiveService{
		getTrackingIndicatorsFunc: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			assert.Equal(t, []int64{10, 20}, studentIDs)
			assert.Equal(t, []string{"Hausaufgaben", "Mensa"}, labels)
			return map[int64][]bool{
				10: {true, false},
				20: {false, true},
			}, nil
		},
	}

	rs := &Resource{
		SettingsService: settings,
		ActiveService:   mockSvc,
	}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10, 20}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, []string{"Hausaufgaben", "Mensa"}, resp.Data.Labels)
	assert.Equal(t, map[int64][]bool{10: {true, false}, 20: {false, true}}, resp.Data.Results)
}

func TestGetTrackingIndicators_ServiceError(t *testing.T) {
	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "Mensa", nil
		},
	)

	mockSvc := &trackingMockActiveService{
		getTrackingIndicatorsFunc: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			return nil, errors.New("service error")
		},
	}

	rs := &Resource{
		SettingsService: settings,
		ActiveService:   mockSvc,
	}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetTrackingIndicators_WhitespaceOnlyLabels(t *testing.T) {
	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return "   ", nil
		},
	)

	rs := &Resource{SettingsService: settings}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data TrackingIndicatorsResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data.Labels)
}

func TestGetTrackingIndicators_PartialLabelsConfigured(t *testing.T) {
	labelValues := map[string]string{
		config.KeyTrackingIndicator1: "Mensa",
		config.KeyTrackingIndicator2: "",
		config.KeyTrackingIndicator3: "Sport",
	}

	settings := newTrackingMockSettingsService(
		func(ctx context.Context, key string) (bool, error) {
			return true, nil
		},
		func(ctx context.Context, key string) (string, error) {
			return labelValues[key], nil
		},
	)

	mockSvc := &trackingMockActiveService{
		getTrackingIndicatorsFunc: func(ctx context.Context, studentIDs []int64, labels []string) (map[int64][]bool, error) {
			assert.Equal(t, []string{"Mensa", "Sport"}, labels)
			return map[int64][]bool{10: {true, true}}, nil
		},
	}

	rs := &Resource{
		SettingsService: settings,
		ActiveService:   mockSvc,
	}

	req := createTrackingRequest(t, TrackingIndicatorsRequest{StudentIDs: []int64{10}})
	rr := httptest.NewRecorder()

	rs.getTrackingIndicators(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}
