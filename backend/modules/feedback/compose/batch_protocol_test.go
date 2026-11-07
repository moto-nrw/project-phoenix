package compose_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/feedback"
	"github.com/moto-nrw/project-phoenix/modules/feedback/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func feedbackBatchModule(t *testing.T, db *bun.DB) feedback.Capability {
	t.Helper()
	settings, err := compose.NewBoundSettings(func(context.Context) (bool, error) { return true, nil }, func(context.Context) (int, error) { return 90, nil })
	require.NoError(t, err)
	module, err := compose.New(compose.Dependencies{DB: db, Settings: settings, Today: func() feedback.Date { return "2026-08-31" }, Observe: func(compose.Observation) {}})
	require.NoError(t, err)
	return module
}

func feedbackBatchInput(studentID int64, index int) feedback.CreateEntry {
	return feedback.CreateEntry{Value: feedback.ValuePositive, Day: "2026-08-31", Time: fmt.Sprintf("12:34:%02d", index), StudentID: studentID, IsMensaFeedback: index%2 == 0}
}

func TestFeedbackBatchPositiveGrowthQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "BatchGrowth", "1a")
	module := feedbackBatchModule(t, db)
	for _, n := range []int{3, 8} {
		inputs := make([]feedback.CreateEntry, n)
		for i := range inputs {
			inputs[i] = feedbackBatchInput(student.ID, i)
		}
		counter := testpkg.CaptureQueries(t, db)
		entries, err := module.SubmitBatch(testpkg.Ctx(t), inputs)
		counter.Stop()
		require.NoError(t, err)
		require.Len(t, entries, n)
		for i, entry := range entries {
			require.Equal(t, inputs[i].Time, entry.Time)
			require.Equal(t, inputs[i].StudentID, entry.StudentID)
			require.Equal(t, inputs[i].IsMensaFeedback, entry.IsMensaFeedback)
			found, err := module.LookupEntry(testpkg.Ctx(t), entry.ID)
			require.NoError(t, err)
			require.Equal(t, entry, found)
		}
		t.Logf("N=%d statements=%d scope=public-service-own-tenant-transaction", n, counter.Total())
		if n == 3 {
			querybudgets.AssertQueryBudget(t, "services.feedback.submit_batch.entries_3", counter.Queries())
		} else {
			querybudgets.AssertQueryBudget(t, "services.feedback.submit_batch.entries_8", counter.Queries())
		}
	}
}

func TestFeedbackBatchFailureRollsBackWithoutReusingIDs(t *testing.T) {
	t.Parallel()
	// The sequence is database-global, so this witness owns a private clone.
	db := testpkg.SetupIsolatedTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "BatchFailure", "1a")
	module := feedbackBatchModule(t, db)
	ctx := testpkg.Ctx(t)
	before, err := module.Submit(ctx, feedbackBatchInput(student.ID, 0))
	require.NoError(t, err)
	var missingStudent int64
	require.NoError(t, db.NewRaw(`SELECT nextval(pg_get_serial_sequence('users.student_profiles', 'id'))`).Scan(ctx, &missingStudent))
	entries, err := module.SubmitBatch(ctx, []feedback.CreateEntry{
		feedbackBatchInput(student.ID, 1),
		feedbackBatchInput(missingStudent, 2),
		feedbackBatchInput(student.ID, 3),
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "foreign key constraint")
	require.Nil(t, entries)
	retained, err := module.FindEntries(ctx, feedback.Filter{StudentID: &student.ID})
	require.NoError(t, err)
	require.Equal(t, []feedback.Entry{before}, retained)
	after, err := module.Submit(ctx, feedbackBatchInput(student.ID, 4))
	require.NoError(t, err)
	// E3 (#5789) permits allocating IDs for the whole failed batch.
	require.Greater(t, after.ID, before.ID+2)
}

func TestFeedbackBatchLaterChunkFailureRollsBackEarlierChunk(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "Feedback", "ChunkRollback", "1a")
	module := feedbackBatchModule(t, db)
	var missingStudent int64
	require.NoError(t, db.NewRaw(`SELECT nextval(pg_get_serial_sequence('users.student_profiles', 'id'))`).Scan(testpkg.Ctx(t), &missingStudent))
	inputs := make([]feedback.CreateEntry, 129)
	for i := range inputs {
		inputs[i] = feedbackBatchInput(student.ID, i%60)
	}
	inputs[128].StudentID = missingStudent
	entries, err := module.SubmitBatch(testpkg.Ctx(t), inputs)
	require.ErrorContains(t, err, "foreign key constraint")
	require.Nil(t, entries)
	retained, err := module.FindEntries(testpkg.Ctx(t), feedback.Filter{StudentID: &student.ID})
	require.NoError(t, err)
	require.Empty(t, retained, "failure in the second chunk must roll back the first chunk too")
}
