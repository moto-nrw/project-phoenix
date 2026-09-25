package users_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
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
// Execute (deleteConfirmed -> lockedSnapshot) pays 2(K+1) student-row reads,
// one lock round trip per locked read, and the doubled owner-count block. The
// scenario uses the real composition of the production root, except that the
// Feedback counter is the shared test double, so its query is not counted.
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
		edges := make([]*userModels.StudentCompanion, 0, companions)
		for i := range companions {
			companion := testpkg.CreateTestStudent(t, db, "Budget", fmt.Sprintf("Companion%d_%d", companions, i), "2a")
			edge, err := repositories.NewStudentCompanionEdge(subject.ID, companion.ID, 1)
			require.NoError(t, err)
			edges = append(edges, edge)
		}
		require.NoError(t, repositories.ReplaceStudentCompanions(ctx, companionLinks(db), subject.ID, edges))

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
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.preview.companions_1", one.preview)
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.preview.companions_4", four.preview)
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_1", one.execute)
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_4", four.execute)
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_1.student_rows", one.executeStudentRows)
	testpkg.AssertQueryBudget(t, "workflows.studentdeletion.execute.companions_4.student_rows", four.executeStudentRows)
}
