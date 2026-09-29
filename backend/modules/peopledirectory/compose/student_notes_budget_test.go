package compose

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestStudentNotesListQueryBudget pins the note card's read against the
// register (#2940). The timeline resolves every author's name, and the obvious
// way to write that is one lookup per entry — which is invisible until a child
// has a year of notes. Two statements, whatever the card holds.
func TestStudentNotesListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Budget", "Kind", "3a")
	audience := peopledirectory.StudentNoteAudience{
		Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
	}

	// Each note gets its own author, so a per-note name lookup would grow the
	// count with the timeline instead of hiding behind one cached account.
	writeNotes := func(count int) {
		t.Helper()
		for i := range count {
			account := testpkg.CreateTestAccount(t, db, "notes-budget")
			person := testpkg.CreateTestPerson(t, db, "Autor", string(rune('A'+i)))
			_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`,
				account.ID, person.ID).Exec(ctx)
			require.NoError(t, err)
			_, err = module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
				StudentID: child.ID, AuthorAccountID: account.ID,
				Kind:       peopledirectory.StudentNoteKindJournal,
				Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
				Body:       "Eintrag.",
			})
			require.NoError(t, err)
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	// The transaction frame (BEGIN, SET LOCAL ROLE, set_config, COMMIT) is the
	// tenant runtime's, not this read's, so the budget counts the two reads the
	// capability actually issues.
	reads := func() []string {
		return counter.Matching(func(sqlLower string) bool {
			return strings.Contains(sqlLower, "users.student_notes") ||
				strings.Contains(sqlLower, "users.persons")
		})
	}

	list := func(expected int) int {
		t.Helper()
		counter.Reset()
		notes, listErr := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
			StudentID: child.ID, Audience: audience,
		})
		require.NoError(t, listErr)
		require.Len(t, notes, expected)
		for _, note := range notes {
			require.NotEmpty(t, note.AuthorName, "the budget only counts if the names are resolved")
		}
		return len(reads())
	}

	writeNotes(3)
	small := list(3)
	writeNotes(5)
	large := list(8)

	assert.Equal(t, small, large, "the statement count must not grow with the note card")
	testpkg.AssertQueryBudget(t, "modules.peopledirectory.student_notes.list", reads())
}
