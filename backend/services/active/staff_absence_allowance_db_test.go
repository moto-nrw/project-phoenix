package active_test

import (
	"context"
	"sort"
	"sync"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCustomAbsenceAllowanceSummarizesOnlyItsOwnType(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Generation")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Leitung")

	svc := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, nil)
	allowanceAware, ok := svc.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	})
	require.True(t, ok)
	allowanceAware.SetAllowanceRepositories(
		repos.StaffAbsenceTypeAllowance,
		repos.StaffAbsenceTypeAllowanceChange,
		repos.StaffAbsence,
	)

	absenceType, err := svc.CreateAbsenceType(ctx, "Regenerationstag")
	require.NoError(t, err)
	require.NoError(t, svc.ConfigureAllowance(
		ctx,
		absenceType.ID,
		true,
		activeModels.AbsenceTypeOverrunBlock,
	))

	summary, err := svc.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
		StaffID:       staff.ID,
		AbsenceTypeID: absenceType.ID,
		Year:          2026,
		EntitledDays:  2,
		Reason:        "Tariflicher Anspruch",
		ChangedBy:     admin.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 2.0, summary.EntitledDays)
	assert.Zero(t, summary.TakenDays)
	assert.Zero(t, summary.ReservedDays)
	assert.Equal(t, 2.0, summary.RemainingDays)

	summary, err = svc.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 2.5, Reason: "Tarifliche Korrektur", ChangedBy: admin.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, 2.5, summary.EntitledDays)
	changes, err := repos.StaffAbsenceTypeAllowanceChange.List(ctx, modelBase.NewQueryOptions())
	require.NoError(t, err)
	var correction *activeModels.StaffAbsenceTypeAllowanceChange
	for _, change := range changes {
		if change.StaffID == staff.ID && change.AbsenceTypeID == absenceType.ID && change.Reason == "Tarifliche Korrektur" {
			correction = change
			break
		}
	}
	require.NotNil(t, correction)
	require.NotNil(t, correction.OldEntitledDays)
	assert.Equal(t, 2.0, *correction.OldEntitledDays)
	assert.Equal(t, 2.5, correction.NewEntitledDays)

	typeID := absenceType.ID
	for _, absence := range []*activeModels.StaffAbsence{
		{
			StaffID: staff.ID, AbsenceType: activeModels.AbsenceTypeOther,
			AbsenceTypeID: &typeID, Status: activeModels.AbsenceStatusReported,
			DateStart: timezone.NewDate(2026, 9, 7), DateEnd: timezone.NewDate(2026, 9, 7),
			HalfDay: true, CreatedBy: admin.ID,
		},
		{
			StaffID: staff.ID, AbsenceType: activeModels.AbsenceTypeOther,
			AbsenceTypeID: &typeID, Status: activeModels.AbsenceStatusRequested,
			DateStart: timezone.NewDate(2026, 10, 5), DateEnd: timezone.NewDate(2026, 10, 5),
			CreatedBy: staff.ID,
		},
		{
			StaffID: staff.ID, AbsenceType: activeModels.AbsenceTypeVacation,
			Status:    activeModels.AbsenceStatusApproved,
			DateStart: timezone.NewDate(2026, 11, 2), DateEnd: timezone.NewDate(2026, 11, 2),
			CreatedBy: staff.ID,
		},
	} {
		require.NoError(t, repos.StaffAbsence.Create(ctx, absence))
	}

	summary, err = svc.GetAllowanceSummary(ctx, staff.ID, absenceType.ID, 2026)
	require.NoError(t, err)
	assert.Equal(t, 0.5, summary.TakenDays)
	assert.Equal(t, 1.0, summary.ReservedDays)
	assert.Equal(t, 1.0, summary.RemainingDays)
}

