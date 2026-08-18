package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupClassListEntryTransitionTest wires the transition service WITH the
// class-list-entry repos — the shared setup leaves them nil, which disables
// the remap entirely.
func setupClassListEntryTransitionTest(t *testing.T) (*educationService.GradeTransitionService, *bun.DB, func()) {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	service := educationService.NewGradeTransitionService(educationService.GradeTransitionServiceDependencies{
		TransitionRepo:      educationRepo.NewGradeTransitionRepository(db),
		StudentRepo:         usersRepo.NewStudentRepository(db),
		PersonRepo:          usersRepo.NewPersonRepository(db),
		ClassTeacherRepo:    educationRepo.NewClassTeacherRepository(db),
		StaffRepo:           usersRepo.NewStaffRepository(db),
		ClassListEntryRepo:  usersRepo.NewClassListEntryRepository(db),
		ClassListEntryAudit: auditRepo.NewClassListEntryChangeRepository(db),
		DB:                  db,
	})

	cleanup := func() {
		_ = db.Close()
	}

	return service, db, cleanup
}

// classListEntryClassesOf reads the current classes an entry name sits in,
// straight from the table, sorted.
func classListEntryClassesOf(t *testing.T, db *bun.DB, ctx context.Context, firstName, lastName string) []string {
	t.Helper()
	var classes []string
	err := db.NewSelect().
		TableExpr("users.class_list_entries").
		Column("school_class").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		OrderExpr("LOWER(BTRIM(school_class)) ASC").
		Scan(ctx, &classes)
	require.NoError(t, err)
	return classes
}

func cleanupClassListEntriesByName(t *testing.T, db *bun.DB, names [][2]string) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	for _, name := range names {
		_, _ = db.NewDelete().TableExpr("users.class_list_entries").
			Where("first_name = ? AND last_name = ?", name[0], name[1]).
			Exec(ctx)
	}
}

// The class-list entries must follow the school-year rollover like the
// students and the Klassenlehrer assignments (#2382): promoted classes carry
// their entries along, graduating classes lose them — and the revert replays
// the recorded ledger backwards, restoring exactly what the apply rewrote
// while leaving pre-existing and newly created entries untouched (#2399
// review blocker).
func TestGradeTransitionService_Apply_RemapsClassListEntries(t *testing.T) {
	service, db, cleanup := setupClassListEntryTransitionTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-cle@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1a-%s", suffix)
	class2 := fmt.Sprintf("2a-%s", suffix)
	class4 := fmt.Sprintf("4a-%s", suffix)
	classOther := fmt.Sprintf("1b-%s", suffix)

	// One child per mapped class so the transition has a locked cohort.
	student1 := testpkg.CreateTestStudent(t, db, "CleMove", "Cohort1", class1)
	student4 := testpkg.CreateTestStudent(t, db, "CleMove", "Cohort4", class4)
	defer testpkg.CleanupActivityFixtures(t, db, student1.ID, student4.ID)

	promoted := testpkg.CreateTestClassListEntry(t, db, "CleZoe", "Promoted", class1)
	graduated := testpkg.CreateTestClassListEntry(t, db, "CleBen", "Graduated", class4)
	unrelated := testpkg.CreateTestClassListEntry(t, db, "CleUli", "Unrelated", classOther)
	defer testpkg.CleanupClassListEntryFixtures(t, db, promoted.ID, graduated.ID, unrelated.ID)
	defer cleanupClassListEntriesByName(t, db, [][2]string{
		{"CleZoe", "Promoted"}, {"CleBen", "Graduated"}, {"CleUli", "Unrelated"}, {"CleNeu", "Dazwischen"},
	})

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class4, nil) // graduates
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleZoe", "Promoted"),
		"promoted class must carry its entry along")
	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleBen", "Graduated"),
		"graduating class must lose its entry")
	assert.Equal(t, []string{classOther}, classListEntryClassesOf(t, db, ctx, "CleUli", "Unrelated"),
		"an entry in an unmapped class must not move")

	// Created BETWEEN apply and revert, in the rename-target class: the
	// mapping-derived reverse rename would drag it to class1 — the ledger
	// replay must leave it alone.
	created := testpkg.CreateTestClassListEntry(t, db, "CleNeu", "Dazwischen", class2)
	defer testpkg.CleanupClassListEntryFixtures(t, db, created.ID)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1}, classListEntryClassesOf(t, db, ctx, "CleZoe", "Promoted"),
		"revert must carry the promoted entry back")
	assert.Equal(t, []string{class4}, classListEntryClassesOf(t, db, ctx, "CleBen", "Graduated"),
		"revert must restore the graduated entry from the ledger, like it reactivates the students")
	assert.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleNeu", "Dazwischen"),
		"an entry created after the apply must stay untouched by the revert")
	assert.Equal(t, []string{classOther}, classListEntryClassesOf(t, db, ctx, "CleUli", "Unrelated"),
		"an entry in an unmapped class must survive the revert unchanged")
}

