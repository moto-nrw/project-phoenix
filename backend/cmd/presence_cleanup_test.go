package cmd

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresenceCleanupRollsBackFailedSchoolAndContinues(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	first, _ := testpkg.CreateTestTenant(t, db)
	second, _ := testpkg.CreateTestTenant(t, db)
	schools, err := repositories.NewOrganizationTenancy(db)
	require.NoError(t, err)
	cc := &cleanupContext{
		TenantRuntime: testpkg.TenantRuntime(t, db),
		Schools:       schools,
		Output:        io.Discard,
	}
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: func(presenceCompose.Observation) {}})
	require.NoError(t, err)
	inputs := make(map[int64]studentpresence.Attendance)
	for _, id := range []int64{first, second} {
		student := testpkg.CreateTestStudentForTenant(t, db, id, "Cleanup", "Student", "3a")
		device := testpkg.CreateTestDeviceForTenant(t, db, id, "cleanup-presence")
		inputs[id] = studentpresence.Attendance{
			StudentID: student.ID, DeviceID: device.ID, Date: testpkg.TodayDate().String(), CheckInTime: time.Now(),
		}
	}
	injected := errors.New("fail after attendance write")
	failFirst := true
	var published []int64
	action := func(ctx context.Context, id int64) (func(), error) {
		if _, included := inputs[id]; !included {
			return func() {}, nil
		}
		_, _, err := module.EnsureAttendance(ctx, inputs[id])
		if err != nil {
			return nil, err
		}
		if failFirst && id == first {
			return nil, injected
		}
		return func() { published = append(published, id) }, nil
	}
	err = forEachPresenceTenant(cc, "test cleanup", action)
	require.ErrorIs(t, err, injected, "CLI must report a failed school")
	assert.Equal(t, []int64{second}, published, "only committed results may be printed")
	count := func(id int64, want int) {
		t.Helper()
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), id)
		rows, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{})
		require.NoError(t, err)
		require.Len(t, rows, want, "school-scoped result")
	}
	count(first, 0)
	count(second, 1)

	failFirst = false
	published = nil
	require.NoError(t, forEachPresenceTenant(cc, "retry cleanup", action))
	assert.Equal(t, []int64{first, second}, published)
	count(first, 1)
	count(second, 1)
}
