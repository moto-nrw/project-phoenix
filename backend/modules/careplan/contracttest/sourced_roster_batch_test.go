package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSourcedRosterBatchSeedsAndRetiresEveryTemplate(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	from := calendar.NewDate(2099, 1, 5)
	setSourcePhaseServiceStartDate(t, env, from)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName(t.Name()), calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate))
	offering := createSourceOffering(t, env, "Stapel", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "batch-roster@example.test", "Stapel", 1)
	first := createSourcedTemplate(t, env, "Stapel eins", offering.ID, nil, period.ID)
	second := createSourcedTemplate(t, env, "Stapel zwei", offering.ID, nil, period.ID)
	bookings := newBookingMaterialization(t, env.db).Bookings
	batch, ok := bookings.(interface {
		ResyncTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
		DetachTemplatesSourcedFromOfferings(context.Context, []int64, calendar.Date) error
	})
	require.True(t, ok)
	run := func(command func(context.Context, []int64, calendar.Date) error, from calendar.Date) {
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(ctx context.Context, _ bun.Tx) error { return command(ctx, []int64{offering.ID}, from) }))
	}
	room := testpkg.CreateTestRoom(t, env.db, "Stapelraum")
	instances := []int64{}
	for _, id := range []int64{first.ID, second.ID} {
		inst := testpkg.CreateTestActivityInstance(t, env.db, from.AddDays(7), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &id, CalendarPeriodID: &period.ID})
		instances = append(instances, inst.ID)
	}
	countParticipants := func() int {
		var count int
		require.NoError(t, env.db.NewSelect().Table("schedule.instance_students").ColumnExpr("COUNT(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Where("instance_id IN (?)", bun.List(instances)).Where("student_id = ?", studentID).Scan(testpkg.Ctx(t), &count))
		return count
	}
	run(batch.ResyncTemplatesSourcedFromOfferings, from)
	assert.Equal(t, 2, countParticipants())
	for _, id := range []int64{first.ID, second.ID} {
		rows := loadTemplateEnrollments(t, env, id)
		require.Len(t, rows, 1)
		assert.Equal(t, studentID, rows[0].StudentID)
		assert.Equal(t, from.String(), rows[0].ValidFrom)
	}
	// One row has started and must be capped; the other starts on the
	// withdrawal boundary and must be deleted. Both sources are cleared.
	_, err := env.db.NewUpdate().Table("activities.student_enrollments").Set("valid_from = ?", from.AddDays(7)).Where("tenant_id = ?", testpkg.Tenant(t)).Where("activity_group_id = ?", second.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	run(batch.DetachTemplatesSourcedFromOfferings, from.AddDays(7))
	rows := loadTemplateEnrollments(t, env, first.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, from.AddDays(7).String(), *rows[0].ValidUntil)
	assert.Empty(t, loadTemplateEnrollments(t, env, second.ID))
	var sources int
	require.NoError(t, env.db.NewSelect().Table("activities.groups").ColumnExpr("COUNT(*)").Where("tenant_id = ?", testpkg.Tenant(t)).Where("id IN (?)", bun.List([]int64{first.ID, second.ID})).Where("source_care_offering_ids IS NOT NULL").Scan(testpkg.Ctx(t), &sources))
	assert.Zero(t, sources)
	assert.Zero(t, countParticipants())
}
