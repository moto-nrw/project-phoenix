package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/database/repositories/education"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// personTag reads the RFID tag currently stored on a person row. An empty
// string means the tag slot is free (COALESCE keeps the scan off bun's
// NULL-into-*string handling, which hands back a pointer to "").
func personTag(t *testing.T, db *bun.DB, personID int64) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var tags []string
	require.NoError(t, db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`COALESCE(tag_id, '')`).
		Where("id = ?", personID).
		Scan(ctx, &tags))
	require.Len(t, tags, 1)
	return tags[0]
}

// studentStatus reads a student's lifecycle status straight from the table,
// bypassing the alumnus filter every ordinary student read carries.
func studentStatus(t *testing.T, db *bun.DB, studentID int64) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var statuses []string
	require.NoError(t, db.NewSelect().
		TableExpr(`users.students`).
		Column("status").
		Where("id = ?", studentID).
		Scan(ctx, &statuses))
	require.Len(t, statuses, 1)
	return statuses[0]
}

// clearPersonTag frees a person's tag slot so the RFID-card fixture can be
// deleted: dropping a card a person still points at fires the composite FK's
// SET NULL on (tenant_id, tag_id).
func clearPersonTag(db *bun.DB, tagID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = db.NewUpdate().
		TableExpr(`users.persons`).
		Set("tag_id = NULL").
		Where("tag_id = ?", tagID).
		Exec(ctx)
}

func TestGradeTransitionRepository_ReleaseStudentTagsByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	// The tag writes run through the People Directory composition (#2661),
	// so the test drives the composed repository the service graph uses.
	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns nothing for an empty id list", func(t *testing.T) {
		released, err := repo.ReleaseStudentTagsByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, released)
	})

	t.Run("returns nothing when no child in the cohort holds a tag", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		student := testpkg.CreateTestStudent(t, db, "Ohne", "Armband", fmt.Sprintf("4no-tag-%s", suffix))

		released, err := repo.ReleaseStudentTagsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.Empty(t, released)
	})

	t.Run("frees held tags and reports what to restore", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		// The card is created FIRST so its cleanup runs last: deleting a card a
		// person still points at trips the FK's SET NULL on the composite key.
		card := testpkg.CreateTestRFIDCard(t, db, "RELTAG")
		defer clearPersonTag(db, card.ID)

		holder := testpkg.CreateTestStudent(t, db, "Mit", "Armband", fmt.Sprintf("4tag-%s", suffix))
		bare := testpkg.CreateTestStudent(t, db, "Ohne", "Armband", fmt.Sprintf("4tag-%s", suffix))

		testpkg.LinkRFIDToStudent(t, db, holder.PersonID, card.ID)

		released, err := repo.ReleaseStudentTagsByIDs(ctx, []int64{holder.ID, bare.ID})
		require.NoError(t, err)

		// Only the child that actually held a bracelet appears — a tagless child
		// must not produce a ledger entry the revert would then try to restore.
		assert.Equal(t, map[int64]string{holder.ID: card.ID}, released)
		assert.Empty(t, personTag(t, db, holder.PersonID),
			"the bracelet must be free so it can be issued to a current child")
	})
}

