// Hermetic integration tests for #7021: an absence in force that predates
// the cascade of every type (#6847) left the shifts and blocks planned at
// that time alone. The Leitung sees those gaps and takes them over.
//
// Invariants:
//   - G1: an approved vacation stored without its cascade is a gap with its
//     open shift and block; an open request, an admin-cancelled shift and a
//     hand-marked absent row are none.
//   - G2: the take-over cancels the shift and marks the block absent under
//     the absence's stamp, leaves hand planning alone, and closes the gap.
//   - G3: an absence no longer in force is skipped and writes nothing.
package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planGapNow is the suite clock: one week before vacationDay, so the day
// lies inside the cascade window and after the live clock's past-day guards.
func planGapNow() time.Time {
	return time.Date(2098, 12, 30, 12, 0, 0, 0, time.UTC)
}

// storeAbsenceWithoutCascade writes an absence row the way the plans saw it
// before #6847: in force, but its days never cascaded.
func (e *sickCascadeEnv) storeAbsenceWithoutCascade(t *testing.T, staffID int64, day calendar.Date, status string) int64 {
	t.Helper()
	var absenceID int64
	require.NoError(t, e.db.NewRaw(
		`INSERT INTO active.staff_absences (tenant_id, staff_id, absence_type, date_start, date_end, status, created_by, working_days, requested_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 1, NOW()) RETURNING id`,
		e.tenantID, staffID, workforce.AbsenceTypeVacation, day, day, status, staffID,
	).Scan(e.ctx, &absenceID))
	return absenceID
}

func (e *sickCascadeEnv) planGaps(t *testing.T) []workforce.AbsencePlanGap {
	t.Helper()
	var gaps []workforce.AbsencePlanGap
	e.inTx(t, func(ctx context.Context) error {
		var err error
		gaps, err = e.absences.SickReports.AbsencePlanGaps(ctx)
		return err
	})
	return gaps
}

// G1 + G2 + G3.
func TestAbsencePlanGaps_PreCascadeVacationIsShownAndTakenOver(t *testing.T) {
	t.Parallel()

	e := buildSickCascadeEnvAt(t, nil, planGapNow)
	requester := e.newVacationRequester(t)
	other := e.newVacationRequester(t)

	open := e.createShift(t, requester.staffID, vacationDay, "08:00", "12:00", nil)
	handCancelled := e.createShift(t, requester.staffID, vacationDay, "13:00", "15:00", func(s *workforce.StaffShift) { s.Cancelled = true })
	_, staffed := e.createBlock(t, vacationDay, timetable.InstanceStatusPlanned, "09:00", requester.staffID, testpkg.InstanceStaffOpts{})
	_, handAbsent := e.createBlock(t, vacationDay, timetable.InstanceStatusPlanned, "10:00", requester.staffID, testpkg.InstanceStaffOpts{IsAbsent: true})
	approved := e.storeAbsenceWithoutCascade(t, requester.staffID, vacationDay, workforce.AbsenceStatusApproved)

	requested := e.createShift(t, other.staffID, vacationDay, "08:00", "12:00", nil)
	pending := e.storeAbsenceWithoutCascade(t, other.staffID, vacationDay, workforce.AbsenceStatusRequested)

	gaps := e.planGaps(t)
	require.Len(t, gaps, 1, "only the approved absence has plan effect")
	assert.Equal(t, workforce.AbsencePlanGap{
		AbsenceID: approved, StaffID: requester.staffID, AbsenceType: workforce.AbsenceTypeVacation,
		DateStart: vacationDay.String(), DateEnd: vacationDay.String(), OpenShifts: 1, OpenBlocks: 1,
	}, gaps[0], "the hand-cancelled shift and the hand-marked row are no gap")

	sync := func(absenceID int64) bool {
		var applied bool
		e.inTx(t, func(ctx context.Context) error {
			var err error
			applied, err = e.absences.SickReports.SyncAbsencePlan(ctx, absenceID, e.admin.ID, &requester.accountID)
			return err
		})
		return applied
	}
	assert.True(t, sync(approved), "the approved absence takes its plan effect")
	assert.False(t, sync(pending), "an open request is skipped")

	assertAbsenceCancelled(t, e.reloadShift(t, open.ID), approved, "planned shift on the vacation day")
	assertAbsentStamp(t, e.reloadRow(t, staffed.ID), approved, "staffed block on the vacation day")
	assert.Nil(t, e.reloadShift(t, handCancelled.ID).SickAbsenceID, "an admin cancellation keeps its own reason")
	assert.Nil(t, e.reloadRow(t, handAbsent.ID).SickAbsenceID, "a hand-marked absence keeps its own reason")
	assert.False(t, e.reloadShift(t, requested.ID).Cancelled, "an open request leaves the plans alone")

	assert.Empty(t, e.planGaps(t), "the taken-over absence is no gap any more")
}
