package compose_test

import (
	"context"
	"sync"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSickReport_MismatchedOwnershipDoesNotDeadlock(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, operation := range []string{"update", "delete own", "delete for"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			e := buildSickCascadeEnv(t)
			absenceID := e.createSickAbsence(t, stampWeekOne, stampWeekOne)
			want := workforce.ErrAbsenceDeleteNotOwned
			if operation == "update" {
				want = workforce.ErrAbsenceUpdateNotOwned
			}
			err := e.raceMismatchedAbsenceWrite(t, func(ctx context.Context) error {
				switch operation {
				case "update":
					end := stampWeekTwo.String()
					_, err := e.absences.SickReports.UpdateAbsence(ctx, e.sub.ID, nil, absenceID, workforce.UpdateAbsenceRequest{DateEnd: &end})
					return err
				case "delete own":
					return e.absences.SickReports.DeleteOwnAbsence(ctx, e.sub.ID, nil, absenceID)
				default:
					return e.absences.SickReports.DeleteAbsenceFor(ctx, e.sub.ID, e.admin.ID, nil, absenceID)
				}
			})
			require.ErrorIs(t, err, want)
			stored, err := e.shifts.FindStaffAbsence(e.ctx, absenceID)
			require.NoError(t, err)
			require.Equal(t, stampWeekOne.String(), stored.DateEnd, "the other staff member's absence stays unchanged")
		})
	}
}

// A valid write holds the caller's balance/absence gates before the mismatched
// request starts. That request must not hold the other subject's recurrence
// gate while waiting for the caller, or the valid write closes a lock cycle.
func (e *sickCascadeEnv) raceMismatchedAbsenceWrite(t *testing.T, write func(context.Context) error) error {
	t.Helper()
	holding, proceed := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(proceed) })
	t.Cleanup(release)
	var ownID int64
	valid := e.startContender(t, func(ctx context.Context) error {
		if err := e.shifts.LockStaffAbsenceWrites(ctx, e.sub.ID); err != nil {
			return err
		}
		close(holding)
		<-proceed
		created, err := e.absences.SickReports.CreateOwnAbsence(ctx, e.sub.ID, nil, workforce.CreateAbsenceRequest{
			AbsenceType: workforce.AbsenceTypeSick, DateStart: stampWeekOne.String(), DateEnd: stampWeekOne.String(),
		})
		if err == nil {
			ownID = created.ID
		}
		return err
	})
	select {
	case <-holding:
	case err := <-valid.done:
		require.FailNow(t, "the valid write failed before holding its gates", "%v", err)
	}
	mismatched := e.startContender(t, write)
	returned, early := e.awaitLockWait(t, mismatched)
	release()
	requireNoDeadlock(t, map[string]chan error{"valid absence write": valid.done})
	stored, err := e.shifts.FindStaffAbsence(e.ctx, ownID)
	require.NoError(t, err, "the caller's valid absence commits")
	require.Equal(t, e.sub.ID, stored.StaffID)
	if returned {
		return early
	}
	return <-mismatched.done
}
