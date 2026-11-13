package contracttest_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api"
	enrollmentTest "github.com/moto-ogs/project-phoenix/modules/enrollment/enrollmenttest"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// An approval puts the child on every future occurrence of its sourced
// template and applies the day rules to those occurrences. It takes the
// student rows and the care-day locks of all their children in one statement
// each, not one pair per occurrence and child (#6981), and the rules still
// reach a sick child and a child excused before the block.
func TestBookingApprovalLocksCareDaysOnceAndAppliesDayRules(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	from := calendar.NewDate(2099, 1, 5)
	setSourcePhaseServiceStartDate(t, env, from)
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t), uniqueSchemaName(t.Name()), calendar.Date(env.sourcePhase.ServiceStartDate), calendar.Date(env.sourcePhase.ServiceEndDate))
	room := testpkg.CreateTestRoom(t, env.db, "Sperrraum")
	staff := testpkg.CreateTestStaff(t, env.db, "Sperr", "Probe")
	counter := testpkg.CaptureQueriesForContext(t, env.db)
	type locks struct{ studentRows, careDays int }
	var measured []locks
	for _, size := range []int{1, 4} {
		offering := createSourceOffering(t, env, fmt.Sprintf("Sperren %d", size), nil)
		group := createSourcedTemplate(t, env, fmt.Sprintf("Sperren %d", size), offering.ID, nil, period.ID)
		instances := []int64{}
		for week := range size + 1 {
			inst := testpkg.CreateTestActivityInstance(t, env.db, from.AddDays(week*7), room.ID, testpkg.ActivityInstanceOpts{ActivityGroupID: &group.ID, CalendarPeriodID: &period.ID})
			instances = append(instances, inst.ID)
		}
		booked := []int64{}
		for i := range size {
			student, _ := submitAndApproveOfferingChild(t, env, offering.ID, fmt.Sprintf("locks-%d-%d@example.test", size, i), fmt.Sprintf("Sperre %d/%d", size, i), 1)
			booked = append(booked, student)
		}
		// Day rules written straight to the tables reach the booked
		// children's rows only through the approval's pass below.
		testpkg.CreateTestStudentStatusDay(t, env.db, booked[0], from, "sick")
		exception := testpkg.CreateTestPickupException(t, env.db, booked[size-1], from.AddDays(size*7), staff.ID, "", "Entschuldigt")
		_, err := env.db.NewUpdate().Table("schedule.student_pickup_exceptions").Set("excused_from = ?", testpkg.WallClock(13, 0)).Set("excused_created_by = ?", staff.ID).
			Where("tenant_id = ?", testpkg.Tenant(t)).Where("id = ?", exception.ID).Exec(testpkg.Ctx(t))
		require.NoError(t, err)

		counter.Reset()
		approveOfferingChildIn(t, env, counter.Context(testpkg.Ctx(t)), offering.ID, fmt.Sprintf("locks-%d-probe@example.test", size), fmt.Sprintf("Sperre %d/neu", size))
		measured = append(measured, locks{
			studentRows: len(counter.Matching(func(sql string) bool {
				return strings.Contains(sql, "users.student_profiles") && strings.Contains(sql, "for update")
			})),
			careDays: len(counter.Matching(func(sql string) bool { return strings.Contains(sql, "care-exception-day") })),
		})

		var rows, sick, excused int
		require.NoError(t, env.db.NewRaw(`SELECT COUNT(*),COUNT(*) FILTER(WHERE a.student_status_day_id IS NOT NULL),COUNT(*) FILTER(WHERE a.pickup_exception_id IS NOT NULL)
   FROM schedule.instance_students AS p LEFT JOIN active.activity_session_attendance AS a ON a.instance_student_id=p.id AND a.tenant_id=p.tenant_id
   WHERE p.tenant_id=? AND p.instance_id IN (?)`, testpkg.Tenant(t), bun.List(instances)).Scan(testpkg.Ctx(t), &rows, &sick, &excused))
		assert.Equal(t, (size+1)*(size+1), rows, "every child is planned on every occurrence")
		assert.Equal(t, 1, sick, "the sick child reads absent on its day")
		assert.Equal(t, 1, excused, "the excused child reads absent from its cutoff")
	}
	t.Logf("student row / care-day lock statements for 2 and 5 occurrences with 2 and 5 children: %+v", measured)
	assert.Equal(t, measured[0], measured[1], "lock statements must not grow with occurrences or children")
	assert.Equal(t, 1, measured[1].careDays, "all care days are locked in one statement")
}

// approveOfferingChildIn submits a child with the offering selected and
// approves it in a transaction on ctx.
func approveOfferingChildIn(t *testing.T, env *decisionTestEnv, ctx context.Context, offeringID int64, guardianEmail, firstName string) {
	t.Helper()
	grade := int16(1)
	submitted, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), env.db, env.requestSvc.Submit, enrollmentTest.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Sperre", GuardianEmail: guardianEmail,
		ConsentFlags: enrollmentTest.FormValuesOf(map[string]any{"agb": true, "data_processing": true, "email_contact": true, "photo": true}),
		Children: []enrollmentTest.SubmitChild{{
			FirstName: firstName, LastName: "Sperre", DateOfBirth: calendar.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade, OfferingIDs: []int64{offeringID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	require.NoError(t, testpkg.WithTenantTx(t, ctx, env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		outcome, err := testpkg.TenantQuery1(t, txCtx, env.db, env.decision.Decide, enrollmentTest.DecideInput{
			RequestID: submitted.Request.ID, ChildID: submitted.Children[0].ID,
			Status: enrollmentTest.DecisionApproved, ReviewedBy: env.creatorID,
		})
		if err == nil && outcome.Child.CreatedStudentID == nil {
			err = fmt.Errorf("approval created no student")
		}
		return err
	}))
}
