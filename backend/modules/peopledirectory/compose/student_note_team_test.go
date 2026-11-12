package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// "Speichern und Team informieren" (#6854) against the real tables: who gets
// a marker, that it clears when read, that it follows its note, and that a
// failed device notification leaves no marker behind.

func noteTeamModule(t *testing.T, db *testpkg.DB) *Directory {
	t.Helper()
	module, err := New(Dependencies{
		StudentDirectory: testpkg.StudentDirectory{},
		DB:               db, StudentNoteDeletionAudit: testStudentNoteDeletionAudit{Db: db}, Observe: func(Observation) {},
	}, nil)
	require.NoError(t, err)
	return module
}

func teamNote(t *testing.T, module *Directory, ctx context.Context, studentID, author int64, visibility string) peopledirectory.StudentNote {
	t.Helper()
	date := calendar.NewDate(2026, 9, 9)
	note, err := module.CreateStudentNote(ctx, peopledirectory.CreateStudentNote{
		StudentID: studentID, AuthorAccountID: author,
		Kind: peopledirectory.StudentNoteKindJournal, Visibility: visibility,
		Body: "Braucht heute Ruhe nach dem Mittag.", Subject: peopledirectory.StudentNoteSubject{Date: &date},
	})
	require.NoError(t, err)
	return note
}

// The capability runs in the caller's school unit, as on the routes.
func inSchool[T any](t *testing.T, db *testpkg.DB, ctx context.Context, call func(context.Context) (T, error)) (value T, err error) {
	t.Helper()
	txErr := testpkg.WithinTenantContext(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		value, err = call(txCtx)
		return err
	})
	if err == nil {
		err = txErr
	}
	return value, err
}

func informTeam(t *testing.T, db *testpkg.DB, module *Directory, ctx context.Context, input peopledirectory.InformStudentNoteTeam) (int, error) {
	t.Helper()
	return inSchool(t, db, ctx, func(txCtx context.Context) (int, error) {
		return module.StudentNoteTeam.InformStudentNoteTeam(txCtx, input)
	})
}

func unreadNotices(t *testing.T, db *testpkg.DB, module *Directory, ctx context.Context, reader int64) ([]peopledirectory.StudentNoteTeamNotice, error) {
	t.Helper()
	return inSchool(t, db, ctx, func(txCtx context.Context) ([]peopledirectory.StudentNoteTeamNotice, error) {
		return module.StudentNoteTeam.ListUnreadStudentNoteTeamNotices(txCtx, reader)
	})
}

func readNotices(t *testing.T, db *testpkg.DB, module *Directory, ctx context.Context, reader, studentID int64) ([]peopledirectory.StudentNoteTeamNotice, error) {
	t.Helper()
	return inSchool(t, db, ctx, func(txCtx context.Context) ([]peopledirectory.StudentNoteTeamNotice, error) {
		return module.StudentNoteTeam.MarkStudentNoteTeamNoticesRead(txCtx, reader, studentID)
	})
}

func noticeNotes(notices []peopledirectory.StudentNoteTeamNotice) []int64 {
	ids := make([]int64, 0, len(notices))
	for _, notice := range notices {
		ids = append(ids, notice.NoteID)
	}
	return ids
}

