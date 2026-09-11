package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestMain(m *testing.M) {
	testutil.SeedTestJWTConfig()
	testpkg.PerTestTenants()
	testpkg.Run(m)
}

type fakeQuery struct {
	file   emergencysnapshot.File
	err    error
	export func(context.Context) (emergencysnapshot.File, error)
}

func (f fakeQuery) Snapshot(context.Context, time.Time) (emergencysnapshot.Snapshot, error) {
	return emergencysnapshot.Snapshot{}, nil
}

func (f fakeQuery) Document(context.Context, time.Time) (emergencysnapshot.Document, error) {
	return emergencysnapshot.Document{}, nil
}

func (f fakeQuery) Export(ctx context.Context) (emergencysnapshot.File, error) {
	if f.export != nil {
		return f.export(ctx)
	}
	return f.file, f.err
}

func TestSnapshotRouteAuthorization(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	foreign := testpkg.NewTenantScope(t, db)
	_, account := testpkg.CreateTestTeacherWithAccount(t, db, "Emergency", "Authorization")
	for _, tc := range []struct {
		name, scope   string
		permissions   []string
		authenticated bool
		status        int
	}{
		{"authorized", "", []string{"users:read"}, true, 200},
		{"missing permission", "", nil, true, 403},
		{"parent portal", "parent", []string{"users:read"}, true, 401},
		{"school portal", "school", []string{"users:read"}, true, 401},
		{"anonymous", "", nil, false, 401},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			rs := NewResource(fakeQuery{export: func(ctx context.Context) (emergencysnapshot.File, error) {
				calls++
				require.Equal(t, testpkg.Tenant(t), tenant.FromContext(ctx))
				_, ok := tenant.TransactionFromContext(ctx)
				require.True(t, ok, "the export must run inside the authenticated school's transaction")
				return emergencysnapshot.File{Filename: "notfallliste.pdf", ContentType: "application/pdf", Data: []byte("%PDF-test")}, nil
			}}, db)
			foreignID := strconv.FormatInt(foreign.TenantID, 10)
			req := httptest.NewRequest(http.MethodPost, "/snapshot/export?tenant_id="+foreignID, strings.NewReader(`{"tenant_id":`+foreignID+`}`))
			if tc.authenticated {
				claims := testutil.DefaultTestClaims()
				claims.ID = int(account.ID)
				claims.TenantID = testpkg.Tenant(t)
				claims.Roles = []string{"staff"}
				claims.IsAdmin = false
				claims.Scope = tc.scope
				claims.Permissions = tc.permissions
				req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
			}
			rr := testutil.ExecuteRequestForTest(t, rs.Router(), req)
			require.Equal(t, tc.status, rr.Code, rr.Body.String())
			if tc.status == 200 {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}

func TestBinaryOwnerFailureWire(t *testing.T) {
	t.Parallel()
	rs := NewResource(fakeQuery{err: errors.New("active: GetStudentsAttendanceStatuses: database operation failed")}, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))
	require.Equal(t, 500, rr.Code)
	require.JSONEq(t, `{"status":"error","error":"active: GetStudentsAttendanceStatuses: database operation failed"}`, rr.Body.String())
}

func TestExportSnapshotStreamsTheFile(t *testing.T) {
	t.Parallel()
	rs := NewResource(fakeQuery{file: emergencysnapshot.File{Filename: "notfallliste.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4")}}, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="notfallliste.pdf"`, rr.Header().Get("Content-Disposition"))
	assert.Equal(t, "8", rr.Header().Get("Content-Length"))
	assert.Equal(t, "%PDF-1.4", rr.Body.String())
}

// One owner-query failure surfaces through the existing error contract.
func TestExportSnapshotReportsProjectionFailures(t *testing.T) {
	t.Parallel()
	rs := NewResource(fakeQuery{err: errors.New("owner query failed")}, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "error")
}

func TestExportSnapshotRejectsMissingProjection(t *testing.T) {
	t.Parallel()
	rs := NewResource(nil, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}
