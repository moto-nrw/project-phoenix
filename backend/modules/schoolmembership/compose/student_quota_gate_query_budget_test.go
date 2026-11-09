//go:build testhooks

package compose_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestStudentQuotaClassGateQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, operation := range []string{"renew", "set_status", "resume_care", "reactivate"} {
		for _, n := range []int{3, 8} {
			t.Run(fmt.Sprintf("%s/n%d", operation, n), func(t *testing.T) {
				t.Parallel()
				ctx := testpkg.OwnCtx(t)
				testpkg.SetTestChildQuota(t, db, 2, 50)
				ids := make([]int64, 0, n)
				for i := range n {
					student := testpkg.CreateTestStudent(t, db, "QuotaGate", fmt.Sprintf("Child%d", i), "1a")
					ids = append(ids, student.ID)
					if operation == "reactivate" {
						testpkg.SetStudentStatus(t, db, student.ID, "alumnus")
					}
				}
				membershipID := testpkg.SeparateStudentMembership(t, db, ids[0])
				if operation == "set_status" {
					testpkg.SetStudentStatus(t, db, ids[0], "inactive")
				}
				if operation == "resume_care" {
					until := calendar.NewDate(2099, time.June, 14)
					testpkg.SetStudentLifecycle(t, db, ids[0], "active", nil, &until)
				}
				owner := api.NewSchoolMembershipSuiteRepositories(testpkg.MustDatabase(db)).SchoolMembership()
				counter := testpkg.CaptureQueriesForContext(t, db)
				callCtx := counter.Context(ctx)
				counter.Reset()
				switch operation {
				case "renew":
					actualID, err := owner.RenewEnrollment(callCtx, schoolmembership.StudentEnrollment{StudentID: ids[0], SchoolClass: "2a", Status: "active"})
					require.NoError(t, err)
					require.Equal(t, membershipID, actualID)
				case "set_status":
					changed, err := owner.SetStatus(callCtx, ids[0], "active")
					require.NoError(t, err)
					require.True(t, changed)
				case "resume_care":
					changed, err := owner.ResumeCare(callCtx, ids[0], "2099-06-15", "active", "2099-06-15")
					require.NoError(t, err)
					require.True(t, changed)
				case "reactivate":
					changed, err := owner.Reactivate(callCtx, ids, "active", schoolmembership.EnforceChildQuota)
					require.NoError(t, err)
					require.ElementsMatch(t, ids, changed)
				}
				queries := counter.Queries()
				classLocks := counter.Matching(func(q string) bool { return strings.HasPrefix(q, "select pg_advisory_xact_lock_shared(") })
				require.Len(t, classLocks, 1, "reuse the held class gate before the quota lock")
				classIndex := slices.IndexFunc(queries, func(q string) bool {
					return strings.HasPrefix(strings.ToLower(q), "select pg_advisory_xact_lock_shared(")
				})
				quotaIndex := slices.IndexFunc(queries, func(q string) bool { return strings.HasPrefix(strings.ToLower(q), "select pg_advisory_xact_lock(") })
				require.GreaterOrEqual(t, quotaIndex, 0)
				require.Less(t, classIndex, quotaIndex, "the first class gate must still precede the quota lock")
				require.Len(t, counter.Selects("platform.schools"), 1, "read the real quota owner, not a quota double")
				require.Len(t, counter.Selects("users.student_school_memberships"), 2, "retain the before and after quota observations")
				assertStudentQuotaClassGateBudget(t, operation, counter.Measurement(queries))
				people := api.NewPeopleRepositorySuiteFactory(testpkg.MustDatabase(db))
				for i, id := range ids {
					student, err := people.Student.FindByID(ctx, id)
					require.NoError(t, err)
					require.Equal(t, id, student.ID)
					require.Equal(t, "active", string(student.Status))
					wantClass := "1a"
					if operation == "renew" && i == 0 {
						wantClass = "2a"
					}
					require.Equal(t, wantClass, student.SchoolClass)
					if operation == "resume_care" && i == 0 {
						require.NotNil(t, student.EnrolledFrom)
						require.Equal(t, calendar.NewDate(2099, time.June, 15), *student.EnrolledFrom)
						require.Nil(t, student.EnrolledUntil)
					}
				}
			})
		}
	}
}

func assertStudentQuotaClassGateBudget(t *testing.T, operation string, queries querybudgets.Measurement) {
	t.Helper()
	switch operation {
	case "renew":
		querybudgets.AssertQueryBudget(t, "modules.schoolmembership.quota_class_gate.renew", queries)
	case "set_status":
		querybudgets.AssertQueryBudget(t, "modules.schoolmembership.quota_class_gate.set_status", queries)
	case "resume_care":
		querybudgets.AssertQueryBudget(t, "modules.schoolmembership.quota_class_gate.resume_care", queries)
	case "reactivate":
		querybudgets.AssertQueryBudget(t, "modules.schoolmembership.quota_class_gate.reactivate", queries)
	default:
		t.Fatalf("unexpected quota write %q", operation)
	}
}
