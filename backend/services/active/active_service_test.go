package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockGroupSupervisorRepository for testing
type MockGroupSupervisorRepository struct {
	mock.Mock
}

func (m *MockGroupSupervisorRepository) FindByID(ctx context.Context, id interface{}) (*active.GroupSupervisor, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) Create(ctx context.Context, entity *active.GroupSupervisor) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockGroupSupervisorRepository) Update(ctx context.Context, entity *active.GroupSupervisor) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockGroupSupervisorRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupSupervisorRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) FindByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, activeGroupID, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, activeGroupIDs, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	args := m.Called(ctx, staffID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) FindActiveByStaffIDForGroup(ctx context.Context, staffID, activeGroupID int64) (*active.GroupSupervisor, error) {
	args := m.Called(ctx, staffID, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.GroupSupervisor), args.Error(1)
}

func (m *MockGroupSupervisorRepository) EndAllSupervisionsForGroup(ctx context.Context, activeGroupID int64) error {
	args := m.Called(ctx, activeGroupID)
	return args.Error(0)
}

// MockCombinedGroupRepository for testing
type MockCombinedGroupRepository struct {
	mock.Mock
}

func (m *MockCombinedGroupRepository) FindByID(ctx context.Context, id interface{}) (*active.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.CombinedGroup), args.Error(1)
}

func (m *MockCombinedGroupRepository) Create(ctx context.Context, entity *active.CombinedGroup) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockCombinedGroupRepository) Update(ctx context.Context, entity *active.CombinedGroup) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockCombinedGroupRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCombinedGroupRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *MockCombinedGroupRepository) FindActive(ctx context.Context) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *MockCombinedGroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.CombinedGroup, error) {
	args := m.Called(ctx, start, end)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.CombinedGroup), args.Error(1)
}

func (m *MockCombinedGroupRepository) EndCombinedGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockCombinedGroupRepository) FindWithGroups(ctx context.Context, id int64) (*active.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.CombinedGroup), args.Error(1)
}

// MockGroupMappingRepository for testing
type MockGroupMappingRepository struct {
	mock.Mock
}

func (m *MockGroupMappingRepository) FindByID(ctx context.Context, id interface{}) (*active.GroupMapping, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.GroupMapping), args.Error(1)
}

func (m *MockGroupMappingRepository) Create(ctx context.Context, entity *active.GroupMapping) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockGroupMappingRepository) Update(ctx context.Context, entity *active.GroupMapping) error {
	args := m.Called(ctx, entity)
	return args.Error(0)
}

func (m *MockGroupMappingRepository) Delete(ctx context.Context, id interface{}) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupMappingRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.GroupMapping, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupMapping), args.Error(1)
}

func (m *MockGroupMappingRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.GroupMapping, error) {
	args := m.Called(ctx, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupMapping), args.Error(1)
}

func (m *MockGroupMappingRepository) FindByCombinedGroupID(ctx context.Context, combinedGroupID int64) ([]*active.GroupMapping, error) {
	args := m.Called(ctx, combinedGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.GroupMapping), args.Error(1)
}

func (m *MockGroupMappingRepository) ExistsByPair(ctx context.Context, combinedGroupID, activeGroupID int64) (bool, error) {
	args := m.Called(ctx, combinedGroupID, activeGroupID)
	return args.Bool(0), args.Error(1)
}

func (m *MockGroupMappingRepository) DeleteByPair(ctx context.Context, combinedGroupID, activeGroupID int64) error {
	args := m.Called(ctx, combinedGroupID, activeGroupID)
	return args.Error(0)
}