func TestGradeTransitionRepository_RestoreStudentTag(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("does nothing without a ledgered tag", func(t *testing.T) {
		// A graduate that held no bracelet ledgers an empty rfid_tag; the revert
		// must not turn that into a write.
		restored, err := repo.RestoreStudentTag(ctx, 0, "")
		require.NoError(t, err)
		assert.False(t, restored)
	})

	t.Run("re-links the tag the transition released", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		card := testpkg.CreateTestRFIDCard(t, db, "RESTORETAG")
		defer clearPersonTag(db, card.ID)

		student := testpkg.CreateTestStudent(t, db, "Zurück", "Armband", fmt.Sprintf("4res-%s", suffix))

		testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)
		_, err := repo.ReleaseStudentTagsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		require.Empty(t, personTag(t, db, student.PersonID))

		restored, err := repo.RestoreStudentTag(ctx, student.ID, card.ID)
		require.NoError(t, err)
		assert.True(t, restored)
		assert.Equal(t, card.ID, personTag(t, db, student.PersonID))
	})

	t.Run("keeps a bracelet the child was given in the meantime", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		oldCard := testpkg.CreateTestRFIDCard(t, db, "OLDTAG")
		newCard := testpkg.CreateTestRFIDCard(t, db, "NEWTAG")
		defer clearPersonTag(db, newCard.ID)

		student := testpkg.CreateTestStudent(t, db, "Neues", "Armband", fmt.Sprintf("4new-%s", suffix))

		testpkg.LinkRFIDToStudent(t, db, student.PersonID, newCard.ID)

		// The tag_id IS NULL guard: the child already wears something, so the old
		// bracelet must not silently overwrite it.
		restored, err := repo.RestoreStudentTag(ctx, student.ID, oldCard.ID)
		require.NoError(t, err)
		assert.False(t, restored)
		assert.Equal(t, newCard.ID, personTag(t, db, student.PersonID))
	})

	t.Run("leaves a reissued bracelet with its current holder", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		card := testpkg.CreateTestRFIDCard(t, db, "REISSUED")
		defer clearPersonTag(db, card.ID)

		graduate := testpkg.CreateTestStudent(t, db, "Weg", "Kind", fmt.Sprintf("4gone-%s", suffix))
		current := testpkg.CreateTestStudent(t, db, "Da", "Kind", fmt.Sprintf("1now-%s", suffix))

		// The bracelet was handed to a current child while the graduate was gone.
		testpkg.LinkRFIDToStudent(t, db, current.PersonID, card.ID)

		restored, err := repo.RestoreStudentTag(ctx, graduate.ID, card.ID)
		require.NoError(t, err)
		assert.False(t, restored, "the current holder wins — the unique index leaves no other option")
		assert.Equal(t, card.ID, personTag(t, db, current.PersonID))
		assert.Empty(t, personTag(t, db, graduate.PersonID))
	})

	t.Run("runs inside the revert transaction via a savepoint", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		card := testpkg.CreateTestRFIDCard(t, db, "TXTAG")
		defer clearPersonTag(db, card.ID)

		student := testpkg.CreateTestStudent(t, db, "Transaktion", "Armband", fmt.Sprintf("4tx-%s", suffix))

		testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)
		_, err := repo.ReleaseStudentTagsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)

		// The real revert calls this inside its transaction, where the write is
		// wrapped in a savepoint so a lost race cannot abort the whole revert.
		var restored bool
		err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
			var restoreErr error
			restored, restoreErr = repo.RestoreStudentTag(txCtx, student.ID, card.ID)
			return restoreErr
		})
		require.NoError(t, err)
		assert.True(t, restored)

		assert.Equal(t, card.ID, personTag(t, db, student.PersonID))
	})
}

func TestGradeTransitionRepository_FindStudentStatesByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("returns an empty map for an empty id list", func(t *testing.T) {
		states, err := repo.FindStudentStatesByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, states)
	})

	t.Run("sees graduates and omits hard-deleted children", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]

		active := testpkg.CreateTestStudent(t, db, "Aktives", "Kind", fmt.Sprintf("1st-%s", suffix))
		graduate := testpkg.CreateTestStudent(t, db, "Abgang", "Kind", fmt.Sprintf("4st-%s", suffix))

		graduated, err := repo.GraduateStudentsByIDs(ctx, []int64{graduate.ID})
		require.NoError(t, err)
		require.EqualValues(t, 1, graduated, "exactly the one child in the cohort graduates")

		// A student id that no longer has a row at all — the state the Abgänge
		// list reads as "purged".
		missingID := graduate.ID + 1_000_000

		states, err := repo.FindStudentStatesByIDs(ctx, []int64{active.ID, graduate.ID, missingID})
		require.NoError(t, err)

		assert.Equal(t, string(users.StudentStatusAlumnus), states[graduate.ID],
			"this is the one read whose whole purpose is to see graduates")
		assert.NotEqual(t, string(users.StudentStatusAlumnus), states[active.ID])
		assert.NotContains(t, states, missingID)
	})
}

