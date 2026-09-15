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

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type moveAuthPersonService struct {
	People
	person *PersonIdentity
	staff  *StaffIdentity
}

func (s moveAuthPersonService) FindByAccountID(_ context.Context, _ int64) (*PersonIdentity, error) {
	return s.person, nil
}

func (s moveAuthPersonService) GetStaffByPersonID(_ context.Context, _ int64) (*StaffIdentity, error) {
	return s.staff, nil
}

func withAdminMoveContext(req *testutil.Request) *testutil.Request {
	claims := adminClaims()
	claims.ID, claims.Permissions = 1, []string{"admin:*"}
	return req.WithContext(claimsCtxWithParent(req.Context(), claims))
}

func withStaffMoveContext(req *testutil.Request) *testutil.Request {
	claims := staffClaims()
	claims.ID, claims.Permissions = 2, []string{"visits:update"}
	return req.WithContext(claimsCtxWithParent(req.Context(), claims))
}

func TestBulkMoveBypassRequiresValidatedAdminPrincipal(t *testing.T) {
	t.Parallel()
	for _, granted := range [][]string{{"admin:*"}, {"*:*"}} {
		claims := staffClaims()
		claims.Permissions = granted
		req := newRequestWithClaims(testutil.MethodPost, "/visits/move-to-group", claims)
		require.True(t, canBypassBulkMoveResourceChecks(req))
	}
	req := withStaffMoveContext(httptest.NewRequest(testutil.MethodPost, "/visits/move-to-group", nil))
	require.False(t, canBypassBulkMoveResourceChecks(req))
	// Raw claims alone cannot bypass the validated principal boundary.
	raw := testpkg.IdentityContext(context.Background(), 2, 42, "", []string{"admin:*"})
	require.False(t, canBypassBulkMoveResourceChecks(req.WithContext(raw)))
}

func TestMoveHandlerOnlyPassesSchoolWideEligibilityForTenantStaff(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		scope       string
		claimTenant int64
		staffTenant int64
		permissions []string
		want        bool
	}{
		{name: "OGS staff", claimTenant: 42, staffTenant: 42, permissions: []string{"visits:update"}, want: true},
		{name: "school portal", scope: "school", claimTenant: 42, staffTenant: 42, permissions: []string{"visits:update"}},
		{name: "parent portal", scope: "parent", staffTenant: 42, permissions: []string{"visits:update"}},
		{name: "operator portal", scope: "platform", staffTenant: 42, permissions: []string{"visits:update"}},
		{name: "missing permission", claimTenant: 42, staffTenant: 42},
		{name: "wrong tenant", claimTenant: 43, staffTenant: 43, permissions: []string{"visits:update"}},
		{name: "foreign staff", claimTenant: 42, staffTenant: 43, permissions: []string{"visits:update"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			staff := &StaffIdentity{ID: 20}
			staff.TenantID = tc.staffTenant
			called := false
			rs := resourceForTest(Resource{
				PersonService: moveAuthPersonService{person: &PersonIdentity{ID: 10}, staff: staff},
				Operations: &stubPresenceOperations{moveStudentsToSession: func(_ context.Context, _ []int64, _ int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					called = true
					require.Equal(t, tc.want, auth.SchoolWideAttendanceEligible)
					require.False(t, auth.BypassResourceChecks)
					return studentpresence.StudentMoveResult{}, operationError("MoveStudentsToActiveGroup", studentpresence.ErrStudentMoveForbidden)
				}},
			})
			ctx := testpkg.ContextForTenant(context.Background(), 42)
			claims := staffClaims()
			claims.ID, claims.TenantID, claims.Scope, claims.Permissions = 2, tc.claimTenant, tc.scope, tc.permissions
			ctx = claimsCtxWithParent(ctx, claims)
			req := httptest.NewRequest(testutil.MethodPost, "/visits/move-to-group", bytes.NewBufferString(`{"student_ids":[123],"target_active_group_id":99}`)).WithContext(ctx)
			rr := httptest.NewRecorder()
			rs.moveStudentsToActiveGroup(rr, req)
			require.True(t, called)
			require.Equal(t, testutil.StatusForbidden, rr.Code)
		})
	}
}