// Test GetActiveGroup
func TestGetActiveGroup(t *testing.T) {
	tests := []struct {
		name          string
		groupID       int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		errorContains string
	}{
		{
			name:    "successful retrieval",
			groupID: 1,
			setupMock: func(repo *SimpleGroupRepo) {
				group := &active.Group{
					Model:     base.Model{ID: 1},
					GroupID:   100,
					StartTime: time.Now(),
				}
				repo.On("FindByID", mock.Anything, int64(1)).Return(group, nil)
			},
			expectError: false,
		},
		{
			name:    "group not found",
			groupID: 999,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindByID", mock.Anything, int64(999)).Return(nil, ErrActiveGroupNotFound)
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:    "database error",
			groupID: 1,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindByID", mock.Anything, int64(1)).Return(nil, errors.New("database connection failed"))
			},
			expectError:   true,
			errorContains: "not found", // Service wraps all errors as ErrActiveGroupNotFound
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.GetActiveGroup(context.Background(), tt.groupID)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test GetActiveGroupsByIDs
func TestGetActiveGroupsByIDs(t *testing.T) {
	tests := []struct {
		name          string
		groupIDs      []int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		expectedCount int
	}{
		{
			name:     "successful retrieval of multiple groups",
			groupIDs: []int64{1, 2, 3},
			setupMock: func(repo *SimpleGroupRepo) {
				result := map[int64]*active.Group{
					1: {Model: base.Model{ID: 1}},
					2: {Model: base.Model{ID: 2}},
					3: {Model: base.Model{ID: 3}},
				}
				repo.On("FindByIDs", mock.Anything, []int64{1, 2, 3}).Return(result, nil)
			},
			expectError:   false,
			expectedCount: 3,
		},
		{
			name:     "empty IDs returns empty map",
			groupIDs: []int64{},
			setupMock: func(repo *SimpleGroupRepo) {
				// Service returns early for empty slice, no repo call expected
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:     "database error",
			groupIDs: []int64{1},
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindByIDs", mock.Anything, []int64{1}).Return(nil, errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.GetActiveGroupsByIDs(context.Background(), tt.groupIDs)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, result)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test ListActiveGroups
func TestListActiveGroups(t *testing.T) {
	tests := []struct {
		name          string
		options       *base.QueryOptions
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		expectedCount int
	}{
		{
			name:    "successful list",
			options: nil,
			setupMock: func(repo *SimpleGroupRepo) {
				groups := []*active.Group{
					{Model: base.Model{ID: 1}},
					{Model: base.Model{ID: 2}},
				}
				repo.On("List", mock.Anything, (*base.QueryOptions)(nil)).Return(groups, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
		{
			name:    "empty list",
			options: nil,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("List", mock.Anything, (*base.QueryOptions)(nil)).Return([]*active.Group{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.ListActiveGroups(context.Background(), tt.options)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test FindActiveGroupsByRoomID
func TestFindActiveGroupsByRoomID(t *testing.T) {
	tests := []struct {
		name          string
		roomID        int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		expectedCount int
	}{
		{
			name:   "finds groups in room",
			roomID: 100,
			setupMock: func(repo *SimpleGroupRepo) {
				groups := []*active.Group{
					{Model: base.Model{ID: 1}},
				}
				repo.On("FindActiveByRoomID", mock.Anything, int64(100)).Return(groups, nil)
			},
			expectError:   false,
			expectedCount: 1,
		},
		{
			name:   "no groups in room",
			roomID: 200,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindActiveByRoomID", mock.Anything, int64(200)).Return([]*active.Group{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.FindActiveGroupsByRoomID(context.Background(), tt.roomID)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test FindActiveGroupsByGroupID
func TestFindActiveGroupsByGroupID(t *testing.T) {
	tests := []struct {
		name          string
		groupID       int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		expectedCount int
	}{
		{
			name:    "finds active groups by education group ID",
			groupID: 50,
			setupMock: func(repo *SimpleGroupRepo) {
				groups := []*active.Group{
					{Model: base.Model{ID: 1}, GroupID: 50},
					{Model: base.Model{ID: 2}, GroupID: 50},
				}
				repo.On("FindActiveByGroupID", mock.Anything, int64(50)).Return(groups, nil)
			},
			expectError:   false,
			expectedCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.FindActiveGroupsByGroupID(context.Background(), tt.groupID)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test FindActiveGroupsByTimeRange
func TestFindActiveGroupsByTimeRange(t *testing.T) {
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now

	tests := []struct {
		name          string
		start         time.Time
		end           time.Time
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		errorContains string
	}{
		{
			name:  "successful time range query",
			start: start,
			end:   end,
			setupMock: func(repo *SimpleGroupRepo) {
				groups := []*active.Group{
					{Model: base.Model{ID: 1}, StartTime: now.Add(-12 * time.Hour)},
				}
				repo.On("FindByTimeRange", mock.Anything, mock.Anything, mock.Anything).Return(groups, nil)
			},
			expectError: false,
		},
		{
			name:  "invalid time range - end before start",
			start: end,
			end:   start,
			setupMock: func(repo *SimpleGroupRepo) {
				// Should fail before calling repo
			},
			expectError:   true,
			errorContains: "invalid time range",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.FindActiveGroupsByTimeRange(context.Background(), tt.start, tt.end)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}
		})
	}
}

// Test GetActiveGroupWithVisits
func TestGetActiveGroupWithVisits(t *testing.T) {
	tests := []struct {
		name          string
		groupID       int64
		setupMock     func(*SimpleGroupRepo, *MockVisitRepository)
		expectError   bool
		errorContains string
	}{
		{
			name:    "successful retrieval with visits",
			groupID: 1,
			setupMock: func(groupRepo *SimpleGroupRepo, visitRepo *MockVisitRepository) {
				group := &active.Group{
					Model:     base.Model{ID: 1},
					GroupID:   100,
					StartTime: time.Now(),
				}
				groupRepo.On("FindByID", mock.Anything, int64(1)).Return(group, nil)
				visitRepo.On("FindByActiveGroupID", mock.Anything, int64(1)).Return([]*active.Visit{
					{Model: base.Model{ID: 1}, StudentID: 100},
				}, nil)
			},
			expectError: false,
		},
		{
			name:    "group not found",
			groupID: 999,
			setupMock: func(groupRepo *SimpleGroupRepo, visitRepo *MockVisitRepository) {
				groupRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, ErrActiveGroupNotFound)
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupRepo := &SimpleGroupRepo{}
			visitRepo := &MockVisitRepository{}
			tt.setupMock(groupRepo, visitRepo)

			svc := &service{
				groupRepo: groupRepo,
				visitRepo: visitRepo,
			}

			result, err := svc.GetActiveGroupWithVisits(context.Background(), tt.groupID)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			groupRepo.AssertExpectations(t)
		})
	}
}

// Test GetActiveGroupWithSupervisors
func TestGetActiveGroupWithSupervisors(t *testing.T) {
	tests := []struct {
		name          string
		groupID       int64
		setupMock     func(*SimpleGroupRepo, *MockGroupSupervisorRepository)
		expectError   bool
		errorContains string
	}{
		{
			name:    "successful retrieval with supervisors",
			groupID: 1,
			setupMock: func(groupRepo *SimpleGroupRepo, supervisorRepo *MockGroupSupervisorRepository) {
				group := &active.Group{
					Model:     base.Model{ID: 1},
					GroupID:   100,
					StartTime: time.Now(),
				}
				groupRepo.On("FindByID", mock.Anything, int64(1)).Return(group, nil)
				supervisorRepo.On("FindByActiveGroupID", mock.Anything, int64(1), true).Return([]*active.GroupSupervisor{
					{Model: base.Model{ID: 1}, StaffID: 100},
				}, nil)
			},
			expectError: false,
		},
		{
			name:    "group not found",
			groupID: 999,
			setupMock: func(groupRepo *SimpleGroupRepo, supervisorRepo *MockGroupSupervisorRepository) {
				groupRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, ErrActiveGroupNotFound)
			},
			expectError:   true,
			errorContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupRepo := &SimpleGroupRepo{}
			supervisorRepo := &MockGroupSupervisorRepository{}
			tt.setupMock(groupRepo, supervisorRepo)

			svc := &service{
				groupRepo:      groupRepo,
				supervisorRepo: supervisorRepo,
			}

			result, err := svc.GetActiveGroupWithSupervisors(context.Background(), tt.groupID)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			groupRepo.AssertExpectations(t)
		})
	}
}

// Test GetDeviceCurrentSession
func TestGetDeviceCurrentSession(t *testing.T) {
	tests := []struct {
		name          string
		deviceID      int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		errorContains string
	}{
		{
			name:     "successful session retrieval",
			deviceID: 1,
			setupMock: func(repo *SimpleGroupRepo) {
				deviceID := int64(1)
				group := &active.Group{
					Model:          base.Model{ID: 123},
					DeviceID:       &deviceID,
					StartTime:      time.Now().Add(-10 * time.Minute),
					TimeoutMinutes: 30,
				}
				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(1)).Return(group, nil)
			},
			expectError: false,
		},
		{
			name:     "no active session",
			deviceID: 2,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(2)).Return(nil, ErrNoActiveSession)
			},
			expectError:   true,
			errorContains: "no active session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			result, err := svc.GetDeviceCurrentSession(context.Background(), tt.deviceID)

			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
				require.NotNil(t, result)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test CleanupAbandonedSessions
func TestCleanupAbandonedSessions(t *testing.T) {
	tests := []struct {
		name          string
		olderThan     time.Duration
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		expectedCount int
	}{
		{
			name:      "no abandoned sessions",
			olderThan: 2 * time.Hour,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindActiveSessionsOlderThan", mock.Anything, mock.Anything).Return([]*active.Group{}, nil)
			},
			expectError:   false,
			expectedCount: 0,
		},
		{
			name:      "database error",
			olderThan: 2 * time.Hour,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindActiveSessionsOlderThan", mock.Anything, mock.Anything).Return(nil, errors.New("db error"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			svc := &service{
				groupRepo: repo,
			}

			count, err := svc.CleanupAbandonedSessions(context.Background(), tt.olderThan)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count)
			}

			repo.AssertExpectations(t)
		})
	}
}

// Test visit operations
func TestGetVisit(t *testing.T) {
	t.Run("successful retrieval", func(t *testing.T) {
		visitRepo := &MockVisitRepository{}
		visit := &active.Visit{
			Model:         base.Model{ID: 1},
			StudentID:     100,
			ActiveGroupID: 200,
		}
		visitRepo.On("FindByID", mock.Anything, int64(1)).Return(visit, nil)

		svc := &service{
			visitRepo: visitRepo,
		}

		result, err := svc.GetVisit(context.Background(), 1)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(1), result.ID)
	})

	t.Run("visit not found", func(t *testing.T) {
		visitRepo := &MockVisitRepository{}
		visitRepo.On("FindByID", mock.Anything, int64(999)).Return(nil, ErrVisitNotFound)

		svc := &service{
			visitRepo: visitRepo,
		}

		result, err := svc.GetVisit(context.Background(), 999)

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "not found")
	})
}

// Test ListVisits
func TestListVisits(t *testing.T) {
	visitRepo := &MockVisitRepository{}
	visitRepo.On("List", mock.Anything, (*base.QueryOptions)(nil)).Return([]*active.Visit{
		{Model: base.Model{ID: 1}},
		{Model: base.Model{ID: 2}},
	}, nil)

	svc := &service{
		visitRepo: visitRepo,
	}

	result, err := svc.ListVisits(context.Background(), nil)

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

// Test FindVisitsByStudentID
func TestFindVisitsByStudentID(t *testing.T) {
	visitRepo := &MockVisitRepository{}
	visitRepo.On("FindActiveByStudentID", mock.Anything, int64(100)).Return([]*active.Visit{
		{Model: base.Model{ID: 1}, StudentID: 100},
	}, nil)

	svc := &service{
		visitRepo: visitRepo,
	}

	result, err := svc.FindVisitsByStudentID(context.Background(), 100)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// Test FindVisitsByActiveGroupID
func TestFindVisitsByActiveGroupID(t *testing.T) {
	visitRepo := &MockVisitRepository{}
	visitRepo.On("FindByActiveGroupID", mock.Anything, int64(200)).Return([]*active.Visit{
		{Model: base.Model{ID: 1}, ActiveGroupID: 200},
		{Model: base.Model{ID: 2}, ActiveGroupID: 200},
	}, nil)

	svc := &service{
		visitRepo: visitRepo,
	}

	result, err := svc.FindVisitsByActiveGroupID(context.Background(), 200)

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

// Test FindVisitsByTimeRange
func TestFindVisitsByTimeRange(t *testing.T) {
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now

	t.Run("successful time range query", func(t *testing.T) {
		visitRepo := &MockVisitRepository{}
		visitRepo.On("FindByTimeRange", mock.Anything, mock.Anything, mock.Anything).Return([]*active.Visit{
			{Model: base.Model{ID: 1}},
		}, nil)

		svc := &service{
			visitRepo: visitRepo,
		}

		result, err := svc.FindVisitsByTimeRange(context.Background(), start, end)

		require.NoError(t, err)
		require.NotNil(t, result)
		assert.Len(t, result, 1)
	})

	t.Run("invalid time range", func(t *testing.T) {
		visitRepo := &MockVisitRepository{}

		svc := &service{
			visitRepo: visitRepo,
		}

		_, err := svc.FindVisitsByTimeRange(context.Background(), end, start) // reversed

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid time range")
	})
}

// Test GetStudentCurrentVisit
func TestGetStudentCurrentVisit(t *testing.T) {
	visitRepo := &MockVisitRepository{}
	visitRepo.On("FindActiveByStudentID", mock.Anything, int64(100)).Return([]*active.Visit{
		{Model: base.Model{ID: 1}, StudentID: 100},
	}, nil)

	svc := &service{
		visitRepo: visitRepo,
	}

	result, err := svc.GetStudentCurrentVisit(context.Background(), 100)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, int64(100), result.StudentID)
}

// Test GetStudentsCurrentVisits
func TestGetStudentsCurrentVisits(t *testing.T) {
	visitRepo := &MockVisitRepository{}
	visitRepo.On("GetCurrentByStudentIDs", mock.Anything, []int64{100, 101}).Return(map[int64]*active.Visit{
		100: {Model: base.Model{ID: 1}, StudentID: 100},
		101: {Model: base.Model{ID: 2}, StudentID: 101},
	}, nil)

	svc := &service{
		visitRepo: visitRepo,
	}

	result, err := svc.GetStudentsCurrentVisits(context.Background(), []int64{100, 101})

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

// Test supervisor operations
func TestGetGroupSupervisor(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("FindByID", mock.Anything, int64(1)).Return(&active.GroupSupervisor{
		Model:   base.Model{ID: 1},
		StaffID: 100,
		GroupID: 200,
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.GetGroupSupervisor(context.Background(), 1)

	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestListGroupSupervisors(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("List", mock.Anything, (*base.QueryOptions)(nil)).Return([]*active.GroupSupervisor{
		{Model: base.Model{ID: 1}},
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.ListGroupSupervisors(context.Background(), nil)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFindSupervisorsByStaffID(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("FindActiveByStaffID", mock.Anything, int64(100)).Return([]*active.GroupSupervisor{
		{Model: base.Model{ID: 1}, StaffID: 100},
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.FindSupervisorsByStaffID(context.Background(), 100)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}

func TestFindSupervisorsByActiveGroupID(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("FindByActiveGroupID", mock.Anything, int64(200), true).Return([]*active.GroupSupervisor{
		{Model: base.Model{ID: 1}, GroupID: 200},
		{Model: base.Model{ID: 2}, GroupID: 200},
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.FindSupervisorsByActiveGroupID(context.Background(), 200)

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestFindSupervisorsByActiveGroupIDs(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("FindByActiveGroupIDs", mock.Anything, []int64{200, 201}, true).Return([]*active.GroupSupervisor{
		{Model: base.Model{ID: 1}, GroupID: 200},
		{Model: base.Model{ID: 2}, GroupID: 201},
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.FindSupervisorsByActiveGroupIDs(context.Background(), []int64{200, 201})

	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestEndSupervision(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("EndSupervision", mock.Anything, int64(1)).Return(nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	err := svc.EndSupervision(context.Background(), 1)

	require.NoError(t, err)
	supervisorRepo.AssertExpectations(t)
}

func TestGetStaffActiveSupervisions(t *testing.T) {
	supervisorRepo := &MockGroupSupervisorRepository{}
	supervisorRepo.On("FindActiveByStaffID", mock.Anything, int64(100)).Return([]*active.GroupSupervisor{
		{Model: base.Model{ID: 1}, StaffID: 100},
	}, nil)

	svc := &service{
		supervisorRepo: supervisorRepo,
	}

	result, err := svc.GetStaffActiveSupervisions(context.Background(), 100)

	require.NoError(t, err)
	assert.Len(t, result, 1)
}
