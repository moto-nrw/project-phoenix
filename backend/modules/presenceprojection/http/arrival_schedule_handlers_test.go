package presenceprojectionhttp_test

import (
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func createStudentsAPITestStaffID(t *testing.T, tc *testContext) int64 {
	t.Helper()

	staff := testpkg.CreateTestStaff(t, tc.db, "Arrival", fmt.Sprintf("Creator-%d", time.Now().UnixNano()))

	return staff.ID
}
