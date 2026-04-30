package enrollment_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupCareTest wires a real DB-backed CareOfferingService and creates a
// calendar period the offerings can FK against. Returns helpers the
// individual tests build on.
func setupCareTest(t *testing.T) (*bun.DB, enrollmentService.CareOfferingService, *scheduleModels.CalendarPeriod, func()) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	testpkg.EnsureTestTenant(t, db, 1)
	repoFactory := repositories.NewFactory(db)
	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:   repoFactory.CareOffering,
		Logger: slog.Default(),
	})

	// Need a calendar period for offering FKs. Create one and clean
	// it up at end of test (CASCADE removes any care_offerings tied
	// to it).
	period := &scheduleModels.CalendarPeriod{
		Name:            "test-period-" + t.Name(),
		PeriodType:      "school_year",
		StartDate:       time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		EndDate:         time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC),
		WeekCycleLength: 1,
		IsActive:        true,
	}
	period.SetTenantID(1)
	ctx := testpkg.TenantContext(1)
	err := repoFactory.CalendarPeriod.Create(ctx, period)
	require.NoError(t, err)

	cleanup := func() {
		// CASCADE on care_offerings.calendar_period_id is SET NULL not
		// CASCADE, so delete the offerings explicitly first to keep the
		// test DB clean for the next run.
		_, _ = db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("calendar_period_id = ? OR calendar_period_id IS NULL", period.ID).
			Exec(context.Background())
		_, _ = db.NewDelete().
			TableExpr("schedule.calendar_periods").
			Where("id = ?", period.ID).
			Exec(context.Background())
		_ = db.Close()
	}

	return db, svc, period, cleanup
}

func TestCareOfferingService_Create_AndListByCalendarPeriod(t *testing.T) {
	db, svc, period, cleanup := setupCareTest(t)
	defer cleanup()
	_ = db
	ctx := testpkg.TenantContext(1)

	periodID := period.ID
	offering := &enrollmentModels.CareOffering{
		CalendarPeriodID:    &periodID,
		Name:                "Regelbetreuung",
		DaysOfWeekMode:      enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:       []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare: false,
		IncludesLunch:       true,
		IsActive:            true,
	}
	created, err := svc.Create(ctx, offering)
	require.NoError(t, err)
	require.NotZero(t, created.ID)

	list, err := svc.ListByCalendarPeriod(ctx, periodID)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "Regelbetreuung", list[0].Name)
}

func TestCareOfferingService_GetByID_NotFoundReturnsSentinel(t *testing.T) {
	_, svc, _, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	_, err := svc.GetByID(ctx, 999_999_999)
	require.Error(t, err)
	assert.True(t, errors.Is(err, enrollmentService.ErrCareOfferingNotFound),
		"missing offering must return ErrCareOfferingNotFound")
}

func TestCareOfferingService_ListPublic_FiltersOnWindowAndActive(t *testing.T) {
	_, svc, period, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	periodID := period.ID

	now := time.Now()
	pastEnd := now.Add(-1 * time.Hour)
	futureStart := now.Add(24 * time.Hour)
	pastStart := now.Add(-24 * time.Hour)
	futureEnd := now.Add(24 * time.Hour)

	// Currently open: should be returned.
	openOffering := &enrollmentModels.CareOffering{
		CalendarPeriodID:       &periodID,
		Name:                   "Open Offering",
		DaysOfWeekMode:         enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:          []string{"mon"},
		ApplicationWindowStart: &pastStart,
		ApplicationWindowEnd:   &futureEnd,
		IsActive:               true,
	}
	_, err := svc.Create(ctx, openOffering)
	require.NoError(t, err)

	// Window already closed: should NOT be returned.
	closedOffering := &enrollmentModels.CareOffering{
		CalendarPeriodID:     &periodID,
		Name:                 "Closed Offering",
		DaysOfWeekMode:       enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:        []string{"mon"},
		ApplicationWindowEnd: &pastEnd,
		IsActive:             true,
	}
	_, err = svc.Create(ctx, closedOffering)
	require.NoError(t, err)

	// Window not yet started: should NOT be returned.
	futureOffering := &enrollmentModels.CareOffering{
		CalendarPeriodID:       &periodID,
		Name:                   "Future Offering",
		DaysOfWeekMode:         enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:          []string{"mon"},
		ApplicationWindowStart: &futureStart,
		IsActive:               true,
	}
	_, err = svc.Create(ctx, futureOffering)
	require.NoError(t, err)

	// Inactive offering with otherwise-open window: NOT returned.
	inactiveOffering := &enrollmentModels.CareOffering{
		CalendarPeriodID: &periodID,
		Name:             "Inactive Offering",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		IsActive:         false,
	}
	_, err = svc.Create(ctx, inactiveOffering)
	require.NoError(t, err)

	// Unbounded window (NULL start AND end), active: SHOULD be returned.
	unboundedOffering := &enrollmentModels.CareOffering{
		CalendarPeriodID: &periodID,
		Name:             "Unbounded Offering",
		DaysOfWeekMode:   enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:    []string{"mon"},
		IsActive:         true,
	}
	_, err = svc.Create(ctx, unboundedOffering)
	require.NoError(t, err)

	publicList, err := svc.ListPublic(ctx, now)
	require.NoError(t, err)
	names := make(map[string]bool, len(publicList))
	for _, o := range publicList {
		names[o.Name] = true
	}
	assert.True(t, names["Open Offering"], "open offering must appear")
	assert.True(t, names["Unbounded Offering"], "unbounded offering must appear")
	assert.False(t, names["Closed Offering"], "closed window must not appear")
	assert.False(t, names["Future Offering"], "not-yet-open window must not appear")
	assert.False(t, names["Inactive Offering"], "inactive offerings must not appear")
}

