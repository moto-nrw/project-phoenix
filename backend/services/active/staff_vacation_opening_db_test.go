package active_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	activeRepos "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepos "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	active "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Vacation takeover (#2132) against the real database: the takeover row, its
// effect on the Resturlaub arithmetic, the guards that keep it a one-shot
// booking, and the deletion tombstone.

// vacationOpeningFixture is one tenant with a staff member whose vacation is
// taken over and an admin who books it. No quota row is created on purpose —
// the takeover must fall back to the registry default of 30/0.
type vacationOpeningFixture struct {
	tenantID int64
	db       *bun.DB
	repos    *repositories.Factory
	ctx      context.Context
	staff    *userModels.Staff
	admin    *userModels.Staff
	svc      active.StaffAbsenceService
	// cutoff is the Stichtag every test books against; year is derived from
	// it, exactly as the service does.
	cutoff timezone.Date
}

func newVacationOpeningFixture(t *testing.T) *vacationOpeningFixture {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	staff := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Urlaub", "Uebernahme")
	admin := testpkg.CreateTestStaffForTenant(t, db, tenantID, "Urlaub", "Leitung")
	repos := repositories.NewFactory(db)
	ctx := testpkg.TenantContext(tenantID)

	t.Cleanup(func() {
		for _, table := range []string{
			"audit.time_tracking_deletions",
			"active.staff_vacation_openings",
			"active.staff_vacation_quotas",
			"active.staff_absences",
		} {
			_, _ = db.NewDelete().ModelTableExpr(table+" AS t").Where("t.tenant_id = ?", tenantID).Exec(context.Background())
		}
		testpkg.CleanupStaffFixtures(t, db, staff.ID, admin.ID)
		testpkg.CleanupTenantTestData(t, db, tenantID)
	})

	svc := active.NewStaffAbsenceService(
		repos.StaffAbsence, repos.WorkSession, repos.StaffVacationQuota,
		repos.StaffAbsenceAudit, wtmIntSettings{}, nil,
	)
	// Setter injection, wired exactly like services/factory.go does.
	openingAware, ok := svc.(interface {
		SetVacationOpeningRepository(activeModels.StaffVacationOpeningRepository)
	})
	require.True(t, ok, "staff absence service must accept the vacation opening repository")
	openingAware.SetVacationOpeningRepository(activeRepos.NewStaffVacationOpeningRepository(db))

	deletionAware, ok := svc.(interface {
		SetDeletionAudit(auditModels.TimeTrackingDeletionRepository)
	})
	require.True(t, ok, "staff absence service must accept the deletion audit repository")
	deletionAware.SetDeletionAudit(auditRepos.NewTimeTrackingDeletionRepository(db))

	return &vacationOpeningFixture{
		tenantID: tenantID,
		db:       db,
		repos:    repos,
		ctx:      ctx,
		staff:    staff,
		admin:    admin,
		svc:      svc,
		cutoff:   openingCutoffDate(t),
	}
}

// openingCutoffDate returns a Stichtag in the past that still leaves room for
// vacation absences EARLIER in the same calendar year. Yesterday satisfies
// both for all but the first days of January. There is deliberately no
// previous-year fallback: the service only books takeovers into the running
// vacation year, so in early January no usable Stichtag exists and the fixture
// skips instead of exercising a case the service rejects by design.
func openingCutoffDate(t *testing.T) timezone.Date {
	t.Helper()
	today := timezone.TodayDate()
	cutoff := today.AddDays(-1)
	if cutoff.Year != today.Year || timezone.NewDate(today.Year, time.January, 1).DaysUntil(cutoff) < 5 {
		t.Skip("no closed Stichtag with room for earlier absences in the current vacation year yet")
	}
	return cutoff
}

// previousWorkingDay walks back to the nearest Monday-Friday day. Vacation
// quota only counts working days, so an absence fixture that must consume
// quota may not land on a weekend — otherwise the guard under test never
// trips and the case passes vacuously on some weekdays.
func previousWorkingDay(d timezone.Date) timezone.Date {
	for d.Weekday() == time.Saturday || d.Weekday() == time.Sunday {
		d = d.AddDays(-1)
	}
	return d
}

