// Roster replacement vs. a concurrent graduation (#405 review).
//
// UpdateGroupEnrollments preserves the enrollments of graduated children,
// because the roster read hides them and a submitted list therefore carries no
// intent about them. That decision used to be made from the Student relation
// loaded with the enrollments, while only the students the call would ADD were
// locked. A grade transition graduating an already-enrolled child in between
// left the decision reading "active" from a stale relation — and deleted the row
// the graduation must keep for the revert and for future materialization.
package activities_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/activities"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// graduatingEnrollmentRepo is the real enrollment repository with one seam: the
// graduation commits right after the roster read returned its (now stale)
// Student relations — the deterministic stand-in for a transition apply that
// wins the race to the student row.
type graduatingEnrollmentRepo struct {
	activitiesModels.StudentEnrollmentRepository
	calls          int
	afterFirstRead func()
}

func (r *graduatingEnrollmentRepo) FindByGroupID(
	ctx context.Context, groupID int64,
) ([]*activitiesModels.StudentEnrollment, error) {
	rows, err := r.StudentEnrollmentRepository.FindByGroupID(ctx, groupID)
	r.calls++
	if r.calls == 1 && r.afterFirstRead != nil {
		r.afterFirstRead()
	}
	return rows, err
}

func TestActivityService_UpdateGroupEnrollments_PreservesChildGraduatedAfterRosterRead(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, db, "alumnus-enrollment-race")
	stays := testpkg.CreateTestStudent(t, db, "Still", "Enrolled", "1a")
	graduating := testpkg.CreateTestStudent(t, db, "About", "ToGraduate", "4a")
	defer testpkg.CleanupActivityFixtures(t, db, group.ID, stays.ID, graduating.ID)

	service := setupActivityService(t, db)
	require.NoError(t, service.UpdateGroupEnrollments(ctx, group.ID, []int64{stays.ID, graduating.ID}))

	repos := repositories.NewFactory(db)
	seam := &graduatingEnrollmentRepo{
		StudentEnrollmentRepository: repos.StudentEnrollment,
		afterFirstRead: func() {
			_, err := db.NewUpdate().
				TableExpr(`users.students`).
				Set("status = ?", string(usersModels.StudentStatusAlumnus)).
				Where("id = ?", graduating.ID).
				Exec(context.Background())
			require.NoError(t, err)
		},
	}
	racing, err := activities.NewService(
		repos.ActivityCategory,
		repos.ActivityGroup,
		repos.ActivitySchedule,
		repos.ActivitySupervisor,
		seam,
		repos.ActiveGroup,
		repos.Staff,
		repos.Student,
	)
	require.NoError(t, err)

	// The staff member submits the roster they saw a moment ago — which listed
	// the child, because they were still active when the page was rendered.
	require.NoError(t, racing.UpdateGroupEnrollments(ctx, group.ID, []int64{stays.ID}))

	assert.Equal(t, 1, enrollmentRowCount(t, db, group.ID, graduating.ID),
		"an enrollment graduated after the roster read must be preserved, not deleted")
	assert.Equal(t, 1, enrollmentRowCount(t, db, group.ID, stays.ID),
		"the submitted child keeps their enrollment")
}

func enrollmentRowCount(t *testing.T, db *bun.DB, groupID, studentID int64) int {
	t.Helper()

	n, err := db.NewSelect().
		TableExpr(`activities.student_enrollments`).
		Where("activity_group_id = ?", groupID).
		Where("student_id = ?", studentID).
		Count(context.Background())
	require.NoError(t, err)
	return n
}
