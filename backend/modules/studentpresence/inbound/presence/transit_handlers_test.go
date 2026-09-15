package presence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssignTransitStudents(t *testing.T) {
	t.Parallel()

	t.Run("assigns selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		var capturedActiveGroupID int64
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				assignTransitStudents: func(_ context.Context, studentIDs []int64, activeGroupID int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
					capturedStudentIDs = studentIDs
					capturedActiveGroupID = activeGroupID
					return studentpresence.TransitAssignResult{
						Assigned:      []int64{42, 84},
						Skipped:       []studentpresence.TransitAssignSkipped{},
						ActiveGroupID: activeGroupID,
						RoomID:        77,
					}, nil
				},
			},
		})

		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42,84],"active_group_id":99}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, testutil.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		assert.Equal(t, int64(99), capturedActiveGroupID)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Transit students assigned successfully", body["message"])
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		rs := resourceForTest(Resource{Operations: &stubPresenceOperations{}})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42]`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, testutil.StatusBadRequest, w.Code)
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		rs := resourceForTest(Resource{Operations: &stubPresenceOperations{}})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[],"active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, testutil.StatusBadRequest, w.Code)
	})

	t.Run("renders service errors", func(t *testing.T) {
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				assignTransitStudents: func(_ context.Context, _ []int64, _ int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
					return studentpresence.TransitAssignResult{}, operationError("AssignTransitStudentsToActiveGroup", studentpresence.ErrGroupAlreadyEnded)
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		assert.Equal(t, testutil.StatusBadRequest, w.Code)
	})

	t.Run("renders the service refusal outside target room scope", func(t *testing.T) {
		calledAssign := false
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				assignTransitStudents: func(_ context.Context, _ []int64, _ int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
					calledAssign = true
					require.Equal(t, int64(20), auth.StaffID)
					require.False(t, auth.BypassResourceChecks)
					return studentpresence.TransitAssignResult{}, operationError("AssignTransitStudentsToActiveGroup", studentpresence.ErrStudentMoveForbidden)
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, testutil.StatusForbidden, w.Code)
		assert.True(t, calledAssign)
	})

	// Target access is now checked against locked state by the service.
	t.Run("assigns transit students authorized by the service", func(t *testing.T) {
		calledAssign := false
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				assignTransitStudents: func(_ context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
					calledAssign = true
					require.Equal(t, int64(20), auth.StaffID)
					require.False(t, auth.BypassResourceChecks)
					return studentpresence.TransitAssignResult{
						Assigned:      studentIDs,
						Skipped:       []studentpresence.TransitAssignSkipped{},
						ActiveGroupID: activeGroupID,
						RoomID:        77,
					}, nil
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, testutil.StatusOK, w.Code)
		assert.True(t, calledAssign)
	})

	t.Run("propagates target lookup failures", func(t *testing.T) {
		calledAssign := false
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				assignTransitStudents: func(_ context.Context, _ []int64, _ int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.TransitAssignResult, error) {
					calledAssign = true
					return studentpresence.TransitAssignResult{}, errors.New("active group lookup failed")
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/transit/assign",
			bytes.NewBufferString(`{"student_ids":[42],"active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.assignTransitStudents(w, req)

		require.Equal(t, testutil.StatusInternalServerError, w.Code)
		assert.True(t, calledAssign)
	})
}
