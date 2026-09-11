package active_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingSlotCheckout struct {
	scheduleModels.InstanceStudentRepository
	afterWrite error
	writes     int
}

func (r *failingSlotCheckout) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, at time.Time) error {
	if err := r.InstanceStudentRepository.UpdateAttendanceCheckout(ctx, instanceID, studentID, at); err != nil {
		return err
	}
	r.writes++
	return r.afterWrite
}

func (r *failingSlotCheckout) UpdateAttendanceCheckoutBatch(ctx context.Context, keys []scheduleModels.InstanceStudentKey, at time.Time) error {
	if err := r.InstanceStudentRepository.UpdateAttendanceCheckoutBatch(ctx, keys, at); err != nil {
		return err
	}
	r.writes++
	return r.afterWrite
}

func TestCheckoutRollsBackAfterSlotWriteAndRetries(t *testing.T) {
	t.Parallel()
	for _, fault := range []string{"slot write", "presence setting"} {
		for _, name := range []string{"single", "batch", "detailed-single", "detailed-batch", "visit-only"} {
			t.Run(fault+"/"+name, func(t *testing.T) {
				batch := name == "batch" || name == "detailed-batch"
				visitOnly := name == "visit-only"
				detailed := name == "detailed-single" || name == "detailed-batch" || visitOnly
				testpkg.OwnTenant(t)
				db := testpkg.SetupTestDB(t)
				module := testSchoolPresence(t, db)
				instances, assignments := repositories.NewAttendanceSyncTestRepositories(repositories.NewUnobservedTimetableDependencies(db).Capability)
				injected := errors.New("fail after slot checkout")
				rows := &failingSlotCheckout{InstanceStudentRepository: assignments}
				var settingsErr error
				if fault == "slot write" {
					rows.afterWrite = injected
				} else {
					settingsErr = injected
				}
				syncer := scheduleService.NewAttendanceSyncService(instances, rows, slog.Default())
				svc, broadcaster := newServiceWithPresenceSync(t, db, module, syncer)
				testpkg.SetTenantRuntime(t, svc, db)
				presenceMode := "binary"
				if detailed {
					presenceMode = "detailed"
				}
				svc.SetSettingsService(&configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) { return presenceMode, settingsErr }})
				staff := testpkg.CreateTestStaff(t, db, "SlotCheckout", "Staff")
				device := testpkg.CreateTestDevice(t, db, "slot-checkout-rollback")
				room := testpkg.CreateTestRoom(t, db, "SlotCheckoutRoom")
				var activeGroupID *int64
				if detailed {
					activity := testpkg.CreateTestActivityGroup(t, db, "SlotCheckoutActivity")
					group := testpkg.CreateTestActiveGroup(t, db, activity.ID, room.ID)
					activeGroupID = &group.ID
				}
				instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), room.ID, testpkg.ActivityInstanceOpts{ActiveGroupID: activeGroupID})
				ctx := testpkg.Ctx(t)
				count := 1
				if batch {
					count = 2
				}
				studentIDs := make([]int64, 0, count)
				visitIDs := make([]int64, 0, count)
				checkedIn := time.Now().Add(-time.Minute)
				for range count {
					student := testpkg.CreateTestStudent(t, db, "SlotCheckout", "Rollback", "3a")
					studentIDs = append(studentIDs, student.ID)
					testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, checkedIn, nil)
					testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusPresent, testpkg.InstanceStudentOpts{CheckedInAt: &checkedIn})
					if detailed {
						visit := testpkg.CreateTestVisit(t, db, student.ID, *activeGroupID, checkedIn, nil)
						visitIDs = append(visitIDs, visit.ID)
					}
				}
				checkout := func() error {
					if visitOnly {
						return svc.EndVisit(ctx, visitIDs[0])
					}
					if batch {
						_, err := svc.ProcessSchoolCheckinBatch(ctx, studentIDs, staff.ID, activeService.SchoolCheckinActionOut)
						return err
					}
					_, err := svc.CheckOutStudent(ctx, studentIDs[0], staff.ID, true)
					return err
				}
				checkoutErr := checkout()
				require.ErrorIs(t, checkoutErr, injected)
				if fault == "presence setting" {
					require.ErrorIs(t, checkoutErr, activeService.ErrDatabaseOperation)
				}
				if fault == "slot write" {
					require.Equal(t, 1, rows.writes, "fault must follow the slot checkout statement")
				} else {
					require.Zero(t, rows.writes, "settings failure must prevent slot writes")
				}
				assert.Empty(t, broadcaster.Calls())
				attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs, OpenOnly: true})
				require.NoError(t, err)
				require.Len(t, attendance, count, "all attendance checkouts must roll back")
				if detailed {
					visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true})
					require.NoError(t, err)
					require.Len(t, visits, count, "all visit checkouts must roll back")
				}
				for _, id := range studentIDs {
					slot, err := assignments.FindByInstanceAndStudent(ctx, instance.ID, id)
					require.NoError(t, err)
					assert.Nil(t, slot.CheckedOutAt, "slot checkout must roll back")
				}
				rows.afterWrite = nil
				settingsErr = nil
				require.NoError(t, checkout())
				if visitOnly {
					require.ErrorIs(t, checkout(), activeService.ErrVisitAlreadyEnded)
				} else {
					require.NoError(t, checkout(), "repeated checkout must be safe")
				}
				attendance, err = module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: studentIDs, OpenOnly: true})
				require.NoError(t, err)
				if visitOnly {
					assert.Len(t, attendance, count, "ending a room visit does not end school attendance")
				} else {
					assert.Empty(t, attendance)
				}
				if detailed {
					visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: studentIDs, OpenOnly: true})
					require.NoError(t, err)
					assert.Empty(t, visits)
				}
				for _, id := range studentIDs {
					slot, err := assignments.FindByInstanceAndStudent(ctx, instance.ID, id)
					require.NoError(t, err)
					assert.NotNil(t, slot.CheckedOutAt)
				}
				wantWrites := 1
				if fault == "slot write" {
					wantWrites++
				}
				assert.Equal(t, wantWrites, rows.writes, "retry closes slots once; idempotent repetition performs no slot write")
			})
		}
	}
}
