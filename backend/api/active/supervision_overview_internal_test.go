package active

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSettingsSvc builds a configtest.Mock backed by a boolValues map,
// mirroring the previous hand-rolled stub's ResolveBool/ResolveBoolForTenant
// behavior: missing keys return an error.
func mockSettingsSvc(boolValues map[string]bool) *configtest.Mock {
	resolveBool := func(_ context.Context, key string) (bool, error) {
		if v, ok := boolValues[key]; ok {
			return v, nil
		}
		return false, fmt.Errorf("not found: %s", key)
	}
	return &configtest.Mock{
		ResolveBoolFn: resolveBool,
		ResolveBoolForTenantFn: func(ctx context.Context, _ int64, key string) (bool, error) {
			return resolveBool(ctx, key)
		},
	}
}

// =============================================================================
// MOCK: Minimal ActiveService stub (only ListActiveGroups needed for success path)
// =============================================================================

type stubActiveService struct {
	listActiveGroupsFunc func(ctx context.Context, options *base.QueryOptions) ([]*activeModel.Group, error)
}

func (s *stubActiveService) GetRoomsByIDs(_ context.Context, _ []int64) ([]*facilityModels.Room, error) {
	return nil, nil
}

func (s *stubActiveService) GetActiveGroupVisitsWithDisplay(_ context.Context, _ int64) ([]*activeModel.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (s *stubActiveService) HasOpenAttendanceOn(_ context.Context, _ timezone.Date) (bool, error) {
	return false, nil
}

func (s *stubActiveService) ConfirmDailyCheckout(_ context.Context, _, _ int64, _ string) (*activeSvc.DailyCheckoutResult, error) {
	return nil, nil
}

func (s *stubActiveService) ListActiveGroups(ctx context.Context, options *base.QueryOptions) ([]*activeModel.Group, error) {
	if s.listActiveGroupsFunc != nil {
		return s.listActiveGroupsFunc(ctx, options)
	}
	return []*activeModel.Group{}, nil
}

// All other interface methods return nil/zero — they are never called by getAllActiveSupervisions
func (s *stubActiveService) GetTrackingIndicators(_ context.Context, _ []int64, _ []string) (map[int64][]bool, error) {
	return map[int64][]bool{}, nil
}
func (s *stubActiveService) SetSettingsService(_ activeSvc.SettingsResolver) {}
func (s *stubActiveService) GetPresenceMode(_ context.Context) string        { return "detailed" }
func (s *stubActiveService) GetActiveGroup(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) CreateActiveGroup(_ context.Context, _ *activeModel.Group) error {
	return nil
}
func (s *stubActiveService) UpdateActiveGroup(_ context.Context, _ *activeModel.Group) error {
	return nil
}
func (s *stubActiveService) DeleteActiveGroup(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) FindDeviceActiveGroupInRoom(_ context.Context, _, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) FindActiveGroupsByGroupID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) EndActiveGroupSession(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) GetActiveGroupWithVisits(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) GetActiveGroupWithSupervisors(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) GetVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) CreateVisit(_ context.Context, _ *activeModel.Visit) error { return nil }
func (s *stubActiveService) UpdateVisit(_ context.Context, _ *activeModel.Visit) error { return nil }
func (s *stubActiveService) DeleteVisit(_ context.Context, _ int64) error              { return nil }
func (s *stubActiveService) ListVisits(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) FindVisitsByStudentID(_ context.Context, _ int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) FindVisitsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) EndVisit(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) GetStudentCurrentVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) GetStudentCurrentVisitWithRoom(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) GetStudentsCurrentVisits(_ context.Context, _ []int64) (map[int64]*activeModel.Visit, error) {
	return nil, nil
}
func (s *stubActiveService) ListStudentsInTransit(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (s *stubActiveService) ListStudentsPresentToday(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (s *stubActiveService) AssignTransitStudentsToActiveGroup(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
	return nil, nil
}
func (s *stubActiveService) MoveStudentsToActiveGroupAuthorized(_ context.Context, _ []int64, _ int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	return nil, nil
}
func (s *stubActiveService) MoveStudentsToTransitAuthorized(_ context.Context, _ []int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	return nil, nil
}
func (s *stubActiveService) CountActiveVisitsByRoomID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (s *stubActiveService) CountActiveVisitsByActiveGroupID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (s *stubActiveService) ListStudentsPresentInRoom(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (s *stubActiveService) GetGroupSupervisor(_ context.Context, _ int64) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) CreateGroupSupervisor(_ context.Context, _ *activeModel.GroupSupervisor) error {
	return nil
}
func (s *stubActiveService) UpdateGroupSupervisor(_ context.Context, _ *activeModel.GroupSupervisor) error {
	return nil
}
func (s *stubActiveService) DeleteGroupSupervisor(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) ListGroupSupervisors(_ context.Context, _ *base.QueryOptions) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) FindSupervisorsByStaffID(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) FindSupervisorsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) FindSupervisorsByActiveGroupIDs(_ context.Context, _ []int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) EndSupervision(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) GetStaffActiveSupervisions(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) GetCombinedGroup(_ context.Context, _ int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (s *stubActiveService) CreateCombinedGroup(_ context.Context, _ *activeModel.CombinedGroup) error {
	return nil
}
func (s *stubActiveService) UpdateCombinedGroup(_ context.Context, _ *activeModel.CombinedGroup) error {
	return nil
}
func (s *stubActiveService) DeleteCombinedGroup(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) ListCombinedGroups(_ context.Context, _ *base.QueryOptions) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (s *stubActiveService) FindActiveCombinedGroups(_ context.Context) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (s *stubActiveService) FindCombinedGroupsByTimeRange(_ context.Context, _, _ time.Time) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (s *stubActiveService) EndCombinedGroup(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) GetCombinedGroupWithGroups(_ context.Context, _ int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (s *stubActiveService) CreateCombinedGroupWithGroups(_ context.Context, _ *activeModel.CombinedGroup, _ []int64) error {
	return nil
}
func (s *stubActiveService) AddGroupToCombination(_ context.Context, _, _ int64) error { return nil }
func (s *stubActiveService) RemoveGroupFromCombination(_ context.Context, _, _ int64) error {
	return nil
}
func (s *stubActiveService) GetGroupMappingsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}
func (s *stubActiveService) GetGroupMappingsByCombinedGroupID(_ context.Context, _ int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}

// Activity session stubs
func (s *stubActiveService) StartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) CheckActivityConflict(_ context.Context, _, _ int64) (*activeSvc.ActivityConflictInfo, error) {
	return nil, nil
}
func (s *stubActiveService) EndActivitySession(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) ForceStartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) GetDeviceCurrentSession(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) UpdateActiveGroupSupervisors(_ context.Context, _ int64, _ []int64) (*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) ProcessSessionTimeout(_ context.Context, _ int64) (*activeSvc.TimeoutResult, error) {
	return nil, nil
}
func (s *stubActiveService) UpdateSessionActivity(_ context.Context, _ int64) error { return nil }
func (s *stubActiveService) ValidateSessionTimeout(_ context.Context, _ int64, _ int) error {
	return nil
}
func (s *stubActiveService) GetSessionTimeoutInfo(_ context.Context, _ int64) (*activeSvc.SessionTimeoutInfo, error) {
	return nil, nil
}
func (s *stubActiveService) CleanupAbandonedSessions(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (s *stubActiveService) EndDailySessions(_ context.Context) (*activeSvc.DailySessionCleanupResult, error) {
	return nil, nil
}
func (s *stubActiveService) GetDashboardAnalytics(_ context.Context) (*activeSvc.DashboardAnalytics, error) {
	return nil, nil
}
func (s *stubActiveService) GetActiveGroupsByIDs(_ context.Context, _ []int64) (map[int64]*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) GetStudentAttendanceStatus(_ context.Context, _ int64) (*activeSvc.AttendanceStatus, error) {
	return nil, nil
}
func (s *stubActiveService) GetStudentsAttendanceStatuses(_ context.Context, _ []int64) (map[int64]*activeSvc.AttendanceStatus, error) {
	return nil, nil
}
func (s *stubActiveService) ToggleStudentAttendance(_ context.Context, _, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (s *stubActiveService) CheckInStudent(_ context.Context, _, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (s *stubActiveService) CheckOutStudent(_ context.Context, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (s *stubActiveService) CheckOutStudentFromDevice(_ context.Context, _, _ int64) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (s *stubActiveService) ProcessSchoolCheckinBatch(_ context.Context, _ []int64, _ int64, _ string) (*activeSvc.SchoolCheckinBatchResult, error) {
	return nil, nil
}
func (s *stubActiveService) CheckTeacherStudentAccess(_ context.Context, _, _ int64) (bool, error) {
	return false, nil
}
func (s *stubActiveService) GetUnclaimedActiveGroups(_ context.Context) ([]*activeModel.Group, error) {
	return nil, nil
}
func (s *stubActiveService) ClaimActiveGroup(_ context.Context, _, _ int64, _ string) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (s *stubActiveService) GetCrossTenantStudents(_ context.Context, _ int64) ([]activeModel.CrossTenantStudent, error) {
	return nil, nil
}

// =============================================================================
// HELPER: build request with JWT claims in context
// =============================================================================

func newRequestWithClaims(method, path string, claims jwt.AppClaims) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	return req.WithContext(ctx)
}

func adminClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:       42,
		IsAdmin:  true,
		Roles:    []string{"admin"},
		TenantID: 10,
	}
}

func staffClaims() jwt.AppClaims {
	return jwt.AppClaims{
		ID:       99,
		IsAdmin:  false,
		Roles:    []string{"user"},
		TenantID: 10,
	}
}

// =============================================================================
// TESTS: isAdminWithSupervisionOverview
// =============================================================================

func TestIsAdminWithSupervisionOverview_NonAdmin(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
	}
	r := newRequestWithClaims("GET", "/", staffClaims())

	assert.False(t, rs.isAdminWithSupervisionOverview(r))
}

