package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These exercise the note card against the real table: the audience predicate
// the store builds, the authorship rule, the soft deletion and the tenant
// boundary. Every one of them is a SQL statement whose correctness nothing
// upstream can prove.

func notesModule(t *testing.T, db *testpkg.DB) *peopledirectory.Module {
	t.Helper()
	module, err := New(Dependencies{DB: db, Observe: func(Observation) {}})
	require.NoError(t, err)
	return module
}

// noteAuthor creates an account with a person, so a read can resolve the
// author's name the way the timeline renders it.
func noteAuthor(t *testing.T, db *testpkg.DB, first, last string) int64 {
	t.Helper()
	account := testpkg.CreateTestAccount(t, db, "student-notes-"+last)
	person := testpkg.CreateTestPerson(t, db, first, last)
	_, err := db.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`,
		account.ID, person.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	return account.ID
}

func bodies(notes []peopledirectory.StudentNote) []string {
	result := make([]string, 0, len(notes))
	for _, note := range notes {
		result = append(result, note.Body)
	}
	return result
}

func TestStudentNotesRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Mila", "Roundtrip", "3a")
	author := noteAuthor(t, db, "Sara", "Betreuerin")
	date := calendar.NewDate(2026, 9, 9)

	created, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
		StudentID: child.ID, AuthorAccountID: author,
		Kind:       peopledirectory.StudentNoteKindJournal,
		Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
		Category:   peopledirectory.StudentNoteCategoryPositive,
		Body:       "  Hat heute vorgelesen.  ",
		Subject:    peopledirectory.StudentNoteSubject{Date: &date},
	})
	require.NoError(t, err)
	assert.Equal(t, "Hat heute vorgelesen.", created.Body, "the body is trimmed on the way in")
	assert.Equal(t, peopledirectory.StudentNoteOriginStaff, created.Origin)
	require.NotNil(t, created.AuthorAccountID)
	assert.Equal(t, author, *created.AuthorAccountID)
	require.NotNil(t, created.Subject.Date)
	assert.Equal(t, date, *created.Subject.Date, "the day survives the DATE column unshifted")
	assert.False(t, created.Edited())

	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
		},
	})
	require.NoError(t, err)
	require.Len(t, notes, 1)
	assert.Equal(t, "Sara Betreuerin", notes[0].AuthorName, "the timeline resolves the author's name")

	updated, err := module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: created.ID, StudentID: child.ID, ActorAccountID: author,
		Kind:       peopledirectory.StudentNoteKindJournal,
		Visibility: peopledirectory.StudentNoteVisibilityCareTeam,
		Body:       "Hat heute zweimal vorgelesen.",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hat heute zweimal vorgelesen.", updated.Body)
	assert.Equal(t, peopledirectory.StudentNoteVisibilityCareTeam, updated.Visibility)
	assert.True(t, updated.Edited(), "the trigger moves updated_at past created_at")
	require.NotNil(t, updated.Subject.Date, "a correction keeps the day it was written for")

	require.NoError(t, module.DeleteStudentNote(ctx, peopledirectory.DeleteStudentNote{
		ID: created.ID, StudentID: child.ID, ActorAccountID: author,
	}))

	notes, err = module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities: []string{
				peopledirectory.StudentNoteVisibilityAllStaff,
				peopledirectory.StudentNoteVisibilityCareTeam,
			},
			ReaderAccountID: author,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, notes, "a removed note leaves every timeline, including its author's")
}

// The audience predicate is the security boundary of this feature. It is built
// from four OR branches, and a wrong bracket would widen all of them at once.
func TestStudentNotesAudience(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)
	tenantID := testpkg.Tenant(t)

	child := testpkg.CreateTestStudent(t, db, "Tim", "Audience", "3a")
	author := noteAuthor(t, db, "Ana", "Autorin")
	group := testpkg.CreateTestEducationGroupForTenant(t, db, tenantID, "Audience-Gruppe")
	otherGroup := testpkg.CreateTestEducationGroupForTenant(t, db, tenantID, "Audience-Fremdgruppe")
	date := calendar.NewDate(2026, 9, 9)

	write := func(visibility, body string, educationGroupID *int64) {
		t.Helper()
		_, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
			StudentID: child.ID, AuthorAccountID: author,
			Kind: peopledirectory.StudentNoteKindJournal, Visibility: visibility, Body: body,
			Subject: peopledirectory.StudentNoteSubject{Date: &date, EducationGroupID: educationGroupID},
		})
		require.NoError(t, err)
	}
	write(peopledirectory.StudentNoteVisibilityAllStaff, "Team", nil)
	write(peopledirectory.StudentNoteVisibilityCareTeam, "Betreuungsteam", nil)
	write(peopledirectory.StudentNoteVisibilityGroupLeads, "Leitung", &group.ID)

	list := func(audience peopledirectory.StudentNoteAudience) []string {
		t.Helper()
		notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
			StudentID: child.ID, Audience: audience,
		})
		require.NoError(t, err)
		return bodies(notes)
	}

	stranger := peopledirectory.StudentNoteAudience{
		Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
	}
	assert.Equal(t, []string{"Team"}, list(stranger),
		"a colleague without the child sees only the team note")

	careTeam := peopledirectory.StudentNoteAudience{
		Visibilities: []string{
			peopledirectory.StudentNoteVisibilityAllStaff,
			peopledirectory.StudentNoteVisibilityCareTeam,
		},
	}
	assert.ElementsMatch(t, []string{"Team", "Betreuungsteam"}, list(careTeam),
		"the care team sees its note, but not the leadership's")

	lead := peopledirectory.StudentNoteAudience{
		Visibilities:         []string{peopledirectory.StudentNoteVisibilityAllStaff},
		LedEducationGroupIDs: []int64{group.ID},
	}
	assert.ElementsMatch(t, []string{"Team", "Leitung"}, list(lead),
		"the lead of the referenced group reaches its note")

	wrongLead := peopledirectory.StudentNoteAudience{
		Visibilities:         []string{peopledirectory.StudentNoteVisibilityAllStaff},
		LedEducationGroupIDs: []int64{otherGroup.ID},
	}
	assert.Equal(t, []string{"Team"}, list(wrongLead),
		"leading another group unlocks nothing")

	assert.ElementsMatch(t, []string{"Team", "Betreuungsteam", "Leitung"},
		list(peopledirectory.StudentNoteAudience{
			Visibilities:    []string{peopledirectory.StudentNoteVisibilityAllStaff},
			ReaderAccountID: author,
		}),
		"an author keeps sight of everything they wrote, whatever audience it has")
}

func TestStudentNotesOnlyTheAuthorCorrects(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Nour", "Autorschaft", "3a")
	author := noteAuthor(t, db, "Ben", "Verfasser")
	colleague := noteAuthor(t, db, "Kim", "Kollegin")
	date := calendar.NewDate(2026, 9, 9)

	note, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
		StudentID: child.ID, AuthorAccountID: author,
		Kind:       peopledirectory.StudentNoteKindJournal,
		Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
		Body:       "Meine Beobachtung.",
		Subject:    peopledirectory.StudentNoteSubject{Date: &date},
	})
	require.NoError(t, err)

	correction := peopledirectory.UpdateStudentNote{
		ID: note.ID, StudentID: child.ID, ActorAccountID: colleague,
		Kind:       peopledirectory.StudentNoteKindJournal,
		Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
		Body:       "Umgeschrieben.",
	}
	_, err = module.UpdateStudentNote(ctx, correction)
	require.ErrorIs(t, err, peopledirectory.ErrStudentNoteNotAuthor)

	correction.ActorAccountID = author
	_, err = module.UpdateStudentNote(ctx, correction)
	require.NoError(t, err)
}

// A carried-over hint has no author, so nobody may put their wording under it.
func TestStudentNotesCarriedOverHintIsNotEditable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Elif", "Uebernommen", "3a")
	editor := noteAuthor(t, db, "Tom", "Bearbeiter")

	var noteID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.student_notes (tenant_id, student_id, origin, kind, visibility, body)
		VALUES (?, ?, 'master_data', 'permanent', 'all_staff', ?) RETURNING id`,
		testpkg.Tenant(t), child.ID, "Aus den Betreuernotizen.").Scan(ctx, &noteID))

	_, err := module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: noteID, StudentID: child.ID, ActorAccountID: editor,
		Kind:       peopledirectory.StudentNoteKindPermanent,
		Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
		Body:       "Neu formuliert.",
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNoteImmutable)
}

