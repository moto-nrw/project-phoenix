package active

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
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
		req = withAdminMoveContext(req)
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
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("rejects regular staff outside target room scope", func(t *testing.T) {
		calledAssign := false
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				getActiveGroupFunc: func(_ context.Context, id int64) (*activeModel.Group, error) {
					return &activeModel.Group{Model: base.Model{ID: id}, RoomID: 77}, nil
				},
				getStaffActiveSupervisionsFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
					return []*activeModel.GroupSupervisor{{GroupID: 123}}, nil
				},
				assignTransitStudentsToActiveGroupFunc: func(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
					calledAssign = true
					return nil, nil
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
		assert.False(t, calledAssign)
	})

	// #2329: supervising the TARGET active group is the whole per-request check;
	// which children are being claimed is no longer scoped per student.
	t.Run("allows target supervisor to assign open transit students", func(t *testing.T) {
		calledAssign := false
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				getActiveGroupFunc: func(_ context.Context, id int64) (*activeModel.Group, error) {
					return &activeModel.Group{Model: base.Model{ID: id}, RoomID: 77}, nil
				},
				getStaffActiveSupervisionsFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
					return []*activeModel.GroupSupervisor{{GroupID: 99}}, nil
				},
				assignTransitStudentsToActiveGroupFunc: func(_ context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.TransitAssignResult, error) {
					calledAssign = true
					return &activeSvc.TransitAssignResult{
						Assigned:      studentIDs,
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
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, calledAssign)
	})

	t.Run("propagates target lookup failures", func(t *testing.T) {
		calledAssign := false
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				getActiveGroupFunc: func(_ context.Context, _ int64) (*activeModel.Group, error) {
					return nil, errors.New("active group lookup failed")
				},
				getStaffActiveSupervisionsFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
					return []*activeModel.GroupSupervisor{{GroupID: 99}}, nil
				},
				assignTransitStudentsToActiveGroupFunc: func(_ context.Context, _ []int64, _ int64) (*activeSvc.TransitAssignResult, error) {
					calledAssign = true
					return nil, nil
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		assert.False(t, calledAssign)
	})
}
