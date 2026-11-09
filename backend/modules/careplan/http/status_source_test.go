package careplanhttp_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An edit of other fields of a child the guardians reported sick leaves the
// day's report as it is: the status day keeps the parents as its source and
// its report time (#6291). Before, every update while the flag was set
// rewrote today's day as the OGS's own manual entry.
func TestUpdateStudent_KeepsTheSourceOfAStandingStatus(t *testing.T) {
	t.Parallel()

	date := calendar.NewDate(2026, 11, 10)
	now := date.BerlinMidnight().Add(11 * time.Hour)
	reported := date.BerlinMidnight().Add(7 * time.Hour)
	tc := setupStudentsRoute(t, func() time.Time { return now })
	student := testpkg.CreateTestStudent(t, tc.db, "Paula", "Elternmeldung", "PE1")
	_, err := tc.db.ExecContext(testpkg.Ctx(t), `INSERT INTO active.student_status_days (tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?, 'sick', ?, 'parent')`, testpkg.Tenant(t), student.ID, date, reported)
	require.NoError(t, err)
	_, err = tc.db.ExecContext(testpkg.Ctx(t), `UPDATE users.student_care_profiles SET sick = true, sick_since = ?
		WHERE membership_id IN (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ?)`, reported, student.ID)
	require.NoError(t, err)

	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, "PUT", fmt.Sprintf("/%d", student.ID),
		map[string]any{"last_name": "Umbenannt"}), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

	var day struct {
		Source     string    `bun:"source"`
		ReportedAt time.Time `bun:"reported_at"`
		Cleared    bool      `bun:"cleared"`
	}
	require.NoError(t, tc.db.NewRaw(`SELECT source, reported_at, cleared_at IS NOT NULL AS cleared FROM active.student_status_days
		WHERE student_id = ? AND date = ? AND status = 'sick'`, student.ID, date).Scan(testpkg.Ctx(t), &day))
	assert.Equal(t, "parent", day.Source)
	assert.True(t, reported.Equal(day.ReportedAt), "report time %s", day.ReportedAt)
	assert.False(t, day.Cleared)
}
