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
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
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
	t.Cleanup(func() { _ = db.Close() })
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

// The input checks run before anything is resolved, so they need no child and
// no school setting at all.
func TestSubmitPickupChangeRequestRejectsBadInput(t *testing.T) {
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
	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
		chain.StudentID+999999, timezone.TodayDate().AddDays(1), pickup, "Arzttermin")

	require.Error(t, err, "ein nicht verknuepftes Kind muss abgewiesen werden")
}

// Schools that do not run parent-side pickup changes must not receive one.
func TestSubmitPickupChangeRequestRespectsSchoolSetting(t *testing.T) {
	svc, db := buildPickupChangeService(t, false)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	pickup := timezone.WallClock(time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC))
	_, err := svc.SubmitPickupChangeRequest(context.Background(), chain.AccountID,
		chain.StudentID, timezone.TodayDate().AddDays(1), pickup, "Arzttermin")

	require.ErrorIs(t, err, parentService.ErrPickupChangeDisabled)
}

// The window is today through two months out. Yesterday is over, and a date
// far in the future is a typo rather than a plan.
func TestSubmitPickupChangeRequestBoundsTheDate(t *testing.T) {
	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

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
	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)
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
	svc, db := buildPickupChangeService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.ListPickupChangeRequests(context.Background(), chain.AccountID, chain.StudentID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}
