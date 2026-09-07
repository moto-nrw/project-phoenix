package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	deviceAuth "github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type plannedStatusFault struct {
	activeModels.StudentStatusDayRepository
	readErr, writeErr     error
	upsertErr, historyErr error
	upserts, clears       int
	writes                int
}

func (r *plannedStatusFault) FindActiveByStudentAndDateRange(ctx context.Context, id int64, from, until timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.StudentStatusDayRepository.FindActiveByStudentAndDateRange(ctx, id, from, until)
}

func (r *plannedStatusFault) FindActiveByStudentIDsAndDate(ctx context.Context, ids []int64, day timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.StudentStatusDayRepository.FindActiveByStudentIDsAndDate(ctx, ids, day)
}

func (r *plannedStatusFault) MarkClearedByID(ctx context.Context, id int64, at time.Time, source string) error {
	if err := r.StudentStatusDayRepository.MarkClearedByID(ctx, id, at, source); err != nil {
		return err
	}
	r.writes++
	return r.writeErr
}

type plannedStudentFault struct {
	userModels.StudentRepository
	writeErr error
	writes   int
}

type checkinAttributionFault struct {
	activeModels.GroupRepository
	readErr error
}

func (r *checkinAttributionFault) FindActiveByDeviceID(ctx context.Context, id int64) (*activeModels.Group, error) {
	if r.readErr != nil {
		return nil, r.readErr
	}
	return r.GroupRepository.FindActiveByDeviceID(ctx, id)
}

func (r *plannedStudentFault) Update(ctx context.Context, student *userModels.Student) error {
	if err := r.StudentRepository.Update(ctx, student); err != nil {
		return err
	}
	r.writes++
	return r.writeErr
}

func TestPlannedStatusCheckinFailuresRollbackAndRetry(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"single", "batch", "visit"} {
		stages := []string{"read", "status write", "student write", "sick setting", "excused setting", "presence setting", "device lookup"}
		if mode == "visit" {
			stages = append(stages, "staff attribution")
		}
		for _, stage := range stages {
			t.Run(mode+"/"+stage, func(t *testing.T) {
				testStatusCheckinRollback(t, mode, stage, "planned")
			})
		}
	}
}

func TestLiveStatusCheckinFailuresRollbackAndRetry(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"sick", "excused"} {
		for _, mode := range []string{"single", "batch", "visit"} {
			for _, stage := range []string{"upsert", "history clear", "student write"} {
				t.Run(kind+"/"+mode+"/"+stage, func(t *testing.T) {
					testStatusCheckinRollback(t, mode, stage, kind)
				})
			}
		}
	}
}

