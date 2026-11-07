package presence_test

import (
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	active "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateVisitStatusClearKeepsLockedEducationGroup(t *testing.T) {
	t.Parallel()
	for _, flag := range []string{"sick", "excused"} {
		t.Run(flag, func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupTestDB(t)
			ctx := testpkg.Ctx(t)
			svc, broadcaster := newServiceWithBroadcaster(t, db, testSchoolPresence(t, db))
			active.ConfigureForTest(svc, active.WithTenantRuntime(testpkg.TenantRuntime(t, db)), active.WithSettings(presenceSettingsStub{
				presenceMode: active.PresenceModeDetailed, sickClearMode: active.ClearModeNextCheckin, excusedClearMode: active.ClearModeNextCheckin,
			}))
			student := testpkg.CreateTestStudent(t, db, "Status", "Routing", "3a")
			group := testpkg.CreateTestEducationGroup(t, db, "Status routing group")
			assignStudentToEducationGroup(t, db, ctx, student.ID, group.ID)
			people := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
			row, err := people.Student.FindByID(ctx, student.ID)
			require.NoError(t, err)
			set := true
			since := time.Now().Add(-time.Hour)
			if flag == "sick" {
				row.Sick, row.SickSince = &set, &since
			} else {
				row.Excused, row.ExcusedSince = &set, &since
			}
			require.NoError(t, people.Student.Update(ctx, row))
			staff := testpkg.CreateTestStaff(t, db, "Status", "Staff")
			device := testpkg.CreateTestDevice(t, db, "status-routing-"+flag)
			session := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			ctx = api.WithAttendanceStaff(ctx, staff.ID, staff.TenantID)
			ctx = api.WithAttendanceDevice(ctx, device.ID, device.TenantID)
			require.NoError(t, svc.CreateVisit(ctx, &studentpresence.Visit{StudentID: student.ID, ActiveGroupID: session.ID, EntryTime: time.Now()}))
			updated, err := owners.PresenceStudents(testpkg.MustDatabase(db), people.Student).FindByID(ctx, student.ID)
			require.NoError(t, err)
			if flag == "sick" {
				require.NotNil(t, updated.Sick)
				assert.False(t, *updated.Sick)
				assert.Nil(t, updated.SickSince)
			} else {
				require.NotNil(t, updated.Excused)
				assert.False(t, *updated.Excused)
				assert.Nil(t, updated.ExcusedSince)
			}
			require.NotNil(t, updated.GroupID)
			assert.Equal(t, group.ID, *updated.GroupID)
			events := broadcaster.EventsOfType(active.EventStudentCheckIn)
			require.Len(t, events, 2)
			for _, event := range events {
				require.NotNil(t, event.Data.GroupIDs)
				assert.Equal(t, []string{strconv.FormatInt(group.ID, 10)}, *event.Data.GroupIDs)
			}
		})
	}
}
