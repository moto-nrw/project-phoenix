package parent_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/services"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/schedule/scheduletest"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildPickupChangeService wires the parent service for the pickup-change flow
// (#2304). CareRequests stays nil on purpose: every case below is rejected
// before the request service would be reached, which is exactly the boundary
// worth pinning — a parent must never get as far as writing a row when the
// input, the school's setting, or the date says no.
func buildPickupChangeService(t *testing.T, pickupChangeEnabled bool) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentPickupChangeEnabled: pickupChangeEnabled,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		DB:     db,
		Logger: slog.Default(),
	})
	return svc, db
}

// buildPickupChangeServiceWithRequests adds the real request service, so the
// submit/list/withdraw round trip runs against actual rows.
func buildPickupChangeServiceWithRequests(t *testing.T) (parentService.Service, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)

	careRequests := scheduleSvc.NewCareScheduleRequestServiceWithPickupChanges(
		repos.CareScheduleChangeRequest,
		repos.Student,
		repos.Person,
		sf.ArrivalSchedule,
		sf.PickupSchedule,
		repos.StudentPickupException,
		repos.Attendance,
		scheduleSvc.NewPickupAutoExcusalSyncer(
			repos.StudentPickupException,
			scheduletest.NewPickupBaselineService(repos.StudentPickupSchedule, repos.RequestChildOffering, repos.CareOffering),
			repos.InstanceStudent,
			db,
		),
		sf.UserContext,
		nil, // emitter — best-effort, after commit
		nil, // broadcaster — cache fan-out
		slog.Default(),
		sf.StudentAudit,
	)

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		PickupExceptionRepo: repos.StudentPickupException,
		AttendanceRepo:      repos.Attendance,
		CareRequests:        careRequests,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentPickupChangeEnabled: true,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		DB:     db,
		Logger: slog.Default(),
	})
	return svc, db, repos
}

// TestPickupChangeRoundTrip walks the flow a parent actually performs: submit a
// change, see it in their own list, then take it back.
func TestPickupChangeRoundTrip(t *testing.T) {
	t.Parallel()

	svc, db, repos := buildPickupChangeServiceWithRequests(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := testpkg.Ctx(t)
	date := timezone.TodayDate().AddDays(3)
	pickup := timezone.WallClock(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC))

	created, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "Arzttermin")
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.Equal(t, scheduleModels.CareRequestStatusPending, created.Status)

	listed, err := svc.ListPickupChangeRequests(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	found := false
	for _, row := range listed {
		if row.ID == created.ID {
			found = true
		}
	}
	assert.True(t, found, "die eigene Anfrage steht in der Liste")

	withdrawn, err := svc.WithdrawPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, created.ID)
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusWithdrawn, withdrawn.Status)

	// Nothing was applied: only a staff approval may move a pickup time.
	applied, err := repos.StudentPickupException.FindByStudentIDAndDate(
		tenant.WithTenantID(ctx, chain.TenantID), chain.StudentID, date)
	require.NoError(t, err)
	assert.Nil(t, applied, "eine Anfrage allein aendert keine Abholzeit")
}

func TestWithdrawPickupChangeRequestRejectsEndedCare(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildPickupChangeServiceWithRequests(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.Ctx(t)
	date := timezone.TodayDate().AddDays(3)
	pickup := timezone.WallClock(time.Date(2026, 1, 1, 14, 30, 0, 0, time.UTC))

	request, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "Arzttermin")
	require.NoError(t, err)

	_, err = db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", timezone.TodayDate().AddDays(-1)).
		Where("id = ?", chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	_, err = svc.WithdrawPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, request.ID)
	require.ErrorIs(t, err, parentService.ErrChildCareEnded)
}

// A second open request for the same day would leave staff with two answers to
// give, so the first one has to block it.
func TestPickupChangeRejectsASecondOpenRequestForTheSameDay(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildPickupChangeServiceWithRequests(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := context.Background()
	date := timezone.TodayDate().AddDays(4)
	pickup := timezone.WallClock(time.Date(2026, 1, 1, 14, 0, 0, 0, time.UTC))

	_, err := svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "Arzttermin")
	require.NoError(t, err)

	_, err = svc.SubmitPickupChangeRequest(ctx, chain.AccountID, chain.StudentID, date, pickup, "Noch ein Termin")
	require.Error(t, err, "eine zweite offene Anfrage fuer denselben Tag muss abgewiesen werden")
}

