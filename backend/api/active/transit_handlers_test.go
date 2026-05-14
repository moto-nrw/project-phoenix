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

func TestAssignTransitStudents(t *testing.T) {
	t.Run("assigns selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		var capturedActiveGroupID int64
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				assignTransitStudentsToActiveGroupFunc: func(_ context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.TransitAssignResult, error) {
					capturedStudentIDs = studentIDs
					capturedActiveGroupID = activeGroupID
					return &activeSvc.TransitAssignResult{
						Assigned:      []int64{42, 84},
						Skipped:       []activeSvc.TransitAssignSkipped{},
						ActiveGroupID: activeGroupID,
						RoomID:        77,
					}, nil
				},
			},
		}

		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42,84],"active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		assert.Equal(t, int64(99), capturedActiveGroupID)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Transit students assigned successfully", body["message"])
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		rs := &Resource{ActiveService: &trackingMockActiveService{}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42]`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		rs := &Resource{ActiveService: &trackingMockActiveService{}}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[],"active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("renders service errors", func(t *testing.T) {
		rs := &Resource{
			ActiveService: &trackingMockActiveService{
				assignTransitStudentsToActiveGroupFunc: func(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
					return nil, &activeSvc.ActiveError{
						Op:  "AssignTransitStudentsToActiveGroup",
						Err: activeSvc.ErrActiveGroupAlreadyEnded,
					}
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}