func testStatusCheckinRollback(t *testing.T, mode, stage, kind string) {
	t.Helper()
	testpkg.OwnTenant(t)
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos, err := repositories.NewActiveTestRepositories(db)
	require.NoError(t, err)
	devices, err := repositories.NewDeviceRepository(db)
	require.NoError(t, err)
	deviceFault := activeService.NewCheckinDeviceFault(devices)
	groupFault := &checkinAttributionFault{GroupRepository: repos.ActiveGroup}
	presence := testSchoolPresence(t, db)
	statuses := &plannedStatusFault{StudentStatusDayRepository: repos.StudentStatusDay}
	students := &plannedStudentFault{StudentRepository: repos.Student}
	broadcaster := testpkg.NewRecordingBroadcaster()
	svc := activeService.NewService(activeService.ServiceDependencies{
		SchoolPresence: presence, StudentRepo: students, StudentStatusRepo: statuses,
		StaffRepo: repos.Staff, PersonRepo: repos.Person, TeacherRepo: repos.Teacher,
		EducationGroupRepo: repos.Group, GroupRepo: groupFault, DeviceRepo: deviceFault,
		RoomRepo: repos.Room, ActivityGroupRepo: repos.ActivityGroup,
		DB: db, Broadcaster: broadcaster,
	})
	testpkg.SetTenantRuntime(t, svc, db)
	var settingsErr error
	svc.SetSettingsService(&configtest.Mock{
		HasTenantOverrideFn: func(context.Context, string) (bool, error) { return true, nil },
		ResolveStringFn: func(_ context.Context, key string) (string, error) {
			if settingsErr != nil && stage == "presence setting" && key == configModel.KeyPresenceMode {
				return "", settingsErr
			}
			if settingsErr != nil && ((stage == "sick setting" && key == configModel.KeySickClearMode) || (stage == "excused setting" && key == configModel.KeyExcusedClearMode)) {
				return "", settingsErr
			}
			if key == configModel.KeyPresenceMode {
				if mode == "visit" {
					return "detailed", nil
				}
				return "binary", nil
			}
			if kind != "planned" {
				return "next_checkin", nil
			}
			return "manual", nil
		},
	})
	device := testpkg.EnsureWebManualDevice(t, db)
	staff := testpkg.CreateTestStaff(t, db, "Planned", "Rollback")
	ctx = context.WithValue(ctx, deviceAuth.CtxDevice, devicePrincipal(device.ID, device.TenantID))
	ctx = context.WithValue(ctx, deviceAuth.CtxStaff, staffPrincipal(staff))
	student := testpkg.CreateTestStudent(t, db, "Planned", "Rollback", "3a")
	var groupID int64
	if mode == "visit" {
		groupID = testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t)).ID
	}
	sick := true
	since := time.Now().Add(-time.Hour)
	if kind == "excused" {
		student.Excused, student.ExcusedSince = &sick, &since
	} else {
		student.Sick, student.SickSince = &sick, &since
	}
	require.NoError(t, repos.Student.Update(ctx, student))
	status := &activeModels.StudentStatusDay{StudentID: student.ID, Date: timezone.TodayDate(), Status: activeModels.StudentStatusDaySick, ReportedAt: time.Now(), Source: activeModels.StudentStatusSourcePlanned}
	if kind == "planned" {
		require.NoError(t, repos.StudentStatusDay.UpsertReported(ctx, status))
	}
	injected := errors.New("planned status transaction fault")
	switch stage {
	case "sick setting", "excused setting", "presence setting":
		settingsErr = injected
	case "read":
		statuses.readErr = injected
	case "upsert":
		statuses.upsertErr = injected
	case "history clear":
		statuses.historyErr = injected
	case "status write":
		statuses.writeErr = injected
	case "student write":
		students.writeErr = injected
	}
	checkin := func() error {
		checkinCtx, deviceID := ctx, device.ID
		if stage == "device lookup" {
			deviceID = 0
			checkinCtx = context.WithValue(testpkg.Ctx(t), deviceAuth.CtxStaff, staffPrincipal(staff))
		}
		if stage == "staff attribution" {
			checkinCtx = context.WithValue(testpkg.Ctx(t), deviceAuth.CtxDevice, devicePrincipal(device.ID, device.TenantID))
		}
		if mode == "visit" {
			return svc.CreateVisit(checkinCtx, &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: groupID, EntryTime: time.Now()})
		}
		if mode == "batch" {
			_, err := svc.ProcessSchoolCheckinBatch(ctx, []int64{student.ID}, staff.ID, activeService.SchoolCheckinActionIn)
			return err
		}
		_, err := svc.CheckInStudent(checkinCtx, student.ID, staff.ID, deviceID, true)
		return err
	}
	if stage == "device lookup" {
		deviceFault.ReadErr = injected
	}
	if stage == "staff attribution" {
		groupFault.readErr = injected
	}
	err = checkin()
	require.ErrorIs(t, err, injected)
	require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	if kind == "planned" && (stage == "status write" || stage == "student write") {
		require.Equal(t, 1, statuses.writes)
	}
	if kind != "planned" {
		require.Equal(t, 1, statuses.upserts)
		if stage != "upsert" {
			require.Equal(t, 1, statuses.clears)
		}
	}
	if stage == "student write" {
		require.Equal(t, 1, students.writes)
	}
	attendance, err := presence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, attendance, "attendance must roll back with status clearing")
	visits, err := presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, visits, "a failed check-in must not leave a room visit")
	remaining, err := repos.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, status.Date, status.Date)
	require.NoError(t, err)
	if kind == "planned" {
		require.Len(t, remaining, 1, "status clearing must roll back")
	} else {
		assert.Empty(t, remaining)
		history, err := repos.StudentStatusDay.FindByStudentAndDateRange(ctx, student.ID, status.Date, status.Date)
		require.NoError(t, err)
		assert.Empty(t, history, "failed check-in must not retain inserted history, even if cleared")
	}
	storedStudent, err := repos.Student.FindByID(ctx, student.ID)
	require.NoError(t, err)
	flag, flagSince := storedStudent.Sick, storedStudent.SickSince
	if kind == "excused" {
		flag, flagSince = storedStudent.Excused, storedStudent.ExcusedSince
	}
	require.NotNil(t, flag)
	assert.True(t, *flag, "student flag clearing must roll back")
	require.NotNil(t, flagSince)
	assert.WithinDuration(t, since, *flagSince, time.Microsecond)
	assert.Empty(t, broadcaster.Calls())
	statuses.readErr, statuses.writeErr, students.writeErr = nil, nil, nil
	settingsErr = nil
	deviceFault.ReadErr = nil
	groupFault.readErr = nil
	statuses.upsertErr, statuses.historyErr = nil, nil
	require.NoError(t, checkin())
	if mode == "visit" {
		require.ErrorIs(t, checkin(), activeService.ErrStudentAlreadyActive)
	} else {
		require.NoError(t, checkin())
	}
	visits, err = presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	if mode == "visit" {
		require.Len(t, visits, 1, "a repeated visit check-in must not duplicate the visit")
		assert.Equal(t, groupID, visits[0].ActiveGroupID)
		assert.Nil(t, visits[0].ExitTime)
	} else {
		assert.Empty(t, visits)
	}
	attendance, err = presence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, attendance, 1, "retries must not duplicate attendance")
	if stage == "staff attribution" {
		assert.Zero(t, attendance[0].CheckedInBy, "a device without a supervisor retains device-only attribution")
		assert.Equal(t, device.ID, attendance[0].DeviceID)
	}
	remaining, err = repos.StudentStatusDay.FindActiveByStudentAndDateRange(ctx, student.ID, status.Date, status.Date)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	storedStudent, err = repos.Student.FindByID(ctx, student.ID)
	require.NoError(t, err)
	flag, flagSince = storedStudent.Sick, storedStudent.SickSince
	if kind == "excused" {
		flag, flagSince = storedStudent.Excused, storedStudent.ExcusedSince
	}
	require.NotNil(t, flag)
	assert.False(t, *flag)
	assert.Nil(t, flagSince)
}

func (r *plannedStatusFault) UpsertReported(ctx context.Context, row *activeModels.StudentStatusDay) error {
	if err := r.StudentStatusDayRepository.UpsertReported(ctx, row); err != nil {
		return err
	}
	r.upserts++
	return r.upsertErr
}
func (r *plannedStatusFault) MarkCleared(ctx context.Context, id int64, status string, day timezone.Date, at time.Time, source string) error {
	if err := r.StudentStatusDayRepository.MarkCleared(ctx, id, status, day, at, source); err != nil {
		return err
	}
	r.clears++
	return r.historyErr
}
