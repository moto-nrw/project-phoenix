package students_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failedSnapshotAttendance struct {
	activeService.Service
	reads int
}

func (s *failedSnapshotAttendance) GetStudentsAttendanceStatuses(context.Context, []int64) (map[int64]*activeService.AttendanceStatus, error) {
	s.reads++
	return nil, errors.New("snapshot attendance read unavailable")
}

func TestStudentListAndExportRejectFailedPresenceSnapshot(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Snapshot", "Unavailable", "3a")
	account := testpkg.CreateTestAccount(t, tc.db, "snapshot-reader")
	fault := &failedSnapshotAttendance{Service: tc.resource.ActiveService}
	tc.resource.ActiveService = fault
	for _, route := range []string{"list", "export"} {
		t.Run(route, func(t *testing.T) {
			req := testutil.NewRequest(http.MethodGet, "/", nil)
			if route == "export" {
				req = birthdayExportRequest(t, `{"format":"xlsx","preset":"birthday_list"}`)
			}
			before := fault.reads
			response := authExec(t, tc, req, testutil.AdminTestClaims(int(account.ID)), []string{"admin:*"})
			require.Equal(t, before+1, fault.reads, "request must reach the failed presence read")
			assert.Equal(t, http.StatusInternalServerError, response.Code, response.Body.String())
			assert.Contains(t, response.Header().Get("Content-Type"), "application/json")
		})
	}
}