// The input checks run before anything is resolved, so they need no child and
// no school setting at all.
func TestSubmitPickupChangeRequestRejectsBadInput(t *testing.T) {
	t.Parallel()

	svc, _ := buildPickupChangeService(t, true)
	tomorrow := timezone.TodayDate().AddDays(1)
	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))

	t.Run("ohne Uhrzeit", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), 1, 1, tomorrow, time.Time{}, "Arzttermin")
		require.ErrorIs(t, err, parentService.ErrNoCareException)
	})

	t.Run("ohne Grund", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), 1, 1, tomorrow, pickup, "   ")
		require.ErrorIs(t, err, parentService.ErrCareExceptionReasonRequired)
	})

	t.Run("Grund zu lang", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), 1, 1, tomorrow, pickup,
			strings.Repeat("a", 256))
		require.ErrorIs(t, err, parentService.ErrCareExceptionReasonTooLong)
	})

	// 255 runes is the limit, and it counts runes, not bytes: an Umlaut must not
	// cost two characters of a parent's explanation.
	t.Run("255 Umlaute sind kein zu langer Grund", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), 1, 1, tomorrow, pickup,
			strings.Repeat("ä", 255))
		assert.NotErrorIs(t, err, parentService.ErrCareExceptionReasonTooLong)
	})
}

// A child the account is not linked to must be refused before any school
// setting or date is considered.
func TestSubmitPickupChangeRequestRejectsForeignChild(t *testing.T) {
	t.Parallel()

	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
		chain.StudentID+999999, timezone.TodayDate().AddDays(1), pickup, "Arzttermin")

	require.Error(t, err, "ein nicht verknuepftes Kind muss abgewiesen werden")
}

// Schools that do not run parent-side pickup changes must not receive one.
func TestSubmitPickupChangeRequestRespectsSchoolSetting(t *testing.T) {
	t.Parallel()

	svc, db := buildPickupChangeService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
		chain.StudentID, timezone.TodayDate().AddDays(1), pickup, "Arzttermin")

	require.ErrorIs(t, err, parentService.ErrPickupChangeDisabled)
}

// The window is today through two months out. Yesterday is over, and a date
// far in the future is a typo rather than a plan.
func TestSubmitPickupChangeRequestBoundsTheDate(t *testing.T) {
	t.Parallel()

	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	today := timezone.TodayDate()

	t.Run("gestern", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
			chain.StudentID, today.AddDays(-1), pickup, "Arzttermin")
		require.ErrorIs(t, err, parentService.ErrPastCareDate)
	})

	t.Run("zu weit voraus", func(t *testing.T) {
		_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
			chain.StudentID, today.AddDays(200), pickup, "Arzttermin")
		require.ErrorIs(t, err, parentService.ErrCareDateTooFar)
	})
}

// Listing and withdrawing are guarded by the same relationship check as the
// submit, so a foreign child must not be readable or writable either.
func TestPickupChangeReadAndWithdrawRejectForeignChild(t *testing.T) {
	t.Parallel()

	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	foreign := chain.StudentID + 999999

	t.Run("list", func(t *testing.T) {
		_, err := svc.ListPickupChangeRequests(context.Background(), chain.AccountID, foreign)
		require.Error(t, err)
	})

	t.Run("withdraw", func(t *testing.T) {
		_, err := svc.WithdrawPickupChangeRequest(context.Background(), chain.AccountID, foreign, 1)
		require.Error(t, err)
	})
}

// Without a wired request service the flow must fail loudly rather than report
// an empty list, which would read to a parent as "no requests".
func TestPickupChangeRequiresConfiguredRequestService(t *testing.T) {
	t.Parallel()

	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.ListPickupChangeRequests(context.Background(), chain.AccountID, chain.StudentID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}
