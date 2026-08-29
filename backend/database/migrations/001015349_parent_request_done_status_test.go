package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The four queues must accept the new terminal state; the Down must refuse to
// narrow the constraint while such a row exists, because there is no honest
// value to rewrite a staff decision into.
func TestParentRequestDoneStatusIsAcceptedAndCannotBeNarrowedAway(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	for _, target := range parentRequestDoneStatusValues {
		var allowed bool
		require.NoError(t, db.NewRaw(fmt.Sprintf(`
			SELECT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conrelid = '%s.%s'::regclass
				  AND contype = 'c'
				  AND pg_get_constraintdef(oid) LIKE '%%''done''%%'
			)
		`, target.Schema, target.Table)).Scan(ctx, &allowed),
			"reading the status CHECK of %s.%s", target.Schema, target.Table)
		require.Truef(t, allowed, "%s.%s must accept the done status", target.Schema, target.Table)
	}
}

func TestParentRequestDoneStatusDownRefusesWhileDoneRowsExist(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID, _ := testpkg.CreateTestTenant(t, db)
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Done", "Row", "1a")
	guardian := testpkg.CreateTestAccount(t, db, "parent-request-done-guardian@example.com")

	_, err := db.ExecContext(ctx, `
		INSERT INTO active.excused_absence_requests
			(tenant_id, student_id, submitted_by, dates, note, status, absence_status)
		VALUES (?, ?, ?, to_jsonb(ARRAY[(CURRENT_DATE - 3)::TEXT]), 'Vergangener Tag', 'done', 'excused')
	`, tenantID, student.ID, guardian.ID)
	require.NoError(t, err)

	// Down checks every table before narrowing any, so this refusal leaves the
	// schema untouched — no other test in the package sees a narrowed CHECK.
	require.Error(t, parentRequestDoneDown(ctx, db))
}
