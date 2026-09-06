package active_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMovementPresenceModeFailurePreservesStateAndRetries(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"assign", "move", "transit"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			presence := testSchoolPresence(t, db)
			svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
			testpkg.SetTenantRuntime(t, svc, db)
			injected := errors.New("movement presence mode unavailable")
			settingsErr := injected
			svc.SetSettingsService(&configtest.Mock{ResolveStringFn: func(context.Context, string) (string, error) {
				return activeService.PresenceModeDetailed, settingsErr
			}})
			ctx := testpkg.Ctx(t)
			student := testpkg.CreateTestStudent(t, db, "Movement", "Settings", "3a")
			staff := testpkg.CreateTestStaff(t, db, "Movement", "Staff")
			device := testpkg.CreateTestDevice(t, db, "movement-settings")
			target := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now(), nil)
			var sourceID int64
			if operation != "assign" {
				source := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
				sourceID = testpkg.CreateTestVisit(t, db, student.ID, source.ID, time.Now(), nil).ID
			}
			perform := func() error {
				switch operation {
				case "assign":
					_, err := svc.AssignTransitStudentsToActiveGroup(ctx, []int64{student.ID}, target.ID)
					return err
				case "move":
					_, err := svc.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, target.ID, activeSvcBypassAuth)
					return err
				default:
					_, err := svc.MoveStudentsToTransitAuthorized(ctx, []int64{student.ID}, activeSvcBypassAuth)
					return err
				}
			}
			err := perform()
			require.ErrorIs(t, err, injected)
			require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
			assert.Empty(t, broadcaster.Calls())
			visits, err := presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			if sourceID == 0 {
				assert.Empty(t, visits)
			} else {
				require.Len(t, visits, 1)
				assert.Equal(t, sourceID, visits[0].ID)
				assert.Nil(t, visits[0].ExitTime)
			}
			settingsErr = nil
			require.NoError(t, perform())
			require.NoError(t, perform())
			visits, err = presence.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}, OpenOnly: true})
			require.NoError(t, err)
			if operation == "transit" {
				assert.Empty(t, visits)
			} else {
				require.Len(t, visits, 1)
				assert.Equal(t, target.ID, visits[0].ActiveGroupID)
			}
			attendance, err := presence.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, attendance, 1)
			assert.Nil(t, attendance[0].CheckOutTime)
		})
	}
}

func TestVisitMoveCreationFailureRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"assign", "move"} {
		t.Run(operation, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			module := testSchoolPresence(t, db)
			presence := &failingVisitCreation{StudentPresence: module, afterCreate: errors.New("fail after movement visit insert")}
			svc, broadcaster := newServiceWithBroadcaster(t, db, presence)
			testpkg.SetTenantRuntime(t, svc, db)
			student := testpkg.CreateTestStudent(t, db, "Move", "Rollback", "3a")
			staff := testpkg.CreateTestStaff(t, db, "Move", "Staff")
			device := testpkg.CreateTestDevice(t, db, "move-rollback-device")
			target := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			ctx := testpkg.Ctx(t)
			testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now(), nil)
			var sourceID int64
			if operation == "move" {
				source := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
				sourceVisit := testpkg.CreateTestVisit(t, db, student.ID, source.ID, time.Now(), nil)
				sourceID = sourceVisit.ID
			}
			perform := func() error {
				if operation == "assign" {
					_, err := svc.AssignTransitStudentsToActiveGroup(ctx, []int64{student.ID}, target.ID)
					return err
				}
				_, err := svc.MoveStudentsToActiveGroupAuthorized(ctx, []int64{student.ID}, target.ID, activeSvcBypassAuth)
				return err
			}

			err := perform()
			require.ErrorIs(t, err, activeService.ErrDatabaseOperation)
			require.NotZero(t, presence.createdID, "fault must follow the insert")
			_, err = module.FindVisit(ctx, presence.createdID)
			require.ErrorIs(t, err, studentpresence.ErrVisitNotFound)
			if sourceID != 0 {
				source, err := module.FindVisit(ctx, sourceID)
				require.NoError(t, err)
				assert.Nil(t, source.ExitTime, "source closure must roll back with failed target insertion")
			}
			assert.Empty(t, broadcaster.Calls())

			presence.afterCreate = nil
			require.NoError(t, perform())
			require.NoError(t, perform())
			visits, err := module.ListVisits(ctx, studentpresence.VisitFilter{StudentIDs: []int64{student.ID}, OpenOnly: true})
			require.NoError(t, err)
			require.Len(t, visits, 1)
			assert.Equal(t, target.ID, visits[0].ActiveGroupID)
			attendance, err := module.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{student.ID}})
			require.NoError(t, err)
			require.Len(t, attendance, 1)
			assert.Nil(t, attendance[0].CheckOutTime, "movement must preserve school attendance")
		})
	}
}
