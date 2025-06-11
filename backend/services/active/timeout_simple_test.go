package active

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Simple mock for testing core logic without transaction complexity
type SimpleGroupRepo struct {
	mock.Mock
}

func (m *SimpleGroupRepo) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	args := m.Called(mock.Anything, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *SimpleGroupRepo) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	args := m.Called(mock.Anything, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *SimpleGroupRepo) FindByID(ctx context.Context, id interface{}) (*active.Group, error) {
	args := m.Called(mock.Anything, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*active.Group), args.Error(1)
}

func (m *SimpleGroupRepo) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	args := m.Called(mock.Anything, id, mock.Anything)
	return args.Error(0)
}

func (m *SimpleGroupRepo) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	args := m.Called(mock.Anything, mock.Anything)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Group), args.Error(1)
}

// Stub methods for interface compliance
func (m *SimpleGroupRepo) Create(ctx context.Context, entity *active.Group) error { return nil }
func (m *SimpleGroupRepo) Update(ctx context.Context, entity *active.Group) error { return nil }
func (m *SimpleGroupRepo) Delete(ctx context.Context, id interface{}) error       { return nil }
func (m *SimpleGroupRepo) List(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) EndSession(ctx context.Context, id int64) error { return nil }
func (m *SimpleGroupRepo) FindBySourceIDs(ctx context.Context, sourceIDs []int64, sourceType string) ([]*active.Group, error) {
	return nil, nil
}
// FindWithRelations - missing method from interface
func (m *SimpleGroupRepo) FindWithRelations(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) CheckActivityDeviceConflict(ctx context.Context, activityID, excludeDeviceID int64) (bool, *active.Group, error) {
	return false, nil, nil
}
func (m *SimpleGroupRepo) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	return false, nil, nil
}
func (m *SimpleGroupRepo) FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}
func (m *SimpleGroupRepo) FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*active.Group, error) {
	return nil, nil
}

// Simple MockVisitRepository for testing
type MockVisitRepository struct {
	mock.Mock
}

func (m *MockVisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	args := m.Called(mock.Anything, activeGroupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*active.Visit), args.Error(1)
}

// Stub methods for interface compliance
func (m *MockVisitRepository) FindByID(ctx context.Context, id interface{}) (*active.Visit, error) {
	return nil, nil
}
func (m *MockVisitRepository) Create(ctx context.Context, entity *active.Visit) error { return nil }
func (m *MockVisitRepository) Update(ctx context.Context, entity *active.Visit) error { return nil }
func (m *MockVisitRepository) Delete(ctx context.Context, id interface{}) error       { return nil }
func (m *MockVisitRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	return nil, nil
}
func (m *MockVisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	return nil, nil
}
func (m *MockVisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	return nil, nil
}
func (m *MockVisitRepository) EndVisit(ctx context.Context, id int64) error { return nil }
func (m *MockVisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	return 0, nil
}
func (m *MockVisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	return 0, nil
}
func (m *MockVisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	return 0, nil
}
func (m *MockVisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	return nil, nil
}
func (m *MockVisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	return 0, nil
}

func TestUpdateSessionActivity_Simple(t *testing.T) {
	tests := []struct {
		name          string
		activeGroupID int64
		setupMock     func(*SimpleGroupRepo)
		expectError   bool
		errorContains string
	}{
		{
			name:          "successful activity update",
			activeGroupID: 123,
			setupMock: func(repo *SimpleGroupRepo) {
				activeGroup := &active.Group{
					Model:          base.Model{ID: 123},
					StartTime:      time.Now().Add(-10 * time.Minute),
					LastActivity:   time.Now().Add(-5 * time.Minute),
					TimeoutMinutes: 30,
				}

				repo.On("FindByID", mock.Anything, int64(123)).Return(activeGroup, nil)
				repo.On("UpdateLastActivity", mock.Anything, int64(123), mock.Anything).Return(nil)
			},
			expectError: false,
		},
		{
			name:          "session not found",
			activeGroupID: 456,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindByID", mock.Anything, int64(456)).Return(nil, ErrActiveGroupNotFound)
			},
			expectError:   true,
			errorContains: "not found",
		},
		{
			name:          "session already ended",
			activeGroupID: 789,
			setupMock: func(repo *SimpleGroupRepo) {
				endTime := time.Now()
				endedGroup := &active.Group{
					Model:          base.Model{ID: 789},
					StartTime:      time.Now().Add(-30 * time.Minute),
					EndTime:        &endTime,
					LastActivity:   time.Now().Add(-10 * time.Minute),
					TimeoutMinutes: 30,
				}

				repo.On("FindByID", mock.Anything, int64(789)).Return(endedGroup, nil)
			},
			expectError:   true,
			errorContains: "already ended",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			// Create service with minimal dependencies
			service := &service{
				groupRepo: repo,
			}

			// Execute test
			err := service.UpdateSessionActivity(context.Background(), tt.activeGroupID)

			// Assertions
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mock expectations
			repo.AssertExpectations(t)
		})
	}
}

