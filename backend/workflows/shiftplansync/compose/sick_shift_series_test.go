// Hermetic integration tests for shifts a series materializes after a sick
// report (#6302).
//
// Invariants:
//   - S1: a shift materialized from a series after a sick report is, on the
//     report's full sick days, in the state the cascade gives a shift
//     materialized before: cancelled, detached, with the neutral reason and
//     the report's stamp. Other days stay planned.
//   - S2: half days and half boundary days are left planned, as the cascade
//     leaves them (timetable.SickReport.CoversFullDay).
//   - S3: shortening or deleting the report reactivates the later shifts
//     exactly like the earlier ones.
//
// #6302 adds no lock and no lock order; concurrent sick-report writes are
// #6232's.
//
// The sick reports run through the real Workforce absence writes and their
// cascade; the shifts through the real series service.
package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createMondaySeries plans the subject every Monday of stampWeekOne to
// stampWeekThree in [start, end) and returns the series.
func (e *sickCascadeEnv) createMondaySeries(t *testing.T, periodID int64, start, end string) int64 {
	t.Helper()
	var seriesID int64
	e.inTx(t, func(ctx context.Context) error {
		result, err := e.Planning.CreateSeries(ctx, workforce.CreateStaffShiftSeries{StaffShiftSeriesInput: workforce.StaffShiftSeriesInput{
			StaffID: e.subject.ID, Weekdays: []int{1}, StartTime: clock(start), EndTime: clock(end),
			CalendarPeriodID: periodID, ValidFrom: stampWeekOne.String(), ValidUntil: stampWeekThree.AddDays(1).String(),
			ActorStaffID: e.admin.ID,
		}})
		seriesID = result.SeriesID
		return err
	})
	return seriesID
}

func (e *sickCascadeEnv) createSeriesPeriod(t *testing.T) int64 {
	t.Helper()
	period := testpkg.CreateTestCalendarPeriod(t, e.db, fmt.Sprintf("Series-Period-%d", time.Now().UnixNano()),
		calendar.NewDate(2026, time.August, 1), calendar.NewDate(2026, time.December, 31))
	return period.ID
}

// seriesShift loads the series' only shift on date.
func (e *sickCascadeEnv) seriesShift(t *testing.T, seriesID int64, date calendar.Date) workforce.StaffShift {
	t.Helper()
	shifts, err := e.shifts.ListStaffShifts(e.ctx, workforce.StaffShiftFilter{StaffID: e.subject.ID, From: date.String(), To: date.String()})
	require.NoError(t, err)
	var found []workforce.StaffShift
	for _, shift := range shifts {
		if shift.SeriesID != nil && *shift.SeriesID == seriesID {
			found = append(found, shift)
		}
	}
	require.Len(t, found, 1, "series %d has exactly one shift on %s", seriesID, date)
	return found[0]
}

// sickState is the part of a shift the sick cascade decides.
type sickState struct {
	Cancelled     bool
	Detached      bool
	ChangeReason  *string
	SickAbsenceID *int64
}

func sickStateOf(shift workforce.StaffShift) sickState {
	return sickState{Cancelled: shift.Cancelled, Detached: shift.Detached, ChangeReason: shift.ChangeReason, SickAbsenceID: shift.SickAbsenceID}
}

func assertSickCancelled(t *testing.T, shift workforce.StaffShift, absenceID int64, msg string) {
	t.Helper()
	assert.True(t, shift.Cancelled, msg+": cancelled")
	if assert.NotNil(t, shift.SickAbsenceID, msg+": stamp") {
		assert.Equal(t, absenceID, *shift.SickAbsenceID, msg+": stamp names the report")
	}
	if assert.NotNil(t, shift.ChangeReason, msg+": reason") {
		assert.Equal(t, workforce.SickShiftChangeReason, *shift.ChangeReason, msg+": neutral reason")
	}
}

func assertPlanned(t *testing.T, shift workforce.StaffShift, msg string) {
	t.Helper()
	assert.False(t, shift.Cancelled, msg+": planned")
	assert.Nil(t, shift.SickAbsenceID, msg+": no stamp")
}

func (e *sickCascadeEnv) deleteSickAbsence(t *testing.T, absenceID int64) {
	t.Helper()
	e.inTx(t, func(ctx context.Context) error {
		return e.absences.SickReports.DeleteAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, absenceID)
	})
}

