package httpadapter_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	activitiesAPI "github.com/moto-nrw/project-phoenix/modules/timetable/http/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func countComparisonRouters(t *testing.T, db *bun.DB) (chi.Router, chi.Router, timetable.Capability, activityContract.ActivityService) {
	t.Helper()
	module, service, people, caller, err := rootAPI.NewActivitiesTestModule(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	resource := activitiesAPI.NewResource(service, module.Timetable, rootAPI.NewTimetableSupervisorDirectory(people), caller, rootAPI.TimetableHTTPTransactions)
	candidate := chi.NewRouter()
	candidate.Mount("/api/activities", resource.Router(rootAPI.ProtectedTenantRoutes))
	scalar := *resource
	scalar.ActivityService = scalarEnrollmentCountService{service}
	original := chi.NewRouter()
	original.Mount("/api/activities", scalar.Router(rootAPI.ProtectedTenantRoutes))
	return original, candidate, module.Timetable, service
}

func TestStudentActivityCountsKeepFirstDecoderFailure(t *testing.T) {
	t.Parallel()
	for _, available := range []bool{false, true} {
		t.Run(fmt.Sprintf("available_%t", available), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			original, candidate, owner, service := countComparisonRouters(t, db)
			target := testpkg.CreateTestStudent(t, db, "Healthy", "Target", "1a")
			low := testpkg.CreateTestStudent(t, db, "Later group", "Lower ID", "1b")
			high := testpkg.CreateTestStudent(t, db, "First group", "Higher ID", "1c")
			var laterEnrollmentID, laterGroupID int64
			for i, child := range []*testpkg.Student{high, low} {
				group := testpkg.CreateTestActivityGroup(t, db, fmt.Sprintf("Fault %02d", i))
				_, err := db.NewUpdate().Model(group).Set("is_open = TRUE").WherePK().Exec(context.Background())
				require.NoError(t, err)
				ids := []int64{child.ID}
				if !available {
					ids = append(ids, target.ID)
				}
				for _, id := range ids {
					row, err := owner.CreateStudentEnrollment(testpkg.Ctx(t), timetable.StudentEnrollmentInput{StudentID: id, ActivityGroupID: group.ID, ValidFrom: "2026-01-05"})
					require.NoError(t, err)
					if id == low.ID {
						laterEnrollmentID, laterGroupID = row.ID, group.ID
					}
				}
			}
			// Malformed sealed values are decoder faults, not SQL failures. A
			// union read reaches low first, unlike the original group order.
			_, err := db.NewRaw(`UPDATE users.student_care_profiles SET health_data = ?
				WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id IN (?, ?))`, []byte("invalid-ciphertext"), high.ID, low.ID).Exec(context.Background())
			require.NoError(t, err)
			path := fmt.Sprintf("/api/activities/students/%d", target.ID)
			if available {
				path += "/available"
			}
			for _, enrollmentFault := range []bool{false, true} {
				if enrollmentFault {
					_, err := db.NewRaw(`UPDATE activities.student_enrollments SET kept_without_booking = '{"1":7}'::jsonb WHERE id = ?`, laterEnrollmentID).Exec(context.Background())
					require.NoError(t, err)
					_, err = service.GetEnrolledStudents(testpkg.Ctx(t), laterGroupID)
					require.ErrorContains(t, err, "kept_without_booking", "the later enrollment decoder fault must actually execute")
				}
				before := enrollmentCountState(t, db)
				want := enrollmentCountRequest(t, original, path)
				counter := testpkg.CaptureQueries(t, db)
				got := enrollmentCountRequest(t, candidate, path)
				counter.Stop()
				require.Equal(t, http.StatusInternalServerError, want.Code, want.Body.String())
				require.Contains(t, want.Body.String(), fmt.Sprintf("student %d:", high.ID))
				require.Equal(t, want.Code, got.Code)
				require.Equal(t, want.Header(), got.Header())
				require.Equal(t, want.Body.String(), got.Body.String())
				require.Zero(t, counter.WriteRows())
				require.Equal(t, before, enrollmentCountState(t, db))
				t.Logf("FAULT HTTP %d headers=%v body=%s SQL=%s", got.Code, got.Header(), got.Body.String(), mustEnrollmentCountJSON(t, counter.Queries()))
			}

		})
	}
}

func TestStudentActivityCountsKeepSQLFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	original, candidate, owner, _ := countComparisonRouters(t, db)
	target := testpkg.CreateTestStudent(t, db, "Healthy", "Target", "1a")
	group := testpkg.CreateTestActivityGroup(t, db, "Names SQL fault")
	_, err := owner.CreateStudentEnrollment(testpkg.Ctx(t), timetable.StudentEnrollmentInput{StudentID: target.ID, ActivityGroupID: group.ID, ValidFrom: "2026-01-05"})
	require.NoError(t, err)
	// Relation ACL belongs only to this disposable clone, never a global role.
	_, err = db.NewRaw(`REVOKE SELECT ON users.persons FROM phoenix_tenant`).Exec(context.Background())
	require.NoError(t, err)
	path := fmt.Sprintf("/api/activities/students/%d", target.ID)
	before := enrollmentCountState(t, db)
	want := enrollmentCountRequest(t, original, path)
	counter := testpkg.CaptureQueries(t, db)
	got := enrollmentCountRequest(t, candidate, path)
	counter.Stop()
	require.Equal(t, http.StatusInternalServerError, want.Code, want.Body.String())
	require.Contains(t, want.Body.String(), "(SQLSTATE=42501)")
	require.Equal(t, want.Code, got.Code)
	require.Equal(t, want.Header(), got.Header())
	require.Equal(t, want.Body.String(), got.Body.String())
	require.Zero(t, counter.WriteRows())
	require.Equal(t, before, enrollmentCountState(t, db))
	t.Logf("FAULT HTTP %d body=%s SQL=%s", got.Code, got.Body.String(), mustEnrollmentCountJSON(t, counter.Queries()))
	_, err = db.NewRaw(`GRANT SELECT ON users.persons TO phoenix_tenant`).Exec(context.Background())
	require.NoError(t, err)
}

func TestStudentActivityCountsKeepAccessAndInvalidIDResponses(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	original, candidate, _, _ := countComparisonRouters(t, db)
	student := testpkg.CreateTestStudent(t, db, "Target", "Controls", "1a")
	for _, path := range []string{
		"/api/activities/students/invalid", "/api/activities/students/invalid/available",
		fmt.Sprintf("/api/activities/students/%d", student.ID), fmt.Sprintf("/api/activities/students/%d/available", student.ID),
	} {
		for _, permitted := range []bool{false, true} {
			claims := testutil.DefaultTestClaims()
			if !permitted {
				claims.Permissions = []string{"students:read"}
			}
			want := testutil.ExecuteWithAuth(t, original, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), claims)
			got := testutil.ExecuteWithAuth(t, candidate, testutil.NewAuthenticatedRequest(t, http.MethodGet, path, nil), claims)
			require.Equal(t, want.Code, got.Code)
			require.Equal(t, want.Header(), got.Header())
			require.Equal(t, want.Body.String(), got.Body.String())
			if !permitted {
				require.Equal(t, http.StatusForbidden, got.Code)
			}
		}
	}
}
