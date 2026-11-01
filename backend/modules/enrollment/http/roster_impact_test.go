package enrollmenthttp_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Schule am Berg, October 2026 (#5102): the Randstunde of class 1b is a
// Regeltermin with the target Klasse 1b and hand-picked children, no offering
// source. Taking Friday out of the child's booking left the child in every
// Friday occurrence; nothing told the office. The correction now reports the
// Regeltermin as not adjusted, and the confirmed removal takes the child off
// Fridays from the effective date on only.
func TestUpdateChildOfferings_ReportsHandMaintainedRegelterminAndRemovesFridays(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	period := offeringSourcePeriod(t, env)

	randstunde := createAdjustmentCareOfferingWith(t, env, "Kurzbetreuung", nil)
	sourced := createCareOfferingTemplateGroup(t, env.db, "Kurzbetreuung Block")
	sourced.TargetGroupType = timetable.TargetGroupTypeOffering
	sourced.SourceCareOfferingIDs = []int64{randstunde.ID}
	sourced.CalendarPeriodID = &period.ID
	_, err := env.db.NewUpdate().Model(sourced).
		Column("target_group_type", "source_care_offering_ids", "calendar_period_id").
		WherePK().Exec(ctx)
	require.NoError(t, err)
	handTemplate := createCareOfferingTemplateGroup(t, env.db, "RDST 1b")
	handTemplate.CalendarPeriodID = &period.ID
	_, err = env.db.NewUpdate().Model(handTemplate).Column("calendar_period_id").WherePK().Exec(ctx)
	require.NoError(t, err)
	_, err = env.db.NewRaw(`INSERT INTO activities.group_targets
		(tenant_id, activity_group_id, target_group_type, target_school_class)
		VALUES (?, ?, 'klasse', '1b')`, testpkg.Tenant(t), handTemplate.ID).Exec(ctx)
	require.NoError(t, err)
	for _, weekday := range []int{weekdayMonday, timetable.WeekdayFriday} {
		createCareOfferingTemplateSchedule(t, env.db, sourced.ID, weekday, &period.ID)
		createCareOfferingTemplateSchedule(t, env.db, handTemplate.ID, weekday, &period.ID)
	}

	submitted, err := env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(t), PhaseID: env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "AmBerg",
		GuardianEmail: "schule-am-berg-5102@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Mia", LastName: "AmBerg",
			DateOfBirth:       calendar.NewDate(2019, 3, 4),
			TargetGradeLevel:  testpkg.Int16Ptr(1),
			TargetSchoolClass: testpkg.StrPtr("1b"),
			OfferingIDs:       []int64{randstunde.ID},
			OfferingDays: []enrollmentAPI.SubmitOfferingDays{{
				OfferingID: randstunde.ID, SelectedDays: []string{"mon", "fri"},
			}},
		}},
	})
	require.NoError(t, err)
	childID := submitted.Children[0].ID
	outcome, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID

	// The school placed the child in class 1b, which the Randstunde targets.
	_, err = env.db.NewRaw(`UPDATE users.student_school_memberships SET school_class = '1b'
		WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`,
		testpkg.Tenant(t), studentID).Exec(ctx)
	require.NoError(t, err)
	// The office picked the child by hand as well, for every weekday.
	_, err = env.timetable.CreateStudentEnrollment(ctx, timetable.StudentEnrollmentInput{
		StudentID: studentID, ActivityGroupID: handTemplate.ID,
		ValidFrom: string(env.sourcePhase.ServiceStartDate), CalendarPeriodID: &period.ID,
	})
	require.NoError(t, err)
	room := testpkg.CreateTestRoom(t, env.db, "RDST-Raum")
	occurrence := func(date calendar.Date) int64 {
		instance := testpkg.CreateTestActivityInstance(t, env.db, date, room.ID, testpkg.ActivityInstanceOpts{
			ActivityGroupID: &handTemplate.ID, CalendarPeriodID: &period.ID, Title: handTemplate.Name,
		})
		testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, "")
		return instance.ID
	}
	fridayBefore := occurrence(calendar.NewDate(2027, 5, 28))
	mondayAfter := occurrence(calendar.NewDate(2027, 5, 31))
	fridayAfter := occurrence(calendar.NewDate(2027, 6, 4))
	checkedIn := testpkg.CreateTestActivityInstance(t, env.db, calendar.NewDate(2027, 6, 11), room.ID, testpkg.ActivityInstanceOpts{
		ActivityGroupID: &handTemplate.ID, CalendarPeriodID: &period.ID, Title: handTemplate.Name,
	})
	checkInAt := calendar.NewDate(2027, 6, 11).BerlinMidnight().Add(9 * time.Hour)
	testpkg.CreateTestInstanceStudent(t, env.db, checkedIn.ID, studentID, "", testpkg.InstanceStudentOpts{CheckedInAt: &checkInAt})

	stichtag := calendar.NewDate(2027, 5, 31)
	update, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Randstunde freitags abgemeldet", EffectiveFrom: &stichtag,
		Offerings: []enrollmentAPI.OfferingAdjustmentSelection{{
			OfferingID: randstunde.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)

	impact := update.RosterImpact
	assert.Equal(t, stichtag, impact.EffectiveFrom)
	require.Len(t, impact.Adjusted, 1, "the offering-sourced block follows the booking")
	assert.Equal(t, sourced.ID, impact.Adjusted[0].TemplateID)
	assert.Equal(t, []int{timetable.WeekdayFriday}, impact.Adjusted[0].RemovedWeekdays)
	assert.Empty(t, impact.Adjusted[0].AddedWeekdays)
	require.Len(t, impact.NotAdjusted, 1, "only the hand-maintained Randstunde keeps the child")
	assert.Equal(t, handTemplate.ID, impact.NotAdjusted[0].TemplateID)
	assert.Equal(t, handTemplate.Name, impact.NotAdjusted[0].Name)
	assert.Equal(t, []int{timetable.WeekdayFriday}, impact.NotAdjusted[0].Weekdays)
	assert.True(t, impact.NotAdjusted[0].ViaTarget, "Klasse 1b adds the child to occurrences created later")

	days := api.NewStudentTemplateDaysTestModule(testpkg.NewDatabase(t, env.db), func() calendar.Date { return decisionTestToday })
	// #5497: another enrollment that books Friday again from a later day
	// ends the gap there; „Entfernen“ then stops at that day.
	rebooked := calendar.NewDate(2027, 6, 14)
	bounded, err := testpkg.TenantQuery1(t, ctx, env.db, days.ManualStudentPlan, timetable.ManualStudentPlanQuery{
		StudentID: studentID, From: stichtag, Weekdays: []int{timetable.WeekdayFriday},
		NetLossWeekdays: []int{timetable.WeekdayFriday},
		Bookings:        []timetable.StudentDayBooking{{Weekday: weekdayMonday, CountsAsCare: true}},
		OtherBookings:   []timetable.StudentDayBooking{{Weekday: timetable.WeekdayFriday, CountsAsCare: true, From: &rebooked}},
	})
	require.NoError(t, err)
	require.Len(t, bounded, 1, "the later booking does not carry the Stichtag")
	assert.Equal(t, []int{timetable.WeekdayFriday}, bounded[0].Weekdays)
	require.NotNil(t, bounded[0].Until, "the later booking ends the gap")
	assert.Equal(t, rebooked, *bounded[0].Until)
	_, err = testpkg.TenantQuery1(t, ctx, env.db, days.RemoveStudentTemplateDays, timetable.StudentTemplateDaysRemoval{
		TemplateID: handTemplate.ID, StudentID: studentID,
		Weekdays: []int{timetable.WeekdayFriday}, From: decisionTestToday.AddDays(-1),
	})
	require.ErrorIs(t, err, timetable.ErrStudentTemplateDaysInvalid, "past days are never rewritten")
	removed, err := testpkg.TenantQuery1(t, ctx, env.db, days.RemoveStudentTemplateDays, timetable.StudentTemplateDaysRemoval{
		TemplateID: handTemplate.ID, StudentID: studentID,
		Weekdays: []int{timetable.WeekdayFriday}, From: stichtag,
	})
	require.NoError(t, err)
	assert.Equal(t, timetable.StudentTemplateDaysRemoved{RosterRows: 1, Occurrences: 1}, removed)

	assert.True(t, instanceHasStudent(t, env, fridayBefore, studentID), "a Friday before the Stichtag stays")
	assert.True(t, instanceHasStudent(t, env, mondayAfter, studentID), "Monday stays")
	assert.False(t, instanceHasStudent(t, env, fridayAfter, studentID), "Fridays from the Stichtag on lose the child")
	assert.True(t, instanceHasStudent(t, env, checkedIn.ID, studentID), "recorded attendance stays")

	var handRows []studentEnrollment
	for _, row := range listStudentEnrollmentRowsForDecisionTest(t, env, studentID) {
		if row.ActivityGroupID == handTemplate.ID {
			handRows = append(handRows, row)
		}
	}
	require.Len(t, handRows, 2, "the hand row ends at the Stichtag and continues without Friday")
	assert.Nil(t, handRows[0].EnrollmentRequestChildID)
	require.NotNil(t, handRows[0].ValidUntil)
	assert.Equal(t, stichtag, *handRows[0].ValidUntil)
	assert.Empty(t, handRows[0].SelectedWeekdays)
	assert.Equal(t, stichtag, handRows[1].ValidFrom)
	assert.Nil(t, handRows[1].ValidUntil)
	require.NotNil(t, handRows[1].Weekday, "the continuation keeps the editor-owned weekday scope")
	assert.Equal(t, weekdayMonday, *handRows[1].Weekday)
	assert.Empty(t, handRows[1].SelectedWeekdays, "selected weekdays would mark the row as booking-owned")
	assert.Nil(t, handRows[1].EnrollmentRequestChildID)

	after, err := testpkg.TenantQuery1(t, ctx, env.db, days.ManualStudentPlan, timetable.ManualStudentPlanQuery{
		StudentID: studentID, From: stichtag, Weekdays: []int{timetable.WeekdayFriday},
	})
	require.NoError(t, err)
	assert.Empty(t, after, "after the removal nothing plans the child on Fridays any more")
}

func instanceHasStudent(t *testing.T, env *decisionTestEnv, instanceID, studentID int64) bool {
	t.Helper()
	count, err := env.db.NewSelect().
		TableExpr("schedule.instance_students").
		Where("tenant_id = ?", testpkg.Tenant(t)).
		Where("instance_id = ?", instanceID).
		Where("student_id = ?", studentID).
		Count(context.Background())
	require.NoError(t, err)
	return count > 0
}
