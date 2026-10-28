package compose

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The OGS releases a note for the school (#4797): off by default, set and
// withdrawn by the note's author, read by the school as the live released
// notes of one child.

func TestSchoolSharedNotesRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Mila", "Freigabe", "3a")
	author := noteAuthor(t, db, "Sara", "Freigabe")
	create := func(body string, shared bool) peopledirectory.StudentNote {
		t.Helper()
		note, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
			StudentID: child.ID, AuthorAccountID: author,
			Kind:       peopledirectory.StudentNoteKindPermanent,
			Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
			Body:       body, SharedWithSchool: shared,
		})
		require.NoError(t, err)
		return note
	}
	shared := func() []peopledirectory.StudentNote {
		t.Helper()
		notes, err := testpkg.TenantQuery1(t, ctx, db, module.ListSchoolSharedNotes, child.ID)
		require.NoError(t, err)
		return notes
	}

	_, err := module.ListSchoolSharedNotes(ctx, child.ID)
	require.ErrorIs(t, err, tenant.ErrTransactionRequired, "the school reads in its own unit, which also logs the read")

	internal := create("Nur für die OGS.", false)
	assert.False(t, internal.SharedWithSchool, "a note is not released unless the OGS says so")
	released := create("Für die Lehrkraft freigegeben.", true)
	assert.True(t, released.SharedWithSchool)

	notes := shared()
	require.Len(t, notes, 1, "a note that is not released stays invisible for the school")
	assert.Equal(t, released.ID, notes[0].ID)
	assert.Equal(t, "Sara Freigabe", notes[0].AuthorName)

	// An edit without the field keeps the release.
	reworded, err := module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: released.ID, StudentID: child.ID, ActorAccountID: author,
		Kind: released.Kind, Visibility: released.Visibility, Body: "Neu formuliert.",
	})
	require.NoError(t, err)
	assert.True(t, reworded.SharedWithSchool, "an edit without the field keeps the release")

	// The author withdraws the release; the next read no longer finds it.
	off := false
	withdrawn, err := module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: released.ID, StudentID: child.ID, ActorAccountID: author,
		Kind: released.Kind, Visibility: released.Visibility, Body: "Neu formuliert.", SharedWithSchool: &off,
	})
	require.NoError(t, err)
	assert.False(t, withdrawn.SharedWithSchool)
	assert.Empty(t, shared(), "a withdrawn release takes effect on the next read")

	// Only the author changes the release, like every other part of the note.
	on := true
	colleague := noteAuthor(t, db, "Kai", "Kollege")
	_, err = module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: internal.ID, StudentID: child.ID, ActorAccountID: colleague,
		Kind: internal.Kind, Visibility: internal.Visibility, Body: internal.Body, SharedWithSchool: &on,
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNoteNotAuthor)
	assert.Empty(t, shared())
}

// TestSchoolSharedNotesOnlyStaffNotes pins the table rule: only a note an OGS
// staff member wrote can be released. A day hint from moto schule and a
// carried-over master-data hint never can.
func TestSchoolSharedNotesOnlyStaffNotes(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Mila", "Herkunft", "3a")
	author := noteAuthor(t, db, "Sara", "Herkunft")
	insert := func(origin, kind string, subjectDate any) error {
		_, err := db.NewRaw(`INSERT INTO users.student_notes
			(tenant_id, student_id, author_account_id, origin, kind, visibility, body, subject_date, shared_with_school)
			VALUES (?, ?, ?, ?, ?, 'all_staff', 'Text', ?, TRUE)`,
			testpkg.Tenant(t), child.ID, author, origin, kind, subjectDate).Exec(ctx)
		return err
	}
	require.NoError(t, insert("staff", "permanent", nil))
	assert.Error(t, insert("school", "journal", "2026-09-09"), "a day hint from the school is never shared back")
	assert.Error(t, insert("master_data", "permanent", nil), "a carried-over hint has no author to release it")
}

// TestSchoolSharedNotesListQueryBudget pins the school's read (#2940): the
// released notes and one bulk read of the authors' names, whatever the child
// holds.
func TestSchoolSharedNotesListQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	module, err := New(Dependencies{StudentDirectory: testpkg.StudentDirectory{}, DB: db, Observe: func(Observation) {}}, nil)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Budget", "Freigabe", "3a")

	writeNotes := func(count int) {
		t.Helper()
		for i := range count {
			account := testpkg.CreateTestAccount(t, db, "shared-notes-budget")
			person := testpkg.CreateTestPerson(t, db, "Autor", string(rune('A'+i)))
			_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`, account.ID, person.ID).Exec(ctx)
			require.NoError(t, err)
			_, err = module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
				StudentID: child.ID, AuthorAccountID: account.ID,
				Kind:       peopledirectory.StudentNoteKindPermanent,
				Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
				Body:       "Beobachtung.", SharedWithSchool: true,
			})
			require.NoError(t, err)
		}
	}

	counter := testpkg.CaptureQueries(t, db)
	reads := func() []string {
		return counter.Matching(func(sqlLower string) bool {
			return strings.Contains(sqlLower, "users.student_notes") || strings.Contains(sqlLower, "users.persons")
		})
	}
	list := func(expected int) int {
		t.Helper()
		counter.Reset()
		notes, listErr := testpkg.TenantQuery1(t, ctx, db, module.ListSchoolSharedNotes, child.ID)
		require.NoError(t, listErr)
		require.Len(t, notes, expected)
		for _, note := range notes {
			require.NotEmpty(t, note.AuthorName, "the budget only counts if the names are resolved")
		}
		return len(reads())
	}

	writeNotes(2)
	small := list(2)
	writeNotes(4)
	large := list(6)

	assert.Equal(t, small, large, "the statement count must not grow with the released notes")
	testpkg.AssertQueryBudget(t, "modules.peopledirectory.school_shared_notes.list", reads())
}
