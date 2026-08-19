package users_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
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
	return usersService.NewStudentService(factory.Student, factory.PrivacyConsent, factory.StudentCompanion, nil)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
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
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
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

func TestStudentService_ReplaceCompanions_ExtensionRecordsCompanionAudit(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "Confirm")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)
	defer testpkg.CleanupStaffFixtures(t, db, staff.ID)

	ctx := context.WithValue(testpkg.Ctx(t), jwt.CtxClaims, jwt.AppClaims{
		ID:        int(account.ID),
		FirstName: "Clara",
		LastName:  "Confirm",
	})
	factory := repositories.NewFactory(db)
	audit := usersService.NewStudentAuditService(factory.StudentFieldEdit, slog.Default())
	service := usersService.NewStudentService(
		factory.Student,
		factory.PrivacyConsent,
		factory.StudentCompanion,
		audit,
	)

	subject := testpkg.CreateTestStudent(t, db, "AuditSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "AuditCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "fri")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:           []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
		AccompaniedDays: map[string]bool{"mon": true},
	})
	require.NoError(t, err)
	require.Len(t, conflicts, 1)

	conflicts, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
		AccompaniedDays:      map[string]bool{"mon": true},
		ExtendCompanionPlans: true,
		ActorAccountID:       account.ID,
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	history, err := audit.GetChangeHistory(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, auditModels.StudentFieldDepartureDays, history[0].FieldName)
	assert.Equal(t, account.ID, history[0].EditedBy)
	assert.Equal(t, "Clara Confirm", history[0].EditedByName)
}

// TestStudentUpdate_CompanionLinkSatisfiesNoteRequirement pins that a child
// whose "mit wem" is answered by a structured link can still be saved by a
// caller that knows nothing about links (status days, sick/excused clearing,
// care-request approval). Before the link was derived in the repository, those
// paths failed with ErrDepartureCompanionNoteRequired.
func TestStudentUpdate_CompanionLinkSatisfiesNoteRequirement(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
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

// TestStudentService_ReplaceCompanions_RefusesOrphaningCompanion pins that
// removing a link may not strand the child at the far end of it. The edge is
// stored once and read from both cards, so dropping it edits the OTHER child's
// record: if that link was their only "mit wem" detail, they are left with an
// accompanied plan, no note and no link — Stammdaten that contradict themselves
// and block every later unrelated edit of that child.
func TestStudentService_ReplaceCompanions_RefusesOrphaningCompanion(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "OrphanSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "OrphanCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	// The link is now the companion's only "mit wem" detail.
	clearCompanionNote(t, db, companion.ID)

	// ACT — the subject drops the companion from their list.
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{})
	assert.ErrorIs(t, err, usersService.ErrCompanionWouldLoseDeparture)

	// The refusal happens before any write: the link is still there.
	links, err := service.ListCompanions(ctx, companion.ID)
	require.NoError(t, err)
	assert.Len(t, links, 1, "nothing may have been deleted")

	// The same removal is fine once the companion carries their own note.
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{})
	require.NoError(t, err)
}

// TestStudentService_ReplaceCompanions_RefusesStrandingRemovedWeekday pins that
// the removal check is per weekday, mirroring Student.Validate's cover check. A
// pair that stays linked on Monday still strands the far child on Tuesday when
// the Tuesday edge was their only "mit wem" answer for that day — the surviving
// Monday link must not vouch for it.
func TestStudentService_ReplaceCompanions_RefusesStrandingRemovedWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "DaySubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "DayCompanion", "Companion", "1a")
	third := testpkg.CreateTestStudent(t, db, "DayThird", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID, third.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID, third.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon", "tue")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon", "tue")
	setAccompaniedDays(t, db, ctx, third.ID, "tue")

	_, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon", "tue"}}},
	})
	require.NoError(t, err)

	// Both of the companion's accompanied days are now answered ONLY by the
	// subject's edges.
	clearCompanionNote(t, db, companion.ID)

	// ACT — the subject keeps the link but drops its Tuesday.
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionWouldLoseDeparture)

	// The refusal happens before any write: the Tuesday edge is still there.
	links, err := service.ListCompanions(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, []string{"mon", "tue"}, links[0].Weekdays)

	// A THIRD child covering the removed Tuesday makes the same narrowing legal
	// — coverage counts per day, not per surviving companion.
	_, err = service.ReplaceCompanions(ctx, third.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"tue"}}},
	})
	require.NoError(t, err)

	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
}

