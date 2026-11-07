//go:build testhooks

package compose_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/require"
)

func TestEmptyStudentNotesQueryBudgetAvoidsAuthorLookup(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := api.NewPeopleRepositorySuitePeople(testpkg.NewDatabase(t, db))
	child := testpkg.CreateTestStudent(t, db, "Empty", "Timeline", "3a")
	counter := testpkg.CaptureQueriesForContext(t, db)
	counter.Stop()
	ctx := counter.Context(testpkg.Ctx(t))
	counter.Reset()
	counter.Start()
	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
		},
	})
	counter.Stop()
	require.NoError(t, err)
	require.NotNil(t, notes, "the public empty list remains an empty array")
	require.Empty(t, notes)
	require.Zero(t, counter.WriteRows(), "reading an empty timeline must not write")
	queries, err := json.Marshal(counter.Queries())
	require.NoError(t, err)
	t.Logf("empty_student_notes total_statements=%d note_selects=%d author_selects=%d queries_json=%s",
		counter.Total(), len(counter.Selects("users.student_notes")), len(counter.Selects("users.persons")), queries)
	require.Len(t, counter.Selects("users.student_notes"), 1, "the authoritative note read must still execute")
	require.Empty(t, counter.Selects("users.persons"), "an empty timeline has no author accounts to resolve")
	querybudgets.AssertQueryBudget(t, "modules.peopledirectory.student_notes.empty", counter.Matching(func(query string) bool {
		return strings.Contains(query, "users.student_notes") || strings.Contains(query, "users.persons")
	}))
}

func TestEmptyStudentNotesHonorsCancelledContext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module := api.NewPeopleRepositorySuitePeople(testpkg.NewDatabase(t, db))
	child := testpkg.CreateTestStudent(t, db, "Cancelled", "Timeline", "3a")
	ctx, cancel := context.WithCancel(testpkg.Ctx(t))
	cancel()
	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
		},
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, notes)
}