func TestMoveStudentsToActiveGroup(t *testing.T) {
	t.Parallel()

	t.Run("moves selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		var capturedActiveGroupID int64
		targetGroupID := int64(99)
		targetRoomID := int64(77)
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				moveStudentsToSession: func(_ context.Context, studentIDs []int64, activeGroupID int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					capturedStudentIDs = studentIDs
					capturedActiveGroupID = activeGroupID
					return studentpresence.StudentMoveResult{
						Moved:         []int64{42, 84},
						Unchanged:     []int64{},
						Skipped:       []studentpresence.StudentMoveSkipped{},
						ActiveGroupID: &targetGroupID,
						RoomID:        &targetRoomID,
					}, nil
				},
			},
		})

		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42,84],"target_active_group_id":99}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, testutil.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		assert.Equal(t, int64(99), capturedActiveGroupID)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students moved successfully", body["message"])
	})

	t.Run("rejects missing required fields", func(t *testing.T) {
		rs := resourceForTest(Resource{Operations: &stubPresenceOperations{}})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[],"target_active_group_id":99}`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		assert.Equal(t, testutil.StatusBadRequest, w.Code)
	})

	t.Run("rejects all-not-present moves as conflict", func(t *testing.T) {
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				moveStudentsToSession: func(_ context.Context, _ []int64, _ int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					return studentpresence.StudentMoveResult{}, operationError("MoveStudentsToActiveGroup", studentpresence.ErrStudentsNotPresent)
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, testutil.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})

	// #2329: the handler no longer decides per student — it hands the caller's
	// staff identity to the service and surfaces the service's refusal.
	t.Run("surfaces a service-side move refusal as 403", func(t *testing.T) {
		calledMove := false
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				moveStudentsToSession: func(_ context.Context, studentIDs []int64, activeGroupID int64, auth studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					calledMove = true
					assert.Equal(t, []int64{42}, studentIDs)
					assert.Equal(t, int64(99), activeGroupID)
					assert.Equal(t, int64(20), auth.StaffID)
					assert.False(t, auth.BypassResourceChecks)
					return studentpresence.StudentMoveResult{}, operationError("MoveStudentsToActiveGroup", studentpresence.ErrStudentMoveForbidden)
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, testutil.StatusForbidden, w.Code)
		assert.True(t, calledMove)
	})

	t.Run("all_staff visibility does not bypass move resource checks", func(t *testing.T) {
		rs := resourceForTest(Resource{
			SettingsService: scopeSettings(configModel.OverviewScopeAllStaff),
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
		})
		req := withStaffMoveContext(httptest.NewRequest(testutil.MethodPost, "/api/active/visits/move-to-group", nil))
		w := httptest.NewRecorder()

		auth, ok := rs.bulkStudentMoveAuthorization(w, req)

		require.True(t, ok)
		require.NotNil(t, auth)
		assert.Equal(t, int64(20), auth.StaffID)
		assert.False(t, auth.BypassResourceChecks)
	})

	t.Run("moves students the service authorizes", func(t *testing.T) {
		calledMove := false
		targetRoomID := int64(77)
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				moveStudentsToSession: func(_ context.Context, studentIDs []int64, activeGroupID int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					calledMove = true
					return studentpresence.StudentMoveResult{
						Moved:         studentIDs,
						Unchanged:     []int64{},
						Skipped:       []studentpresence.StudentMoveSkipped{},
						ActiveGroupID: &activeGroupID,
						RoomID:        &targetRoomID,
					}, nil
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, testutil.StatusOK, w.Code)
		assert.True(t, calledMove)
	})

	t.Run("propagates target lookup failures", func(t *testing.T) {
		calledMove := false
		rs := resourceForTest(Resource{
			PersonService: moveAuthPersonService{
				person: &PersonIdentity{ID: 10},
				staff:  &StaffIdentity{ID: 20},
			},
			Operations: &stubPresenceOperations{
				moveStudentsToSession: func(_ context.Context, _ []int64, _ int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					calledMove = true
					return studentpresence.StudentMoveResult{}, errors.New("active group lookup failed")
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-group",
			bytes.NewBufferString(`{"student_ids":[42],"target_active_group_id":99}`),
		)
		req = withStaffMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToActiveGroup(w, req)

		require.Equal(t, testutil.StatusInternalServerError, w.Code)
		assert.True(t, calledMove)
	})
}

func TestMoveStudentsToTransit(t *testing.T) {
	t.Parallel()

	t.Run("moves selected students", func(t *testing.T) {
		var capturedStudentIDs []int64
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				moveStudentsToTransit: func(_ context.Context, studentIDs []int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					capturedStudentIDs = studentIDs
					return studentpresence.StudentMoveResult{
						Moved:     []int64{42},
						Unchanged: []int64{84},
						Skipped:   []studentpresence.StudentMoveSkipped{},
					}, nil
				},
			},
		})

		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42,84]}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		require.Equal(t, testutil.StatusOK, w.Code)
		assert.Equal(t, []int64{42, 84}, capturedStudentIDs)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students moved to transit successfully", body["message"])
	})

	t.Run("rejects malformed json", func(t *testing.T) {
		rs := resourceForTest(Resource{Operations: &stubPresenceOperations{}})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42]`),
		)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		assert.Equal(t, testutil.StatusBadRequest, w.Code)
	})

	t.Run("rejects all-not-present moves as conflict", func(t *testing.T) {
		rs := resourceForTest(Resource{
			Operations: &stubPresenceOperations{
				moveStudentsToTransit: func(_ context.Context, _ []int64, _ studentpresence.StudentMoveAuthorization) (studentpresence.StudentMoveResult, error) {
					return studentpresence.StudentMoveResult{}, operationError("MoveStudentsToTransit", studentpresence.ErrStudentsNotPresent)
				},
			},
		})
		req := httptest.NewRequest(
			testutil.MethodPost,
			"/api/active/visits/move-to-transit",
			bytes.NewBufferString(`{"student_ids":[42]}`),
		)
		req = withAdminMoveContext(req)
		w := httptest.NewRecorder()

		rs.moveStudentsToTransit(w, req)

		require.Equal(t, testutil.StatusConflict, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		assert.Equal(t, "Students Not Present", body["status"])
	})
}
