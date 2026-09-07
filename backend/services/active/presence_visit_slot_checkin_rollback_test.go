package active_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	deviceAuth "github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingVisitSlotCheckIn struct {
	scheduleModels.InstanceStudentRepository
	failAt string
	writes []string
	err    error
}

func (r *failingVisitSlotCheckIn) after(phase string, err error) error {
	if err != nil {
		return err
	}
	r.writes = append(r.writes, phase)
	if phase == r.failAt {
		return r.err
	}
	return nil
}

func (r *failingVisitSlotCheckIn) UpdateAttendanceFromCheckin(ctx context.Context, instanceID, studentID int64, at time.Time) (bool, error) {
	changed, err := r.InstanceStudentRepository.UpdateAttendanceFromCheckin(ctx, instanceID, studentID, at)
	return changed, r.after("check-in", err)
}

func (r *failingVisitSlotCheckIn) CreateUnplannedPresentIfAbsent(ctx context.Context, instanceID, studentID int64, at time.Time) (*scheduleModels.InstanceStudent, error) {
	row, err := r.InstanceStudentRepository.CreateUnplannedPresentIfAbsent(ctx, instanceID, studentID, at)
	return row, r.after("insert", err)
}

func (r *failingVisitSlotCheckIn) UpdateAttendanceCheckout(ctx context.Context, instanceID, studentID int64, at time.Time) error {
	return r.after("checkout", r.InstanceStudentRepository.UpdateAttendanceCheckout(ctx, instanceID, studentID, at))
}

func TestVisitCheckInRollsBackAfterEachSlotWriteAndRetries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		unplanned bool
		failAt    string
	}{
		{"planned check-in", false, "check-in"},
		{"planned completed interval", false, "checkout"},
		{"unplanned insert", true, "insert"},
		{"unplanned completed interval", true, "checkout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			module := testSchoolPresence(t, db)
			instances, assignments := repositories.NewAttendanceSyncTestRepositories(repositories.NewUnobservedTimetableDependencies(db).Capability)
			injected := errors.New("fail after visit slot write")
			rows := &failingVisitSlotCheckIn{InstanceStudentRepository: assignments, failAt: tc.failAt, err: injected}
			syncer := scheduleService.NewAttendanceSyncService(instances, rows, slog.Default())
			svc, broadcaster := newServiceWithPresenceSync(t, db, module, syncer)
			testpkg.SetTenantRuntime(t, svc, db)
			student := testpkg.CreateTestStudent(t, db, "VisitSlot", "Rollback", "3a")
			staff := testpkg.CreateTestStaff(t, db, "VisitSlot", "Staff")
			device := testpkg.CreateTestDevice(t, db, "visit-slot-rollback")
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			instance := testpkg.CreateTestActivityInstance(t, db, testpkg.TodayDate(), group.RoomID, testpkg.ActivityInstanceOpts{ActiveGroupID: &group.ID})
			if !tc.unplanned {
				testpkg.CreateTestInstanceStudent(t, db, instance.ID, student.ID, scheduleModels.AttendanceStatusExpected)
			}
			ctx := context.WithValue(testpkg.Ctx(t), deviceAuth.CtxDevice, device)
			ctx = context.WithValue(ctx, deviceAuth.CtxStaff, staff)
			visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now().Add(-time.Hour)}
			if tc.failAt == "checkout" {
				exit := visit.EntryTime.Add(time.Minute)
				visit.ExitTime = &exit
			}

			err := svc.CreateVisit(ctx, visit)
			require.ErrorIs(t, err, injected)
			require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
			require.Contains(t, rows.writes, tc.failAt)
			assert.Empty(t, broadcaster.Calls())
			visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			assert.Empty(t, visits)
			attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			assert.Empty(t, attendance)
			slot, err := assignments.FindByInstanceAndStudent(ctx, instance.ID, student.ID)
			require.NoError(t, err)
			if tc.unplanned {
				assert.Nil(t, slot, "unplanned slot insert must roll back")
			} else {
				require.NotNil(t, slot)
				assert.Nil(t, slot.CheckedInAt)
				assert.Nil(t, slot.CheckedOutAt)
				assert.Equal(t, scheduleModels.AttendanceStatusExpected, slot.Status)
			}

			rows.failAt = ""
			require.NoError(t, svc.CreateVisit(ctx, visit))
			visits, err = module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, visits, 1)
			attendance, err = module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, attendance, 1)
			slot, err = assignments.FindByInstanceAndStudent(ctx, instance.ID, student.ID)
			require.NoError(t, err)
			require.NotNil(t, slot.CheckedInAt)
			assert.True(t, slot.CheckedInAt.Equal(visits[0].EntryTime))
			assert.True(t, slot.CheckedInAt.Equal(attendance[0].CheckInTime))
			assert.Equal(t, tc.unplanned, slot.IsUnplanned)
			if visit.ExitTime == nil {
				assert.Nil(t, slot.CheckedOutAt)
			} else {
				require.NotNil(t, slot.CheckedOutAt)
				assert.True(t, slot.CheckedOutAt.Equal(*visits[0].ExitTime))
			}
		})
	}
}
