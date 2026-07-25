package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// tagOf reads the RFID tag currently stored on a student's person row. An empty
// string means the tag was released (COALESCE keeps the scan off bun's
// NULL-into-*string handling, which hands back a pointer to "").
func tagOf(t *testing.T, ctx context.Context, db *bun.DB, personID int64) string {
	t.Helper()

	var tags []string
	require.NoError(t, db.NewSelect().
		TableExpr(`users.persons`).
		ColumnExpr(`COALESCE(tag_id, '')`).
		Where("id = ?", personID).
		Scan(ctx, &tags))
	require.Len(t, tags, 1)
	return tags[0]
}

// TestGradeTransitionService_Apply_ReleasesRFIDTag covers the P1 fix (#405
// review): graduation is a soft delete, so a graduate keeps their person row and
// would keep holding the bracelet — a state the kiosk can SEE (GET
// /api/iot/rfid/{tagId} resolves the person unfiltered) but never resolve
// (DELETE /api/students/{id}/rfid runs through the alumnus gate). The apply
// releases the tag and ledgers it in grade_transition_history so the revert can
// hand it back.
func TestGradeTransitionService_Apply_ReleasesRFIDTag(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-rfid-release@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4rfid-%s", suffix)

	// The card is created FIRST so its cleanup runs last: deleting a card a
	// person still points at trips the FK's SET NULL on the composite key.
	card := testpkg.CreateTestRFIDCard(t, db, "GRADTAG")
	defer testpkg.CleanupRFIDCards(t, db, card.ID)

	student := testpkg.CreateTestStudent(t, db, "Armband", "Abgang", gradClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	testpkg.LinkRFIDToStudent(t, db, student.PersonID, card.ID)

	// Free the tag before the card fixture is deleted: dropping a card a person
	// still points at fires the composite FK's SET NULL on (tenant_id, tag_id).
	defer func() {
		_, _ = db.NewUpdate().
			TableExpr(`users.persons`).
			Set("tag_id = NULL").
			Where("tag_id = ?", card.ID).
			Exec(context.Background())
	}()

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Empty(t, tagOf(t, ctx, db, student.PersonID),
		"graduation must free the bracelet so it can be issued to a current child")

	var ledgered []*string
	require.NoError(t, db.NewSelect().
		TableExpr(`education.grade_transition_history`).
		Column("rfid_tag").
		Where("transition_id = ?", transition.ID).
		Where("student_id = ?", student.ID).
		Scan(ctx, &ledgered))
	require.Len(t, ledgered, 1)
	require.NotNil(t, ledgered[0], "the released tag must be recorded for the revert")
	assert.Equal(t, card.ID, *ledgered[0])

	_, err = service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Equal(t, card.ID, tagOf(t, ctx, db, student.PersonID),
		"the revert must hand the bracelet back")
}

// TestGradeTransitionService_Revert_KeepsReissuedRFIDTag pins the other half:
// the bracelet is a physical object. If it was handed to a current child during
// the alumnus window, the revert must leave it with that child (users.persons
// carries UNIQUE (tenant_id, tag_id), so the alternative is a failed revert) and
// say so in the result warnings.
func TestGradeTransitionService_Revert_KeepsReissuedRFIDTag(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	service := newRosterReconcilingTransitionService(t, db)

	ctx, cancel := context.WithTimeout(testpkg.TenantContext(1), 20*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-rfid-reissue@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	gradClass := fmt.Sprintf("4reissue-%s", suffix)

	card := testpkg.CreateTestRFIDCard(t, db, "REISSUETAG")
	defer testpkg.CleanupRFIDCards(t, db, card.ID)

	leaver := testpkg.CreateTestStudent(t, db, "Armband", "Weitergabe", gradClass)
	newcomer := testpkg.CreateTestStudent(t, db, "Armband", "Neu", fmt.Sprintf("1reissue-%s", suffix))
	defer testpkg.CleanupActivityFixtures(t, db, leaver.ID, newcomer.ID)

	testpkg.LinkRFIDToStudent(t, db, leaver.PersonID, card.ID)

	// Free the tag before the card fixture is deleted: dropping a card a person
	// still points at fires the composite FK's SET NULL on (tenant_id, tag_id).
	defer func() {
		_, _ = db.NewUpdate().
			TableExpr(`users.persons`).
			Set("tag_id = NULL").
			Where("tag_id = ?", card.ID).
			Exec(context.Background())
	}()

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, gradClass, nil) // graduate
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	_, err := service.Apply(ctx, transition.ID, account.ID)
	require.NoError(t, err)
	require.Empty(t, tagOf(t, ctx, db, leaver.PersonID))

	// The freed bracelet goes to a child who is still here.
	testpkg.LinkRFIDToStudent(t, db, newcomer.PersonID, card.ID)

	result, err := service.Revert(ctx, transition.ID, account.ID)
	require.NoError(t, err)

	assert.Empty(t, tagOf(t, ctx, db, leaver.PersonID),
		"the current holder wins — the revert must not take the bracelet back")
	assert.Equal(t, card.ID, tagOf(t, ctx, db, newcomer.PersonID))

	require.NotEmpty(t, result.Warnings)
	assert.Contains(t, fmt.Sprint(result.Warnings), "RFID",
		"the admin must be told the bracelet stayed where it is")
}
