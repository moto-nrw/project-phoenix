// Fingerprint encoding for the stale-preview guard (#405 review).
//
// The digest binds an admin's confirmation to the cohort AND the mapping actions
// they reviewed. Encoding the action as a "graduate" target sentinel made a
// promotion into a class literally named "graduate" serialize exactly like the
// graduation of the same source class: editing one into the other left the digest
// unchanged, so apply performed a bulk action nobody confirmed — in the
// destructive direction.
package education_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGradeTransitionService_Fingerprint_PromotionToGraduateNamedClassIsNotGraduation(t *testing.T) {
	t.Parallel()

	service, db, cleanup := setupGradeTransitionServiceTest(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	account := testpkg.CreateTestAccount(t, db, "transition-fingerprint-sentinel@test.local")
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("4fp-%s", suffix)
	// A real school may name a class exactly like the old sentinel.
	targetClass := "graduate"

	student := testpkg.CreateTestStudent(t, db, "Fingerprint", "Child", fromClass)
	defer testpkg.CleanupActivityFixtures(t, db, student.ID)

	transition := testpkg.CreateTestGradeTransition(t, db, "2025-2026", account.ID)
	testpkg.CreateTestGradeTransitionMapping(t, db, transition.ID, fromClass, &targetClass)
	defer testpkg.CleanupGradeTransitionFixtures(t, db, transition.ID)

	// What the admin reviewed: a PROMOTION into the class named "graduate".
	promotePreview, err := service.Preview(ctx, transition.ID)
	require.NoError(t, err)
	require.NotEmpty(t, promotePreview.Fingerprint)
	require.Equal(t, 1, promotePreview.ToPromote, "mapping with a target class is a promotion")
	require.Equal(t, 0, promotePreview.ToGraduate)

	// Another admin turns that mapping into a graduation (Abgang) — the same
	// source class, the same single child, a completely different outcome.
	_, err = db.NewUpdate().
		TableExpr("education.grade_transition_mappings").
		Set("to_class = NULL").
		Where("transition_id = ?", transition.ID).
		Exec(ctx)
	require.NoError(t, err)

	graduatePreview, err := service.Preview(ctx, transition.ID)
	require.NoError(t, err)
	require.Equal(t, 1, graduatePreview.ToGraduate)
	assert.NotEqual(t, promotePreview.Fingerprint, graduatePreview.Fingerprint,
		"a promotion into a class named %q must not digest like a graduation", targetClass)

	// The confirmation the admin actually gave must no longer apply.
	_, err = service.ApplyChecked(ctx, transition.ID, account.ID, promotePreview.Fingerprint)
	require.ErrorIs(t, err, educationService.ErrPreviewStale)

	// And the child was not graduated behind that refused apply.
	var status string
	require.NoError(t, db.NewSelect().TableExpr("users.students").Column("status").
		Where("id = ?", student.ID).Scan(ctx, &status))
	assert.Equal(t, "active", status)

	// The freshly reviewed graduation fingerprint is accepted.
	_, err = service.ApplyChecked(ctx, transition.ID, account.ID, graduatePreview.Fingerprint)
	require.NoError(t, err)
}
