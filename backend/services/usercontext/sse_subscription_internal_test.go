package usercontext

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	facilityModels "github.com/moto-nrw/project-phoenix/models/facilities"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
// MOCK: Minimal ActiveService for SSE tests
// =============================================================================

type mockActiveSvcForSSE struct {
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

func (m *mockActiveSvcForSSE) ConfirmDailyCheckout(_ context.Context, _, _ int64, _ string) (*activeSvc.DailyCheckoutResult, error) {
	return nil, nil
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
func (m *mockActiveSvcForSSE) ListStudentsPresentToday(_ context.Context) ([]int64, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) AssignTransitStudentsToActiveGroup(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) MoveStudentsToActiveGroupAuthorized(_ context.Context, _ []int64, _ int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) MoveStudentsToTransitAuthorized(_ context.Context, _ []int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
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
func (m *mockActiveSvcForSSE) StartActivitySessionWithSupervisors(_ context.Context, _, _ int64, _ []int64, _ *int64) (*activeModel.Group, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckActivityConflict(_ context.Context, _, _ int64) (*activeSvc.ActivityConflictInfo, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) EndActivitySession(_ context.Context, _ int64) error { return nil }
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
func (m *mockActiveSvcForSSE) CheckOutStudentFromDevice(_ context.Context, _, _ int64) (*activeSvc.AttendanceResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) ProcessSchoolCheckinBatch(_ context.Context, _ []int64, _ int64, _ string) (*activeSvc.SchoolCheckinBatchResult, error) {
	return nil, nil
}
func (m *mockActiveSvcForSSE) CheckTeacherStudentAccess(_ context.Context, _, _ int64) (bool, error) {
	return false, nil
}
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

func ctxWithClaimsAndPermissions(isAdmin bool, permissions ...string) context.Context {
	ctx := ctxWithClaims(isAdmin)
	return context.WithValue(ctx, jwt.CtxPermissions, permissions)
}

type ssePersonRepoStub struct {
	userModels.PersonRepository
	person *userModels.Person
}

func (s *ssePersonRepoStub) FindByAccountID(_ context.Context, _ int64) (*userModels.Person, error) {
	return s.person, nil
}

type sseStaffRepoStub struct {
	userModels.StaffRepository
}

func (s *sseStaffRepoStub) FindByPersonID(_ context.Context, _ int64) (*userModels.Staff, error) {
	return nil, nil
}

// =============================================================================
// TESTS: resolveSupervisions
// =============================================================================

func TestResolveSSESubscription_WildcardAdminWithoutStaff(t *testing.T) {
	for _, permission := range []string{"admin:*", "*:*"} {
		t.Run(permission, func(t *testing.T) {
			rs := &userContextService{
				personRepo: &ssePersonRepoStub{
					person: &userModels.Person{Model: base.Model{ID: 99}},
				},
				staffRepo: &sseStaffRepoStub{},
				sseSettings: mockSettingsSvc(map[string]bool{
					configModel.KeyAdminSupervisionOverview: true,
				}),
				sseActiveSvc: &mockActiveSvcForSSE{
					listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
						return nil, nil
					},
				},
				logger: slog.Default(),
			}

			subscription, err := rs.ResolveSSESubscription(
				ctxWithClaimsAndPermissions(false, permission),
			)

			require.NoError(t, err)
			assert.Zero(t, subscription.StaffID)
			assert.Empty(t, subscription.AllTopics)
		})
	}
}

func TestResolveSupervisions_AdminWithSettingEnabled(t *testing.T) {
	// Admin SSE path now enumerates active.groups directly so unclaimed groups
	// still receive live events. Synthetic GroupSupervisor entries are built
	// from the list.
	now := time.Now()
	activeGroups := []*activeModel.Group{
		{Model: base.Model{ID: 10}, StartTime: now.Add(-time.Hour)},
		{Model: base.Model{ID: 11}, StartTime: now.Add(-time.Hour)},
	}

	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return activeGroups, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSSESupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, int64(10), result[0].GroupID)
	assert.Equal(t, int64(11), result[1].GroupID)
}

func TestResolveSupervisions_AdminWithSettingDisabled(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 200}, GroupID: 20, StaffID: 42},
	}

	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, staffID int64) ([]*activeModel.GroupSupervisor, error) {
				assert.Equal(t, int64(42), staffID)
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSSESupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, int64(20), result[0].GroupID)
}