// TestStudentService_CheckCompanionTrim_RefusesStrandingRemovedWeekday pins the
// per-weekday rule for the plan-narrowing path: the subject dropping "Anderes
// Kind" from Tuesday trims the Tuesday edge, and the surviving Monday edge of
// the SAME pair must not count as the far child's Tuesday answer.
func TestStudentService_CheckCompanionTrim_RefusesStrandingRemovedWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "TrimDaySubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "TrimDayCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon", "tue")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon", "tue")

	_, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon", "tue"}}},
	})
	require.NoError(t, err)
	clearCompanionNote(t, db, companion.ID)

	assert.ErrorIs(t,
		service.CheckCompanionTrim(ctx, subject.ID, map[string]bool{"mon": true}),
		usersService.ErrCompanionWouldLoseDeparture,
	)
	assert.ErrorIs(t, func() error {
		_, err := service.TrimCompanionsToDays(ctx, subject.ID, map[string]bool{"mon": true})
		return err
	}(), usersService.ErrCompanionWouldLoseDeparture)

	// Nothing was trimmed by the refused calls.
	links, err := service.ListCompanions(ctx, companion.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, []string{"mon", "tue"}, links[0].Weekdays)
}

// TestStudentService_CheckCompanionTrim_RefusesOrphaningCompanion pins the same
// rule for the path where the client narrows its own departure plan WITHOUT
// submitting a list. The trim drops whole links too, and it runs after the
// update's first writes — so the refusal has to be available before them.
func TestStudentService_CheckCompanionTrim_RefusesOrphaningCompanion(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "TrimOrphanSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "TrimOrphanCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")

	_, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	clearCompanionNote(t, db, companion.ID)

	// Dropping "Anderes Kind" from Monday would drop the only edge.
	assert.ErrorIs(t,
		service.CheckCompanionTrim(ctx, subject.ID, map[string]bool{}),
		usersService.ErrCompanionWouldLoseDeparture,
	)
	// Keeping Monday changes nothing, so there is nothing to refuse.
	require.NoError(t, service.CheckCompanionTrim(ctx, subject.ID, map[string]bool{"mon": true}))
}

// TestStudentService_ReplaceCompanions_EnforcesLimitOnBothEnds pins that
// MaxStudentCompanions caps EVERY child's list, not just the submitted one. An
// edge counts on both cards, so checking only the subject would let eleven
// children each name the same child as their single companion and leave that
// child over the cap — and unable to edit their own list afterwards, because
// the full list is then rejected.
func TestStudentService_ReplaceCompanions_EnforcesLimitOnBothEnds(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	// One popular child, MaxStudentCompanions children already linked to them,
	// and one more trying to join.
	popular := testpkg.CreateTestStudent(t, db, "PopularChild", "Companion", "1a")
	ids := []int64{popular.ID}
	peers := make([]int64, 0, usersService.MaxStudentCompanions+1)
	for i := 0; i <= usersService.MaxStudentCompanions; i++ {
		peer := testpkg.CreateTestStudent(t, db, "LimitPeer", "Companion", "1a")
		peers = append(peers, peer.ID)
		ids = append(ids, peer.ID)
	}
	defer testpkg.CleanupActivityFixtures(t, db, ids...)
	defer cleanupCompanionEdges(t, db, ids...)

	for _, id := range ids {
		setAccompaniedDays(t, db, ctx, id, "mon")
	}

	for _, peer := range peers[:usersService.MaxStudentCompanions] {
		_, err := service.ReplaceCompanions(ctx, peer, usersService.CompanionUpdate{
			Links: []userModels.CompanionLink{{CompanionStudentID: popular.ID, Weekdays: []string{"mon"}}},
		})
		require.NoError(t, err)
	}

	// ACT — one more child submits a single-entry list, which is well under the
	// cap for them but pushes the popular child over it.
	_, err := service.ReplaceCompanions(ctx, peers[usersService.MaxStudentCompanions], usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: popular.ID, Weekdays: []string{"mon"}}},
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionAtLimit)

	// Re-submitting an unchanged list is NOT rejected: the subject's own edges
	// are excluded from the companion's degree.
	_, err = service.ReplaceCompanions(ctx, peers[0], usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: popular.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
}