// Naming a note of another child must not reach past the child the caller
// authorized against — neither on read nor on write.
func TestStudentNotesStayWithTheirChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	mine := testpkg.CreateTestStudent(t, db, "Anna", "Meins", "3a")
	other := testpkg.CreateTestStudent(t, db, "Paul", "Anders", "3a")
	author := noteAuthor(t, db, "Eva", "Schreiberin")
	date := calendar.NewDate(2026, 9, 9)

	note, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
		StudentID: other.ID, AuthorAccountID: author,
		Kind:       peopledirectory.StudentNoteKindJournal,
		Visibility: peopledirectory.StudentNoteVisibilityAllStaff,
		Body:       "Gehört Paul.",
		Subject:    peopledirectory.StudentNoteSubject{Date: &date},
	})
	require.NoError(t, err)

	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: mine.ID, NoteID: note.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities:    []string{peopledirectory.StudentNoteVisibilityAllStaff},
			ReaderAccountID: author,
		},
	})
	require.NoError(t, err)
	assert.Empty(t, notes, "naming a foreign note under the wrong child returns nothing")

	err = module.DeleteStudentNote(ctx, peopledirectory.DeleteStudentNote{
		ID: note.ID, StudentID: mine.ID, ActorAccountID: author,
	})
	require.ErrorIs(t, err, peopledirectory.ErrStudentNoteNotFound)
}