func TestGradeTransitionRepository_AnonymizeHistoryForStudent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-anonymize")

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	purged := testpkg.CreateTestStudent(t, db, "Gelöschtes", "Kind", fmt.Sprintf("4anon-%s", suffix))
	kept := testpkg.CreateTestStudent(t, db, "Bleibendes", "Kind", fmt.Sprintf("4anon-%s", suffix))
	rfidTag := "rfid-deleted-student"

	require.NoError(t, repo.CreateHistoryBatch(ctx, []*educationModels.GradeTransitionHistory{
		{
			TransitionID: transition.ID,
			StudentID:    purged.ID,
			PersonName:   "Mika Muster",
			FromClass:    "4a",
			Action:       educationModels.ActionGraduated,
			RFIDTag:      &rfidTag,
		},
		{
			TransitionID: transition.ID,
			StudentID:    kept.ID,
			PersonName:   "Nina Beispiel",
			FromClass:    "4a",
			Action:       educationModels.ActionGraduated,
		},
	}))

	historyOf := func(studentID int64) *educationModels.GradeTransitionHistory {
		records, err := repo.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		for _, record := range records {
			if record.StudentID == studentID {
				return record
			}
		}
		t.Fatalf("no ledger row for student %d", studentID)
		return nil
	}

	require.NoError(t, repo.AnonymizeHistoryForStudent(ctx, purged.ID))

	// person_name is a denormalized copy with no foreign key: it survives the
	// hard delete of both the student and the person row, so "endgültig löschen"
	// is only true once this replaced it.
	assert.Equal(t, education.PurgedStudentPlaceholder, historyOf(purged.ID).PersonName)
	assert.Nil(t, historyOf(purged.ID).RFIDTag)
	assert.Equal(t, "Nina Beispiel", historyOf(kept.ID).PersonName, "only the purged child's rows may change")

	// Idempotent: the second call matches no row after both identifying fields
	// have been cleared.
	require.NoError(t, repo.AnonymizeHistoryForStudent(ctx, purged.ID))
	assert.Equal(t, education.PurgedStudentPlaceholder, historyOf(purged.ID).PersonName)
	assert.Nil(t, historyOf(purged.ID).RFIDTag)
}

func TestGradeTransitionRepository_GetMappingsByTransitionIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-batch-mappings")

	t.Run("returns an empty map for an empty id list", func(t *testing.T) {
		grouped, err := repo.GetMappingsByTransitionIDs(ctx, nil)
		require.NoError(t, err)
		assert.Empty(t, grouped)
	})

	t.Run("groups mappings by their transition", func(t *testing.T) {
		first := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
		second := testpkg.CreateTestGradeTransition(t, db, "2026-2027", account.ID)
		bare := testpkg.CreateTestGradeTransition(t, db, "2027-2028", account.ID)

		toClass := "2a"
		testpkg.CreateTestGradeTransitionMapping(t, db, first.ID, "1a", &toClass)
		testpkg.CreateTestGradeTransitionMapping(t, db, first.ID, "4a", nil)
		testpkg.CreateTestGradeTransitionMapping(t, db, second.ID, "1b", &toClass)

		grouped, err := repo.GetMappingsByTransitionIDs(ctx, []int64{first.ID, second.ID, bare.ID})
		require.NoError(t, err)

		assert.Len(t, grouped[first.ID], 2)
		assert.Len(t, grouped[second.ID], 1)
		// A transition without mappings must be absent rather than an empty
		// slice — the list view distinguishes "no mappings" from "not asked".
		assert.NotContains(t, grouped, bare.ID)
	})
}

func TestGradeTransitionRepository_PromoteStudentsByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("promotes nobody for an empty cohort", func(t *testing.T) {
		affected, err := repo.PromoteStudentsByIDs(ctx, nil, "1a", "2a")
		require.NoError(t, err)
		assert.Zero(t, affected)
	})

	t.Run("moves exactly the locked cohort", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1p-%s", suffix)
		toClass := fmt.Sprintf("2p-%s", suffix)
		otherClass := fmt.Sprintf("3p-%s", suffix)

		promoted := testpkg.CreateTestStudent(t, db, "Aufsteigendes", "Kind", fromClass)
		elsewhere := testpkg.CreateTestStudent(t, db, "Anderes", "Kind", otherClass)

		// elsewhere is in the cohort by id but not in fromClass: the from-class
		// guard keeps the statement idempotent and must skip it.
		affected, err := repo.PromoteStudentsByIDs(ctx, []int64{promoted.ID, elsewhere.ID}, fromClass, toClass)
		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)

		classes, err := repo.GetStudentsByClasses(ctx, []string{toClass, otherClass})
		require.NoError(t, err)
		byID := make(map[int64]string, len(classes))
		for _, info := range classes {
			byID[info.StudentID] = info.SchoolClass
		}
		assert.Equal(t, toClass, byID[promoted.ID])
		assert.Equal(t, otherClass, byID[elsewhere.ID])
	})
}

