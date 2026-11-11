package timetablehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// gateTakingMaterialization takes the recurrence and grade-transition gates
// in the create's transaction, as the real materialization does, and then
// reports the window it was asked for.
type gateTakingMaterialization struct {
	*mockMaterializationService
	lock timetable.RecurrenceWriteLock
	err  error
}

func (m *gateTakingMaterialization) MaterializeForTenant(ctx context.Context, from, to calendar.Date, source timetable.MaterializationSource) (*timetable.MaterializationResult, error) {
	if m.err = m.lock.LockRecurrenceWritesThenGradeTransitions(ctx); m.err != nil {
		return nil, m.err
	}
	return m.mockMaterializationService.MaterializeForTenant(ctx, from, to, source)
}

// A template create writes its supervisor rows, which take their staff rows,
// and then materializes in the same transaction. Both tenant gates come
// before the staff rows (#6583, tenant.LockStaffRows): otherwise the lock
// order refuses the materialization's grade-transition gate and the new
// template yields no occurrences.
func TestTemplateCreateMaterializesAfterItsSupervisorRows(t *testing.T) {
	t.Parallel()

	mat := &gateTakingMaterialization{
		mockMaterializationService: &mockMaterializationService{result: &timetable.MaterializationResult{InstancesCreated: 2}},
		lock:                       testTimetableData(testpkg.SetupTestDB(t)).RecurrenceLock(),
	}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res, s.db)

	body := createTemplateBody(s, "Tpl-Create-Lock-Order")
	body["materialize_from"] = "2026-05-04"
	body["materialize_to"] = "2026-05-08"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	require.NoError(t, mat.err, "the materialization takes its gates after the supervisor rows")
	assert.Equal(t, 2, decodeTemplateData[createTemplateResponse](t, w).InstancesCreated)
}

// A writer holding A must still be able to take B while a template submitted
// as [B, A] waits for A. Otherwise an ascending deviation save can deadlock
// with template creation. Include people named only on a weekday roster.
func TestTemplateCreateLocksCompleteRosterInAscendingOrder(t *testing.T) {
	t.Parallel()
	for _, weekdayRoster := range []bool{false, true} {
		name := "shared_roster"
		if weekdayRoster {
			name = "weekday_roster"
		}
		t.Run(name, func(t *testing.T) {
			testpkg.SetupIsolatedTestDB(t)
			s := buildTemplateModule(t, &mockMaterializationService{})
			require.Less(t, s.staffA, s.staffB)
			ctx, cancel := context.WithCancel(s.ctx)
			holder, err := s.db.BeginTx(ctx, nil)
			require.NoError(t, err)
			var running sync.WaitGroup
			t.Cleanup(func() {
				cancel()
				_ = holder.Rollback()
				running.Wait()
			})
			var holderPID int
			require.NoError(t, holder.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &holderPID))
			_, err = holder.NewRaw("SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE", s.staffA).Exec(ctx)
			require.NoError(t, err)

			body := createTemplateBody(s, "Ascending template roster")
			body["staff_ids"] = []int64{s.staffB, s.staffA}
			if weekdayRoster {
				body["staff_ids"] = []int64{s.staffB}
				body["weekday_assignments"] = []map[string]any{{
					"weekday": timetable.WeekdayWednesday, "staff_ids": []int64{s.staffA}, "primary_staff_id": s.staffA,
				}}
			}
			router := templateRouter(ctx, s.res, s.db)
			payload, err := json.Marshal(body)
			require.NoError(t, err)
			req := httptest.NewRequest(http.MethodPost, "/templates", bytes.NewReader(payload)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			done := make(chan *httptest.ResponseRecorder, 1)
			running.Go(func() {
				w := httptest.NewRecorder()
				testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(w, req)
				done <- w
			})
			awaitTemplateStaffWaiter(t, ctx, holder, holderPID, done)
			_, err = holder.NewRaw("SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE NOWAIT", s.staffB).Exec(ctx)
			require.NoError(t, err, "template must wait for A before locking B; an ascending deviation save must be able to finish")
			require.NoError(t, holder.Commit())
			w := <-done
			require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
			created := decodeTemplateData[createTemplateResponse](t, w)
			var staffIDs []int64
			for _, row := range templateSupervisors(t, s, created.TemplateID) {
				staffIDs = append(staffIDs, row.StaffID)
			}
			require.ElementsMatch(t, []int64{s.staffA, s.staffB}, staffIDs)
		})
	}
}

func awaitTemplateStaffWaiter(t *testing.T, ctx context.Context, tx bun.Tx, holderPID int, done <-chan *httptest.ResponseRecorder) {
	t.Helper()
	poll := time.NewTicker(10 * time.Millisecond)
	defer poll.Stop()
	for {
		var waiting bool
		require.NoError(t, tx.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks l
WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))`, holderPID).Scan(ctx, &waiting))
		if waiting {
			return
		}
		select {
		case w := <-done:
			t.Fatalf("template creation finished instead of waiting for A: status=%d body=%s", w.Code, w.Body.String())
		case <-poll.C:
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
}
