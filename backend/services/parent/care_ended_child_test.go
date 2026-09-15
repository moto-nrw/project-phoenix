// The parents portal after the child has left the OGS (#2487).
//
// The contract from the acceptance criteria: the family keeps READ access to
// everything that happened while their child was here, and cannot submit
// anything new. The exit reason is never part of what they see.
package parent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// endCareFor moves the child's enrollment interval into the past, which is the
// state the day after a legitimate exit.
func endCareFor(t *testing.T, db *bun.DB, studentID int64) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", timezone.NewDate(2026, 8, 24).AddDays(-1)).
		Where("id = ?", studentID).
		Exec(testpkg.WithPackageTenantRuntime(context.Background()))
	require.NoError(t, err)
}

func TestParentPortal_CareEndedChildIsReadOnly(t *testing.T) {
	t.Parallel()

	// One pool per package (#2419): SetupTestDB hands out the same *bun.DB the
	// service wiring below already runs against.
	db := testpkg.SetupTestDB(t)
	svc, _, _, _ := buildAbsenceApprovalServices(t, false, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	// While the child is still in care the family can report an absence.
	_, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.NewDate(2026, 8, 24).AddDays(2)}, "Fieber",
		activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)

	endCareFor(t, db, chain.StudentID)

	t.Run("no new direct absence can be reported", func(t *testing.T) {
		_, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
			[]timezone.Date{timezone.NewDate(2026, 8, 24).AddDays(3)}, "Fieber",
			activeModels.StudentStatusDaySick, nil)
		require.ErrorIs(t, err, parentService.ErrChildCareEnded)
	})

	t.Run("no pending absence can be reported", func(t *testing.T) {
		pendingSvc, _, _, _ := buildAbsenceApprovalServices(t, true, false)
		_, err := pendingSvc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
			[]timezone.Date{timezone.NewDate(2026, 8, 24).AddDays(3)}, "Fieber",
			activeModels.StudentStatusDaySick, nil)
		require.ErrorIs(t, err, parentService.ErrChildCareEnded)
	})

	t.Run("what happened stays readable", func(t *testing.T) {
		days, err := svc.ListSickDays(ctx, chain.AccountID, chain.StudentID,
			timezone.NewDate(2026, 8, 24).AddDays(-30), timezone.NewDate(2026, 8, 24).AddDays(30))
		require.NoError(t, err,
			"reading the child's past absences must keep working after the exit")
		assert.NotNil(t, days)
	})

	t.Run("every write capability goes off, reading stays on", func(t *testing.T) {
		flags, err := svc.ChildFeatures(ctx, chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.True(t, flags.CareEnded, "the portal is told why the buttons are gone")
		assert.False(t, flags.SickNoteEnabled)
		assert.False(t, flags.NotesEnabled)
		assert.False(t, flags.RequestSubmitEnabled)
		assert.False(t, flags.PickupChangeEnabled)
		assert.False(t, flags.PickupManageAllowed)
		assert.False(t, flags.GuardianContactManageAllowed)
		assert.False(t, flags.MasterDataEditEnabled)
		assert.False(t, flags.MasterDataRequestEnabled)
		assert.False(t, flags.RelatedAccountsInviteEnabled)
		assert.False(t, flags.RelatedAccountsRemoveEnabled)
	})

	t.Run("the child list marks the care as ended", func(t *testing.T) {
		children, err := svc.ListChildrenForAccount(ctx, chain.AccountID)
		require.NoError(t, err)
		require.Len(t, children, 1, "the child does not disappear from the portal")
		assert.True(t, children[0].CareEnded(careplan.Date(timezone.NewDate(2026, 8, 24).String())),
			"the card is labelled 'Betreuung beendet' rather than removed")
	})
}
