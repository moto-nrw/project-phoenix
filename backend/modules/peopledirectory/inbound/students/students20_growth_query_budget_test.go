package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

// Real owner reads through the mounted resource. JWT claims are supplied by
// the test router; production session authentication is outside this seam.
func TestStudents20PeopleGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	for _, route := range []string{"change-history", "note-subjects"} {
		t.Run(route, func(t *testing.T) {
			testpkg.OwnTenant(t)
			testpkg.SetupIsolatedTestDB(t)
			tc := setupStudentsRoute(t)
			if route == "note-subjects" {
				_, activities := api.NewActiveTestTimetable(testpkg.NewDatabase(t, tc.db), testpkg.TenantRuntime(t, tc.db))
				tc.resource.ActiveEnrollments = students20Enrollments{activities}
			}
			child := testpkg.CreateTestStudent(t, tc.db, "People", "Subject", "S20")
			other := testpkg.CreateTestStudent(t, tc.db, "People", "Excluded", "S20")
			actor := testpkg.CreateTestAccount(t, tc.db, "s20-people@example.com")
			expected := make([]map[string]any, 0, 8)
			counter := testpkg.CaptureQueriesForContext(t, tc.db)
			for _, n := range []int{3, 8} {
				for len(expected) < n {
					i := len(expected)
					if route == "change-history" {
						old, newValue := "before", fmt.Sprintf("after %02d", i)
						stamp := time.Date(2026, 8, 24, 12, i, 0, 0, time.UTC)
						row := &testpkg.StudentFieldEdit{TenantID: testpkg.Tenant(t), StudentID: child.ID, EditedBy: actor.ID, EditedByName: "Editor", FieldName: "school_class", OldValue: &old, NewValue: &newValue, CreatedAt: stamp}
						_, err := tc.db.NewInsert().Model(row).Returning("id").Exec(testpkg.Ctx(t))
						require.NoError(t, err)
						expected = append([]map[string]any{{"id": strconv.FormatInt(row.ID, 10), "field": "school_class", "old_value": old, "new_value": newValue, "edited_by": "Editor", "changed_at": stamp.Format(time.RFC3339)}}, expected...)
					} else {
						name := fmt.Sprintf("Activity%02d", i)
						group := testpkg.CreateTestActivityGroup(t, tc.db, name)
						row := &testpkg.StudentEnrollment{TenantID: testpkg.Tenant(t), StudentID: child.ID, ActivityGroupID: group.ID, ValidFrom: calendar.TodayDate().AddDays(-1)}
						_, err := tc.db.NewInsert().Model(row).Exec(testpkg.Ctx(t))
						require.NoError(t, err)
						expected = append(expected, map[string]any{"id": strconv.FormatInt(group.ID, 10), "name": name})
					}
				}
				counter.Reset()
				router := testutil.NewRouter()
				router.Mount("/api/students", tc.router())
				req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/%s", child.ID, route), nil)
				req = req.WithContext(counter.Context(req.Context()))
				rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				var body struct {
					Data json.RawMessage `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
				if route == "change-history" {
					var rows []map[string]any
					require.NoError(t, json.Unmarshal(body.Data, &rows))
					for _, row := range rows {
						stamp, err := time.Parse(time.RFC3339, row["changed_at"].(string))
						require.NoError(t, err)
						row["changed_at"] = stamp.UTC().Format(time.RFC3339)
					}
					require.Equal(t, expected, rows)
				} else {
					var rows map[string]any
					require.NoError(t, json.Unmarshal(body.Data, &rows))
					values := make([]any, 0, n)
					for _, row := range expected {
						values = append(values, row)
					}
					require.Equal(t, map[string]any{"activities": values}, rows)
				}
				require.Zero(t, counter.WriteRows())
				assertStudents20PeopleBudget(t, route, n, counter.Measurement(counter.Queries()))
				t.Logf("students20 %s N%d SQL=%d", route, n, counter.Total())
			}
			foreignTenant, _ := testpkg.CreateTestTenant(t, tc.db)
			foreign := testpkg.CreateTestStudentForTenant(t, tc.db, foreignTenant, "Foreign", "Private", "S20")
			denied := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/%s", foreign.ID, route), nil), testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
			require.Equal(t, http.StatusNotFound, denied.Code, denied.Body.String())
			require.NotContains(t, denied.Body.String(), "Private")

			// The target child's collections are not reused for a different child.
			rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d/%s", other.ID, route), nil), testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
			require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
			if route == "change-history" {
				require.JSONEq(t, `[]`, string(jsonDataStudents20(t, rr.Body.Bytes())))
			} else {
				require.JSONEq(t, `{"activities":[]}`, string(jsonDataStudents20(t, rr.Body.Bytes())))
			}
		})
	}
}

func jsonDataStudents20(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data
}

// Mirrors the production root reduction, but calls the real Timetable owner.
type students20Enrollments struct {
	activitycontract.ActivityService
}

func (s students20Enrollments) ActiveEnrollmentGroups(ctx context.Context, ids []int64, on calendar.Date) (map[int64][]peopledirectory.StudentActivityGroup, error) {
	groups, err := s.GetActiveStudentEnrollmentsByStudentIDs(ctx, ids, on)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]peopledirectory.StudentActivityGroup, len(groups))
	for id, rows := range groups {
		for _, row := range rows {
			if row != nil {
				result[id] = append(result[id], peopledirectory.StudentActivityGroup{ID: row.ID, Name: row.Name})
			}
		}
	}
	return result, nil
}

func assertStudents20PeopleBudget(t *testing.T, route string, n int, queries querybudgets.Measurement) {
	t.Helper()
	switch fmt.Sprintf("%s.n%d", route, n) {
	case "change-history.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.change_history.n3", queries)
	case "change-history.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.change_history.n8", queries)
	case "note-subjects.n3":
		querybudgets.AssertQueryBudget(t, "api.students.students20.note_subjects.n3", queries)
	case "note-subjects.n8":
		querybudgets.AssertQueryBudget(t, "api.students.students20.note_subjects.n8", queries)
	default:
		t.Fatalf("unregistered students20 route %s N%d", route, n)
	}
}
