package sse

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// MOCK: SettingsService
// =============================================================================

type mockSettingsSvc struct {
	boolValues map[string]bool
}

func (m *mockSettingsSvc) GetSchema(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *mockSettingsSvc) GetSchemaForOperator(_ context.Context, _ []string) (*configSvc.SettingsSchema, error) {
	return nil, nil
}
func (m *mockSettingsSvc) Resolve(_ context.Context, key string) (any, error) {
	if v, ok := m.boolValues[key]; ok {
		return v, nil
	}
	return nil, fmt.Errorf("not found: %s", key)
}
func (m *mockSettingsSvc) ResolveString(_ context.Context, _ string) (string, error) {
	return "", nil
}
func (m *mockSettingsSvc) ResolveStringForTenant(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (m *mockSettingsSvc) ResolveBool(_ context.Context, key string) (bool, error) {
	if v, ok := m.boolValues[key]; ok {
		return v, nil
	}
	return false, fmt.Errorf("not found: %s", key)
}
func (m *mockSettingsSvc) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	return m.ResolveBool(context.Background(), key)
}
func (m *mockSettingsSvc) ResolveInt(_ context.Context, _ string) (int, error) { return 0, nil }
func (m *mockSettingsSvc) ResolveIntForTenant(_ context.Context, _ int64, _ string) (int, error) {
	return 0, nil
}
func (m *mockSettingsSvc) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (m *mockSettingsSvc) SetValue(_ context.Context, _ string, _ any, _ *int64, _ []string) error {
	return nil
}
func (m *mockSettingsSvc) ResetValue(_ context.Context, _ string, _ *int64, _ []string) error {
	return nil
}
func (m *mockSettingsSvc) GetLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}
func (m *mockSettingsSvc) SetLoginImageURL(_ context.Context, _ int64, _ string) (string, error) {
	return "", nil
}
func (m *mockSettingsSvc) ClearLoginImageURL(_ context.Context, _ int64) (string, error) {
	return "", nil
}

// =============================================================================
// MOCK: Minimal ActiveService for SSE tests
// =============================================================================

type mockActiveSvcForSSE struct {
	getAllFunc   func(ctx context.Context) ([]*activeModel.GroupSupervisor, error)
	getStaffFunc func(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error)
	listFunc     func(ctx context.Context, opts *base.QueryOptions) ([]*activeModel.Group, error)
}

func (m *mockActiveSvcForSSE) GetRoomsByIDs(_ context.Context, _ []int64) ([]*facilityModels.Room, error) {
	return nil, nil
}

func (m *mockActiveSvcForSSE) GetActiveGroupVisitsWithDisplay(_ context.Context, _ int64) ([]*activeModel.VisitWithStudentDisplay, error) {
	return nil, nil
}

func (m *mockActiveSvcForSSE) HasOpenAttendanceOn(_ context.Context, _ timezone.Date) (bool, error) {
	return false, nil
}

func (m *mockActiveSvcForSSE) GetAllActiveSupervisions(ctx context.Context) ([]*activeModel.GroupSupervisor, error) {
	if m.getAllFunc != nil {
		return m.getAllFunc(ctx)
	}
	return []*activeModel.GroupSupervisor{}, nil
}
func (m *mockActiveSvcForSSE) GetStaffActiveSupervisions(ctx context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
	if m.getStaffFunc != nil {
		return m.getStaffFunc(ctx, staffID)
	}
	return []*activeModel.GroupSupervisor{}, nil
}

func (m *mockActiveSvcForSSE) GetTrackingIndicators(_ context.Context, _ []int64, _ []string) (map[int64][]bool, error) {
	return map[int64][]bool{}, nil
}

func (m *mockActiveSvcForSSE) SetSettingsService(_ activeSvc.SettingsResolver) {}
func (m *mockActiveSvcForSSE) GetPresenceMode(_ context.Context) string        { return "detailed" }

