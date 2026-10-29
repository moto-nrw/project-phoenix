package contracttest_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// A parent asks to drop Ganztag on Mondays and keeps the Randstunde (#5342,
// anonymised production pattern). The approval reports only the Regeltermin
// the remaining Randstunde no longer carries: the class Randstunde until
// 12:45 stays carried, so the review card does not wait for "Fertig" because
// of it; the Lernzeit until 14:30 lost its booking and is reported.
func TestOfferingChangeRequestService_Decide_ReportsOnlyRegeltermineWithoutCarryingBooking(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	careOffering := func(name, pickup string) *offeringContract.CareOfferingRecord {
		return createAdjustmentCareOfferingWith(t, env, name, func(o *offeringContract.CareOfferingRecord) {
			o.CountsAsCare, o.CountsAsCareSet = true, true
			o.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		})
	}
	randstunde := careOffering("Randstunde", "12:45")
	ganztag := careOffering("Ganztag bis 14.30", "14:30")
	_, _, studentID := submitApprovedAdjustmentChild(t, env, "offer-change-carry-5342@example.com", "Carry",
		[]*offeringContract.CareOfferingRecord{randstunde, ganztag})
	switchDate := calendar.Date(env.sourcePhase.ServiceStartDate).AddDays(150)
	for switchDate.Weekday() != time.Monday {
		switchDate = switchDate.AddDays(1)
	}

	period := testpkg.CreateTestCalendarPeriod(t, env.db, "Carry "+t.Name(),
		calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate))
	room := testpkg.CreateTestRoom(t, env.db, "Carry")
	classRandstunde := createCareOfferingTemplateGroup(t, env.db, "RDST 1b")
	lernzeit := createCareOfferingTemplateGroup(t, env.db, "Lernzeit JG 1")
	for _, planned := range []struct {
		groupID    int64
		start, end string
	}{{classRandstunde.ID, "11:45", "12:45"}, {lernzeit.ID, "13:45", "14:30"}} {
		instance := testpkg.CreateTestActivityInstance(t, env.db, switchDate.AddDays(7), room.ID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &planned.groupID, CalendarPeriodID: &period.ID,
			StartHHMM: planned.start, EndHHMM: planned.end,
		})
		testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, "")
	}

	row, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.SubmitOfferingChange, careplan.CreateOfferingChangeInput{
		StudentID: studentID, AccountID: env.creatorID, EffectiveFrom: switchDate,
		Selections: []careplan.OfferingChangeSelection{{OfferingID: randstunde.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	impact, err := testpkg.TenantQuery1(t, ctx, testpkg.SetupTestDB(t), svc.Decide, careplan.OfferingChangeDecisionInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	require.Len(t, impact.NotAdjusted, 1)
	assert.Equal(t, lernzeit.ID, impact.NotAdjusted[0].TemplateID)
	assert.Equal(t, []int{1}, impact.NotAdjusted[0].Weekdays)
}
