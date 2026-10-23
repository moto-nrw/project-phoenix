package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type familyHTTPRow struct {
	kind, table, route string
	id                 int64
}

func familyHTTPRows(t *testing.T, tc *testContext) ([]familyHTTPRow, int64) {
	t.Helper()
	chain := testpkg.CreateTestParentGuardianChain(t, tc.db)
	owner := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, tc.db)).CarePlan()
	ctx := testpkg.Ctx(t)
	care, err := owner.CreateCareScheduleRequest(ctx, carerequests.Request{StudentID: chain.StudentID, SubmittedBy: chain.AccountID, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"14:00"}]}`)})
	require.NoError(t, err)
	excused, err := owner.CreateExcusedAbsenceRequest(ctx, careplan.ExcusedAbsenceRequest{StudentID: chain.StudentID, SubmittedBy: chain.AccountID, Dates: []excusedrequests.Date{excusedrequests.Date(studentsTestToday.AddDays(1))}, AbsenceStatus: "excused", Status: "pending"})
	require.NoError(t, err)
	master, err := owner.CreateStudentDataRequest(ctx, careplan.StudentDataChangeRequest{StudentID: chain.StudentID, SubmittedBy: chain.AccountID, Target: "person", FieldKey: "first_name", Status: "pending", OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Max"`)})
	require.NoError(t, err)
	return []familyHTTPRow{
		{"care_schedule", "schedule.care_schedule_change_requests", "care-schedule-change-requests", care.ID},
		{"excused", "active.excused_absence_requests", "excused-absence-requests", excused.ID},
		{"master_data", "users.student_data_change_requests", "master-data-change-requests", master.ID},
	}, chain.StudentID
}

func familyHTTPCalls(row familyHTTPRow) []struct{ path, body string } {
	calls := []struct{ path, body string }{
		{fmt.Sprintf("/%s/%d/decide", row.route, row.id), `{"approve":true,"reason":"Reviewed"}`},
		{fmt.Sprintf("/%s/%d/decide", row.route, row.id), `{"approve":false,"reason":"Reviewed"}`},
		{fmt.Sprintf("/change-requests/%s/%d/correct", row.kind, row.id), `{"approve":false,"reason":"Corrected"}`},
	}
	if row.kind != "master_data" {
		calls = append(calls, struct{ path, body string }{fmt.Sprintf("/change-requests/%s/%d/mark-done", row.kind, row.id), `{}`})
	}
	return calls
}

func requireFamilyNotFoundHTTP(t *testing.T, rr *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	var problem struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
	require.Equal(t, "general.input", problem.Code)
}

func TestFamilyRequestNotFoundHTTP(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Request", "Reviewer")
	rows, _ := familyHTTPRows(t, tc)
	var foreign []familyHTTPRow
	t.Run("foreign fixture", func(t *testing.T) {
		testpkg.OwnTenant(t)
		foreign, _ = familyHTTPRows(t, setupStudentsRoute(t, fixedCalendarClock))
	})
	for i, row := range rows {
		var missing int64
		require.NoError(t, tc.db.NewRaw("SELECT nextval(pg_get_serial_sequence(?, 'id'))", row.table).Scan(t.Context(), &missing))
		for name, id := range map[string]int64{"missing": missing, "foreign": foreign[i].id} {
			t.Run(row.kind+"/"+name, func(t *testing.T) {
				row.id = id
				for _, call := range familyHTTPCalls(row) {
					rr := authExec(t, tc, testutil.NewRequest(http.MethodPost, call.path, strings.NewReader(call.body)), testutil.AdminTestClaims(int(reviewer.ID)), []string{"admin:*"})
					requireFamilyNotFoundHTTP(t, rr)
				}
			})
		}
	}
}

func TestFamilyRequestDecisionStateErrorsHTTP(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, tc.db, "State", "Reviewer")
	for _, state := range []string{"stale", "not pending"} {
		rows, _ := familyHTTPRows(t, tc)
		for _, row := range rows {
			t.Run(row.kind+"/"+state, func(t *testing.T) {
				body := `{"approve":false,"reason":"Reviewed","expected_version":"outdated"}`
				wantCode := "students.change_request_stale"
				if state == "not pending" {
					_, err := tc.db.NewUpdate().TableExpr(row.table).Set("status = 'rejected'").Where("id = ?", row.id).Exec(t.Context())
					require.NoError(t, err)
					wantCode = "students.change_request_not_pending"
				}
				rr := authExec(t, tc, testutil.NewRequest(http.MethodPost, fmt.Sprintf("/%s/%d/decide", row.route, row.id), strings.NewReader(body)), testutil.AdminTestClaims(int(reviewer.ID)), []string{"admin:*"})
				require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
				var problem struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
				require.Equal(t, wantCode, problem.Code)
			})
		}
	}
}