// An entry the admin deliberately removed between apply and revert stays
// removed: the ledger replay must not resurrect the child under the
// pre-transition class name.
func TestGradeTransitionService_Revert_KeepsAdminDeletedEntriesDeleted(t *testing.T) {
	service, db, cleanup := setupClassListEntryTransitionTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-cle-del@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1c-%s", suffix)
	class2 := fmt.Sprintf("2c-%s", suffix)

	student := testpkg.CreateTestStudent(t, db, "CleDel", "Cohort", class1)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	entry := testpkg.CreateTestClassListEntry(t, db, "CleMia", "Entfernt", class1)
	defer testpkg.CleanupClassListEntryFixtures(t, db, entry.ID)
	defer cleanupClassListEntriesByName(t, db, [][2]string{{"CleMia", "Entfernt"}})

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Equal(t, []string{class2}, classListEntryClassesOf(t, db, ctx, "CleMia", "Entfernt"))

	// The admin takes the child off the list after the apply.
	_, err = db.NewDelete().TableExpr("users.class_list_entries").
		Where("first_name = ? AND last_name = ?", "CleMia", "Entfernt").
		Exec(testpkg.TenantContext(1))
	require.NoError(t, err)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleMia", "Entfernt"),
		"a deliberately deleted entry must not be resurrected by the revert")
}

// Restores that would duplicate a child are skipped: an identical entry the
// admin re-created under the old class, or a student who got a real record
// under that name and class in the meantime — either way the child already
// has exactly one row on the class list.
func TestGradeTransitionService_Revert_SkipsCollidingRestores(t *testing.T) {
	service, db, cleanup := setupClassListEntryTransitionTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-cle-coll@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1d-%s", suffix)
	class2 := fmt.Sprintf("2d-%s", suffix)

	cohort := testpkg.CreateTestStudent(t, db, "CleColl", "Cohort", class1)
	defer testpkg.CleanupActivityFixtures(t, db, cohort.ID)

	entryAnna := testpkg.CreateTestClassListEntry(t, db, "CleAnna", "Kollision", class1)
	entryBen := testpkg.CreateTestClassListEntry(t, db, "CleBen2", "Kollision", class1)
	defer testpkg.CleanupClassListEntryFixtures(t, db, entryAnna.ID, entryBen.ID)
	defer cleanupClassListEntriesByName(t, db, [][2]string{
		{"CleAnna", "Kollision"}, {"CleBen2", "Kollision"},
	})

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	// Between apply and revert: Anna gets re-created as an entry under the
	// OLD class name, Ben gets a real student record under the old class.
	annaAgain := testpkg.CreateTestClassListEntry(t, db, "CleAnna", "Kollision", class1)
	defer testpkg.CleanupClassListEntryFixtures(t, db, annaAgain.ID)
	benStudent := testpkg.CreateTestStudent(t, db, "CleBen2", "Kollision", class1)
	defer testpkg.CleanupActivityFixtures(t, db, benStudent.ID)

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, []string{class1}, classListEntryClassesOf(t, db, ctx, "CleAnna", "Kollision"),
		"the re-created entry survives; the ledger restore must not duplicate it")
	assert.Empty(t, classListEntryClassesOf(t, db, ctx, "CleBen2", "Kollision"),
		"a child with a real student record must not get a second class-list row")
}

// A transition over classes without any entries writes no ledger — apply and
// revert both no-op cleanly.
func TestGradeTransitionService_ClassListEntries_NoEntriesNoLedger(t *testing.T) {
	service, db, cleanup := setupClassListEntryTransitionTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-cle-empty@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	class1 := fmt.Sprintf("1e-%s", suffix)
	class2 := fmt.Sprintf("2e-%s", suffix)

	cohort := testpkg.CreateTestStudent(t, db, "CleEmpty", "Cohort", class1)
	defer testpkg.CleanupActivityFixtures(t, db, cohort.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, class1, testpkg.StrPtr(class2))
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	count, err := db.NewSelect().
		TableExpr("education.grade_transition_class_list_entries").
		Where("transition_id = ?", transition.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "no entries, no ledger rows")

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)
}
