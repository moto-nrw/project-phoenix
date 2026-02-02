package active

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// mockGroupRepository is a minimal mock implementation of active.GroupRepository
type mockGroupRepository struct {
	findByIDFunc   func(ctx context.Context, id interface{}) (*active.Group, error)
	endSessionFunc func(ctx context.Context, id int64) error
}

func (m *mockGroupRepository) Create(ctx context.Context, entity *active.Group) error {
	return nil
}

func (m *mockGroupRepository) FindByID(ctx context.Context, id interface{}) (*active.Group, error) {
	if m.findByIDFunc != nil {
		return m.findByIDFunc(ctx, id)
	}
	return &active.Group{
		Model: base.Model{ID: 1},
	}, nil
}

func (m *mockGroupRepository) Update(ctx context.Context, entity *active.Group) error {
	return nil
}

func (m *mockGroupRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockGroupRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByRoomID(ctx context.Context, roomID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupID(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) EndSession(ctx context.Context, id int64) error {
	if m.endSessionFunc != nil {
		return m.endSessionFunc(ctx, id)
	}
	return nil
}

func (m *mockGroupRepository) FindWithRelations(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithVisits(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindWithSupervisors(ctx context.Context, id int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByGroupIDWithDevice(ctx context.Context, groupID int64) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceID(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceIDWithRelations(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveByDeviceIDWithNames(ctx context.Context, deviceID int64) (*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) CheckRoomConflict(ctx context.Context, roomID int64, excludeGroupID int64) (bool, *active.Group, error) {
	return false, nil, nil
}

func (m *mockGroupRepository) UpdateLastActivity(ctx context.Context, id int64, lastActivity time.Time) error {
	return nil
}

func (m *mockGroupRepository) FindActiveSessionsOlderThan(ctx context.Context, cutoffTime time.Time) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindInactiveSessions(ctx context.Context, inactiveDuration time.Duration) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindUnclaimed(ctx context.Context) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindActiveGroups(ctx context.Context) ([]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]*active.Group, error) {
	return nil, nil
}

func (m *mockGroupRepository) GetOccupiedRoomIDs(ctx context.Context, roomIDs []int64) (map[int64]bool, error) {
	return nil, nil
}

// mockVisitRepository is a minimal mock implementation of active.VisitRepository
type mockVisitRepository struct {
	findByActiveGroupIDFunc func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error)
	endVisitFunc            func(ctx context.Context, id int64) error
}

func (m *mockVisitRepository) Create(ctx context.Context, entity *active.Visit) error {
	return nil
}

func (m *mockVisitRepository) FindByID(ctx context.Context, id interface{}) (*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) Update(ctx context.Context, entity *active.Visit) error {
	return nil
}

func (m *mockVisitRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockVisitRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveByStudentID(ctx context.Context, studentID int64) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID)
	}
	return []*active.Visit{}, nil
}

func (m *mockVisitRepository) FindByTimeRange(ctx context.Context, start, end time.Time) ([]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) EndVisit(ctx context.Context, id int64) error {
	if m.endVisitFunc != nil {
		return m.endVisitFunc(ctx, id)
	}
	return nil
}

func (m *mockVisitRepository) TransferVisitsFromRecentSessions(ctx context.Context, newActiveGroupID, deviceID int64) (int, error) {
	return 0, nil
}

func (m *mockVisitRepository) DeleteExpiredVisits(ctx context.Context, studentID int64, retentionDays int) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) DeleteVisitsBeforeDate(ctx context.Context, studentID int64, beforeDate time.Time) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) GetVisitRetentionStats(ctx context.Context) (map[int64]int, error) {
	return nil, nil
}

func (m *mockVisitRepository) CountExpiredVisits(ctx context.Context) (int64, error) {
	return 0, nil
}

func (m *mockVisitRepository) GetCurrentByStudentID(ctx context.Context, studentID int64) (*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) GetCurrentByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*active.Visit, error) {
	return nil, nil
}

func (m *mockVisitRepository) FindActiveVisits(ctx context.Context) ([]*active.Visit, error) {
	return nil, nil
}

// mockGroupSupervisorRepository is a minimal mock implementation of active.GroupSupervisorRepository
type mockGroupSupervisorRepository struct {
	findByActiveGroupIDFunc func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error)
	endSupervisionFunc      func(ctx context.Context, id int64) error
}

func (m *mockGroupSupervisorRepository) Create(ctx context.Context, entity *active.GroupSupervisor) error {
	return nil
}

func (m *mockGroupSupervisorRepository) FindByID(ctx context.Context, id interface{}) (*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) Update(ctx context.Context, entity *active.GroupSupervisor) error {
	return nil
}

func (m *mockGroupSupervisorRepository) Delete(ctx context.Context, id interface{}) error {
	return nil
}

