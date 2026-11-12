package timetracking

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// #6702: a rebooking takes the subject's work-time gates before it pins the
// target Abwesenheitsart, like every absence write. Before, it pinned the
// art first and waited for the gates; a booking of the same subject and art
// held the gates (its sick-report lock) and waited for the art: 40P01.
//
// A third transaction holds the art row, so the rebooking waits there first
// and the booking queues behind it. When the holder releases, both commit.
func TestRebookingAndBookingOfTheSameSubjectDoNotDeadlock(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)
	ctx := testpkg.Ctx(t)
	db := testpkg.SetupTestDB(t)
	require.Same(t, f.tc.db, db, "the fixture runs on the package database")

	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int64
	require.NoError(t, holder.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &holderPID))
	_, err = holder.ExecContext(ctx, `SELECT id FROM active.staff_absence_types WHERE id = ? FOR UPDATE`, f.typeID)
	require.NoError(t, err)

	rebook := startRequest(func() *httptest.ResponseRecorder { return f.rebook(t, f.body("Umbuchung", false)) })
	rebookPID := awaitBlockedBy(t, db, rebook, 0, holderPID)
	friday := testpkg.Date(2026, time.September, 18)
	book := startRequest(func() *httptest.ResponseRecorder {
		return postAbsence(t, f.tc, f.token, f.subjectID, map[string]any{
			"absence_type": "other", "absence_type_id": f.typeID,
			"date_start": friday.String(), "date_end": friday.String(),
		})
	})
	awaitBlockedBy(t, db, book, rebookPID, holderPID, rebookPID)
	require.NoError(t, holder.Rollback())

	requireStatus(t, rebook, http.StatusOK, "the rebooking")
	requireStatus(t, book, http.StatusCreated, "the booking")
	require.Equal(t, []string{"other:" + strconv.FormatInt(f.typeID, 10), "other:" + strconv.FormatInt(f.typeID, 10), "other:" + strconv.FormatInt(f.typeID, 10)}, f.storedTypes(t))
}

type pendingRequest chan *httptest.ResponseRecorder

func startRequest(serve func() *httptest.ResponseRecorder) pendingRequest {
	done := make(pendingRequest, 1)
	go func() { done <- serve() }()
	return done
}

// awaitBlockedBy waits until a backend other than except waits for one of
// the blockers and returns its pid; the request must still be running.
func awaitBlockedBy(t *testing.T, db *bun.DB, request pendingRequest, except int64, blockers ...int64) int64 {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case rec := <-request:
			require.FailNow(t, "the request finished instead of waiting", "%d %s", rec.Code, rec.Body.String())
		default:
		}
		var pids []int64
		require.NoError(t, db.NewRaw(`SELECT pid FROM pg_stat_activity
			WHERE pid <> ? AND pg_blocking_pids(pid) && ARRAY[?]::int[]`, except, bun.List(blockers)).Scan(context.Background(), &pids))
		if len(pids) > 0 {
			return pids[0]
		}
	}
	require.FailNow(t, "the request never waited")
	return 0
}

func requireStatus(t *testing.T, request pendingRequest, status int, what string) {
	t.Helper()
	select {
	case rec := <-request:
		require.Equal(t, status, rec.Code, "%s: %s", what, rec.Body.String())
	case <-time.After(30 * time.Second):
		require.FailNow(t, what+" never finished")
	}
}
