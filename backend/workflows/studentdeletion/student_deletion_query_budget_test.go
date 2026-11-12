package studentdeletion_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"

	"github.com/moto-ogs/project-phoenix/api"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
	"github.com/moto-ogs/project-phoenix/workflows/studentdeletion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// studentRowRead matches the People Directory student-row read behind
// ReadEnrollmentStudent, locked or not. Companion-name lookups select
// "student".id AS student_id and are not student-row reads.
func studentRowRead(sql string) bool {
	return strings.HasPrefix(strings.TrimSpace(sql), `select "student".id, "student".created_at`)
}

// The deletion workflow crosses nine owners inside one tenant transaction and,
// on the locked path, holds student-row locks for the subject and every linked
// child while it reads each owner count twice (#3411). These budgets turn that
// cost into numbers: Preview takes no lock and must not grow with companions;
// Execute (deleteConfirmed -> lockedSnapshot) pays 2K+1 student-row reads:
// the graph locks the subject once and passes its record to the snapshot.
// Companion plan reads and the doubled owner-count block remain fresh. The
// scenario uses the owner composition of the production root; the Feedback
// counter, the authorization and the verified-staff check are test doubles,
// so their statements are not counted.
func TestStudentDeletionWorkflow_QueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := testpkg.Ctx(t)
	f := newDeletionFixture(t, db)
	workflow := f.workflow(t)
	counter := testpkg.CaptureQueriesForContext(t, db)
	type measured struct{ preview, execute, executeStudentRows []string }
	run := func(companions int) measured {
		subject := testpkg.CreateTestStudent(t, db, "Budget", fmt.Sprintf("Subject%d", companions), "2a")
		edges := make([]*careplan.CompanionEdge, 0, companions)
		for i := range companions {
			companion := testpkg.CreateTestStudent(t, db, "Budget", fmt.Sprintf("Companion%d_%d", companions, i), "2a")
			edge, err := api.NewStudentCompanionEdge(subject.ID, companion.ID, 1)
			require.NoError(t, err)
			edges = append(edges, edge)
		}
		require.NoError(t, api.ReplaceStudentCompanions(ctx, companionLinks(db), subject.ID, edges))

		counter.Reset()
		preview, err := workflow.Preview(counter.Context(ctx), subject.ID)
		require.NoError(t, err)
		require.Equal(t, companions, preview.Counts.CompanionLinks)
		var out measured
		out.preview = counter.Queries()

		counter.Reset()
		result, err := workflow.Execute(counter.Context(ctx), subject.ID, confirm(preview, studentdeletion.ReasonTestData))
		require.NoError(t, err)
		require.Len(t, result.CompanionIDs, companions, "the locked path must have walked every companion")
		out.execute = counter.Queries()
		out.executeStudentRows = counter.Matching(studentRowRead)
		return out
	}

	one := run(1)
	four := run(4)
	assert.Len(t, four.preview, len(one.preview), "the unlocked preview must not grow with linked children")
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.preview.companions_1", counter.Measurement(one.preview))
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.preview.companions_4", counter.Measurement(four.preview))
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_1", counter.Measurement(one.execute))
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_4", counter.Measurement(four.execute))
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_1.student_rows", counter.Measurement(one.executeStudentRows))
	querybudgets.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_4.student_rows", counter.Measurement(four.executeStudentRows))
}