// addVacationAbsence writes an absence row directly, mirroring the overview
// fixture — the vacation request flow is not under test here.
func (f *vacationOpeningFixture) addVacationAbsence(t *testing.T, status string, start, end timezone.Date) {
	t.Helper()
	absence := &activeModels.StaffAbsence{
		StaffID:     f.staff.ID,
		AbsenceType: activeModels.AbsenceTypeVacation,
		Status:      status,
		DateStart:   start,
		DateEnd:     end,
		CreatedBy:   f.staff.ID,
	}
	absence.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffAbsence.Create(f.ctx, absence))
}

func (f *vacationOpeningFixture) set(remainingDays float64) (*activeModels.StaffVacationOpening, error) {
	return f.svc.SetVacationOpening(f.ctx, f.staff.ID, f.admin.ID, active.SetVacationOpeningRequest{
		EffectiveDate: f.cutoff,
		RemainingDays: remainingDays,
		Note:          "Übernahme aus Altsystem",
	})
}

// TestSetVacationOpening_DerivesTakenBeforeFromQuota is the core arithmetic:
// the admin enters the Resturlaub, the service stores the days taken before
// the introduction, and the summary reproduces the entered Resturlaub.
func TestSetVacationOpening_DerivesTakenBeforeFromQuota(t *testing.T) {
	f := newVacationOpeningFixture(t)

	opening, err := f.set(12.5)
	require.NoError(t, err)
	require.NotNil(t, opening)

	// Quota default 30/0: 30 + 0 − 12,5 already taken before the Stichtag.
	assert.InDelta(t, 17.5, opening.TakenBeforeDays, 0.001)
	assert.InDelta(t, 12.5, opening.EnteredRemainingDays, 0.001)
	assert.Equal(t, f.cutoff, opening.EffectiveDate)
	assert.Equal(t, f.cutoff.Year, opening.Year)
	assert.Equal(t, f.admin.ID, opening.DecidedBy)
	assert.Equal(t, f.tenantID, opening.TenantID)

	summary, err := f.svc.GetVacationQuotaSummary(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.InDelta(t, 17.5, summary.TakenBeforeDays, 0.001)
	assert.InDelta(t, 12.5, summary.RemainingDays, 0.001,
		"the entered Resturlaub must survive the round trip through the summary")
	assert.InDelta(t, 30, summary.EntitledDays, 0.001, "the Jahresanspruch stays untouched")
	require.NotNil(t, summary.Opening, "the summary carries the takeover row for the admin UI")
	assert.Equal(t, opening.ID, summary.Opening.ID)

	// The takeover belongs to its own year only.
	nextYear, err := f.svc.GetVacationQuotaSummary(f.ctx, f.staff.ID, f.cutoff.Year+1)
	require.NoError(t, err)
	assert.Zero(t, nextYear.TakenBeforeDays)
	assert.Nil(t, nextYear.Opening)

	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, opening.ID, stored.ID)
}

// TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount: a staff
// member who already overdrew their vacation in the old system must be
// representable — the takeover is signed on both ends.
func TestSetVacationOpening_NegativeRemainingAllowsOverdrawnAccount(t *testing.T) {
	f := newVacationOpeningFixture(t)

	opening, err := f.set(-2)
	require.NoError(t, err)
	require.NotNil(t, opening)
	assert.InDelta(t, 32, opening.TakenBeforeDays, 0.001)

	summary, err := f.svc.GetVacationQuotaSummary(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.InDelta(t, -2, summary.RemainingDays, 0.001,
		"an overdrawn vacation account must stay negative instead of being clamped to zero")
}

// TestSetVacationOpening_RejectsSecondOpeningForSameYear pins the one-shot
// rule: corrections are delete + re-create so the tombstone keeps the history.
func TestSetVacationOpening_RejectsSecondOpeningForSameYear(t *testing.T) {
	f := newVacationOpeningFixture(t)

	_, err := f.set(10)
	require.NoError(t, err)

	_, err = f.set(8)
	require.ErrorIs(t, err, active.ErrVacationOpeningExists)

	// The first booking is untouched.
	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.InDelta(t, 10, stored.EnteredRemainingDays, 0.001)
}

// TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff guards the double
// count: days already booked in-app before the Stichtag are also part of the
// old system's Resturlaub figure.
func TestSetVacationOpening_RejectsVacationAbsencesBeforeCutoff(t *testing.T) {
	f := newVacationOpeningFixture(t)

	// Must be a working day: a Saturday absence spends no quota and the guard
	// would not trip, letting the case pass vacuously on some weekdays.
	before := previousWorkingDay(f.cutoff.AddDays(-1))
	f.addVacationAbsence(t, activeModels.AbsenceStatusApproved, before, before)

	_, err := f.set(12)
	require.ErrorIs(t, err, active.ErrVacationOpeningAbsencesBeforeCutoff)

	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.Nil(t, stored, "nothing may be booked when the guard trips")
}

func TestSetVacationOpening_AllowsVacationBeginningOnWeekendBeforeCutoff(t *testing.T) {
	f := newVacationOpeningFixture(t)
	for f.cutoff.Weekday() != time.Monday {
		f.cutoff = f.cutoff.AddDays(-1)
	}

	// The absence begins on Saturday but only consumes quota on the Monday
	// Stichtag, which belongs to the moto-era calculation.
	f.addVacationAbsence(t, activeModels.AbsenceStatusApproved, f.cutoff.AddDays(-2), f.cutoff)

	opening, err := f.set(12)
	require.NoError(t, err)
	require.NotNil(t, opening)
}

// TestSetVacationOpening_IgnoresDeclinedAndLaterAbsences: only absences that
// actually spend quota before the Stichtag block the takeover.
func TestSetVacationOpening_IgnoresDeclinedAndLaterAbsences(t *testing.T) {
	f := newVacationOpeningFixture(t)

	before := f.cutoff.AddDays(-1)
	f.addVacationAbsence(t, activeModels.AbsenceStatusDeclined, before, before)
	f.addVacationAbsence(t, activeModels.AbsenceStatusCanceled, before, before)
	// Starting ON the Stichtag is already inside the moto era.
	f.addVacationAbsence(t, activeModels.AbsenceStatusApproved, f.cutoff, f.cutoff)
	// A sick day before the Stichtag never touches the vacation quota.
	sick := &activeModels.StaffAbsence{
		StaffID:     f.staff.ID,
		AbsenceType: activeModels.AbsenceTypeSick,
		Status:      activeModels.AbsenceStatusApproved,
		DateStart:   before,
		DateEnd:     before,
		CreatedBy:   f.staff.ID,
	}
	sick.SetTenantID(f.tenantID)
	require.NoError(t, f.repos.StaffAbsence.Create(f.ctx, sick))

	opening, err := f.set(12)
	require.NoError(t, err)
	require.NotNil(t, opening)
	assert.InDelta(t, 18, opening.TakenBeforeDays, 0.001)
}

// TestSetVacationOpening_RejectsOpenCutoff: the Stichtag needs a closed day,
// otherwise today's bookings would be counted on both sides.
func TestSetVacationOpening_RejectsOpenCutoff(t *testing.T) {
	f := newVacationOpeningFixture(t)

	for _, effectiveDate := range []timezone.Date{
		timezone.TodayDate(),
		timezone.TodayDate().AddDays(1),
	} {
		_, err := f.svc.SetVacationOpening(f.ctx, f.staff.ID, f.admin.ID, active.SetVacationOpeningRequest{
			EffectiveDate: effectiveDate,
			RemainingDays: 12,
			Note:          "Übernahme aus Altsystem",
		})
		require.ErrorIs(t, err, active.ErrVacationOpeningInvalid)
		assert.Contains(t, err.Error(), "effective_date must be before today")
	}

	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, timezone.TodayDate().Year)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

// TestSetVacationOpening_RejectsPastVacationYear: a Stichtag in a closed year
// would rebaseline a historical vacation account. The import handler bounds
// this already; the service must bound it too so the per-staff route cannot
// bypass the restriction.
func TestSetVacationOpening_RejectsPastVacationYear(t *testing.T) {
	f := newVacationOpeningFixture(t)

	lastYear := timezone.NewDate(timezone.TodayDate().Year-1, time.June, 10)
	_, err := f.svc.SetVacationOpening(f.ctx, f.staff.ID, f.admin.ID, active.SetVacationOpeningRequest{
		EffectiveDate: lastYear,
		RemainingDays: 12,
		Note:          "Übernahme aus Altsystem",
	})
	require.ErrorIs(t, err, active.ErrVacationOpeningInvalid)
	assert.Contains(t, err.Error(), "effective_date must be in the current vacation year")

	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, lastYear.Year)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

// TestSetVacationOpening_RequiresNote: the takeover row IS the audit record,
// so it may not be booked anonymously.
func TestSetVacationOpening_RequiresNote(t *testing.T) {
	f := newVacationOpeningFixture(t)

	_, err := f.svc.SetVacationOpening(f.ctx, f.staff.ID, f.admin.ID, active.SetVacationOpeningRequest{
		EffectiveDate: f.cutoff,
		RemainingDays: 12,
		Note:          "   ",
	})
	require.ErrorIs(t, err, active.ErrVacationOpeningInvalid)
}

// TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary: the delete
// removes the arithmetic but not the history.
func TestDeleteVacationOpening_WritesTombstoneAndRestoresSummary(t *testing.T) {
	f := newVacationOpeningFixture(t)

	opening, err := f.set(12.5)
	require.NoError(t, err)

	require.NoError(t, f.svc.DeleteVacationOpening(f.ctx, f.staff.ID, f.admin.ID, f.cutoff.Year))

	stored, err := f.svc.GetVacationOpening(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.Nil(t, stored)

	summary, err := f.svc.GetVacationQuotaSummary(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.Zero(t, summary.TakenBeforeDays)
	assert.Nil(t, summary.Opening)
	assert.InDelta(t, 30, summary.RemainingDays, 0.001,
		"without a takeover the Resturlaub is the plain Jahresanspruch again")

	var tombstones []auditModels.TimeTrackingDeletion
	require.NoError(t, f.db.NewSelect().
		Model(&tombstones).
		ModelTableExpr(`audit.time_tracking_deletions AS "time_tracking_deletion"`).
		Where(`"time_tracking_deletion".staff_id = ?`, f.staff.ID).
		Where(`"time_tracking_deletion".source = ?`, auditModels.TimeTrackingDeletionSourceVacationOpening).
		Scan(f.ctx))
	require.Len(t, tombstones, 1, "the delete must leave exactly one tombstone")
	assert.Equal(t, opening.ID, tombstones[0].SourceID)
	assert.Equal(t, f.admin.ID, tombstones[0].DeletedBy)
	assert.Equal(t, f.tenantID, tombstones[0].TenantID)

	var payload activeModels.StaffVacationOpening
	require.NoError(t, json.Unmarshal(tombstones[0].Payload, &payload),
		"the tombstone payload must be the deleted row verbatim")
	assert.InDelta(t, 17.5, payload.TakenBeforeDays, 0.001)
	assert.InDelta(t, 12.5, payload.EnteredRemainingDays, 0.001)

	// Delete + re-create is the sanctioned correction path.
	corrected, err := f.set(9)
	require.NoError(t, err)
	assert.InDelta(t, 21, corrected.TakenBeforeDays, 0.001)
}

// TestDeleteVacationOpening_MissingRowIsNotFound keeps the handler's 404
// mapping honest.
func TestDeleteVacationOpening_MissingRowIsNotFound(t *testing.T) {
	f := newVacationOpeningFixture(t)

	err := f.svc.DeleteVacationOpening(f.ctx, f.staff.ID, f.admin.ID, f.cutoff.Year)
	require.ErrorIs(t, err, active.ErrVacationOpeningNotFound)
}

// TestSetVacationOpening_RespectsCustomQuota: the takeover derives from the
// stored Jahresanspruch, not from the 30/0 default.
func TestSetVacationOpening_RespectsCustomQuota(t *testing.T) {
	f := newVacationOpeningFixture(t)

	require.NoError(t, f.svc.UpsertVacationQuota(f.ctx, f.staff.ID, f.cutoff.Year, 26, 4))

	opening, err := f.set(12.5)
	require.NoError(t, err)
	// 26 + 4 − 12,5
	assert.InDelta(t, 17.5, opening.TakenBeforeDays, 0.001)

	summary, err := f.svc.GetVacationQuotaSummary(f.ctx, f.staff.ID, f.cutoff.Year)
	require.NoError(t, err)
	assert.InDelta(t, 12.5, summary.RemainingDays, 0.001)
}
