package active

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMoveStudentsToActiveGroup(t *testing.T) {
	t.Run("moves selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		var capturedActiveGroupID int64
		targetGroupID := int64(99)
		targetRoomID := int64(77)
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				moveStudentsToActiveGroupFunc: func(_ context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.StudentMoveResult, error) {
					capturedStudentIDs = studentIDs
					capturedActiveGroupID = activeGroupID
					return &activeSvc.StudentMoveResult{
						Moved:         []int64{42, 84},
						Unchanged:     []int64{},
						Skipped:       []activeSvc.StudentMoveSkipped{},
						ActiveGroupID: &targetGroupID,
						RoomID:        &targetRoomID,
					}, nil
				},
			},
		}

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42,84],"target_active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		assert.Equal(t, int64(99), capturedActiveGroupID)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students moved successfully", body["message"])
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		rs := &Resource{ActiveService: &trackingMockActiveService{}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[],"target_active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects all-not-present moves as conflict", func(t *testing.T) {
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				moveStudentsToActiveGroupFunc: func(_ context.Context, _ []int64, _ int64) (*activeSvc.StudentMoveResult, error) {
					return nil, &activeSvc.ActiveError{Op: "MoveStudentsToActiveGroup", Err: activeSvc.ErrStudentsNotPresent}
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})
}

func TestMoveStudentsToTransit(t *testing.T) {
	t.Run("moves selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				moveStudentsToTransitFunc: func(_ context.Context, studentIDs []int64) (*activeSvc.StudentMoveResult, error) {
					capturedStudentIDs = studentIDs
					return &activeSvc.StudentMoveResult{
						Moved:     []int64{42},
						Unchanged: []int64{84},
						Skipped:   []activeSvc.StudentMoveSkipped{},
					}, nil
				},
			},
		}

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42,84]}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students moved to transit successfully", body["message"])
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		rs := &Resource{ActiveService: &trackingMockActiveService{}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42]`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects all-not-present moves as conflict", func(t *testing.T) {
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				moveStudentsToTransitFunc: func(_ context.Context, _ []int64) (*activeSvc.StudentMoveResult, error) {
					return nil, &activeSvc.ActiveError{Op: "MoveStudentsToTransit", Err: activeSvc.ErrStudentsNotPresent}
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42]}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})
}