// Pin the wire contract when a row disappears during the locked read, not
// just when the initial lookup already sees a missing/foreign ID.
func TestFamilyRequestDeletedWhileWaitingHTTP(t *testing.T) {
	t.Parallel()
	familyRequestDeletionHTTP(t, "request")
}

func TestFamilyRequestChildDeletedWhileWaitingHTTP(t *testing.T) {
	t.Parallel()
	familyRequestDeletionHTTP(t, "child")
}

func familyRequestDeletionHTTP(t *testing.T, deletion string) {
	t.Helper()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, db, "Request", "Reviewer")
	for _, kind := range []string{"care_schedule", "excused", "master_data"} {

		t.Run(kind+"/"+deletion, func(t *testing.T) {
			rows, studentID := familyHTTPRows(t, tc)
			var row familyHTTPRow
			for _, candidate := range rows {
				if candidate.kind == kind {
					row = candidate
				}
			}
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
			defer cancel()
			people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
			done := make(chan *httptest.ResponseRecorder, 1)
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
				if deletion == "child" {
					if err := people.LockStudent(deleteCtx, studentID); err != nil {
						return err
					}
				} else {
					var id int64
					if err := tx.NewSelect().TableExpr(row.table).Column("id").Where("id = ?", row.id).For("UPDATE").Scan(deleteCtx, &id); err != nil {
						return err
					}
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
				call := familyHTTPCalls(row)[1]
				go func() {
					done <- authExec(t, tc, testutil.NewRequest(http.MethodPost, call.path, strings.NewReader(call.body)).WithContext(ctx), testutil.AdminTestClaims(int(reviewer.ID)), []string{"admin:*"})
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(deleteCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "HTTP decision must wait on the first connection")
				if deletion == "child" {
					_, err := people.DeleteStudent(deleteCtx, studentID)
					return err
				}
				_, err := tx.NewDelete().TableExpr(row.table).Where("id = ?", row.id).Exec(deleteCtx)
				return err
			})
			require.NoError(t, err)
			select {
			case rr := <-done:
				requireFamilyNotFoundHTTP(t, rr)
			case <-ctx.Done():
				t.Fatal("HTTP decision did not finish")
			}
		})
	}
}

func TestBulkRequestDeletedWhileWaitingHTTP(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	_, reviewer := testpkg.CreateTestTeacherWithAccount(t, db, "Bulk", "Reviewer")
	owner := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db)).CarePlan()
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	for _, deletion := range []string{"request", "child"} {
		t.Run(deletion, func(t *testing.T) {
			rows, studentID := familyHTTPRows(t, tc)
			row := rows[2]
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
			defer cancel()
			request, err := owner.FindStudentDataRequest(ctx, row.id, false)
			require.NoError(t, err)
			other, err := owner.CreateStudentDataRequest(ctx, careplan.StudentDataChangeRequest{
				StudentID: studentID, SubmittedBy: request.SubmittedBy, Target: "person", FieldKey: "last_name", Status: "pending", OldValue: json.RawMessage(`"Schneider"`), NewValue: json.RawMessage(`"Becker"`),
			})
			require.NoError(t, err)
			body := fmt.Sprintf(`{"requests":[{"kind":"master_data","id":"%d","expected_version":"%s"},{"kind":"master_data","id":"%d","expected_version":"%s"}],"reason":"Reviewed"}`, row.id, careplan.ParentRequestVersion(request.UpdatedAt), other.ID, careplan.ParentRequestVersion(other.UpdatedAt))
			done := make(chan *httptest.ResponseRecorder, 1)
			err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
				if deletion == "child" {
					if err := people.LockStudent(deleteCtx, studentID); err != nil {
						return err
					}
				} else {
					var id int64
					if err := tx.NewSelect().TableExpr(row.table).Column("id").Where("id = ?", row.id).For("UPDATE").Scan(deleteCtx, &id); err != nil {
						return err
					}
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
				go func() {
					done <- authExec(t, tc, testutil.NewRequest(http.MethodPost, "/change-requests/bulk-approve", strings.NewReader(body)).WithContext(ctx), testutil.AdminTestClaims(int(reviewer.ID)), []string{"admin:*"})
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(deleteCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "bulk decision must wait on the first connection")
				if deletion == "child" {
					_, err := people.DeleteStudent(deleteCtx, studentID)
					return err
				}
				_, err := tx.NewDelete().TableExpr(row.table).Where("id = ?", row.id).Exec(deleteCtx)
				return err
			})
			require.NoError(t, err)
			select {
			case rr := <-done:
				require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
				var problem struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &problem))
				require.Equal(t, "students.change_request_stale", problem.Code)
			case <-ctx.Done():
				t.Fatal("bulk HTTP decision did not finish")
			}
		})
	}
}