func (m *mockGroupSupervisorRepository) List(ctx context.Context, options *base.QueryOptions) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) FindActiveByStaffID(ctx context.Context, staffID int64) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) FindByActiveGroupID(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	if m.findByActiveGroupIDFunc != nil {
		return m.findByActiveGroupIDFunc(ctx, activeGroupID, activeOnly)
	}
	return []*active.GroupSupervisor{}, nil
}

func (m *mockGroupSupervisorRepository) FindByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
	return nil, nil
}

func (m *mockGroupSupervisorRepository) EndSupervision(ctx context.Context, id int64) error {
	if m.endSupervisionFunc != nil {
		return m.endSupervisionFunc(ctx, id)
	}
	return nil
}

func (m *mockGroupSupervisorRepository) GetStaffIDsWithSupervisionToday(ctx context.Context) ([]int64, error) {
	return nil, nil
}

// TestEndActivitySession_FindByActiveGroupIDError tests the error path when finding supervisors fails.
// This covers the error path inside the transaction when supervisorRepo.FindByActiveGroupID
// returns an error, causing a transaction rollback.
func TestEndActivitySession_FindByActiveGroupIDError(t *testing.T) {
	ctx := context.Background()

	// Create a mock DB and transaction using sqlmock
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())

	// Expect BEGIN and ROLLBACK for the transaction
	mock.ExpectBegin()
	mock.ExpectRollback()

	// Create mock repositories
	groupRepo := &mockGroupRepository{
		findByIDFunc: func(ctx context.Context, id interface{}) (*active.Group, error) {
			// Return an active group (EndTime is nil)
			return &active.Group{
				Model: base.Model{ID: 1},
			}, nil
		},
	}

	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
			// Return empty visits (no visits to process)
			return []*active.Visit{}, nil
		},
	}

	// Configure supervisor repository to return error inside transaction
	mockError := errors.New("mock supervisor lookup error")
	supervisorRepo := &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
			return nil, mockError
		},
	}

	// Create service with mocks
	svc := &service{
		groupRepo:      groupRepo,
		visitRepo:      visitRepo,
		supervisorRepo: supervisorRepo,
		txHandler:      base.NewTxHandler(bunDB),
		broadcaster:    nil,
	}

	// ACT
	err = svc.EndActivitySession(ctx, 1)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock supervisor lookup error")
	assert.Contains(t, err.Error(), "EndActivitySession")

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}

// TestEndActivitySession_EndSupervisionError tests the error path when ending supervision fails inside the transaction.
// This covers line ~618 in session_service.go:
//
//	return err
//
// when supervisorRepo.EndSupervision returns an error inside the RunInTx callback.
// The transaction is expected to rollback when this error occurs.
func TestEndActivitySession_EndSupervisionError(t *testing.T) {
	ctx := context.Background()

	// Create a mock DB and transaction using sqlmock
	mockDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = mockDB.Close() }()

	bunDB := bun.NewDB(mockDB, pgdialect.New())

	// Expect BEGIN and ROLLBACK for the transaction
	mock.ExpectBegin()
	mock.ExpectRollback()

	// Create mock repositories
	groupRepo := &mockGroupRepository{
		findByIDFunc: func(ctx context.Context, id interface{}) (*active.Group, error) {
			// Return an active group
			return &active.Group{
				Model: base.Model{ID: 1},
			}, nil
		},
		endSessionFunc: func(ctx context.Context, id int64) error {
			return nil // Should not be reached
		},
	}

	visitRepo := &mockVisitRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64) ([]*active.Visit, error) {
			// Return empty visits
			return []*active.Visit{}, nil
		},
		endVisitFunc: func(ctx context.Context, id int64) error {
			return nil // Should not be reached
		},
	}

	// Configure supervisor repository to return a supervisor, then error when ending it
	mockError := errors.New("mock error")
	supervisorRepo := &mockGroupSupervisorRepository{
		findByActiveGroupIDFunc: func(ctx context.Context, activeGroupID int64, activeOnly bool) ([]*active.GroupSupervisor, error) {
			// Return one supervisor
			return []*active.GroupSupervisor{
				{Model: base.Model{ID: 1}},
			}, nil
		},
		endSupervisionFunc: func(ctx context.Context, id int64) error {
			// Error when trying to end supervision
			return mockError
		},
	}

	// Create service with mocks
	svc := &service{
		groupRepo:      groupRepo,
		visitRepo:      visitRepo,
		supervisorRepo: supervisorRepo,
		txHandler:      base.NewTxHandler(bunDB),
		broadcaster:    nil,
	}

	// ACT
	err = svc.EndActivitySession(ctx, 1)

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mock error")
	assert.Contains(t, err.Error(), "EndActivitySession")

	// Verify all expectations were met
	err = mock.ExpectationsWereMet()
	assert.NoError(t, err)
}