func TestStudentNotesDoNotCrossTenants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	otherTenant := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenant)
	foreign := testpkg.CreateTestStudentForTenant(t, db, otherTenant, "Fremd", "Kind", "2a")
	_, err := db.NewRaw(`
		INSERT INTO users.student_notes (tenant_id, student_id, kind, visibility, body, subject_date)
		VALUES (?, ?, 'journal', 'all_staff', ?, '2026-09-09')`,
		otherTenant, foreign.ID, "Notiz einer anderen Schule.").Exec(ctx)
	require.NoError(t, err)

	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: foreign.ID,
		Audience: peopledirectory.StudentNoteAudience{
			Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
		},
	})
	require.NoError(t, err)
	assert.Empty(t, notes, "another school's note card must stay invisible")
}

// Kind narrows the read to one lifetime: the Stammdaten tab asks for the
// durable hints only, the timeline for everything.
func TestStudentNotesFilterByKind(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := notesModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Rana", "Arten", "3a")
	author := noteAuthor(t, db, "Jo", "Schreiber")
	audience := peopledirectory.StudentNoteAudience{
		Visibilities: []string{peopledirectory.StudentNoteVisibilityAllStaff},
	}
	date := calendar.NewDate(2026, 9, 9)

	for _, note := range []struct{ kind, body string }{
		{peopledirectory.StudentNoteKindPermanent, "Dauerhafter Hinweis"},
		{peopledirectory.StudentNoteKindJournal, "Eintrag"},
	} {
		subject := peopledirectory.StudentNoteSubject{}
		if note.kind == peopledirectory.StudentNoteKindJournal {
			subject.Date = &date
		}
		_, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
			StudentID: child.ID, AuthorAccountID: author, Kind: note.kind,
			Visibility: peopledirectory.StudentNoteVisibilityAllStaff, Body: note.body,
			Subject: subject,
		})
		require.NoError(t, err)
	}

	permanent, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID, Kind: peopledirectory.StudentNoteKindPermanent, Audience: audience,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"Dauerhafter Hinweis"}, bodies(permanent))

	all, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID, Audience: audience,
	})
	require.NoError(t, err)
	assert.Len(t, all, 2, "without a kind the card returns both lifetimes")
}