func TestResolveSupervisions_NonAdmin(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 300}, GroupID: 30, StaffID: 42},
	}

	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true, // enabled but user is not admin
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(false)
	result, err := rs.resolveSSESupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_NilSettingsService(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 400}, GroupID: 40, StaffID: 42},
	}

	rs := &userContextService{
		sseSettings: nil,
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true) // admin but no settings service
	result, err := rs.resolveSSESupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_SettingErrorFallsBack(t *testing.T) {
	staffSupervisions := []*activeModel.GroupSupervisor{
		{Model: base.Model{ID: 500}, GroupID: 50, StaffID: 42},
	}

	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{}), // key missing → error
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return staffSupervisions, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSSESupervisions(ctx, 42)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestResolveSupervisions_NilActiveServiceReturnsError(t *testing.T) {
	tests := []struct {
		name     string
		isAdmin  bool
		settings SSESettingsResolver
	}{
		{
			name:    "non-admin",
			isAdmin: false,
		},
		{
			name:    "admin without settings service",
			isAdmin: true,
		},
		{
			name:    "admin with overview disabled",
			isAdmin: true,
			settings: mockSettingsSvc(map[string]bool{
				configModel.KeyAdminSupervisionOverview: false,
			}),
		},
		{
			name:     "admin with setting error",
			isAdmin:  true,
			settings: mockSettingsSvc(map[string]bool{}),
		},
		{
			name:    "admin with overview enabled",
			isAdmin: true,
			settings: mockSettingsSvc(map[string]bool{
				configModel.KeyAdminSupervisionOverview: true,
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rs := &userContextService{
				sseSettings: tt.settings,
				logger:      slog.Default(),
			}

			result, err := rs.resolveSSESupervisions(ctxWithClaims(tt.isAdmin), 42)

			require.ErrorContains(t, err, "SSE active service is not configured")
			assert.Nil(t, result)
		})
	}
}

func TestResolveSupervisions_StaffSupervisionsError(t *testing.T) {
	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: false,
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return nil, fmt.Errorf("database connection lost")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true) // admin but setting disabled → falls back to staff
	result, err := rs.resolveSSESupervisions(ctx, 42)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestResolveSupervisions_NonAdminStaffError(t *testing.T) {
	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{}),
		sseActiveSvc: &mockActiveSvcForSSE{
			getStaffFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
				return nil, fmt.Errorf("timeout")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(false) // non-admin
	result, err := rs.resolveSSESupervisions(ctx, 42)

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestResolveSupervisions_GetAllError(t *testing.T) {
	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return nil, fmt.Errorf("database error")
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSSESupervisions(ctx, 42)

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

	rs := &userContextService{
		sseSettings: mockSettingsSvc(map[string]bool{
			configModel.KeyAdminSupervisionOverview: true,
		}),
		sseActiveSvc: &mockActiveSvcForSSE{
			listFunc: func(_ context.Context, _ *base.QueryOptions) ([]*activeModel.Group, error) {
				return activeGroups, nil
			},
		},
		logger: slog.Default(),
	}

	ctx := ctxWithClaims(true)
	result, err := rs.resolveSSESupervisions(ctx, 42)
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, int64(50), result[0].GroupID)
	assert.Equal(t, int64(51), result[1].GroupID)
}

// =============================================================================
// TESTS: SSESetupError
// =============================================================================

func TestSSESetupError_ErrorMessage(t *testing.T) {
	err := &SSESetupError{Message: "Account not found", Status: http.StatusUnauthorized}

	assert.Equal(t, "SSE setup: Account not found", err.Error())
	assert.Equal(t, http.StatusUnauthorized, err.Status)
}

func TestSSESetupError_AsMatchesTypedError(t *testing.T) {
	var err error = &SSESetupError{Message: "forbidden", Status: http.StatusForbidden}

	var setupErr *SSESetupError
	require.True(t, errors.As(err, &setupErr), "Should match *SSESetupError via errors.As")
	assert.Equal(t, "forbidden", setupErr.Message)
	assert.Equal(t, http.StatusForbidden, setupErr.Status)
}

func TestSSESetupError_AsDistinguishesErrors(t *testing.T) {
	var setupErr *SSESetupError
	assert.False(t, errors.As(assert.AnError, &setupErr),
		"Regular error should not match *SSESetupError")
}
