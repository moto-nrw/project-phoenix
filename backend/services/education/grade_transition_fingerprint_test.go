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
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGradeTransitionWorkflow_Fingerprint_PromotionToGraduateNamedClassIsNotGraduation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	f := newTransitionFixture(t, db)
	wf := f.workflow(t)

	ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 15*time.Second)
	defer cancel()

	suffix := uuid.Must(uuid.NewV4()).String()[:8]
	fromClass := fmt.Sprintf("4fp-%s", suffix)
	// A real school may name a class exactly like the old sentinel.
	targetClass := "graduate"

	student := testpkg.CreateTestStudent(t, db, "Fingerprint", "Child", fromClass)

	id := f.createDraft(t, ctx, "2025-2026", promote(fromClass, targetClass))

	// What the admin reviewed: a PROMOTION into the class named "graduate".
	promotePreview, err := wf.Preview(ctx, id)
	require.NoError(t, err)
	require.NotEmpty(t, promotePreview.Fingerprint)
	require.Equal(t, 1, promotePreview.ToPromote, "mapping with a target class is a promotion")
	require.Equal(t, 0, promotePreview.ToGraduate)

	// Another admin turns that mapping into a graduation (Abgang) — the same
	// source class, the same single child, a completely different outcome.
	_, err = wf.UpdateDraft(ctx, id, gradetransition.DraftPatch{
		Mappings: []gradetransition.Mapping{graduate(fromClass)},
	})
	require.NoError(t, err)

	graduatePreview, err := wf.Preview(ctx, id)
	require.NoError(t, err)
	require.Equal(t, 1, graduatePreview.ToGraduate)
	assert.NotEqual(t, promotePreview.Fingerprint, graduatePreview.Fingerprint,
		"a promotion into a class named %q must not digest like a graduation", targetClass)

	// The confirmation the admin actually gave must no longer apply.
	_, err = wf.Apply(ctx, id, promotePreview.Fingerprint)
	require.ErrorIs(t, err, gradetransition.ErrPreviewStale)

	// And the child was not graduated behind that refused apply.
	var status string
	require.NoError(t, db.NewSelect().TableExpr("users.student_school_memberships").Column("status").
		Where("student_profile_id = ?", student.ID).Where("deleted_at IS NULL").Scan(ctx, &status))
	assert.Equal(t, string(users.StudentStatusActive), status)

	// The freshly reviewed graduation fingerprint is accepted.
	_, err = wf.Apply(ctx, id, graduatePreview.Fingerprint)
	require.NoError(t, err)
}
