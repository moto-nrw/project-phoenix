package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// newCompanionTestService wires the real repositories behind the student
// service — the companion rules span three tables, so a mock would only test
// the mock.
func newCompanionTestService(db *bun.DB) usersService.StudentService {
	factory := repositories.NewFactory(db)
	return usersService.NewStudentService(factory.Student, factory.PrivacyConsent, factory.StudentCompanion)
}

// cleanupCompanionEdges removes every edge touching the given students. The
// edges are written by a delete+insert, so the test never learns their row ids.
func cleanupCompanionEdges(t *testing.T, db *bun.DB, studentIDs ...int64) {
	t.Helper()
	_, err := db.NewDelete().
		TableExpr("users.student_companions").
		Where("student_low_id IN (?) OR student_high_id IN (?)", bun.List(studentIDs), bun.List(studentIDs)).
		Exec(context.Background())
	if err != nil {
		t.Logf("Warning: failed to cleanup users.student_companions: %v", err)
	}
}

// setAccompaniedDays gives the student an "Anderes Kind" departure plan on the
// given weekdays, which is the precondition for carrying a companion link. The
// free-text note comes along because an accompanied plan must say "mit wem" and
// there is no link yet at this point.
func setAccompaniedDays(t *testing.T, db *bun.DB, ctx context.Context, studentID int64, days ...string) *userModels.Student {
	t.Helper()
	repo := repositories.NewFactory(db).Student

	student, err := repo.FindByID(ctx, studentID)
	require.NoError(t, err)
	require.NotNil(t, student)

	note := "Nachbarskind"
	student.AllowedDepartureModes = userModels.WithAccompaniedDays(student.AllowedDepartureModes, days)
	student.DepartureCompanionNote = &note
	require.NoError(t, repo.Update(ctx, student))
	return student
}

// clearCompanionNote drops the free-text note straight in SQL, leaving a child
// whose "mit wem" is answered only by the structured link. The API cannot
// express that order (the note is required until a link exists), but stored
// rows reach it as soon as a note-carrying child gets linked and the note is
// removed.
func clearCompanionNote(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("departure_companion_note = NULL").
		Where("id = ?", studentID).
		Exec(context.Background())
	require.NoError(t, err)
}

// TestStudentService_TrimCompanionsToDays_DropsRemovedWeekdays pins the rule
// that a link may not outlive the departure day that justifies it: narrowing
// the plan from Mon+Tue to Mon must drop the Tuesday edge, or the Kindersuche
// keeps grouping the children on a day the Stammdaten forbid.
func TestStudentService_TrimCompanionsToDays_DropsRemovedWeekdays(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "TrimSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "TrimCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon", "tue")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon", "tue")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{
			{CompanionStudentID: companion.ID, Weekdays: []string{"mon", "tue"}},
		},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	// ACT — the plan now only allows Monday.
	remaining, err := service.TrimCompanionsToDays(ctx, subject.ID, map[string]bool{"mon": true})
	require.NoError(t, err)

	require.Len(t, remaining, 1)
	assert.Equal(t, []string{"mon"}, remaining[0].Weekdays)

	// The trim is symmetric: the Tuesday edge is gone from BOTH cards.
	fromCompanion, err := service.ListCompanions(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, fromCompanion, 1)
	assert.Equal(t, []string{"mon"}, fromCompanion[0].Weekdays)
}