func TestStudentNoteTeamTellsTheColleaguesAndClearsOnRead(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := noteTeamModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Lio", "Teamnotiz", "2b")
	author := noteAuthor(t, db, "Anja", "Autorin")
	colleague := noteAuthor(t, db, "Ben", "Kollege")
	other := noteAuthor(t, db, "Cem", "Kollege")
	note := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)

	var notified []int64
	informed, err := informTeam(t, db, module, ctx, peopledirectory.InformStudentNoteTeam{
		NoteID: note.ID, StudentID: child.ID, ActorAccountID: author,
		RecipientAccountIDs: []int64{author, colleague, other, colleague},
		Notify: func(_ context.Context, recipients []int64) error {
			notified = recipients
			return nil
		},
	})
	require.NoError(t, err)
	assert.Equal(t, 2, informed, "the author is not told about their own note, a repeated colleague once")
	assert.Equal(t, []int64{colleague, other}, notified, "the device notification reaches the same colleagues")

	unread, err := unreadNotices(t, db, module, ctx, colleague)
	require.NoError(t, err)
	require.Len(t, unread, 1)
	assert.Equal(t, note.ID, unread[0].NoteID)
	assert.Equal(t, child.ID, unread[0].StudentID)
	assert.Equal(t, "Lio", unread[0].StudentFirstName)
	assert.Equal(t, "Teamnotiz", unread[0].StudentLastName)
	assert.Equal(t, "Anja Autorin", unread[0].AuthorName)

	own, err := unreadNotices(t, db, module, ctx, author)
	require.NoError(t, err)
	assert.Empty(t, own, "the author has no marker")

	remaining, err := readNotices(t, db, module, ctx, colleague, child.ID)
	require.NoError(t, err)
	assert.Empty(t, remaining, "opening the child's notes clears the colleague's marker")
	stillUnread, err := unreadNotices(t, db, module, ctx, other)
	require.NoError(t, err)
	assert.Equal(t, []int64{note.ID}, noticeNotes(stillUnread), "one colleague reading clears nobody else's marker")

	_, err = informTeam(t, db, module, ctx, peopledirectory.InformStudentNoteTeam{
		NoteID: note.ID, StudentID: child.ID, ActorAccountID: author, RecipientAccountIDs: []int64{colleague},
	})
	require.NoError(t, err)
	again, err := unreadNotices(t, db, module, ctx, colleague)
	require.NoError(t, err)
	assert.Equal(t, []int64{note.ID}, noticeNotes(again), "telling the team again makes the note unread again")

	t.Run("another school never sees the markers", func(t *testing.T) {
		foreign, err := unreadNotices(t, db, module, testpkg.OwnCtx(t), colleague)
		require.NoError(t, err)
		assert.Empty(t, foreign)
	})
}

func TestStudentNoteTeamOnlyTheAuthorSharesANoteForTheWholeTeam(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := noteTeamModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Mia", "Teamregel", "2b")
	sibling := testpkg.CreateTestStudent(t, db, "Noa", "Teamregel", "2b")
	author := noteAuthor(t, db, "Anja", "Regel")
	colleague := noteAuthor(t, db, "Ben", "Regel")
	wide := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	narrow := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityCareTeam)

	inform := func(noteID, studentID, actor int64) error {
		_, err := informTeam(t, db, module, ctx, peopledirectory.InformStudentNoteTeam{
			NoteID: noteID, StudentID: studentID, ActorAccountID: actor, RecipientAccountIDs: []int64{author, colleague},
		})
		return err
	}
	require.ErrorIs(t, inform(wide.ID, child.ID, colleague), peopledirectory.ErrStudentNoteNotAuthor)
	require.ErrorIs(t, inform(narrow.ID, child.ID, author), peopledirectory.ErrStudentNoteTeamAudience,
		"a note for the care team only has no recipient list for the whole team")
	require.ErrorIs(t, inform(wide.ID, sibling.ID, author), peopledirectory.ErrStudentNoteNotFound,
		"a note is shared only through the child it belongs to")

	unread, err := unreadNotices(t, db, module, ctx, colleague)
	require.NoError(t, err)
	assert.Empty(t, unread)
}

