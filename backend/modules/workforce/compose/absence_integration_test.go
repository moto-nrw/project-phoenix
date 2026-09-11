package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Every table this module owns is tenant-scoped through the ambient tenant of
// the context plus row-level security. The tests below write the same shape
// of row into two tenants and assert that reads, locks and deletes through
// the capability never cross that boundary.

func testAbsence(staffID int64, absenceType, status string, day timezone.Date) workforce.StaffAbsence {
	return workforce.StaffAbsence{
		StaffID: staffID, AbsenceType: absenceType, Status: status,
		DateStart: day.String(), DateEnd: day.String(), CreatedBy: staffID,
	}
}

func TestStaffAbsencesAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)

	local := testpkg.CreateTestStaff(t, db, "Absence", "Local")
	foreign := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Absence", "Foreign")
	day := timezone.NewDate(2026, 3, 9)
	capability := buildWorkforce(t, db)

	localAbsence, err := capability.CreateStaffAbsence(ctx, testAbsence(local.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusReported, day))
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), localAbsence.TenantID, "the ambient tenant is stamped on the row")
	foreignAbsence, err := capability.CreateStaffAbsence(foreignCtx, testAbsence(foreign.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusReported, day))
	require.NoError(t, err)
	assert.Equal(t, foreignTenantID, foreignAbsence.TenantID)

	listed, err := capability.ListStaffAbsences(ctx, workforce.StaffAbsenceFilter{OverlapFrom: day.String(), OverlapTo: day.String()})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, localAbsence.ID, listed[0].ID)

	_, err = capability.FindStaffAbsence(ctx, foreignAbsence.ID)
	require.ErrorIs(t, err, workforce.ErrStaffAbsenceNotFound, "a foreign row is invisible, not an error of another kind")

	absent, err := capability.StaffAbsenceMapForDate(ctx, day.String())
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{local.ID: workforce.AbsenceTypeSick}, absent)

	// A delete addressed at a foreign row is a no-op in the caller's tenant.
	require.NoError(t, capability.DeleteStaffAbsence(ctx, foreignAbsence.ID))
	stillThere, err := capability.FindStaffAbsence(foreignCtx, foreignAbsence.ID)
	require.NoError(t, err)
	assert.Equal(t, foreignAbsence.ID, stillThere.ID)

	// An update addressed at a foreign row reports not found instead of
	// rewriting the other tenant's row.
	foreignAbsence.Note = "crossed"
	_, err = capability.UpdateStaffAbsence(ctx, foreignAbsence)
	require.ErrorIs(t, err, workforce.ErrStaffAbsenceNotFound)
}

func TestStaffAbsenceMapForDatePrefersSickOverVacation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	staff := testpkg.CreateTestStaff(t, db, "Priority", "Staff")
	day := timezone.NewDate(2026, 3, 10)
	capability := buildWorkforce(t, db)

	absenceType, err := capability.CreateStaffAbsenceType(ctx, workforce.StaffAbsenceTypeFields{Name: "Prioritätstag", IsActive: true})
	require.NoError(t, err)

	vacation := testAbsence(staff.ID, workforce.AbsenceTypeVacation, workforce.AbsenceStatusApproved, day)
	_, err = capability.CreateStaffAbsence(ctx, vacation)
	require.NoError(t, err)
	custom := testAbsence(staff.ID, workforce.AbsenceTypeOther, workforce.AbsenceStatusReported, day)
	custom.AbsenceTypeID = &absenceType.ID
	_, err = capability.CreateStaffAbsence(ctx, custom)
	require.NoError(t, err)
	_, err = capability.CreateStaffAbsence(ctx, testAbsence(staff.ID, workforce.AbsenceTypeSick, workforce.AbsenceStatusRequested, day))
	require.NoError(t, err, "a pending sick note is stored but not effective")

	types, err := capability.StaffAbsenceMapForDate(ctx, day.String())
	require.NoError(t, err)
	assert.Equal(t, workforce.AbsenceTypeVacation, types[staff.ID], "vacation beats other; the pending sick note does not count")

	typeIDs, err := capability.StaffAbsenceTypeIDMapForDate(ctx, day.String())
	require.NoError(t, err)
	_, hasCustom := typeIDs[staff.ID]
	assert.False(t, hasCustom, "the winning absence carries no school-defined type")
}

func TestStaffAbsenceTypesAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	capability := buildWorkforce(t, db)

	local, err := capability.CreateStaffAbsenceType(ctx, workforce.StaffAbsenceTypeFields{Name: "Regenerationstag", IsActive: true})
	require.NoError(t, err)
	assert.Equal(t, workforce.AbsenceTypeOther, local.BaseType, "every school-defined type inherits the arithmetic of 'other'")
	foreignType, err := capability.CreateStaffAbsenceType(foreignCtx, workforce.StaffAbsenceTypeFields{Name: "Regenerationstag", IsActive: true})
	require.NoError(t, err, "the same name may exist in another tenant")

	_, err = capability.CreateStaffAbsenceType(ctx, workforce.StaffAbsenceTypeFields{Name: "regenerationstag", IsActive: true})
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNameTaken, "the per-tenant name index is case-insensitive")

	listed, err := capability.ListStaffAbsenceTypes(ctx)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, local.ID, listed[0].ID)

	_, err = capability.LockStaffAbsenceType(ctx, foreignType.ID)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)

	foreignType.Name = "Umbenannt"
	_, err = capability.UpdateStaffAbsenceType(ctx, foreignType)
	require.ErrorIs(t, err, workforce.ErrAbsenceTypeNotFound)
	unchanged, err := capability.FindStaffAbsenceType(foreignCtx, foreignType.ID)
	require.NoError(t, err)
	assert.Equal(t, "Regenerationstag", unchanged.Name)

	staff := testpkg.CreateTestStaff(t, db, "Type", "User")
	absence := testAbsence(staff.ID, workforce.AbsenceTypeOther, workforce.AbsenceStatusReported, timezone.NewDate(2026, 3, 11))
	absence.AbsenceTypeID = &local.ID
	_, err = capability.CreateStaffAbsence(ctx, absence)
	require.NoError(t, err)
	inUse, err := capability.StaffAbsenceTypeInUse(ctx, local.ID)
	require.NoError(t, err)
	assert.True(t, inUse)
	foreignInUse, err := capability.StaffAbsenceTypeInUse(foreignCtx, foreignType.ID)
	require.NoError(t, err)
	assert.False(t, foreignInUse)
}

func TestStaffAbsenceAuditIsTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	capability := buildWorkforce(t, db)

	local, localAccount := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Local")
	foreign, foreignAccount := testpkg.CreateTestStaffWithAccountForTenant(t, db, foreignTenantID, "Audit", "Foreign")
	day := timezone.NewDate(2026, 3, 12)
	localAbsence, err := capability.CreateStaffAbsence(ctx, testAbsence(local.ID, workforce.AbsenceTypeVacation, workforce.AbsenceStatusRequested, day))
	require.NoError(t, err)
	foreignAbsence, err := capability.CreateStaffAbsence(foreignCtx, testAbsence(foreign.ID, workforce.AbsenceTypeVacation, workforce.AbsenceStatusRequested, day))
	require.NoError(t, err)

	localAudit, err := capability.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{AbsenceID: localAbsence.ID, ToStatus: workforce.AbsenceStatusApproved, ActorID: localAccount.ID})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), localAudit.TenantID)
	assert.False(t, localAudit.ChangedAt.IsZero(), "the database stamps the change instant")
	foreignAudit, err := capability.RecordStaffAbsenceAudit(foreignCtx, workforce.StaffAbsenceAudit{AbsenceID: foreignAbsence.ID, ToStatus: workforce.AbsenceStatusApproved, ActorID: foreignAccount.ID})
	require.NoError(t, err)
	assert.Equal(t, foreignTenantID, foreignAudit.TenantID)

	_, err = capability.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{AbsenceID: localAbsence.ID, ToStatus: ""})
	require.ErrorIs(t, err, workforce.ErrInvalidStaffAbsence)

	var tenantIDs []int64
	require.NoError(t, db.NewSelect().TableExpr("active.staff_absence_audit").Column("tenant_id").
		Where("absence_id IN (?)", bun.List([]int64{localAbsence.ID, foreignAbsence.ID})).OrderExpr("id ASC").Scan(ctx, &tenantIDs))
	assert.Equal(t, []int64{testpkg.Tenant(t), foreignTenantID}, tenantIDs)
}

func TestGroupSubstitutionsAreTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	foreignTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, foreignTenantID)
	foreignCtx := testpkg.TenantContext(foreignTenantID)
	capability := buildWorkforce(t, db)

	localGroup := testpkg.CreateTestEducationGroup(t, db, "Substitution Local")
	localStaff := testpkg.CreateTestStaff(t, db, "Substitute", "Local")
	foreignGroup := testpkg.CreateTestEducationGroupForTenant(t, db, foreignTenantID, "Substitution Foreign")
	foreignStaff := testpkg.CreateTestStaffForTenant(t, db, foreignTenantID, "Substitute", "Foreign")
	day := timezone.NewDate(2026, 3, 13)

	local, err := capability.CreateGroupSubstitution(ctx, workforce.GroupSubstitution{
		TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: localGroup.ID, SubstituteStaffID: localStaff.ID,
		StartDate: day.String(), EndDate: day.AddDays(2).String(), Reason: "Gruppenübergabe",
	})
	require.NoError(t, err)
	assert.Equal(t, testpkg.Tenant(t), local.TenantID)
	foreign, err := capability.CreateGroupSubstitution(foreignCtx, workforce.GroupSubstitution{
		TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: foreignGroup.ID, SubstituteStaffID: foreignStaff.ID,
		StartDate: day.String(), EndDate: day.AddDays(2).String(), Reason: "Gruppenübergabe",
	})
	require.NoError(t, err)

	listed, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{On: day.AddDays(1).String()})
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, local.ID, listed[0].ID)

	explicitForeign, err := capability.ListGroupSubstitutions(ctx, workforce.GroupSubstitutionFilter{TenantID: foreignTenantID})
	require.NoError(t, err)
	assert.Empty(t, explicitForeign, "an explicit tenant predicate never widens the ambient tenant")

	_, err = capability.FindGroupSubstitution(ctx, foreign.ID)
	require.ErrorIs(t, err, workforce.ErrGroupSubstitutionNotFound)
	_, err = capability.LockGroupSubstitution(ctx, foreign.ID)
	require.ErrorIs(t, err, workforce.ErrGroupSubstitutionNotFound)

	require.NoError(t, capability.DeleteGroupSubstitution(ctx, foreign.ID))
	deleted, err := capability.DeleteGroupSubstitutionsForStaff(ctx, foreignStaff.ID, day.String())
	require.NoError(t, err)
	assert.Zero(t, deleted)
	stillThere, err := capability.FindGroupSubstitution(foreignCtx, foreign.ID)
	require.NoError(t, err)
	assert.Equal(t, foreign.ID, stillThere.ID)

	_, err = capability.CreateGroupSubstitution(ctx, workforce.GroupSubstitution{
		TargetType: workforce.GroupSubstitutionTypeGroupHandover, GroupID: localGroup.ID, SubstituteStaffID: localStaff.ID,
		StartDate: day.AddDays(3).String(), EndDate: day.String(),
	})
	require.ErrorIs(t, err, workforce.ErrInvalidGroupSubstitution, "an inverted period is rejected before it reaches SQL")
}

// An absence decision is two authoritative writes in one unit of work: the
// absence row and its audit entry. A failure after either write must leave
// nothing behind, and the retry must then produce exactly one of each.
func TestAbsenceDecisionRollsBackAfterEachWriteAndRetriesCleanly(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Rollback", "Absence")
	day := timezone.NewDate(2026, 3, 16)
	capability := buildWorkforce(t, db)
	injected := errors.New("injected failure")

	decide := func(ctx context.Context, failAfter int) error {
		if err := capability.LockStaffAbsenceWrites(ctx, staff.ID); err != nil {
			return err
		}
		created, err := capability.CreateStaffAbsence(ctx, testAbsence(staff.ID, workforce.AbsenceTypeVacation, workforce.AbsenceStatusApproved, day))
		if err != nil {
			return err
		}
		if failAfter == 1 {
			return injected
		}
		if _, err := capability.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{
			AbsenceID: created.ID, ToStatus: workforce.AbsenceStatusApproved, ActorID: account.ID,
		}); err != nil {
			return err
		}
		if failAfter == 2 {
			return injected
		}
		return nil
	}
	countRows := func(table string) int {
		count, err := db.NewSelect().TableExpr(table).Where("tenant_id = ?", testpkg.Tenant(t)).Count(context.Background())
		require.NoError(t, err)
		return count
	}

	for _, failAfter := range []int{1, 2} {
		err := testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context) error {
			return decide(ctx, failAfter)
		})
		require.ErrorIs(t, err, injected)
		assert.Zero(t, countRows("active.staff_absences"), "failure after write %d must roll the absence back", failAfter)
		assert.Zero(t, countRows("active.staff_absence_audit"), "failure after write %d must roll the audit back", failAfter)
	}

	require.NoError(t, testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), func(ctx context.Context) error {
		return decide(ctx, 0)
	}))
	assert.Equal(t, 1, countRows("active.staff_absences"))
	assert.Equal(t, 1, countRows("active.staff_absence_audit"))
}