// S1 + S3: the series materialized after the report ends up exactly like the
// one materialized before it, while the report stands, after it is shortened
// and after it is deleted.
func TestSickReport_SeriesShiftsMaterializedLaterMatchEarlierOnes(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	earlier := e.createMondaySeries(t, periodID, "08:00", "10:00")
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekThree)
	later := e.createMondaySeries(t, periodID, "13:00", "15:00")

	assertSameSickState := func(day calendar.Date, msg string) {
		t.Helper()
		assert.Equal(t, sickStateOf(e.seriesShift(t, earlier, day)), sickStateOf(e.seriesShift(t, later, day)), msg)
	}

	assertPlanned(t, e.seriesShift(t, later, stampWeekOne), "day before the report")
	assertSickCancelled(t, e.seriesShift(t, earlier, stampWeekTwo), absenceID, "earlier shift on the first sick day")
	assertSickCancelled(t, e.seriesShift(t, later, stampWeekTwo), absenceID, "later shift on the first sick day")
	assertSickCancelled(t, e.seriesShift(t, later, stampWeekThree), absenceID, "later shift on the last sick day")
	for _, day := range []calendar.Date{stampWeekOne, stampWeekTwo, stampWeekThree} {
		assertSameSickState(day, "sick state while the report stands, "+day.String())
	}

	e.shortenSickAbsence(t, absenceID, stampWeekThree, stampWeekThree)
	assertPlanned(t, e.seriesShift(t, later, stampWeekTwo), "removed sick day")
	assertSickCancelled(t, e.seriesShift(t, later, stampWeekThree), absenceID, "kept sick day")
	for _, day := range []calendar.Date{stampWeekTwo, stampWeekThree} {
		assertSameSickState(day, "sick state after shortening, "+day.String())
	}

	e.deleteSickAbsence(t, absenceID)
	assertPlanned(t, e.seriesShift(t, later, stampWeekThree), "deleted report")
	assertSameSickState(stampWeekThree, "sick state after deletion")
}

// S2: the half boundary days of a range stay planned; the full day inside is
// cancelled.
func TestSickReport_SeriesMaterializationSkipsHalfBoundaryDays(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekThree)
	_, err := e.db.NewRaw(`UPDATE active.staff_absences SET start_half_day = TRUE, end_half_day = TRUE WHERE id = ? AND tenant_id = ?`,
		absenceID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	seriesID := e.createMondaySeries(t, periodID, "08:00", "12:00")

	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekOne), "half start day")
	assertSickCancelled(t, e.seriesShift(t, seriesID, stampWeekTwo), absenceID, "full day inside the range")
	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekThree), "half end day")
}

// S2: a half-day report never cancels a shift; a full-day report of the same
// person does.
func TestSickReport_SeriesMaterializationSkipsHalfDayReports(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	e.inTx(t, func(ctx context.Context) error {
		_, err := e.absences.SickReports.CreateAbsenceFor(ctx, e.subject.ID, e.admin.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: "sick", DateStart: stampWeekOne.String(), DateEnd: stampWeekOne.String(), HalfDay: true,
		})
		return err
	})
	fullDayID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)

	seriesID := e.createMondaySeries(t, periodID, "08:00", "12:00")

	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekOne), "half sick day")
	assertSickCancelled(t, e.seriesShift(t, seriesID, stampWeekTwo), fullDayID, "full sick day")
	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekThree), "healthy day")
}

// A declined report takes no plan effect, as in the Betreuungsplan.
func TestSickReport_SeriesMaterializationIgnoresDeclinedReport(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	_, err := e.db.NewRaw(`UPDATE active.staff_absences SET status = 'declined' WHERE id = ? AND tenant_id = ?`, absenceID, e.tenantID).Exec(e.ctx)
	require.NoError(t, err)

	seriesID := e.createMondaySeries(t, periodID, "08:00", "12:00")

	assertPlanned(t, e.seriesShift(t, seriesID, stampWeekTwo), "declined report")
}

// A rule edit re-plans the series: a sick-cancelled occurrence stays the one
// row of its date, still owned by the report.
func TestSickReport_SeriesSplitKeepsSickCancelledOccurrence(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnv(t)
	periodID := e.createSeriesPeriod(t)
	absenceID := e.createSickAbsence(t, stampWeekTwo, stampWeekTwo)
	seriesID := e.createMondaySeries(t, periodID, "08:00", "12:00")

	var successorID int64
	e.inTx(t, func(ctx context.Context) error {
		result, err := e.Planning.SplitSeries(ctx, workforce.SplitStaffShiftSeries{
			SeriesID: seriesID, EffectiveDate: stampWeekOne.String(), StartTime: clock("09:00"), EndTime: clock("12:00"),
			ActorStaffID: e.admin.ID,
		})
		successorID = result.SeriesID
		return err
	})

	assertPlanned(t, e.seriesShift(t, successorID, stampWeekOne), "re-planned healthy day")
	assertSickCancelled(t, e.seriesShift(t, successorID, stampWeekTwo), absenceID, "re-planned sick day")

	e.deleteSickAbsence(t, absenceID)
	assertPlanned(t, e.seriesShift(t, successorID, stampWeekTwo), "deleted report after the split")
}