func TestValidateSessionTimeout_Simple(t *testing.T) {
	tests := []struct {
		name           string
		deviceID       int64
		timeoutMinutes int
		setupMock      func(*SimpleGroupRepo)
		expectError    bool
		errorContains  string
	}{
		{
			name:           "valid timeout - session is timed out",
			deviceID:       1,
			timeoutMinutes: 30,
			setupMock: func(repo *SimpleGroupRepo) {
				activeGroup := &active.Group{
					Model:          base.Model{ID: 123},
					StartTime:      time.Now().Add(-45 * time.Minute),
					LastActivity:   time.Now().Add(-35 * time.Minute), // 35 minutes ago
					TimeoutMinutes: 30,
				}
				deviceID := int64(1)
				activeGroup.DeviceID = &deviceID

				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(1)).Return(activeGroup, nil)
			},
			expectError: false,
		},
		{
			name:           "invalid timeout - session not yet timed out",
			deviceID:       2,
			timeoutMinutes: 30,
			setupMock: func(repo *SimpleGroupRepo) {
				activeGroup := &active.Group{
					Model:          base.Model{ID: 456},
					StartTime:      time.Now().Add(-20 * time.Minute),
					LastActivity:   time.Now().Add(-10 * time.Minute), // Only 10 minutes ago
					TimeoutMinutes: 30,
				}
				deviceID := int64(2)
				activeGroup.DeviceID = &deviceID

				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(2)).Return(activeGroup, nil)
			},
			expectError:   true,
			errorContains: "not yet timed out",
		},
		{
			name:           "invalid timeout minutes - too high",
			deviceID:       3,
			timeoutMinutes: 500, // > 480 max
			setupMock: func(repo *SimpleGroupRepo) {
				activeGroup := &active.Group{
					Model:          base.Model{ID: 789},
					StartTime:      time.Now().Add(-45 * time.Minute),
					LastActivity:   time.Now().Add(-35 * time.Minute),
					TimeoutMinutes: 30,
				}
				deviceID := int64(3)
				activeGroup.DeviceID = &deviceID

				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(3)).Return(activeGroup, nil)
			},
			expectError:   true,
			errorContains: "invalid timeout minutes",
		},
		{
			name:           "no active session",
			deviceID:       4,
			timeoutMinutes: 30,
			setupMock: func(repo *SimpleGroupRepo) {
				repo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(4)).Return(nil, ErrNoActiveSession)
			},
			expectError:   true,
			errorContains: "no active session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock
			repo := &SimpleGroupRepo{}
			tt.setupMock(repo)

			// Create service with minimal dependencies
			service := &service{
				groupRepo: repo,
			}

			// Execute test
			err := service.ValidateSessionTimeout(context.Background(), tt.deviceID, tt.timeoutMinutes)

			// Assertions
			if tt.expectError {
				require.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				require.NoError(t, err)
			}

			// Verify mock expectations
			repo.AssertExpectations(t)
		})
	}
}

func TestGetSessionTimeoutInfo_Simple(t *testing.T) {
	tests := []struct {
		name       string
		deviceID   int64
		setupMock  func(*SimpleGroupRepo, *MockVisitRepository)
		expectInfo bool
	}{
		{
			name:     "successful timeout info retrieval",
			deviceID: 1,
			setupMock: func(groupRepo *SimpleGroupRepo, visitRepo *MockVisitRepository) {
				activeGroup := &active.Group{
					Model:          base.Model{ID: 123},
					GroupID:        456,
					StartTime:      time.Now().Add(-20 * time.Minute),
					LastActivity:   time.Now().Add(-10 * time.Minute),
					TimeoutMinutes: 30,
				}
				deviceID := int64(1)
				activeGroup.DeviceID = &deviceID

				groupRepo.On("FindActiveByDeviceIDWithNames", mock.Anything, int64(1)).Return(activeGroup, nil)

				// Mock active visits
				activeVisits := []*active.Visit{
					{Model: base.Model{ID: 1}, StudentID: 100, ActiveGroupID: 123},
					{Model: base.Model{ID: 2}, StudentID: 101, ActiveGroupID: 123},
				}
				visitRepo.On("FindByActiveGroupID", mock.Anything, int64(123)).Return(activeVisits, nil)
			},
			expectInfo: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			groupRepo := &SimpleGroupRepo{}
			visitRepo := &MockVisitRepository{}
			tt.setupMock(groupRepo, visitRepo)

			// Create service
			service := &service{
				groupRepo: groupRepo,
				visitRepo: visitRepo,
			}

			// Execute test
			info, err := service.GetSessionTimeoutInfo(context.Background(), tt.deviceID)

			// Assertions
			if tt.expectInfo {
				require.NoError(t, err)
				require.NotNil(t, info)
				assert.Equal(t, int64(123), info.SessionID)
				assert.Equal(t, int64(456), info.ActivityID)
				assert.Equal(t, 30, info.TimeoutMinutes)
				assert.Equal(t, 2, info.ActiveStudentCount)
				assert.False(t, info.IsTimedOut) // 10 minutes < 30 minute timeout
			} else {
				require.Error(t, err)
				assert.Nil(t, info)
			}

			// Verify mock expectations
			groupRepo.AssertExpectations(t)
			visitRepo.AssertExpectations(t)
		})
	}
}
