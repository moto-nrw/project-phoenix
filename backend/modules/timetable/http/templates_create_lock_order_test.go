package timetablehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

// A template create with a materialization window writes the new roster's
// supervisor rows and then materializes every template of the window in the
// same transaction. The other templates' supervisors join the create's
// ascending staff batch (#6702): the earlier template is planned for A, the
// new one for B (A < B). Before, the create locked B for its roster and the
// run then locked A after B, which the lock order refuses; the best-effort
// run failed and the window stayed empty.
func TestTemplateCreateLocksTheWindowPeopleWithItsRoster(t *testing.T) {
	t.Parallel()
	mat := testTimetableData(testpkg.SetupTestDB(t), fixedTemplateClock).materialization
	s := buildTemplateModule(t, mat, fixedTemplateClock)
	defer s.cleanupFn()
	require.Less(t, s.staffA, s.staffB)
	period := testpkg.CreateTestCalendarPeriod(t, s.db, "Window people period", calendar.NewDate(2026, 1, 1), calendar.NewDate(2026, 12, 31))
	testpkg.SetCalendarPeriodActive(t, s.db, period, true)
	router := templateRouter(s.ctx, s.res, s.db)

	earlier := createTemplateBody(s, "Window people earlier")
	earlier["week_pattern"] = 0
	earlier["staff_ids"], earlier["primary_staff_id"] = []int64{s.staffA}, s.staffA
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", earlier)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())

	created := createTemplateBody(s, "Window people created")
	created["week_pattern"] = 0
	created["staff_ids"], created["primary_staff_id"] = []int64{s.staffB}, s.staffB
	created["materialize_from"], created["materialize_to"] = "2026-08-31", "2026-09-04"
	w = doTemplateJSON(t, router, http.MethodPost, "/templates", created)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	assert.Equal(t, 4, decodeTemplateData[createTemplateResponse](t, w).InstancesCreated,
		"Monday and Wednesday of both templates are planned with their supervisors")
}

// partlyWritingMaterialization writes a row in the create's transaction and
// then fails, as a run that created some occurrences before an error does.
type partlyWritingMaterialization struct {
	*mockMaterializationService
	name string
}

func (m *partlyWritingMaterialization) MaterializeForTenant(ctx context.Context, _, _ calendar.Date, _ timetable.MaterializationSource) (*timetable.MaterializationResult, error) {
	txValue, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, errors.New("materialization runs in the create's transaction")
	}
	if _, err := txValue.(bun.Tx).NewRaw(`INSERT INTO schedule.planning_tracks (tenant_id, name, color, sort_order) VALUES (?, ?, '#112233', 0)`,
		tenant.FromContext(ctx), m.name).Exec(ctx); err != nil {
		return nil, err
	}
	return nil, errors.New("materialization failed after its first write")
}

// The window materialization after a create is best effort: its failure is
// logged and the template stays saved. Its partial writes must not stay
// with it (#6702): the run gets a savepoint of its own.
func TestTemplateCreateRollsBackAFailedMaterialization(t *testing.T) {
	t.Parallel()
	mat := &partlyWritingMaterialization{mockMaterializationService: &mockMaterializationService{}, name: "Partial materialization"}
	s := buildTemplateModule(t, mat)
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res, s.db)
	body := createTemplateBody(s, "Tpl-Partial-Materialization")
	body["materialize_from"], body["materialize_to"] = "2026-05-04", "2026-05-08"
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	assert.NotZero(t, decodeTemplateData[createTemplateResponse](t, w).TemplateID, "the template stays saved")
	var partial int
	require.NoError(t, s.db.NewRaw(`SELECT count(*) FROM schedule.planning_tracks WHERE tenant_id = ? AND name = ?`, testpkg.Tenant(t), mat.name).Scan(s.ctx, &partial))
	assert.Zero(t, partial, "the failed run's writes are rolled back with its savepoint")
}

func TestTemplateCreateSurvivesAnOversizedMaterializationWindow(t *testing.T) {
	t.Parallel()
	mat := testTimetableData(testpkg.SetupTestDB(t), fixedTemplateClock).materialization
	s := buildTemplateModule(t, mat, fixedTemplateClock)
	defer s.cleanupFn()
	body := createTemplateBody(s, "Oversized materialization window")
	body["materialize_from"], body["materialize_to"] = "2026-05-04", "2026-08-31"
	w := doTemplateJSON(t, templateRouter(s.ctx, s.res, s.db), http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	response := decodeTemplateData[createTemplateResponse](t, w)
	require.NotZero(t, response.TemplateID)
	assert.Zero(t, response.InstancesCreated)
	var saved int
	require.NoError(t, s.db.NewRaw(`SELECT count(*) FROM activities.groups WHERE tenant_id = ? AND id = ?`,
		testpkg.Tenant(t), response.TemplateID).Scan(s.ctx, &saved))
	assert.Equal(t, 1, saved, "optional materialization must not roll back the template")
}