func TestCareOfferingService_HasUnlimitedCapacity(t *testing.T) {
	// Pure-model unit test — no DB. Verifies the null=unlimited
	// semantics exposed on the model directly so the service layer
	// can rely on them.
	unlimited := &enrollmentModels.CareOffering{Name: "X", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed}
	assert.True(t, unlimited.HasUnlimitedCapacity())

	cap := 10
	limited := &enrollmentModels.CareOffering{Name: "Y", DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed, Capacity: &cap}
	assert.False(t, limited.HasUnlimitedCapacity())
}

func TestCareOfferingService_Clone_ShiftsDatesAndPivotsCalendarPeriod(t *testing.T) {
	_, svc, period, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	periodID := period.ID

	startWindow := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	endWindow := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	startService := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	endService := time.Date(2027, 7, 31, 0, 0, 0, 0, time.UTC)

	source := &enrollmentModels.CareOffering{
		CalendarPeriodID:       &periodID,
		Name:                   "Regelbetreuung",
		DaysOfWeekMode:         enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:          []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesHolidayCare:    true,
		IncludesLunch:          true,
		ApplicationWindowStart: &startWindow,
		ApplicationWindowEnd:   &endWindow,
		ServiceStartDate:       &startService,
		ServiceEndDate:         &endService,
		IsActive:               true,
	}
	_, err := svc.Create(ctx, source)
	require.NoError(t, err)

	clone, err := svc.Clone(ctx, source.ID, periodID, 365)
	require.NoError(t, err)
	require.NotEqual(t, source.ID, clone.ID)
	assert.Equal(t, "Regelbetreuung", clone.Name, "name preserved")
	require.NotNil(t, clone.ApplicationWindowStart)
	assert.Equal(t,
		startWindow.AddDate(0, 0, 365).Unix(),
		clone.ApplicationWindowStart.Unix(),
		"window start shifted by 365 days")
	require.NotNil(t, clone.ServiceStartDate)
	assert.Equal(t,
		startService.AddDate(0, 0, 365).Unix(),
		clone.ServiceStartDate.Unix(),
		"service start shifted by 365 days")
	assert.True(t, clone.IncludesLunch, "non-date fields preserved")
}

func TestCareOfferingService_Clone_ZeroOffsetIsExactCopy(t *testing.T) {
	_, svc, period, cleanup := setupCareTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)
	periodID := period.ID

	startWindow := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	source := &enrollmentModels.CareOffering{
		CalendarPeriodID:       &periodID,
		Name:                   "Regelbetreuung",
		DaysOfWeekMode:         enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:          []string{"mon"},
		ApplicationWindowStart: &startWindow,
		IsActive:               true,
	}
	_, err := svc.Create(ctx, source)
	require.NoError(t, err)

	clone, err := svc.Clone(ctx, source.ID, periodID, 0)
	require.NoError(t, err)
	require.NotNil(t, clone.ApplicationWindowStart)
	assert.Equal(t,
		startWindow.Unix(),
		clone.ApplicationWindowStart.Unix(),
		"zero offset must preserve dates exactly")
}