func TestCustomAbsenceAllowanceOverrunPolicy(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Block")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Block")
	svc := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, nil)
	allowanceAware := svc.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	})
	allowanceAware.SetAllowanceRepositories(
		repos.StaffAbsenceTypeAllowance,
		repos.StaffAbsenceTypeAllowanceChange,
		repos.StaffAbsence,
	)

	absenceType, err := svc.CreateAbsenceType(ctx, "Tariftag")
	require.NoError(t, err)
	require.NoError(t, svc.ConfigureAllowance(ctx, absenceType.ID, true, activeModels.AbsenceTypeOverrunBlock))
	_, err = svc.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 0.5, Reason: "Tarifvertrag", ChangedBy: admin.ID,
	})
	require.NoError(t, err)

	preview, err := svc.PreviewAllowanceBooking(
		ctx, staff.ID, absenceType.ID,
		timezone.NewDate(2026, 9, 7), timezone.NewDate(2026, 9, 7), false,
	)
	require.ErrorIs(t, err, active.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays)

	require.NoError(t, svc.ConfigureAllowance(ctx, absenceType.ID, true, activeModels.AbsenceTypeOverrunWarn))
	preview, err = svc.PreviewAllowanceBooking(
		ctx, staff.ID, absenceType.ID,
		timezone.NewDate(2026, 9, 7), timezone.NewDate(2026, 9, 7), false,
	)
	require.NoError(t, err)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays)

	// Re-entering the same half-day is merged by the absence service and must
	// not consume the half-day twice during the pre-save quota check.
	typeID := absenceType.ID
	require.NoError(t, repos.StaffAbsence.Create(ctx, &activeModels.StaffAbsence{
		StaffID: staff.ID, AbsenceType: activeModels.AbsenceTypeOther,
		AbsenceTypeID: &typeID, Status: activeModels.AbsenceStatusReported,
		DateStart: timezone.NewDate(2026, 9, 7), DateEnd: timezone.NewDate(2026, 9, 7),
		HalfDay: true, CreatedBy: admin.ID,
	}))
	require.NoError(t, svc.ConfigureAllowance(ctx, absenceType.ID, true, activeModels.AbsenceTypeOverrunBlock))
	preview, err = svc.PreviewAllowanceBooking(
		ctx, staff.ID, absenceType.ID,
		timezone.NewDate(2026, 9, 7), timezone.NewDate(2026, 9, 7), true,
	)
	require.NoError(t, err)
	require.Len(t, preview, 1)
	assert.Zero(t, preview[0].RemainingDays)

	preview, err = svc.PreviewAllowanceBooking(
		ctx, staff.ID, absenceType.ID,
		timezone.NewDate(2026, 9, 7), timezone.NewDate(2026, 9, 7), false,
	)
	require.ErrorIs(t, err, active.ErrAbsenceTypeAllowanceExceeded)
	require.Len(t, preview, 1)
	assert.Equal(t, -0.5, preview[0].RemainingDays)
}

func TestCustomAbsenceAllowanceDoesNotCarryAcrossYears(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Jahreswechsel")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Jahreswechsel")
	svc := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, nil)
	allowanceAware := svc.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	})
	allowanceAware.SetAllowanceRepositories(
		repos.StaffAbsenceTypeAllowance,
		repos.StaffAbsenceTypeAllowanceChange,
		repos.StaffAbsence,
	)

	absenceType, err := svc.CreateAbsenceTypeWithConfig(ctx, "Gesundheitstag", true, activeModels.AbsenceTypeOverrunBlock)
	require.NoError(t, err)
	for _, year := range []int{2026, 2027} {
		_, err = svc.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
			StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: year,
			EntitledDays: 1, Reason: "Jahresanspruch", ChangedBy: admin.ID,
		})
		require.NoError(t, err)
	}

	preview, err := svc.PreviewAllowanceBooking(
		ctx, staff.ID, absenceType.ID,
		timezone.NewDate(2026, 12, 31), timezone.NewDate(2027, 1, 1), false,
	)
	require.NoError(t, err)
	require.Len(t, preview, 2)
	assert.Equal(t, 2026, preview[0].Year)
	assert.Zero(t, preview[0].RemainingDays)
	assert.Equal(t, 2027, preview[1].Year)
	assert.Zero(t, preview[1].RemainingDays)
}

