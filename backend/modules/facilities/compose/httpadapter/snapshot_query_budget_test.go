package httpadapter

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
)

func TestRoomSnapshotExportQueryBudget(t *testing.T) {
	t.Parallel()
	tc := setupRoomsRoute(t)
	add := func(from, to int) {
		for i := from; i < to; i++ {
			testpkg.CreateTestRoom(t, tc.db, fmt.Sprintf("Snapshot Budget %d", i))
		}
	}
	add(0, 3)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	run := func() []string {
		counter.Reset()
		req := testutil.NewRequest(http.MethodPost, "/export", strings.NewReader(`{"format":"xlsx"}`))
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteWithAuth(t, tc.router, req, testutil.AdminTestClaims(1))
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		return counter.Operation("SELECT")
	}
	small := run()
	add(3, 8)
	large := run()
	t.Logf("query budget: 3 rooms -> %d reads, 8 rooms -> %d reads", len(small), len(large))
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "api.rooms.snapshot_export.reads", large)
}
