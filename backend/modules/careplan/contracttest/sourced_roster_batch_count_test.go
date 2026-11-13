package contracttest_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSourcedRosterBatchStatementsDoNotGrowWithDatesAndChildren(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	from := calendar.NewDate(2099, 1, 5)
	setSourcePhaseServiceStartDate(t, env, from)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName(t.Name()), calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate))
	room := testpkg.CreateTestRoom(t, env.db, "Stapelraum")
	staff := testpkg.CreateTestStaff(t, env.db, "Stapel", "Abwesenheit")
	counter := testpkg.CaptureQueriesForContext(t, env.db)
	var statements []int
	for _, size := range []int{1, 5} {
		offering := createSourceOffering(t, env, fmt.Sprintf("Stapel %d", size), nil)
		students := []int64{}
		for i := range size + 1 {
			student, _ := submitAndApproveOfferingChild(t, env, offering.ID, fmt.Sprintf("batch-%d-%d@example.test", size, i), fmt.Sprintf("Kind %d/%d", size, i), 1)
			students = append(students, student)
		}
		testpkg.CreateTestStudentStatusDay(t, env.db, students[0], from, "sick")
		exception := testpkg.CreateTestPickupException(t, env.db, students[size], from.AddDays(size*7), staff.ID, "", "Entschuldigt")
		_, err := env.db.NewUpdate().Table("schedule.student_pickup_exceptions").Set("excused_from = ?", testpkg.WallClock(13, 0)).Set("excused_created_by = ?", staff.ID).Where("tenant_id = ?", testpkg.Tenant(t)).Where("id = ?", exception.ID).Exec(testpkg.Ctx(t))
		require.NoError(t, err)
		instances := []int64{}
		for i := range size {
			group := createSourcedTemplate(t, env, fmt.Sprintf("Stapel %d/%d", size, i), offering.ID, nil, period.ID)
			for day := range size + 1 {
				inst := testpkg.CreateTestActivityInstance(t, env.db, from.AddDays(day*7), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID, CalendarPeriodID: &period.ID})
				instances = append(instances, inst.ID)
			}
		}
		bookings := newBookingMaterialization(t, env.db).Bookings
		batch := bookings.(interface {
			ResyncTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
		})
		counter.Reset()
		require.NoError(t, testpkg.WithTenantTx(t, counter.Context(testpkg.Ctx(t)), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error {
			return batch.ResyncTemplatesSourcedFromOfferings(ctx, []int64{offering.ID}, from)
		}))
		statements = append(statements, counter.Total())
		var rows, sick, excused int
		require.NoError(t, env.db.NewRaw(`SELECT COUNT(*),COUNT(*) FILTER(WHERE a.student_status_day_id IS NOT NULL),COUNT(*) FILTER(WHERE a.pickup_exception_id IS NOT NULL)
   FROM schedule.instance_students AS p LEFT JOIN active.activity_session_attendance AS a ON a.instance_student_id=p.id AND a.tenant_id=p.tenant_id
   WHERE p.tenant_id=? AND p.instance_id IN (?)`, testpkg.Tenant(t), bun.List(instances)).Scan(testpkg.Ctx(t), &rows, &sick, &excused))
		assert.Equal(t, size*(size+1)*(size+1), rows)
		assert.Equal(t, size, sick)
		assert.Equal(t, size, excused)
	}
	assert.Equal(t, statements[0], statements[1], "statements must not grow with dates, children or populated templates")
	t.Logf("1/5 templates, 2/6 dates and children: %v statements", statements)
}
