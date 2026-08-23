package active

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// CountActiveVisitsByRoomID Tests
// =============================================================================

func TestCountActiveVisitsByRoomID_Success(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 5, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 42)

	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestCountActiveVisitsByRoomID_Error(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 0, errors.New("db error")
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 42)

	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "CountActiveVisitsByRoomID")
}

func TestCountActiveVisitsByRoomID_ZeroCount(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 0, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// CountActiveVisitsByActiveGroupID Tests
// =============================================================================

func TestCountActiveVisitsByActiveGroupID_Success(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 3, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountActiveVisitsByActiveGroupID_Error(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 0, errors.New("db error")
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "CountActiveVisitsByActiveGroupID")
}

func TestCountActiveVisitsByActiveGroupID_ZeroCount(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 0, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// GetStudentCurrentVisit Tests (refactored implementation)
// =============================================================================

func TestGetStudentCurrentVisit_Success(t *testing.T) {
	t.Parallel()

	testStudentID := int64(42)
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return &active.Visit{
				Model:     base.Model{ID: 100},
				StudentID: studentID,
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), testStudentID)

	require.NoError(t, err)
	require.NotNil(t, visit)
	assert.Equal(t, int64(100), visit.ID)
	assert.Equal(t, testStudentID, visit.StudentID)
}

func TestGetStudentCurrentVisit_NotFound(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, &base.DatabaseError{Op: "get current", Err: sql.ErrNoRows}
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisit")
}

func TestGetStudentCurrentVisit_DatabaseError(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, errors.New("connection refused")
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisit")
}

func TestGetStudentCurrentVisit_NilVisit(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
}

// =============================================================================
// GetStudentCurrentVisitWithRoom Tests
// =============================================================================

func TestGetStudentCurrentVisitWithRoom_Success(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return &active.Visit{
				Model:     base.Model{ID: 200},
				StudentID: studentID,
				ActiveGroup: &active.Group{
					Model:  base.Model{ID: 50},
					RoomID: 10,
					Room:   &facilities.Room{Name: "Test Room"},
				},
			}, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, visit)
	assert.Equal(t, int64(200), visit.ID)
	require.NotNil(t, visit.ActiveGroup)
	require.NotNil(t, visit.ActiveGroup.Room)
	assert.Equal(t, "Test Room", visit.ActiveGroup.Room.Name)
}

func TestGetStudentCurrentVisitWithRoom_NotFound(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, &base.DatabaseError{Op: "get with room", Err: sql.ErrNoRows}
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisitWithRoom")
}

func TestGetStudentCurrentVisitWithRoom_DatabaseError(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, errors.New("connection reset")
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisitWithRoom")
}

func TestGetStudentCurrentVisitWithRoom_NilVisit(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
}

// =============================================================================
// ListStudentsPresentInRoom Tests (#1323)
//
// Service-level wrapper around repo.ListActiveStudentIDsByRoomID. Behavior we
// pin: passes through the room ID, returns IDs verbatim on success, wraps
// errors with the service-error envelope so the handler layer can map them.
// =============================================================================

func TestListStudentsPresentInRoom_ReturnsIDsFromRepo(t *testing.T) {
	t.Parallel()

	expected := []int64{11, 22, 33}
	var receivedRoom int64
	visitRepo := &mockVisitRepository{
		listActiveStudentIDsByRoomIDFunc: func(_ context.Context, roomID int64) ([]int64, error) {
			receivedRoom = roomID
			return expected, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	ids, err := svc.ListStudentsPresentInRoom(context.Background(), 42)

	require.NoError(t, err)
	assert.Equal(t, int64(42), receivedRoom, "service must pass through the room ID without mutation")
	assert.Equal(t, expected, ids)
}

func TestListStudentsPresentInRoom_EmptyResult(t *testing.T) {
	t.Parallel()

	visitRepo := &mockVisitRepository{
		listActiveStudentIDsByRoomIDFunc: func(_ context.Context, _ int64) ([]int64, error) {
			return nil, nil
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	ids, err := svc.ListStudentsPresentInRoom(context.Background(), 1)

	require.NoError(t, err)
	assert.Empty(t, ids)
}

func TestListStudentsPresentInRoom_WrapsRepoError(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("db unavailable")
	visitRepo := &mockVisitRepository{
		listActiveStudentIDsByRoomIDFunc: func(_ context.Context, _ int64) ([]int64, error) {
			return nil, repoErr
		},
	}

	svc := &service{ServiceDependencies: ServiceDependencies{VisitRepo: visitRepo}}
	ids, err := svc.ListStudentsPresentInRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, ids)
	assert.Contains(t, err.Error(), "ListStudentsPresentInRoom",
		"error envelope must name the service operation so handler-side mapping has a clear surface")
	assert.Contains(t, err.Error(), "db unavailable",
		"underlying repo error must remain wrapped (errors.Is/Unwrap chain)")
}
