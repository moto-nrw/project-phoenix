package presenceprojectionhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func studentHealthViewCount(t *testing.T, db *bun.DB, accountID, studentID int64) int {
	t.Helper()
	var count int
	require.NoError(t, db.NewSelect().
		TableExpr("audit.data_access_log").
		ColumnExpr("count(*)").
		Where("resource_type = ?", "student_health_view").
		Where("actor_account_id = ?", accountID).
		Where("student_id = ?", studentID).
		Scan(context.Background(), &count))
	return count
}

// TestStudentDetailRecordsEveryReadOfTheHealthField: the health field is
// stored sealed (#5472) and the child's record shows it as before; every
// answer that shows it lands in the school's access log. A child without a
// health field and a reader without users:read disclose none and log none.
func TestStudentDetailRecordsEveryReadOfTheHealthField(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	withHealth := testpkg.CreateTestStudent(t, tc.db, "Mila", "Gesundheitsprotokoll", "3a")
	without := testpkg.CreateTestStudent(t, tc.db, "Jonas", "Gesundheitsprotokoll", "3a")
	testpkg.SetTestStudentHealth(t, tc.db, withHealth.ID, testpkg.StrPtr("Nussallergie"))
	actor := testpkg.CreateTestAccount(t, tc.db, "student-health-view@example.test")

	read := func(student int64, perms []string) string {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest(http.MethodGet, fmt.Sprintf("/%d", student), nil),
			testutil.AdminTestClaims(int(actor.ID)), perms)
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		var body struct {
			Data struct {
				HealthInfo string `json:"health_info"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		return body.Data.HealthInfo
	}

	_, sealed := testpkg.ReadTestStudentHealth(t, tc.db, withHealth.ID)
	require.NotEmpty(t, sealed)
	assert.NotContains(t, string(sealed), "Nussallergie", "the table holds no plaintext")

	assert.Equal(t, "Nussallergie", read(withHealth.ID, []string{"users:read", "students:read"}))
	assert.Equal(t, "Nussallergie", read(withHealth.ID, []string{"users:read", "students:read"}))
	assert.Equal(t, 2, studentHealthViewCount(t, tc.db, actor.ID, withHealth.ID), "every read that shows the health field is logged")

	assert.Empty(t, read(without.ID, []string{"users:read", "students:read"}))
	assert.Zero(t, studentHealthViewCount(t, tc.db, actor.ID, without.ID), "no health field, no disclosure")

	assert.Empty(t, read(withHealth.ID, []string{"students:read"}), "students:read alone stays redacted")
	assert.Equal(t, 2, studentHealthViewCount(t, tc.db, actor.ID, withHealth.ID), "a redacted answer logs nothing")
}