func TestIsAdminWithSupervisionOverview_NilSettings(t *testing.T) {
	t.Parallel()

	rs := &Resource{SettingsService: nil}
	r := newRequestWithClaims("GET", "/", adminClaims())

	assert.False(t, rs.isAdminWithSupervisionOverview(r))
}

func TestIsAdminWithSupervisionOverview_SettingDisabled(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}),
	}
	r := newRequestWithClaims("GET", "/", adminClaims())

	assert.False(t, rs.isAdminWithSupervisionOverview(r))
}

func TestIsAdminWithSupervisionOverview_SettingEnabled(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
	}
	r := newRequestWithClaims("GET", "/", adminClaims())

	assert.True(t, rs.isAdminWithSupervisionOverview(r))
}

func TestIsAdminWithSupervisionOverview_SettingError(t *testing.T) {
	t.Parallel()

	// Key not in map → ResolveBool returns error
	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{}),
	}
	r := newRequestWithClaims("GET", "/", adminClaims())

	assert.False(t, rs.isAdminWithSupervisionOverview(r))
}

// =============================================================================
// TESTS: getAllActiveSupervisions handler
// =============================================================================

func TestGetAllActiveSupervisions_ForbiddenForNonAdmin(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", staffClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_OpenCareAllowsPermissionBearingStaff(t *testing.T) {
	t.Parallel()

	settings := mockSettingsSvc(map[string]bool{})
	settings.ResolveStringFn = func(_ context.Context, key string) (string, error) {
		assert.Equal(t, configModel.KeyGroupMode, key)
		return configModel.GroupModeOpenCare, nil
	}
	rs := &Resource{SettingsService: settings, ActiveService: &stubActiveService{}}
	r := newRequestWithClaims("GET", "/active/supervisors/all", staffClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestGetAllActiveSupervisions_ForbiddenWhenSettingsNil(t *testing.T) {
	t.Parallel()

	rs := &Resource{SettingsService: nil}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_ForbiddenWhenSettingDisabled(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}),
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_ForbiddenOnSettingError(t *testing.T) {
	t.Parallel()

	// Empty boolValues → ResolveBool returns error for the key
	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{}),
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestGetAllActiveSupervisions_SuccessEmptyGroups(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		ActiveService: &stubActiveService{},
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])
}

func TestGetAllActiveSupervisions_SuccessWithActiveGroups(t *testing.T) {
	t.Parallel()

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	endedTime := now.Add(-1 * time.Hour)

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		ActiveService: &stubActiveService{
			listActiveGroupsFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return []*activeModel.Group{
					// Active group (no end time)
					{
						Model:     base.Model{ID: 100},
						StartTime: past,
					},
					// Ended group (has end time in the past → IsActive() = false)
					{
						Model:     base.Model{ID: 101},
						StartTime: past,
						EndTime:   &endedTime,
					},
				}, nil
			},
		},
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])

	// Only the active group should be in the response (the ended one is filtered out)
	data, ok := resp["data"].([]any)
	require.True(t, ok)
	assert.Len(t, data, 1)
}

func TestGetAllActiveSupervisions_ServiceError(t *testing.T) {
	t.Parallel()

	rs := &Resource{
		SettingsService: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		ActiveService: &stubActiveService{
			listActiveGroupsFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return nil, fmt.Errorf("database error")
			},
		},
	}
	r := newRequestWithClaims("GET", "/active/supervisors/all", adminClaims())
	w := httptest.NewRecorder()

	rs.getAllActiveSupervisions(w, r)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