// TestStudentService_CheckCompanionConflicts_ValidatesConfirmedRetry pins that
// the pre-write check still runs the full validation when the client already
// confirmed the plan extension. The check is the ONLY chance to reject: the
// tenant transaction commits on every non-5xx response, so a rejection raised
// after the first write would keep that write.
func TestStudentService_CheckCompanionConflicts_ValidatesConfirmedRetry(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "ConfirmSubject", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID)
	defer cleanupCompanionEdges(t, db, subject.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")

	// A companion that does not exist (deleted between picking and saving).
	_, err := service.CheckCompanionConflicts(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                []userModels.CompanionLink{{CompanionStudentID: subject.ID + 9_000_000, Weekdays: []string{"mon"}}},
		AccompaniedDays:      map[string]bool{"mon": true},
		ExtendCompanionPlans: true,
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionNotFound)

	// More links than the cap.
	tooMany := make([]userModels.CompanionLink, 0, usersService.MaxStudentCompanions+1)
	for i := range usersService.MaxStudentCompanions + 1 {
		tooMany = append(tooMany, userModels.CompanionLink{
			CompanionStudentID: subject.ID + int64(i) + 1,
			Weekdays:           []string{"mon"},
		})
	}
	_, err = service.CheckCompanionConflicts(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                tooMany,
		AccompaniedDays:      map[string]bool{"mon": true},
		ExtendCompanionPlans: true,
	})
	assert.ErrorIs(t, err, usersService.ErrTooManyCompanions)

	// A weekday the subject's own plan does not allow.
	_, err = service.CheckCompanionConflicts(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                []userModels.CompanionLink{{CompanionStudentID: subject.ID + 9_000_000, Weekdays: []string{"fri"}}},
		AccompaniedDays:      map[string]bool{"mon": true},
		ExtendCompanionPlans: true,
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionDayNotAllowed)
}

// TestStudentService_ReplaceCompanions_ExtendPreservesCompanionFields pins that
// widening a companion's departure plan does not roll back the rest of that
// child's record: the write must come from a freshly locked row, not from the
// snapshot the conflict check read.
func TestStudentService_ReplaceCompanions_ExtendPreservesCompanionFields(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)
	studentRepo := repositories.NewFactory(db).Student

	subject := testpkg.CreateTestStudent(t, db, "ExtendSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "ExtendCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	// The companion may only walk with another child on Friday, so Monday is a
	// conflict the caller has to confirm.
	setAccompaniedDays(t, db, ctx, companion.ID, "fri")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:           []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
		AccompaniedDays: map[string]bool{"mon": true},
	})
	require.NoError(t, err)
	require.Len(t, conflicts, 1)
	assert.Equal(t, companion.ID, conflicts[0].StudentID)
	assert.Equal(t, []string{"mon"}, conflicts[0].Weekdays)

	// A change to the companion committed between the check and the confirmed
	// retry — the classic lost-update shape.
	updated, err := studentRepo.FindByID(ctx, companion.ID)
	require.NoError(t, err)
	updated.SchoolClass = "4b"
	require.NoError(t, studentRepo.Update(ctx, updated))

	conflicts, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
		AccompaniedDays:      map[string]bool{"mon": true},
		ExtendCompanionPlans: true,
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	stored, err := studentRepo.FindByID(ctx, companion.ID)
	require.NoError(t, err)
	assert.Equal(t, "4b", stored.SchoolClass, "extending the departure plan must not revert unrelated fields")

	allowed := userModels.AccompaniedWeekdays(stored.AllowedDepartureModes, stored.DepartureDays)
	assert.True(t, allowed["mon"], "Monday must have been added")
	assert.True(t, allowed["fri"], "Friday must survive — extension is additive")
}

// TestStudentUpdate_CompanionLinkSatisfiesNoteRequirement pins that a child
// whose "mit wem" is answered by a structured link can still be saved by a
// caller that knows nothing about links (status days, sick/excused clearing,
// care-request approval). Before the link was derived in the repository, those
// paths failed with ErrDepartureCompanionNoteRequired.
func TestStudentUpdate_CompanionLinkSatisfiesNoteRequirement(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := testpkg.TenantContext(1)
	service := newCompanionTestService(db)
	studentRepo := repositories.NewFactory(db).Student

	subject := testpkg.CreateTestStudent(t, db, "NoteSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "NoteCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	// The link now answers "mit wem", so the note is removed.
	clearCompanionNote(t, db, subject.ID)

	// A plain reload + update, exactly like an unrelated service would do.
	fresh, err := studentRepo.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.DepartureCompanionNote, "arrange step must have cleared the note")
	fresh.SchoolClass = "2c"

	require.NoError(t, studentRepo.Update(ctx, fresh))

	stored, err := studentRepo.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	assert.Equal(t, "2c", stored.SchoolClass)
}
