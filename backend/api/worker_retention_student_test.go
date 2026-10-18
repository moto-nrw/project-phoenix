package api

import (
	"context"
	"testing"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openCare marks a child without a last care day.
const openCare = -1 << 31

// endedCare stages a child of the school whose last care day lies daysAgo
// days back on the Berlin calendar (negative: ahead), in the given lifecycle
// status. openCare leaves the last care day empty.
func (s retentionSuite) endedCare(t *testing.T, tenantID int64, name, status string, daysAgo int) int64 {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, s.admin, tenantID, name, "Austritt", "3a")
	_, err := s.admin.ExecContext(context.Background(), `UPDATE users.student_school_memberships
		SET status = ?, enrolled_from = NULL,
			enrolled_until = CASE WHEN ? THEN NULL ELSE (CURRENT_TIMESTAMP AT TIME ZONE 'Europe/Berlin')::date - ? END
		WHERE student_profile_id = ? AND deleted_at IS NULL`, status, daysAgo == openCare, daysAgo, student.ID)
	require.NoError(t, err)
	return student.ID
}

// studentDeletionLog is what the deletion log and the tombstone hold for one
// automatically deleted child.
type studentDeletionLog struct {
	DeletionType string `bun:"deletion_type"`
	DeletedBy    string `bun:"deleted_by"`
	Reason       string `bun:"deletion_reason"`
	Automatic    bool   `bun:"automatic"`
	Tombstones   int    `bun:"tombstones"`
}

func (s retentionSuite) studentDeletionLog(t *testing.T, tenantID, studentID int64) []studentDeletionLog {
	t.Helper()
	var rows []studentDeletionLog
	require.NoError(t, s.admin.NewRaw(`
		SELECT deletion.deletion_type, deletion.deleted_by, deletion.deletion_reason,
			(deletion.metadata->>'student_deletion')::boolean AS automatic,
			(SELECT COUNT(*) FROM audit.student_deletions AS tombstone
			 WHERE tombstone.tenant_id = deletion.tenant_id AND tombstone.student_id = deletion.student_id
			   AND tombstone.actor_account_id IS NULL AND tombstone.reason = 'retention_expired') AS tombstones
		FROM audit.data_deletions AS deletion
		WHERE deletion.tenant_id = ? AND deletion.student_id = ?`, tenantID, studentID).Scan(context.Background(), &rows))
	return rows
}

func TestNightlyRetentionDeletesChildrenAfterTheirWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, suite.admin)

	expired := suite.endedCare(t, school, "Abgelaufen", "inactive", 366)
	kept := suite.endedCare(t, school, "Frist", "inactive", 365)
	current := suite.endedCare(t, school, "Laufend", "active", openCare)
	// Resuming care clears the last care day; re-enrolling moves it on.
	resumed := suite.endedCare(t, school, "Wieder", "active", openCare)
	renewed := suite.endedCare(t, school, "Verlängert", "active", -30)
	graduate := suite.endedCare(t, school, "Abgang", "alumnus", 400)
	openGraduate := suite.endedCare(t, school, "Abgang", "alumnus", openCare)
	otherExpired := suite.endedCare(t, otherSchool, "Andere", "inactive", 400)

	enrollments := suite.enrollments(t, school)
	ownRequest := enrollments.request(t, "eigen", settledChild{status: enrollment.ChildStatusApproved, student: expired})
	shared, children := enrollments.requestIn(t, enrollments.phaseID, "geteilt",
		settledChild{status: enrollment.ChildStatusApproved, student: expired},
		settledChild{status: enrollment.ChildStatusApproved, student: kept})

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.student_profiles", expired), "a child gone for more than 365 days is deleted")
	assert.False(t, suite.exists(t, "users.student_profiles", graduate), "a graduate past the window is deleted as well")
	for name, id := range map[string]int64{
		"the last day of the window": kept, "a child without an exit": current, "a resumed child": resumed,
		"a re-enrolled child": renewed, "a graduate without a last care day": openGraduate,
	} {
		assert.True(t, suite.exists(t, "users.student_profiles", id), "%s stays", name)
	}
	assert.True(t, suite.exists(t, "users.student_profiles", otherExpired), "the run of one school never touches another")

	assert.False(t, suite.exists(t, "enrollment.requests", ownRequest), "the accepted enrollment leaves with its child")
	assert.True(t, suite.exists(t, "enrollment.requests", shared), "a request keeps a sibling who stays")
	assert.False(t, suite.exists(t, "enrollment.request_children", children[0]))
	assert.True(t, suite.exists(t, "enrollment.request_children", children[1]))

	for _, id := range []int64{expired, graduate} {
		assert.Equal(t, []studentDeletionLog{{
			DeletionType: "student_retention", DeletedBy: "system", Reason: "retention_expired", Automatic: true, Tombstones: 1,
		}}, suite.studentDeletionLog(t, school, id), "the deletion is logged like a manual one, attributed to the system")
	}
	assert.Empty(t, suite.loggedDeletions(t, school, auditModels.DeletionTypeStudentRetention), "each child is logged on its own, not as a data kind")
	assert.Empty(t, suite.studentDeletionLog(t, otherSchool, otherExpired))

	suite.run(t, school)

	assert.Len(t, suite.studentDeletionLog(t, school, expired), 1, "a second run deletes and logs nothing")
}

func TestNightlyRetentionFollowsTheSchoolsRecordedStudentWindow(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.student_retention_days', '30'::jsonb)`, school)
	require.NoError(t, err)

	expired := suite.endedCare(t, school, "Kurz", "inactive", 31)
	kept := suite.endedCare(t, school, "Kurz", "inactive", 30)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "users.student_profiles", expired), "a shorter contractual window deletes earlier")
	assert.True(t, suite.exists(t, "users.student_profiles", kept))
}

func TestNightlyRetentionCountsOnlyDeletedApprovedEnrollments(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	student := suite.endedCare(t, school, "Nachweis", "inactive", 366)
	enrollments := suite.enrollments(t, school)

	approved := enrollments.request(t, "angenommen", settledChild{status: enrollment.ChildStatusApproved, student: student})
	retained, children := enrollments.requestIn(t, enrollments.phaseID, "offen", settledChild{status: enrollment.ChildStatusSubmitted})
	enrollments.matchChildToStudent(t, children[0], student)

	suite.run(t, school)

	assert.False(t, suite.exists(t, "enrollment.requests", approved))
	assert.True(t, suite.exists(t, "enrollment.requests", retained), "an open enrollment keeps its own retention path")
	assert.True(t, suite.exists(t, "enrollment.request_children", children[0]))

	audit := suite.studentDeletionAudit(t, school, student)
	assert.Equal(t, 1, audit.EnrollmentReferences)
	assert.Equal(t, 2, audit.RecordsDeleted)
}

func TestNightlyRetentionRefusesAStudentWindowOutsideTheFrame(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	school := testpkg.Tenant(t)
	_, err := suite.admin.ExecContext(context.Background(), `INSERT INTO config.setting_values (tenant_id, setting_key, value)
		VALUES (?, 'gdpr.student_retention_days', '7'::jsonb)`, school)
	require.NoError(t, err)
	child := suite.endedCare(t, school, "Zu", "inactive", 400)

	err = testpkg.WithinTenantContext(t, context.Background(), suite.serve, school, func(ctx context.Context) error {
		return suite.retention.runSteps(ctx)
	})

	require.ErrorContains(t, err, "outside 30 to 1095", "a window outside the frame deletes nothing and fails the run")
	assert.True(t, suite.exists(t, "users.student_profiles", child))
}
