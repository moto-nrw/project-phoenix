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
	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 5, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 42)

	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestCountActiveVisitsByRoomID_Error(t *testing.T) {
	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 0, errors.New("db error")
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 42)

	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "CountActiveVisitsByRoomID")
}

func TestCountActiveVisitsByRoomID_ZeroCount(t *testing.T) {
	visitRepo := &mockVisitRepository{
		countActiveByRoomIDFunc: func(ctx context.Context, roomID int64) (int, error) {
			return 0, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByRoomID(context.Background(), 1)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// CountActiveVisitsByActiveGroupID Tests
// =============================================================================

func TestCountActiveVisitsByActiveGroupID_Success(t *testing.T) {
	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 3, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestCountActiveVisitsByActiveGroupID_Error(t *testing.T) {
	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 0, errors.New("db error")
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "CountActiveVisitsByActiveGroupID")
}

func TestCountActiveVisitsByActiveGroupID_ZeroCount(t *testing.T) {
	visitRepo := &mockVisitRepository{
		countActiveByGroupIDFunc: func(ctx context.Context, activeGroupID int64) (int, error) {
			return 0, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	count, err := svc.CountActiveVisitsByActiveGroupID(context.Background(), 10)

	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// GetStudentCurrentVisit Tests (refactored implementation)
// =============================================================================

func TestGetStudentCurrentVisit_Success(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return &active.Visit{
				Model:     base.Model{ID: 100},
				StudentID: studentID,
			}, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, visit)
	assert.Equal(t, int64(100), visit.ID)
	assert.Equal(t, int64(1), visit.StudentID)
}

func TestGetStudentCurrentVisit_NotFound(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, &base.DatabaseError{Op: "get current", Err: sql.ErrNoRows}
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisit")
}

func TestGetStudentCurrentVisit_DatabaseError(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, errors.New("connection refused")
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisit")
}

func TestGetStudentCurrentVisit_NilVisit(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisit(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
}

// =============================================================================
// GetStudentCurrentVisitWithRoom Tests
// =============================================================================

func TestGetStudentCurrentVisitWithRoom_Success(t *testing.T) {
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

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, visit)
	assert.Equal(t, int64(200), visit.ID)
	require.NotNil(t, visit.ActiveGroup)
	require.NotNil(t, visit.ActiveGroup.Room)
	assert.Equal(t, "Test Room", visit.ActiveGroup.Room.Name)
}

func TestGetStudentCurrentVisitWithRoom_NotFound(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, &base.DatabaseError{Op: "get with room", Err: sql.ErrNoRows}
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisitWithRoom")
}

func TestGetStudentCurrentVisitWithRoom_DatabaseError(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, errors.New("connection reset")
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
	assert.Contains(t, err.Error(), "GetStudentCurrentVisitWithRoom")
}

func TestGetStudentCurrentVisitWithRoom_NilVisit(t *testing.T) {
	visitRepo := &mockVisitRepository{
		getCurrentByStudentIDWithRoomFunc: func(ctx context.Context, studentID int64) (*active.Visit, error) {
			return nil, nil
		},
	}

	svc := &service{visitRepo: visitRepo}
	visit, err := svc.GetStudentCurrentVisitWithRoom(context.Background(), 1)

	require.Error(t, err)
	assert.Nil(t, visit)
}