// A failed device notification rolls the markers back: the caller reports
// the team as not informed, and nobody holds a marker the author was told
// failed.
func TestStudentNoteTeamFailedNotificationLeavesNoMarker(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := noteTeamModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Ole", "Fehlschlag", "4a")
	author := noteAuthor(t, db, "Anja", "Fehlschlag")
	colleague := noteAuthor(t, db, "Ben", "Fehlschlag")
	note := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)

	outboxDown := errors.New("push outbox unavailable")
	var informErr error
	// The caller's unit commits anyway, as a request whose handler answered
	// with a refusal would: the savepoint alone has to take the markers back.
	require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, tenant.FromContext(ctx), func(txCtx context.Context) error {
		_, informErr = module.StudentNoteTeam.InformStudentNoteTeam(txCtx, peopledirectory.InformStudentNoteTeam{
			NoteID: note.ID, StudentID: child.ID, ActorAccountID: author, RecipientAccountIDs: []int64{colleague},
			Notify: func(context.Context, []int64) error { return outboxDown },
		})
		return nil
	}))
	require.ErrorIs(t, informErr, outboxDown)

	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_note_team_notices WHERE note_id = ?`, note.ID).Scan(ctx, &rows))
	assert.Zero(t, rows, "the markers roll back with the failed notification, although the caller committed")

	notes, err := module.ListStudentNotes(ctx, peopledirectory.StudentNoteFilter{
		StudentID: child.ID, Audience: peopledirectory.StudentNoteAudience{ReaderAccountID: author},
	})
	require.NoError(t, err)
	assert.Len(t, notes, 1, "the note itself stays saved")
}

// The marker follows its note and its recipient: a narrower audience, a
// deletion and a recipient who left the school end it.
func TestStudentNoteTeamMarkerFollowsNoteAndRecipient(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := noteTeamModule(t, db)
	ctx := testpkg.Ctx(t)

	child := testpkg.CreateTestStudent(t, db, "Pia", "Folge", "1a")
	author := noteAuthor(t, db, "Anja", "Folge")
	colleague := noteAuthor(t, db, "Ben", "Folge")
	leaver := noteAuthor(t, db, "Cem", "Folge")
	narrowed := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	deleted := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	kept := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	for _, note := range []peopledirectory.StudentNote{narrowed, deleted, kept} {
		_, err := informTeam(t, db, module, ctx, peopledirectory.InformStudentNoteTeam{
			NoteID: note.ID, StudentID: child.ID, ActorAccountID: author, RecipientAccountIDs: []int64{colleague, leaver},
		})
		require.NoError(t, err)
	}

	_, err := module.UpdateStudentNote(ctx, peopledirectory.UpdateStudentNote{
		ID: narrowed.ID, StudentID: child.ID, ActorAccountID: author,
		Kind: narrowed.Kind, Visibility: peopledirectory.StudentNoteVisibilityCareTeam, Body: narrowed.Body,
		SubjectDate: narrowed.Subject.Date,
	})
	require.NoError(t, err)
	require.NoError(t, module.DeleteStudentNote(ctx, peopledirectory.DeleteStudentNote{
		ID: deleted.ID, StudentID: child.ID, ActorAccountID: author,
		Authorization: peopledirectory.StudentNoteDeleteAuthorization{Admin: true},
	}))

	unread, err := unreadNotices(t, db, module, ctx, colleague)
	require.NoError(t, err)
	assert.Equal(t, []int64{kept.ID}, noticeNotes(unread))

	testpkg.UnclaimTestAccount(t, db, leaver)
	var rows int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_note_team_notices WHERE recipient_account_id = ?`, leaver).Scan(ctx, &rows))
	assert.Zero(t, rows, "leaving the school takes the markers along")
	// Give the account back to the test's school, so it leaves no shared row.
	testpkg.EnsureAccountTenant(t, db, leaver, testpkg.Tenant(t))
}

// Departed children must not consume the header's bounded result window.
func TestStudentNoteTeamDepartedChildrenDoNotHideEnrolledNotices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := noteTeamModule(t, db)
	ctx := testpkg.Ctx(t)
	child := testpkg.CreateTestStudent(t, db, "Mia", "Aktiv", "2b")
	departed := testpkg.CreateTestStudent(t, db, "Noa", "Abgegangen", "4a")
	author := noteAuthor(t, db, "Anja", "Autorin")
	reader := noteAuthor(t, db, "Ben", "Leser")
	note := teamNote(t, module, ctx, child.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	_, err := informTeam(t, db, module, ctx, peopledirectory.InformStudentNoteTeam{
		NoteID: note.ID, StudentID: child.ID, ActorAccountID: author, RecipientAccountIDs: []int64{reader},
	})
	require.NoError(t, err)
	departedNote := teamNote(t, module, ctx, departed.ID, author, peopledirectory.StudentNoteVisibilityAllStaff)
	// Fill the entire newer window in bulk, without repeating the write flow.
	_, err = db.NewRaw(`INSERT INTO users.student_notes
		(tenant_id, student_id, author_account_id, origin, kind, visibility, body, subject_date)
		SELECT tenant_id, student_id, author_account_id, origin, kind, visibility, body, subject_date
		FROM users.student_notes CROSS JOIN generate_series(1, 49) WHERE id = ?`, departedNote.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO users.student_note_team_notices
		(tenant_id, note_id, recipient_account_id, informed_at)
		SELECT tenant_id, id, ?, clock_timestamp() FROM users.student_notes WHERE student_id = ?`, reader, departed.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("status = ?", peopledirectory.StudentStatusAlumnus).
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("student_profile_id = ?", departed.ID).Exec(ctx)
	require.NoError(t, err)

	unread, err := unreadNotices(t, db, module, ctx, reader)
	require.NoError(t, err)
	require.Equal(t, []int64{note.ID}, noticeNotes(unread), "50 newer notices for a departed child cannot hide an enrolled child's notice")
}
