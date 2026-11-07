package httpadapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activitiesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Count the complete production request, including the tenant transaction.
func TestStudentActivityEnrollmentCountQueryBudget(t *testing.T) {
	t.Parallel()
	for _, available := range []bool{false, true} {
		for _, n := range []int{1, 3, 8} {
			t.Run(fmt.Sprintf("available_%t/N%d", available, n), func(t *testing.T) {
				testpkg.OwnTenant(t)
				db := testpkg.SetupIsolatedTestDB(t)
				module, service, people, caller, err := rootAPI.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
				require.NoError(t, err)
				resource := activitiesAPI.NewResource(service, module.Timetable, rootAPI.NewTimetableSupervisorDirectory(people), caller, rootAPI.TimetableHTTPTransactions)
				router := chi.NewRouter()
				router.Mount("/api/activities", resource.Router(rootAPI.ProtectedTenantRoutes))
				scalar := *resource
				scalar.ActivityService = scalarEnrollmentCountService{service}
				legacyRouter := chi.NewRouter()
				legacyRouter.Mount("/api/activities", scalar.Router(rootAPI.ProtectedTenantRoutes))
				student := testpkg.CreateTestStudent(t, db, "Target", "Child", "1a")
				peer := testpkg.CreateTestStudent(t, db, "Overlapping", "Child", "1b")
				alumni := testpkg.CreateTestStudent(t, db, "Graduated", "Child", "1c")
				ended := testpkg.CreateTestStudent(t, db, "Departed", "Child", "1d")
				endedOn := calendar.NewDate(2025, 1, 1)
				for i := range n {
					group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Count %02d", i))
					_, err := db.NewUpdate().Model(group).Set("is_open = TRUE").WherePK().Exec(context.Background())
					require.NoError(t, err)
					ids := []int64{peer.ID, alumni.ID, ended.ID}
					if !available {
						ids = append(ids, student.ID)
					}
					if available && n > 1 && i == n-1 {
						continue
					}
					for _, id := range ids {
						input := timetable.StudentEnrollmentInput{StudentID: id, ActivityGroupID: group.ID, ValidFrom: "2026-01-05"}
						if id == ended.ID {
							// A departed child's retained booking is closed historical data,
							// not a new live enrollment after their care ended.
							closed := "2025-01-01"
							input.ValidFrom, input.ValidUntil = "2024-01-01", &closed
						}
						_, err := module.Timetable.CreateStudentEnrollment(testpkg.Ctx(t), input)
						require.NoError(t, err)
					}
					// A closed historical occurrence plus the live row is legal and counts twice.
					until := "2026-01-04"
					_, err = module.Timetable.CreateStudentEnrollment(testpkg.Ctx(t), timetable.StudentEnrollmentInput{StudentID: peer.ID, ActivityGroupID: group.ID, ValidFrom: "2025-01-01", ValidUntil: &until})
					require.NoError(t, err)
				}
				testpkg.SetStudentStatus(t, db, alumni.ID, testpkg.StudentStatusAlumnus)
				testpkg.SetStudentLifecycle(t, db, ended.ID, "active", nil, &endedOn)
				foreign := testpkg.UniqueTestTenantID(t)
				testpkg.EnsureTestTenant(t, db, foreign)
				foreignStudent := testpkg.CreateTestStudentForTenant(t, db, foreign, "Foreign", "Child", "2a")
				foreignGroup := testpkg.CreateTestActivityGroupForTenant(t, db, foreign, "Foreign group")
				_, err = db.NewUpdate().Model(foreignGroup).Set("is_open = TRUE").WherePK().Exec(context.Background())
				require.NoError(t, err)
				_, err = module.Timetable.CreateStudentEnrollment(tenant.WithTenantID(testpkg.Ctx(t), foreign), timetable.StudentEnrollmentInput{StudentID: foreignStudent.ID, ActivityGroupID: foreignGroup.ID, ValidFrom: "2026-01-05"})
				require.NoError(t, err)
				path := fmt.Sprintf("/api/activities/students/%d", student.ID)
				if available {
					path += "/available"
				}
				original := testutil.ExecuteWithAuth(t, legacyRouter, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), testutil.DefaultTestClaims())
				before := enrollmentCountState(t, db)
				counter := testpkg.CaptureQueries(t, db)
				rr := testutil.ExecuteWithAuth(t, router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), testutil.DefaultTestClaims())
				counter.Stop()
				require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
				require.Equal(t, original.Code, rr.Code)
				require.Equal(t, original.Header(), rr.Header())
				require.Equal(t, original.Body.String(), rr.Body.String(), "complete scalar and batch responses differ")
				require.Equal(t, before, enrollmentCountState(t, db))
				require.Zero(t, counter.WriteRows())
				for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
					require.Empty(t, counter.Operation(operation))
				}
				require.NotContains(t, rr.Body.String(), "Foreign group")
				var response struct {
					Data []struct {
						EnrollmentCount int `json:"enrollment_count"`
					} `json:"data"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
				require.Len(t, response.Data, n)
				for i, row := range response.Data {
					want := 3
					if available {
						want = 2
						if n > 1 && i == n-1 {
							want = 0
						}
					}
					require.Equal(t, want, row.EnrollmentCount)
				}
				if available {
					querybudgets.AssertQueryBudget(t, "api.activities.student_available.complete", counter.Queries())
				} else {
					querybudgets.AssertQueryBudget(t, "api.activities.student_enrollments.complete", counter.Queries())
				}
				t.Logf("STATE unchanged=true state=%s", before)
				t.Logf("HTTP %d headers=%v body=%s", rr.Code, rr.Header(), rr.Body.String())
				t.Logf("COMPLETE statements=%d SQL=%s", counter.Total(), mustEnrollmentCountJSON(t, counter.Queries()))
			})
		}
	}
}

func enrollmentCountState(t *testing.T, db *bun.DB) string {
	t.Helper()
	var state string
	err := db.NewRaw(`SELECT jsonb_build_object(
		'groups', (SELECT jsonb_agg(to_jsonb(g) ORDER BY id) FROM activities.groups g),
		'profiles', (SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM users.student_profiles p),
		'memberships', (SELECT jsonb_agg(to_jsonb(m) ORDER BY id) FROM users.student_school_memberships m),
		'care', (SELECT jsonb_agg(to_jsonb(c) ORDER BY membership_id) FROM users.student_care_profiles c),
		'persons', (SELECT jsonb_agg(to_jsonb(p) ORDER BY id) FROM users.persons p),
		'enrollments', (SELECT jsonb_agg(to_jsonb(e) ORDER BY id) FROM activities.student_enrollments e),
		'sequences', (SELECT jsonb_agg(to_jsonb(s) ORDER BY schemaname, sequencename) FROM pg_sequences s)
	)::text`).Scan(context.Background(), &state)
	require.NoError(t, err)
	return state
}

func mustEnrollmentCountJSON(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

// This is the pre-batch route contract, using the unchanged scalar roster API.
// Both instances mount the real registrations, permission guards and TenantTx.
type scalarEnrollmentCountService struct {
	activityContract.ActivityService
}

func (s scalarEnrollmentCountService) GetEnrollmentCounts(ctx context.Context, ids []int64) (map[int64]int, error) {
	result := make(map[int64]int)
	for _, id := range ids {
		students, err := s.GetEnrolledStudents(ctx, id)
		if err != nil {
			return nil, err
		}
		result[id] = len(students)
	}
	return result, nil
}

func enrollmentCountRequest(t *testing.T, router chi.Router, path string) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.ExecuteWithAuth(t, router, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), testutil.DefaultTestClaims())
}
