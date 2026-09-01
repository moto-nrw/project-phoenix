package active

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeModel "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type moveAuthPersonService struct {
	userSvc.PersonService
	person *userModel.Person
	staff  *userModel.Staff
}

func (s moveAuthPersonService) FindByAccountID(_ context.Context, _ int64) (*userModel.Person, error) {
	return s.person, nil
}

func (s moveAuthPersonService) GetStaffByPersonID(_ context.Context, _ int64) (*userModel.Staff, error) {
	return s.staff, nil
}

func withAdminMoveContext(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 1, IsAdmin: true})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"admin:*"})
	return req.WithContext(ctx)
}

func withStaffMoveContext(req *http.Request) *http.Request {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, jwt.AppClaims{ID: 2})
	ctx = context.WithValue(ctx, jwt.CtxPermissions, []string{"visits:update"})
	return req.WithContext(ctx)
}

func TestMoveStudentsToActiveGroup(t *testing.T) {
	t.Parallel()

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
		req = withAdminMoveContext(req)
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
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})

	// #2329: the handler no longer decides per student — it hands the caller's
	// staff identity to the service and surfaces the service's refusal.
	t.Run("surfaces a service-side move refusal as 403", func(t *testing.T) {
		calledMove := false
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				moveStudentsToActiveGroupAuthorizedFunc: func(_ context.Context, studentIDs []int64, activeGroupID int64, auth activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
					calledMove = true
					assert.Equal(t, []int64{42}, studentIDs)
					assert.Equal(t, int64(99), activeGroupID)
					assert.Equal(t, int64(20), auth.StaffID)
					assert.False(t, auth.BypassResourceChecks)
					return nil, &activeSvc.ActiveError{Op: "MoveStudentsToActiveGroup", Err: activeSvc.ErrStudentMoveForbidden}
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusForbidden, w.Code)
		assert.True(t, calledMove)
	})

	t.Run("all_staff visibility does not bypass move resource checks", func(t *testing.T) {
		rs := &Resource{
			SettingsService: scopeSettings(configModel.OverviewScopeAllStaff),
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
		}
		req := withStaffMoveContext(httptest.NewRequest(http.MethodPost, "/api/active/visits/move-to-group", nil))
		w := httptest.NewRecorder()

		auth, ok := rs.bulkStudentMoveAuthorization(w, req)

		require.True(t, ok)
		require.NotNil(t, auth)
		assert.Equal(t, int64(20), auth.StaffID)
		assert.False(t, auth.BypassResourceChecks)
	})

	t.Run("moves students the service authorizes", func(t *testing.T) {
		calledMove := false
		targetGroupID := int64(99)
		targetRoomID := int64(77)
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				getActiveGroupFunc: func(_ context.Context, id int64) (*activeModel.Group, error) {
					return &activeModel.Group{Model: base.Model{ID: id}, RoomID: targetRoomID}, nil
				},
				getStaffActiveSupervisionsFunc: func(_ context.Context, _ int64) ([]*activeModel.GroupSupervisor, error) {
					return []*activeModel.GroupSupervisor{{GroupID: targetGroupID}}, nil
				},
				getStudentsAttendanceStatusesFunc: func(_ context.Context, studentIDs []int64) (map[int64]*activeSvc.AttendanceStatus, error) {
					assert.Equal(t, []int64{42}, studentIDs)
					return map[int64]*activeSvc.AttendanceStatus{
						42: {StudentID: 42, Status: "on_yard"},
					}, nil
				},
				checkTeacherStudentAccessFunc: func(_ context.Context, _, _ int64) (bool, error) {
					return false, nil
				},
				getStudentCurrentVisitFunc: func(_ context.Context, _ int64) (*activeModel.Visit, error) {
					return nil, &activeSvc.ActiveError{Op: "GetStudentCurrentVisit", Err: activeSvc.ErrVisitNotFound}
				},
				moveStudentsToActiveGroupFunc: func(_ context.Context, studentIDs []int64, activeGroupID int64) (*activeSvc.StudentMoveResult, error) {
					calledMove = true
					return &activeSvc.StudentMoveResult{
						Moved:         studentIDs,
						Unchanged:     []int64{},
						Skipped:       []activeSvc.StudentMoveSkipped{},
						ActiveGroupID: &activeGroupID,
						RoomID:        &targetRoomID,
					}, nil
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.True(t, calledMove)
	})

	t.Run("propagates target lookup failures", func(t *testing.T) {
		calledMove := false
		rs := &Resource{
			PersonService: moveAuthPersonService{
				person: &userModel.Person{Model: base.Model{ID: 10}},
				staff:  &userModel.Staff{Model: base.Model{ID: 20}},
			},
			ActiveService: &trackingMockActiveService{
				moveStudentsToActiveGroupAuthorizedFunc: func(_ context.Context, _ []int64, _ int64, _ activeSvc.StudentMoveAuthorization) (*activeSvc.StudentMoveResult, error) {
					calledMove = true
					return nil, errors.New("active group lookup failed")
				},
			},
		}
		req := httptest.NewRequest(
			http.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
		assert.True(t, calledMove)
	})
}

func TestMoveStudentsToTransit(t *testing.T) {
	t.Parallel()

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
		req = withAdminMoveContext(req)
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
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		require.Equal(t, http.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})
}
