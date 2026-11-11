package timetablehttp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// Model a deviation save or offboarding holding A's membership. A waiting
// template writer must leave B and A's old supervisor row free for it.
func TestTemplateWritersLockOldAndNewStaffBeforeRows(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"update_shared", "update_weekday", "split_supervisor", "split_reverse", "split_occurrence"} {
		t.Run(name, func(t *testing.T) {
			testpkg.SetupIsolatedTestDB(t)
			mat := &mockMaterializationService{result: &timetable.MaterializationResult{}}
			s := buildTemplateModule(t, mat)
			require.Less(t, s.staffA, s.staffB)
			s.res.Templates = splitTemplateAdministration(s, mat, acceptCareOfferingSeries)
			ctx, cancel := context.WithTimeout(s.ctx, 10*time.Second)
			router := splitRouter(ctx, s.res, s.db, []string{permissions.SchedulesManage})
			sourceBody := createTemplateBody(s, "Source staff locks")
			sourceBody["staff_ids"] = []int64{s.staffA}
			sourceBody["primary_staff_id"] = s.staffA
			if name == "split_occurrence" || name == "split_reverse" {
				sourceBody["staff_ids"] = []int64{s.staffB}
				sourceBody["primary_staff_id"] = s.staffB
			}
			w := doTemplateJSON(t, router, http.MethodPost, "/templates", sourceBody)
			require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
			created := decodeTemplateData[createTemplateResponse](t, w)
			effective := calendar.TodayDate().AddDays(7)
			if name == "split_occurrence" {
				inst := testpkg.CreateTestActivityInstance(t, s.db, effective, s.roomID, testpkg.ActivityInstanceOpts{
					ActivityGroupID: &created.TemplateID, Title: "Old occurrence with removed staff",
				})
				testpkg.CreateTestInstanceStaff(t, s.db, inst.ID, s.staffA, testpkg.InstanceStaffOpts{})
			}
			method, path := http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID)
			body := createTemplateBody(s, "Updated staff locks")
			body["staff_ids"] = []int64{s.staffB, s.staffA}
			if name == "update_weekday" {
				body["staff_ids"] = []int64{s.staffB}
				body["weekday_assignments"] = []map[string]any{{
					"weekday": timetable.WeekdayWednesday, "staff_ids": []int64{s.staffA}, "primary_staff_id": s.staffA,
				}}
			}
			isSplit := name == "split_supervisor" || name == "split_reverse" || name == "split_occurrence"
			if isSplit {
				method, path = http.MethodPost, path+"/split"
				body = splitBody(s, "Successor staff locks", effective)
				body["staff_ids"] = []int64{s.staffB}
				body["primary_staff_id"] = s.staffB
				if name == "split_reverse" {
					body["staff_ids"] = []int64{s.staffA}
					body["primary_staff_id"] = s.staffA
				}
			}
			payload, err := json.Marshal(body)
			require.NoError(t, err)
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
			req := httptest.NewRequest(method, path, bytes.NewReader(payload)).WithContext(ctx)
			req.Header.Set("Content-Type", "application/json")
			done := make(chan *httptest.ResponseRecorder, 1)
			running.Go(func() {
				response := httptest.NewRecorder()
				testpkg.MountedQueryBudgetRouter("/api/timetable", router).ServeHTTP(response, req)
				done <- response
			})
			awaitTemplateStaffWaiter(t, ctx, holder, holderPID, done)
			_, err = holder.NewRaw("SELECT id FROM users.staff_school_memberships WHERE id = ? FOR UPDATE NOWAIT", s.staffB).Exec(ctx)
			require.NoError(t, err, "the writer must wait for A before taking B")
			_, err = holder.NewRaw("SELECT id FROM activities.supervisors WHERE group_id = ? AND staff_id = ? FOR UPDATE NOWAIT", created.TemplateID, sourceBody["primary_staff_id"]).Exec(ctx)
			require.NoError(t, err, "old supervisor rows must remain free until all memberships are locked")
			require.NoError(t, holder.Commit())
			w = <-done
			require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
			if isSplit {
				result := decodeTemplateData[splitTemplateResponse](t, w)
				rows := templateSupervisors(t, s, result.NewTemplateID)
				require.Len(t, rows, 1)
				require.Equal(t, body["primary_staff_id"], rows[0].StaffID)
				if name == "split_occurrence" {
					require.Equal(t, 1, result.DeletedInstances)
				}
			}
		})
	}
}
