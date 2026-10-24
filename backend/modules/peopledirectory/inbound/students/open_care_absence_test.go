package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestAbsenceWriter_PendingNoteReachesReviewer covers the read side of the same
// request: the parent's note is the queue's signal, so the person who decides
// the request must see it on the child's detail page. Since #2329 a staff
// reviewer reads the child fully, so the note travels alongside the rest of the
// record — a badge withheld from its decider would hide work they own.
func TestAbsenceWriter_PendingNoteReachesReviewer(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	_, account := testpkg.CreateTestStaffWithAccount(t, tc.db, "Note", "Reviewer")
	student := testpkg.CreateTestStudent(t, tc.db, "Note", "Kind", "1a")
	_, submitterAccount := testpkg.CreateTestStaffWithAccount(t, tc.db, "Note", "Einreicher")

	const note = "Kommt später, Termin beim Kinderarzt"
	require.NoError(t, testpkg.WithTenantTx(t, context.Background(), tc.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := tc.module.ExcusedRequests.CreateRequest(
			txCtx, student.ID, submitterAccount.ID,
			[]excusedrequests.Date{excusedrequests.Date(calendar.TodayDate())}, note,
		)
		return err
	}))

	rr := authExec(t, tc,
		testutil.NewRequest("GET", fmt.Sprintf("/%d", student.ID), nil),
		testutil.AdminTestClaims(int(account.ID)),
		[]string{"users:read", "users:absence"},
	)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var env struct {
		Data struct {
			HasFullAccess      bool    `json:"has_full_access"`
			PendingExcusedNote *string `json:"pending_excused_note"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	require.True(t, env.Data.HasFullAccess, "a verified staff reviewer reads the child (#2329)")
	require.NotNil(t, env.Data.PendingExcusedNote, "the decider of the request must see its note")
	assert.Equal(t, note, *env.Data.PendingExcusedNote)
}
