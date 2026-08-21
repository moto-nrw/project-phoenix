package schedule_test

import (
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func createRepositoryTestStaffID(tb testing.TB, db *bun.DB) int64 {
	tb.Helper()

	staff := testpkg.CreateTestStaff(tb, db, "Schedule", fmt.Sprintf("Creator-%d", time.Now().UnixNano()))
	tb.Cleanup(func() {
	})

	return staff.ID
}