// TestStudentService_ReplaceCompanions_ExtensionIsPerWeekday pins that the
// caller's confirmation authorizes the WEEKDAYS it named and nothing else. The
// companion rows are unlocked while a human answers the question, so the
// conflict can grow a day in the meantime — widening that day too would change
// a child's departure permission nobody was asked about (#1694).
func TestStudentService_ReplaceCompanions_ExtensionIsPerWeekday(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)
	studentRepo := repositories.NewFactory(db).Student

	subject := testpkg.CreateTestStudent(t, db, "PerDaySubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "PerDayCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon", "tue")
	// The companion allows neither day, so both are conflicts — but only Monday
	// was confirmed.
	setAccompaniedDays(t, db, ctx, companion.ID, "fri")

	links := []userModels.CompanionLink{
		{CompanionStudentID: companion.ID, Weekdays: []string{"mon", "tue"}},
	}
	_, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                links,
		AccompaniedDays:      map[string]bool{"mon": true, "tue": true},
		ExtendCompanionPlans: true,
		AuthorizedExtensions: map[int64]map[string]bool{companion.ID: {"mon": true}},
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionExtensionNotAuthorized)

	stored, err := studentRepo.FindByID(ctx, companion.ID)
	require.NoError(t, err)
	allowed := userModels.AccompaniedWeekdays(stored.AllowedDepartureModes, stored.DepartureDays)
	assert.False(t, allowed["mon"], "nothing may have been widened")
	assert.False(t, allowed["tue"], "the unconfirmed day especially")

	// Confirming both days makes the identical request go through.
	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:                links,
		AccompaniedDays:      map[string]bool{"mon": true, "tue": true},
		ExtendCompanionPlans: true,
		AuthorizedExtensions: map[int64]map[string]bool{companion.ID: {"mon": true, "tue": true}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	stored, err = studentRepo.FindByID(ctx, companion.ID)
	require.NoError(t, err)
	allowed = userModels.AccompaniedWeekdays(stored.AllowedDepartureModes, stored.DepartureDays)
	assert.True(t, allowed["mon"])
	assert.True(t, allowed["tue"])
	assert.True(t, allowed["fri"], "extension stays additive")
}

// TestStudentUpdate_CompanionLinkCoversOnlyItsWeekdays pins that a link answers
// "mit wem" for its own weekdays only. A child that walks with someone on
// Monday but claims "Anderes Kind" on Tuesday as well still has no answer for
// Tuesday, so the departure plan must not be saveable without one.
func TestStudentUpdate_CompanionLinkCoversOnlyItsWeekdays(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)
	studentRepo := repositories.NewFactory(db).Student

	subject := testpkg.CreateTestStudent(t, db, "CoverSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "CoverCompanion", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon", "tue")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")

	conflicts, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)
	require.Empty(t, conflicts)

	// The Monday link is now the subject's only "mit wem" detail — Tuesday has
	// none.
	clearCompanionNote(t, db, subject.ID)

	fresh, err := studentRepo.FindByID(ctx, subject.ID)
	require.NoError(t, err)
	require.Nil(t, fresh.DepartureCompanionNote)
	fresh.SchoolClass = "2c"

	err = studentRepo.Update(ctx, fresh)
	assert.ErrorIs(t, err, userModels.ErrDepartureCompanionNoteRequired,
		"the uncovered accompanied Tuesday must still demand an answer")

	// Dropping the uncovered day (or covering it) makes the same save work.
	fresh.AllowedDepartureModes = userModels.AllowedDepartureModes{
		"mon": []userModels.DepartureMode{userModels.DepartureAccompanied},
	}
	fresh.DepartureDays = nil
	require.NoError(t, studentRepo.Update(ctx, fresh))
}