// Stubs for the rest of active.Service (never called by resolveSupervisions)
func (m *mockActiveSvcForSSE) GetActiveGroup(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CreateActiveGroup(_ context.Context, _ *activeModel.Group) error {
	return nil
}
func (m *mockActiveSvcForSSE) UpdateActiveGroup(_ context.Context, _ *activeModel.Group) error {
	return nil
}
func (m *mockActiveSvcForSSE) DeleteActiveGroup(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) ListActiveGroups(ctx context.Context, opts *base.QueryOptions) ([]*activeModel.Group, error) {
	if m.listFunc != nil {
		return m.listFunc(ctx, opts)
	}
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindActiveGroupsByRoomID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindDeviceActiveGroupInRoom(_ context.Context, _, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindActiveGroupsByGroupID(_ context.Context, _ int64) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindActiveGroupsByTimeRange(_ context.Context, _, _ time.Time) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndActiveGroupSession(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) GetActiveGroupWithVisits(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetActiveGroupWithSupervisors(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CreateVisit(_ context.Context, _ *activeModel.Visit) error { return nil }
func (m *mockActiveSvcForSSE) UpdateVisit(_ context.Context, _ *activeModel.Visit) error { return nil }
func (m *mockActiveSvcForSSE) DeleteVisit(_ context.Context, _ int64) error              { return nil }
func (m *mockActiveSvcForSSE) ListVisits(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindVisitsByStudentID(_ context.Context, _ int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindVisitsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindVisitsByTimeRange(_ context.Context, _, _ time.Time) ([]*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndVisit(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) GetStudentCurrentVisit(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetStudentCurrentVisitWithRoom(_ context.Context, _ int64) (*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetStudentsCurrentVisits(_ context.Context, _ []int64) (map[int64]*activeModel.Visit, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CountActiveVisitsByRoomID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockActiveSvcForSSE) CountActiveVisitsByActiveGroupID(_ context.Context, _ int64) (int, error) {
	return 0, nil
}
func (m *mockActiveSvcForSSE) ListStudentsPresentInRoom(_ context.Context, _ int64) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ListStudentsInTransit(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) AssignTransitStudentsToActiveGroup(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetGroupSupervisor(_ context.Context, _ int64) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CreateGroupSupervisor(_ context.Context, _ *activeModel.GroupSupervisor) error {
	return nil
}
func (m *mockActiveSvcForSSE) UpdateGroupSupervisor(_ context.Context, _ *activeModel.GroupSupervisor) error {
	return nil
}
func (m *mockActiveSvcForSSE) DeleteGroupSupervisor(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) ListGroupSupervisors(_ context.Context, _ *base.QueryOptions) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindSupervisorsByStaffID(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindSupervisorsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindSupervisorsByActiveGroupIDs(_ context.Context, _ []int64) ([]*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndSupervision(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) GetCombinedGroup(_ context.Context, _ int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CreateCombinedGroup(_ context.Context, _ *activeModel.CombinedGroup) error {
	return nil
}
func (m *mockActiveSvcForSSE) UpdateCombinedGroup(_ context.Context, _ *activeModel.CombinedGroup) error {
	return nil
}
func (m *mockActiveSvcForSSE) DeleteCombinedGroup(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) ListCombinedGroups(_ context.Context, _ *base.QueryOptions) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindActiveCombinedGroups(_ context.Context) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) FindCombinedGroupsByTimeRange(_ context.Context, _, _ time.Time) ([]*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndCombinedGroup(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) GetCombinedGroupWithGroups(_ context.Context, _ int64) (*activeModel.CombinedGroup, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CreateCombinedGroupWithGroups(_ context.Context, _ *activeModel.CombinedGroup, _ []int64) error {
	return nil
}
func (m *mockActiveSvcForSSE) AddGroupToCombination(_ context.Context, _, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) RemoveGroupFromCombination(_ context.Context, _, _ int64) error {
	return nil
}
func (m *mockActiveSvcForSSE) GetGroupMappingsByActiveGroupID(_ context.Context, _ int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetGroupMappingsByCombinedGroupID(_ context.Context, _ int64) ([]*activeModel.GroupMapping, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) StartActivitySession(_ context.Context, _, _, _ int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) StartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckActivityConflict(_ context.Context, _, _ int64) (*activeSvc.ActivityConflictInfo, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndActivitySession(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) ForceStartActivitySession(_ context.Context, _, _, _ int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ForceStartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetDeviceCurrentSession(_ context.Context, _ int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) UpdateActiveGroupSupervisors(_ context.Context, _ int64, _ []int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ProcessSessionTimeout(_ context.Context, _ int64) (*activeSvc.TimeoutResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) UpdateSessionActivity(_ context.Context, _ int64) error { return nil }
func (m *mockActiveSvcForSSE) ValidateSessionTimeout(_ context.Context, _ int64, _ int) error {
	return nil
}
func (m *mockActiveSvcForSSE) GetSessionTimeoutInfo(_ context.Context, _ int64) (*activeSvc.SessionTimeoutInfo, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CleanupAbandonedSessions(_ context.Context, _ time.Duration) (int, error) {
	return 0, nil
}
func (m *mockActiveSvcForSSE) EndDailySessions(_ context.Context) (*activeSvc.DailySessionCleanupResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetDashboardAnalytics(_ context.Context) (*activeSvc.DashboardAnalytics, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetActiveGroupsByIDs(_ context.Context, _ []int64) (map[int64]*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetStudentAttendanceStatus(_ context.Context, _ int64) (*activeSvc.AttendanceStatus, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetStudentsAttendanceStatuses(_ context.Context, _ []int64) (map[int64]*activeSvc.AttendanceStatus, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ToggleStudentAttendance(_ context.Context, _, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckInStudent(_ context.Context, _, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckOutStudent(_ context.Context, _, _ int64, _ bool) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckTeacherStudentAccess(_ context.Context, _, _ int64) (bool, error) {
	return false, nil
}
func (m *mockActiveSvcForSSE) BroadcastDailyCheckout(_ context.Context, _ int64) {}
func (m *mockActiveSvcForSSE) GetUnclaimedActiveGroups(_ context.Context) ([]*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ClaimActiveGroup(_ context.Context, _, _ int64, _ string) (*activeModel.GroupSupervisor, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) GetCrossTenantStudents(_ context.Context, _ int64) ([]activeModel.CrossTenantStudent, error) {
	return nil, nil
}

// =============================================================================
// HELPER
// =============================================================================

func ctxWithClaims(isAdmin bool) context.Context {
	claims := jwt.AppClaims{
		ID:       42,
		IsAdmin:  isAdmin,
		TenantID: 10,
	}
	return context.WithValue(context.Background(), jwt.CtxClaims, claims)
}

// =============================================================================
// TESTS: resolveSupervisions
// =============================================================================

func TestResolveSupervisions_AdminWithSettingEnabled(t *testing.T) {
	// Admin SSE path now enumerates active.groups directly so unclaimed groups
	// still receive live events. Synthetic GroupSupervisor entries are built
	// from the list.
	now := time.Now()
	activeGroups := []*activeModel.Group{
		{Model: base.Model{ID: 10}, StartTime: now.Add(-time.Hour)},
		{Model: base.Model{ID: 11}, StartTime: now.Add(-time.Hour)},
	}

	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}},
		activeSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return activeGroups, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(10), result[0].GroupID)
	assert.Equal(t, int64(11), result[1].GroupID)
}

func TestResolveSupervisions_AdminWithSettingDisabled(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 200}, GroupID: 20, StaffID: 42},
	}

	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}},
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
				assert.Equal(t, int64(42), staffID)
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(20), result[0].GroupID)
}

func TestResolveSupervisions_NonAdmin(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 300}, GroupID: 30, StaffID: 42},
	}

	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: true, // enabled but user is not admin
		}},
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(false)
	result, err := rs.resolveSupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_NilSettingsService(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 400}, GroupID: 40, StaffID: 42},
	}

	rs := &Resource{
		settingsSvc: nil,
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true) // admin but no settings service
	result, err := rs.resolveSupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_SettingErrorFallsBack(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 500}, GroupID: 50, StaffID: 42},
	}

	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{}}, // key missing → error
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_StaffSupervisionsError(t *testing.T) {
	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}},
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return nil, fmt.Errorf("database connection lost")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true) // admin but setting disabled → falls back to staff
	result, err := rs.resolveSupervisions(ctx, 42)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestResolveSupervisions_NonAdminStaffError(t *testing.T) {
	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{}},
		activeSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return nil, fmt.Errorf("timeout")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(false) // non-admin
	result, err := rs.resolveSupervisions(ctx, 42)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestResolveSupervisions_GetAllError(t *testing.T) {
	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}},
		activeSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return nil, fmt.Errorf("database error")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSupervisions(ctx, 42)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestResolveSupervisions_AdminIncludesUnclaimedGroups verifies that active
// groups without supervisor rows (e.g. Schulhof without a current claim) are
// included in the SSE topic list — closing the prior divergence between
// HTTP (/supervisors/all → ListActiveGroups) and SSE (→ FindAllActive).
func TestResolveSupervisions_AdminIncludesUnclaimedGroups(t *testing.T) {
	now := time.Now()
	// Two active groups, including one that would have been missed by the
	// previous GetAllActiveSupervisions path (no supervisor row).
	activeGroups := []*activeModel.Group{
		{Model: base.Model{ID: 50}, StartTime: now.Add(-time.Hour)},
		{Model: base.Model{ID: 51}, StartTime: now.Add(-time.Minute)},
	}

	rs := &Resource{
		settingsSvc: &mockSettingsSvc{boolValues: map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}},
		activeSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return activeGroups, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSupervisions(ctx, 42)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, int64(50), result[0].GroupID)
	assert.Equal(t, int64(51), result[1].GroupID)
}
