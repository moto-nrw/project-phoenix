package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	deviceAuth "github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingVisitCreation struct {
	activeService.StudentPresence
	afterCreate error
	createdID   int64
}

func (p *failingVisitCreation) RecordVisit(ctx context.Context, input studentpresence.Visit) (studentpresence.Visit, error) {
	row, err := p.StudentPresence.RecordVisit(ctx, input)
	p.createdID = row.ID
	if err == nil {
		err = p.afterCreate
	}
	return row, err
}

func TestVisitCreationRollsBackAttendanceAndVisitThenRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := testSchoolPresence(t, db)
	presence := &failingVisitCreation{StudentPresence: module, afterCreate: errors.New("fail after visit insert")}
	svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
	testpkg.SetTenantRuntime(t, svc, db)
	student := testpkg.CreateTestStudent(t, db, "Create", "Rollback", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Create", "Staff")
	device := testpkg.CreateTestDevice(t, db, "visit-create-rollback")
	group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
	ctx := context.WithValue(testpkg.Ctx(t), deviceAuth.CtxDevice, devicePrincipal(device.ID, device.TenantID))
	ctx = context.WithValue(ctx, deviceAuth.CtxStaff, staffPrincipal(staff))
	visit := &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: time.Now()}

	err := svc.CreateVisit(ctx, visit)
	require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
	require.NotZero(t, presence.createdID, "fault must occur after the visit insert")
	_, err = module.FindVisit(ctx, presence.createdID)
	require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
	attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	assert.Empty(t, attendance, "the preceding attendance insert must roll back too")
	assert.Empty(t, broadcaster.Calls(), "failed creation must not broadcast")

	presence.afterCreate = nil
	require.NoError(t, svc.CreateVisit(ctx, visit))
	require.NotZero(t, visit.ID)
	attendance, err = module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, attendance, 1)
	err = svc.CreateVisit(ctx, &studentpresence.Visit{
		StudentID: student.ID, ActiveGroupID: group.ID, EntryTime: visit.EntryTime,
	})
	require.ErrorIs(t, err, activeService.ErrStudentAlreadyActive, "existing duplicate-create contract")
	visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
	require.NoError(t, err)
	require.Len(t, visits, 1, "retry must not create a duplicate visit")
}