// TestStudentService_ReplaceCompanions_RefusesStaleList pins the lost-update
// guard: the submitted list REPLACES the stored one, so two staff members
// editing the same child from the same snapshot would otherwise both send
// everything they saw and the later write would delete the link the earlier one
// had just committed. The row locks order the two writes; only the fingerprint
// comparison notices that the second one describes a list that no longer exists.
func TestStudentService_ReplaceCompanions_RefusesStaleList(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "StaleSubject", "Companion", "1a")
	first := testpkg.CreateTestStudent(t, db, "StaleFirst", "Companion", "1a")
	second := testpkg.CreateTestStudent(t, db, "StaleSecond", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, first.ID, second.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, first.ID, second.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, first.ID, "mon")
	setAccompaniedDays(t, db, ctx, second.ID, "mon")

	// Both editors load the same (empty) list.
	loaded, err := service.ListCompanions(ctx, subject.ID)
	require.NoError(t, err)
	snapshot := userModels.CompanionLinksFingerprint(loaded)

	// Editor A saves first: the child now walks with `first`.
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:               []userModels.CompanionLink{{CompanionStudentID: first.ID, Weekdays: []string{"mon"}}},
		ExpectedFingerprint: &snapshot,
	})
	require.NoError(t, err)

	// ACT — editor B saves the list they built on the pre-A snapshot.
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links:               []userModels.CompanionLink{{CompanionStudentID: second.ID, Weekdays: []string{"mon"}}},
		ExpectedFingerprint: &snapshot,
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionsChanged)

	// Nothing was written: editor A's link survives untouched.
	links, err := service.ListCompanions(ctx, subject.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, first.ID, links[0].CompanionStudentID)

	// The read-only pre-check refuses it too, so the HTTP path can answer 409
	// before its transaction writes anything.
	_, err = service.CheckCompanionConflicts(ctx, subject.ID, usersService.CompanionUpdate{
		Links:               []userModels.CompanionLink{{CompanionStudentID: second.ID, Weekdays: []string{"mon"}}},
		ExpectedFingerprint: &snapshot,
	})
	assert.ErrorIs(t, err, usersService.ErrCompanionsChanged)

	// Reloading makes the same edit succeed.
	reloaded, err := service.ListCompanions(ctx, subject.ID)
	require.NoError(t, err)
	fresh := userModels.CompanionLinksFingerprint(reloaded)
	_, err = service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{
			{CompanionStudentID: first.ID, Weekdays: []string{"mon"}},
			{CompanionStudentID: second.ID, Weekdays: []string{"mon"}},
		},
		ExpectedFingerprint: &fresh,
	})
	require.NoError(t, err)
}

// TestStudentService_ListCompanionsForStudents pins the bulk read the offline
// lists use: one call, names joined in, and children without links simply
// absent from the map.
func TestStudentService_ListCompanionsForStudents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	ctx := testpkg.Ctx(t)
	service := newCompanionTestService(db)

	subject := testpkg.CreateTestStudent(t, db, "BulkSubject", "Companion", "1a")
	companion := testpkg.CreateTestStudent(t, db, "BulkCompanion", "Companion", "1a")
	lonely := testpkg.CreateTestStudent(t, db, "BulkLonely", "Companion", "1a")
	defer testpkg.CleanupActivityFixtures(t, db, subject.ID, companion.ID, lonely.ID)
	defer cleanupCompanionEdges(t, db, subject.ID, companion.ID, lonely.ID)

	setAccompaniedDays(t, db, ctx, subject.ID, "mon")
	setAccompaniedDays(t, db, ctx, companion.ID, "mon")

	_, err := service.ReplaceCompanions(ctx, subject.ID, usersService.CompanionUpdate{
		Links: []userModels.CompanionLink{{CompanionStudentID: companion.ID, Weekdays: []string{"mon"}}},
	})
	require.NoError(t, err)

	byStudent, err := service.ListCompanionsForStudents(ctx, []int64{subject.ID, companion.ID, lonely.ID})
	require.NoError(t, err)

	require.Len(t, byStudent[subject.ID], 1)
	assert.Equal(t, companion.ID, byStudent[subject.ID][0].CompanionStudentID)
	assert.Equal(t, "BulkCompanion", byStudent[subject.ID][0].FirstName)

	// The edge is undirected, so it shows up on the far child too — with THEIR
	// companion's name, not their own.
	require.Len(t, byStudent[companion.ID], 1)
	assert.Equal(t, subject.ID, byStudent[companion.ID][0].CompanionStudentID)
	assert.Equal(t, "BulkSubject", byStudent[companion.ID][0].FirstName)

	assert.NotContains(t, byStudent, lonely.ID)
}