func TestGradeTransitionRepository_GraduateAndReactivateByIDs(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	t.Run("graduates nobody for an empty cohort", func(t *testing.T) {
		affected, err := repo.GraduateStudentsByIDs(ctx, nil)
		require.NoError(t, err)
		assert.Zero(t, affected)
	})

	t.Run("graduates and restores the previous status", func(t *testing.T) {
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		student := testpkg.CreateTestStudent(t, db, "Abgang", "Rückkehr", fmt.Sprintf("4gr-%s", suffix))

		before := studentStatus(t, db, student.ID)

		affected, err := repo.GraduateStudentsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.EqualValues(t, 1, affected)
		assert.Equal(t, string(users.StudentStatusAlumnus), studentStatus(t, db, student.ID))

		// A second graduation is a no-op — the status guard makes the apply
		// idempotent rather than double-counting the child.
		affected, err = repo.GraduateStudentsByIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		assert.Zero(t, affected)

		reactivated, err := repo.ReactivateStudentsToStatus(ctx, []int64{student.ID}, before)
		require.NoError(t, err)
		assert.Equal(t, []int64{student.ID}, reactivated)
		assert.Equal(t, before, studentStatus(t, db, student.ID))
	})
}

// TestGradeTransitionRepository_ValidationGuards pins the model-level checks the
// repository runs BEFORE touching the database. Without them a malformed row
// reaches PostgreSQL and comes back as a constraint error the caller cannot
// classify — or, where no constraint exists (an unknown history action), lands
// in the ledger and misleads every later revert.
func TestGradeTransitionRepository_ValidationGuards(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	repo := newPersonComposedGradeTransitionRepository(t, db)
	ctx := testpkg.Ctx(t)

	account := testpkg.CreateTestAccount(t, db, "transition-validation")

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)

	t.Run("Update rejects a malformed academic year", func(t *testing.T) {
		invalid := *transition
		invalid.AcademicYear = "2025/2026"

		require.Error(t, repo.Update(ctx, &invalid))

		unchanged, err := repo.FindByID(ctx, transition.ID)
		require.NoError(t, err)
		assert.Equal(t, "2025-2026", unchanged.AcademicYear)
	})

	t.Run("CreateMappings rejects a mapping without a source class", func(t *testing.T) {
		toClass := "2a"
		err := repo.CreateMappings(ctx, []*educationModels.GradeTransitionMapping{
			{TransitionID: transition.ID, FromClass: "1a", ToClass: &toClass},
			{TransitionID: transition.ID, FromClass: "   ", ToClass: &toClass},
		})
		require.Error(t, err)

		// The whole batch is rejected before the insert, so the valid sibling
		// must not have landed either.
		mappings, err := repo.GetMappings(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, mappings)
	})

	t.Run("CreateHistoryBatch rejects an unknown action", func(t *testing.T) {
		err := repo.CreateHistoryBatch(ctx, []*educationModels.GradeTransitionHistory{
			{
				TransitionID: transition.ID,
				StudentID:    account.ID,
				PersonName:   "Mika Muster",
				FromClass:    "4a",
				Action:       "verschoben",
			},
		})
		require.Error(t, err)

		records, err := repo.GetHistory(ctx, transition.ID)
		require.NoError(t, err)
		assert.Empty(t, records)
	})
}

// newPersonComposedGradeTransitionRepository returns the grade transition
// repository as the service graph composes it: the RFID tag commands are
// served through the People Directory (#2661).
func newPersonComposedGradeTransitionRepository(t *testing.T, db *bun.DB) educationModels.GradeTransitionRepository {
	t.Helper()
	factory, err := repositories.NewFactoryWithPeopleDirectory(db)
	require.NoError(t, err)
	return factory.GradeTransition
}