func TestCustomAbsenceAllowanceIsTenantIsolated(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantA := testpkg.NewTenantScope(t, db)
	tenantB := testpkg.NewTenantScope(t, db)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantA.TenantID, "Rena", "Mandant A")
	admin := testpkg.CreateTestStaffForTenant(t, db, tenantA.TenantID, "Lea", "Mandant A")
	svc := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, nil)
	allowanceAware := svc.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	})
	allowanceAware.SetAllowanceRepositories(
		repos.StaffAbsenceTypeAllowance,
		repos.StaffAbsenceTypeAllowanceChange,
		repos.StaffAbsence,
	)

	absenceType, err := svc.CreateAbsenceTypeWithConfig(
		tenantA.Context(), "Mandantentag", true, activeModels.AbsenceTypeOverrunBlock,
	)
	require.NoError(t, err)
	_, err = svc.SetAllowance(tenantA.Context(), active.SetAbsenceTypeAllowanceRequest{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 2, Reason: "Anspruch Mandant A", ChangedBy: admin.ID,
	})
	require.NoError(t, err)

	allowanceOptions := modelBase.NewQueryOptions()
	allowanceOptions.Filter.Equal("staff_id", staff.ID).Equal("absence_type_id", absenceType.ID).Equal("year", 2026)
	allowances, err := repos.StaffAbsenceTypeAllowance.List(tenantB.Context(), allowanceOptions)
	require.NoError(t, err)
	assert.Empty(t, allowances, "tenant B must not see tenant A's allowance")
	changes, err := repos.StaffAbsenceTypeAllowanceChange.List(tenantB.Context(), modelBase.NewQueryOptions())
	require.NoError(t, err)
	assert.Empty(t, changes, "tenant B must not see tenant A's allowance audit")
}

func TestCustomAbsenceAllowanceSerializesConcurrentCorrections(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	staff := testpkg.CreateTestStaff(t, db, "Rena", "Parallel")
	admin := testpkg.CreateTestStaff(t, db, "Lea", "Parallel")
	svc := active.NewStaffAbsenceTypeService(repos.StaffAbsenceType, nil)
	allowanceAware := svc.(interface {
		SetAllowanceRepositories(
			activeModels.StaffAbsenceTypeAllowanceRepository,
			activeModels.StaffAbsenceTypeAllowanceChangeRepository,
			activeModels.StaffAbsenceRepository,
		)
	})
	allowanceAware.SetAllowanceRepositories(repos.StaffAbsenceTypeAllowance, repos.StaffAbsenceTypeAllowanceChange, repos.StaffAbsence)
	absenceType, err := svc.CreateAbsenceTypeWithConfig(ctx, "Paralleltag", true, activeModels.AbsenceTypeOverrunBlock)
	require.NoError(t, err)
	_, err = svc.SetAllowance(ctx, active.SetAbsenceTypeAllowanceRequest{
		StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
		EntitledDays: 1, Reason: "Ausgangswert", ChangedBy: admin.ID,
	})
	require.NoError(t, err)

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, days := range []float64{2, 3} {
		wg.Add(1)
		go func(entitledDays float64) {
			defer wg.Done()
			<-start
			errs <- testpkg.WithinTenantContext(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context) error {
				_, setErr := svc.SetAllowance(txCtx, active.SetAbsenceTypeAllowanceRequest{
					StaffID: staff.ID, AbsenceTypeID: absenceType.ID, Year: 2026,
					EntitledDays: entitledDays, Reason: "Parallele Korrektur", ChangedBy: admin.ID,
				})
				return setErr
			})
		}(days)
	}
	close(start)
	wg.Wait()
	close(errs)
	for setErr := range errs {
		require.NoError(t, setErr)
	}

	options := modelBase.NewQueryOptions()
	options.Filter.Equal("staff_id", staff.ID).Equal("absence_type_id", absenceType.ID).Equal("year", 2026)
	changes, err := repos.StaffAbsenceTypeAllowanceChange.List(ctx, options)
	require.NoError(t, err)
	require.Len(t, changes, 3)
	sort.Slice(changes, func(i, j int) bool { return changes[i].ID < changes[j].ID })
	require.NotNil(t, changes[1].OldEntitledDays)
	assert.Equal(t, 1.0, *changes[1].OldEntitledDays)
	require.NotNil(t, changes[2].OldEntitledDays)
	assert.Equal(t, changes[1].NewEntitledDays, *changes[2].OldEntitledDays)
}
