package contracttest_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3163: the fixture's today is 24 August 2026, a Monday.
var cutoffFixtureToday = timezone.NewDate(2026, 8, 24)

func cutoffAt(t *testing.T, hour, minute int) careplan.SameDayCutoff {
	t.Helper()
	cutoff, err := careplan.NewSameDayCutoff("11:00",
		time.Date(2026, 8, 24, hour, minute, 0, 0, timezone.Berlin))
	require.NoError(t, err)
	return cutoff
}

func createPickupChangeWithCutoff(
	t *testing.T, f *careFixture, date timezone.Date, cutoff careplan.SameDayCutoff,
) (*carerequests.Request, error) {
	t.Helper()
	return f.svc.CreatePickupChange(f.staffCtx(f.staffAccount), carerequests.PickupChangeCreateInput{
		StudentID:         f.chain.StudentID,
		GuardianAccountID: f.chain.AccountID,
		Date:              date,
		PickupTime:        time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		Reason:            "Arzttermin",
		ReasonRequired:    true,
		Cutoff:            cutoff,
	})
}

func TestCreatePickupChangeHonoursSameDayCutoff(t *testing.T) {
	t.Parallel()

	t.Run("heute vor der Frist", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		_, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 10, 59))
		require.NoError(t, err)
	})

	t.Run("heute genau zur Frist", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		_, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 11, 0))
		require.NoError(t, err)
	})

	t.Run("heute nach der Frist", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		_, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 11, 1))
		require.ErrorIs(t, err, careplan.ErrPickupChangeCutoffPassed)

		pending, err := f.svc.ListPickupChangeRequests(
			f.staffCtx(f.staffAccount), f.chain.StudentID, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
		require.NoError(t, err)
		assert.Empty(t, pending, "a refused request leaves no row behind")
	})

	t.Run("morgen nach der Frist", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		_, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday.AddDays(1), cutoffAt(t, 11, 1))
		require.NoError(t, err)
	})

	t.Run("ohne Frist spät am Tag", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		none, err := careplan.NewSameDayCutoff("", time.Date(2026, 8, 24, 23, 0, 0, 0, timezone.Berlin))
		require.NoError(t, err)
		_, err = createPickupChangeWithCutoff(t, f, cutoffFixtureToday, none)
		require.NoError(t, err)
	})
}

// After the cutoff today is closed in both directions: a request for today
// cannot be changed or moved away, and no request can be moved onto today.
// Requests for later days stay editable.
func TestEditPickupChangeHonoursSameDayCutoff(t *testing.T) {
	t.Parallel()

	edit := func(t *testing.T, f *careFixture, req *carerequests.Request, date timezone.Date, cutoff careplan.SameDayCutoff) error {
		t.Helper()
		_, err := f.svc.EditRequest(f.staffCtx(f.staffAccount), carerequests.EditInput{
			RequestID:         req.ID,
			StudentID:         f.chain.StudentID,
			GuardianAccountID: f.chain.AccountID,
			Date:              date,
			PickupTime:        time.Date(2000, 1, 1, 13, 30, 0, 0, time.UTC),
			Reason:            "Termin verschoben",
			ReasonRequired:    true,
			Cutoff:            cutoff,
		})
		return err
	}

	t.Run("heutige Anfrage ändern", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		req, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 10, 0))
		require.NoError(t, err)

		require.NoError(t, edit(t, f, req, cutoffFixtureToday, cutoffAt(t, 11, 0)), "exactly at the cutoff is still open")
		require.ErrorIs(t, edit(t, f, req, cutoffFixtureToday, cutoffAt(t, 11, 1)), careplan.ErrPickupChangeCutoffPassed)

		row, err := f.repos.CareScheduleChangeRequest.FindByID(f.staffCtx(f.staffAccount), req.ID)
		require.NoError(t, err)
		assert.Equal(t, "13:30", row.Payload["pickup_time"], "the refused edit left the 11:00 state untouched")
		assert.Equal(t, scheduleModels.CareRequestStatusPending, row.Status)
	})

	t.Run("heutige Anfrage auf morgen verschieben", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		req, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 10, 0))
		require.NoError(t, err)

		require.ErrorIs(t, edit(t, f, req, cutoffFixtureToday.AddDays(1), cutoffAt(t, 11, 1)), careplan.ErrPickupChangeCutoffPassed)
	})

	t.Run("morgige Anfrage auf heute verschieben", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		req, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday.AddDays(1), cutoffAt(t, 10, 0))
		require.NoError(t, err)

		require.ErrorIs(t, edit(t, f, req, cutoffFixtureToday, cutoffAt(t, 11, 1)), careplan.ErrPickupChangeCutoffPassed)
	})

	t.Run("morgige Anfrage ändern", func(t *testing.T) {
		t.Parallel()
		f := newPickupChangeFixture(t)
		req, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday.AddDays(1), cutoffAt(t, 10, 0))
		require.NoError(t, err)

		require.NoError(t, edit(t, f, req, cutoffFixtureToday.AddDays(2), cutoffAt(t, 11, 1)))
	})
}

// The cutoff binds guardians only. A request that came in before it stays
// decidable for staff afterwards; the decision path takes no cutoff at all.
func TestStaffDecidesTodaysPickupChangeAfterCutoff(t *testing.T) {
	t.Parallel()

	f := newPickupChangeFixture(t)
	req, err := createPickupChangeWithCutoff(t, f, cutoffFixtureToday, cutoffAt(t, 10, 30))
	require.NoError(t, err)

	decided, err := f.svc.Decide(f.staffCtx(f.staffAccount), carerequests.DecideInput{
		RequestID:  req.ID,
		Approve:    true,
		ReviewedBy: f.staffAccount,
	})
	require.NoError(t, err)
	assert.Equal(t, scheduleModels.CareRequestStatusApproved, decided.Request.Status)

	applied, err := f.repos.StudentPickupException.FindByStudentIDAndDate(
		f.staffCtx(f.staffAccount), f.chain.StudentID, scheduleModels.Date(cutoffFixtureToday))
	require.NoError(t, err)
	require.NotNil(t, applied)
	assert.Equal(t, scheduleModels.ExceptionSourceStaff, applied.Source)
}
