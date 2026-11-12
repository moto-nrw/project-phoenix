package careplanhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestUpdateStudent_ConflictsWithFlagFreeStatusDays(t *testing.T) {
	t.Parallel()
	for _, source := range []string{"parent", "planned", "school"} {
		for _, oldStatus := range []string{"sick", "excused"} {
			t.Run(source+"/"+oldStatus, func(t *testing.T) {
				t.Parallel()
				testpkg.OwnTenant(t)
				// Berlin's day already differs from UTC here.
				day := calendar.NewDate(2026, 8, 24)
				now := day.BerlinMidnight().Add(30 * time.Minute)
				tc := setupStudentsRoute(t, func() time.Time { return now })
				student := testpkg.CreateTestStudent(t, tc.db, "Status", "Vorher", "SC1")
				_, err := tc.db.ExecContext(testpkg.Ctx(t), `INSERT INTO active.student_status_days
      (tenant_id, student_id, date, status, reported_at, source) VALUES (?, ?, ?, ?, ?, ?), (?, ?, ?, ?, ?, ?)`,
					testpkg.Tenant(t), student.ID, day, oldStatus, now.Add(-time.Hour), source,
					testpkg.Tenant(t), student.ID, day.AddDays(1), oldStatus, now.Add(-time.Hour), source)
				require.NoError(t, err)
				newStatus := "excused"
				if oldStatus == newStatus {
					newStatus = "sick"
				}
				path := fmt.Sprintf("/%d", student.ID)
				claims := testutil.AdminTestClaims(1)
				perms := []string{"admin:*"}
				rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path,
					map[string]any{newStatus: true, "last_name": "Teilwrite"}), claims, perms)
				require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
				var failure struct {
					Code string `json:"code"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &failure))
				assert.Equal(t, "students.sick_excused_conflict", failure.Code)
				var name string
				require.NoError(t, tc.db.NewRaw(`SELECT last_name FROM users.persons WHERE id = ?`, student.PersonID).Scan(testpkg.Ctx(t), &name))
				assert.Equal(t, "Vorher", name, "conflict rolls back person writes")
				rows, err := tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, day, day)
				require.NoError(t, err)
				require.Len(t, rows, 1)
				assert.Equal(t, oldStatus, rows[0].Status)
				assert.Equal(t, source, rows[0].Source)

				rr = authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", path,
					map[string]any{oldStatus: false, newStatus: true}), claims, perms)
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var response struct {
					Data map[string]json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
				assert.JSONEq(t, "true", string(response.Data[newStatus]))
				assert.JSONEq(t, "false", string(response.Data[oldStatus]))
				var flags struct {
					Sick    bool
					Excused bool
				}
				require.NoError(t, tc.db.NewRaw(`SELECT sick, excused FROM users.student_care_profiles
                 WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ?)`, student.ID).Scan(testpkg.Ctx(t), &flags))
				assert.Equal(t, newStatus == "sick", flags.Sick)
				assert.Equal(t, newStatus == "excused", flags.Excused)
				rows, err = tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, day, day.AddDays(1))
				require.NoError(t, err)
				require.Len(t, rows, 2)
				for _, row := range rows {
					if row.Date == day {
						assert.Equal(t, newStatus, row.Status)
					} else {
						assert.Equal(t, oldStatus, row.Status)
						assert.Equal(t, source, row.Source)
					}
				}
			})
		}
	}
}

// The profile snapshots false flags, then waits while a flag-free report
// commits. The guard must read the day after acquiring the child lock.
func TestUpdateStudent_StatusDayCommittedWhileProfileWaits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t, fixedCalendarClock)
	student := testpkg.CreateTestStudent(t, db, "Concurrent", "Report", "SC2")
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
	defer cancel()
	done := make(chan *httptest.ResponseRecorder, 1)
	req := testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID), map[string]any{"excused": true})
	req = req.WithContext(ctx)
	claims := testutil.AdminTestClaims(1)
	claims.Permissions = []string{"admin:*"}
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	router := testpkg.MountedQueryBudgetRouter("/api/students", tc.router())
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
		if err := people.LockStudent(txCtx, student.ID); err != nil {
			return err
		}
		var pid int
		if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
			return err
		}
		go func() {
			done <- testutil.ExecuteRequestForTest(t, router, req)
		}()
		require.Eventually(t, func() bool {
			var waiting bool
			err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(txCtx, &waiting)
			return err == nil && waiting
		}, 5*time.Second, 10*time.Millisecond, "profile waits on the reporting transaction")
		_, err := tx.ExecContext(txCtx, `INSERT INTO active.student_status_days
   (tenant_id, student_id, date, status, reported_at, source) VALUES (?, ?, ?, 'sick', ?, 'parent')`,
			testpkg.Tenant(t), student.ID, studentsTestToday, fixedCalendarClock())
		return err
	})
	require.NoError(t, err)
	select {
	case rr := <-done:
		require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	case <-ctx.Done():
		t.Fatal("profile did not finish after report committed")
	}
	rows, err := tc.statusDays.FindActiveByStudentAndDateRange(testpkg.Ctx(t), student.ID, studentsTestToday, studentsTestToday)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "sick", rows[0].Status)
}
