package enrollmenthttp_test

import (
	"fmt"
	"testing"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Booking changes and hand-maintained Regeltermine (#5342). A booking
// carries a Regeltermin of its weekday that ends no later than the
// offering's pickup time. The patterns are the anonymised booking changes of
// a production school: a Randstunde picked up at 12:45, Ganztag variants
// picked up at 14:30 and 16:00, and class Randstunden from 11:45 to 12:45
// that list every child of the class. Further schools book without pickup
// times or keep Gruppe and hand lists; those cases are covered as well.

// carryStichtag is the Monday the booking changes take effect from.
var carryStichtag = calendar.NewDate(2027, 5, 31)

type carryFixture struct {
	t      *testing.T
	env    *decisionTestEnv
	period *calendarPeriod
	seq    int
}

func newCarryFixture(t *testing.T) *carryFixture {
	t.Helper()
	env := setupDecisionTest(t)
	return &carryFixture{t: t, env: env, period: offeringSourcePeriod(t, env)}
}

// offering creates a parent-choice offering; an empty pickup time leaves the
// offering without pickup times.
func (f *carryFixture) offering(name, pickup string, countsAsCare bool) *offeringContract.CareOfferingRecord {
	f.t.Helper()
	offering := createAdjustmentCareOfferingWith(f.t, f.env, name, func(o *offeringContract.CareOfferingRecord) {
		if pickup != "" {
			o.PickupTimes = map[string]string{"mon": pickup, "tue": pickup, "wed": pickup, "thu": pickup, "fri": pickup}
		}
	})
	// The catalog fixture stores every offering as care; the flag is set on
	// the row.
	_, err := f.env.db.NewRaw(`UPDATE enrollment.care_offerings SET counts_as_care = ? WHERE id = ?`,
		countsAsCare, offering.ID).Exec(testpkg.Ctx(f.t))
	require.NoError(f.t, err)
	offering.CountsAsCare = countsAsCare
	return offering
}

// template creates a care Regeltermin without offering source that runs on
// the weekday from start to end; an empty start gives a schedule without
// time. listKind is optional.
func (f *carryFixture) template(name string, weekday int, start, end, listKind string) int64 {
	f.t.Helper()
	ctx := testpkg.Ctx(f.t)
	group := createCareOfferingTemplateGroup(f.t, f.env.db, name)
	_, err := f.env.db.NewRaw(`UPDATE activities.groups SET calendar_period_id = ?, list_kind = NULLIF(?, '') WHERE id = ?`,
		f.period.ID, listKind, group.ID).Exec(ctx)
	require.NoError(f.t, err)
	schedule := &careTemplateSchedule{Weekday: weekday, ActivityGroupID: group.ID, CalendarPeriodID: &f.period.ID}
	if start != "" {
		var timeframeID int64
		require.NoError(f.t, f.env.db.NewRaw(`INSERT INTO schedule.timeframes (tenant_id, start_time, end_time, is_active, description)
			VALUES (?, ?::time, ?::time, TRUE, ?) RETURNING id`,
			testpkg.Tenant(f.t), start, end, fmt.Sprintf("carry-%s-%d", f.t.Name(), group.ID)).Scan(ctx, &timeframeID))
		schedule.TimeframeID = &timeframeID
	}
	schedule.SetTenantID(testpkg.Tenant(f.t))
	require.NoError(f.t, createCareTemplateScheduleRow(ctx, f.env.db, schedule))
	return group.ID
}

// target adds a Klasse, Jahrgang or Gruppe target to the Regeltermin.
func (f *carryFixture) target(templateID int64, targetType string, schoolClass *string, gradeLevel *int16) {
	f.t.Helper()
	var educationGroupID *int64
	if targetType == timetable.TargetGroupTypeEducationGroup {
		educationGroupID = &testpkg.CreateTestEducationGroup(f.t, f.env.db, "Gruppe-"+f.t.Name()).ID
	}
	_, err := f.env.db.NewRaw(`INSERT INTO activities.group_targets
		(tenant_id, activity_group_id, target_group_type, target_school_class, target_grade_level, education_group_id)
		VALUES (?, ?, ?, ?, ?, ?)`, testpkg.Tenant(f.t), templateID, targetType, schoolClass, gradeLevel, educationGroupID).Exec(testpkg.Ctx(f.t))
	require.NoError(f.t, err)
}

// child submits and approves a child of the class with the bookings and
// returns request, child and student.
func (f *carryFixture) child(schoolClass string, bookings map[*offeringContract.CareOfferingRecord][]string) (int64, int64, int64) {
	f.t.Helper()
	ctx := testpkg.Ctx(f.t)
	f.seq++
	ids := make([]int64, 0, len(bookings))
	days := make([]enrollmentAPI.SubmitOfferingDays, 0, len(bookings))
	for offering, selected := range bookings {
		ids = append(ids, offering.ID)
		days = append(days, enrollmentAPI.SubmitOfferingDays{OfferingID: offering.ID, SelectedDays: selected})
	}
	submitted, err := f.env.requestSvc.Submit(ctx, enrollmentAPI.SubmitRequest{
		TenantID: testpkg.Tenant(f.t), PhaseID: f.env.sourcePhase.ID,
		GuardianFirstName: "Eltern", GuardianLastName: "Muster",
		GuardianEmail: fmt.Sprintf("carry-5342-%d@example.com", f.seq),
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentAPI.SubmitChild{{
			FirstName: "Kind", LastName: "Muster",
			DateOfBirth:       calendar.NewDate(2019, 3, 4),
			TargetGradeLevel:  testpkg.Int16Ptr(1),
			TargetSchoolClass: testpkg.StrPtr(schoolClass),
			OfferingIDs:       ids,
			OfferingDays:      days,
		}},
	})
	require.NoError(f.t, err)
	childID := submitted.Children[0].ID
	outcome, err := testpkg.TenantQuery1(f.t, ctx, f.env.db, f.env.decision.Decide, enrollmentAPI.DecideInput{
		RequestID: submitted.Request.ID, ChildID: childID,
		Status: enrollmentNative.DecisionApproved, ReviewedBy: f.env.creatorID,
	})
	require.NoError(f.t, err)
	require.NotNil(f.t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID
	_, err = f.env.db.NewRaw(`UPDATE users.student_school_memberships SET school_class = ?
		WHERE tenant_id = ? AND student_profile_id = ? AND deleted_at IS NULL`,
		schoolClass, testpkg.Tenant(f.t), studentID).Exec(ctx)
	require.NoError(f.t, err)
	return submitted.Request.ID, childID, studentID
}

// plan puts the child on the Regeltermin by hand, as a class list does.
func (f *carryFixture) plan(studentID, templateID int64) {
	f.t.Helper()
	_, err := f.env.timetable.CreateStudentEnrollment(testpkg.Ctx(f.t), timetable.StudentEnrollmentInput{
		StudentID: studentID, ActivityGroupID: templateID,
		ValidFrom: string(f.env.sourcePhase.ServiceStartDate), CalendarPeriodID: &f.period.ID,
	})
	require.NoError(f.t, err)
}

// change books the selection from the Stichtag on and returns the
// Regeltermine reported as not adjusted, by template id.
func (f *carryFixture) change(requestID, childID int64, selection map[*offeringContract.CareOfferingRecord][]string, withdrawal bool) map[int64][]int {
	f.t.Helper()
	offerings := make([]enrollmentAPI.OfferingAdjustmentSelection, 0, len(selection))
	for offering, days := range selection {
		offerings = append(offerings, enrollmentAPI.OfferingAdjustmentSelection{OfferingID: offering.ID, SelectedDays: days})
	}
	stichtag := carryStichtag
	update, err := testpkg.TenantQuery1(f.t, testpkg.Ctx(f.t), f.env.db, f.env.decision.UpdateChildOfferings, enrollmentAPI.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: f.env.creatorID, ActorRole: "admin",
		Reason: "Buchung geändert", EffectiveFrom: &stichtag, Offerings: offerings,
		CompleteWithdrawalConfirmed: withdrawal,
	})
	require.NoError(f.t, err)
	assert.Equal(f.t, stichtag, update.RosterImpact.EffectiveFrom)
	reported := make(map[int64][]int, len(update.RosterImpact.NotAdjusted))
	for _, template := range update.RosterImpact.NotAdjusted {
		reported[template.TemplateID] = template.Weekdays
	}
	return reported
}

const (
	carryMonday = timetable.WeekdayMonday
	carryFriday = timetable.WeekdayFriday
)

// Prod pattern 1 of 2 false notices: the child drops Ganztag on Friday and
// keeps the Randstunde. The class Randstunde 11:45-12:45 keeps its booking.
func TestRosterImpact_GanztagDroppedRandstundeStays_NoNotice(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	ganztag := f.offering("Ganztag bis 14.30", "14:30", true)
	classRandstunde := f.template("RDST 1b", carryFriday, "11:45", "12:45", timetable.ListKindEdgeHours)
	f.target(classRandstunde, timetable.TargetGroupTypeSchoolClass, testpkg.StrPtr("1b"), nil)
	requestID, childID, studentID := f.child("1b", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon", "tue", "wed", "thu", "fri"}, ganztag: {"fri"},
	})
	f.plan(studentID, classRandstunde)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon", "tue", "wed", "thu", "fri"},
	}, false)

	assert.Empty(t, reported, "the Randstunde booking still carries the class Randstunde on Friday")
}

// The true prod case: the child's only booking, the Randstunde, ends on
// Monday, and the class Randstunde keeps the child on Monday.
func TestRosterImpact_RandstundeDropped_ReportsClassRandstunde(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	classRandstunde := f.template("RDST 1b", carryMonday, "11:45", "12:45", timetable.ListKindEdgeHours)
	f.target(classRandstunde, timetable.TargetGroupTypeSchoolClass, testpkg.StrPtr("1b"), nil)
	requestID, childID, studentID := f.child("1b", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon", "fri"},
	})
	f.plan(studentID, classRandstunde)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"},
	}, false)

	assert.Equal(t, map[int64][]int{classRandstunde: {carryMonday}}, reported)
}

// Decided for #5342: Ganztag care runs past 12:45, so a Randstunde dropped
// beside a Ganztag that stays leaves the class Randstunde carried, like the
// Ganztag children the class list already holds.
func TestRosterImpact_RandstundeDroppedGanztagStays_NoNotice(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	ganztag := f.offering("Ganztag bis 14.30", "14:30", true)
	classRandstunde := f.template("RDST 1a", carryMonday, "11:45", "12:45", timetable.ListKindEdgeHours)
	f.target(classRandstunde, timetable.TargetGroupTypeSchoolClass, testpkg.StrPtr("1a"), nil)
	requestID, childID, studentID := f.child("1a", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon"}, ganztag: {"mon"},
	})
	f.plan(studentID, classRandstunde)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		ganztag: {"mon"},
	}, false)

	assert.Empty(t, reported)
}

// A switch on the same day between Ganztag until 16:00 and Ganztag until
// 14:30 keeps the block until 13:45 carried and reports the block until
// 16:00, which the earlier pickup no longer reaches.
func TestRosterImpact_SwitchOnTheSameDay_ReportsOnlyTheBlockAfterPickup(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	late := f.offering("Ganztag bis 16 Uhr", "16:00", true)
	early := f.offering("Ganztag bis 14.30", "14:30", true)
	lunchBlock := f.template("GT 1", carryMonday, "12:45", "13:45", "")
	afternoon := f.template("AG Nachmittag", carryMonday, "14:30", "16:00", timetable.ListKindActivity)
	requestID, childID, studentID := f.child("1a", map[*offeringContract.CareOfferingRecord][]string{late: {"mon"}})
	f.plan(studentID, lunchBlock)
	f.plan(studentID, afternoon)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{early: {"mon"}}, false)

	assert.Equal(t, map[int64][]int{afternoon: {carryMonday}}, reported)
}

// A Jahrgang Regeltermin without class list follows the same rule: the
// Randstunde that stays carries the block until 12:45, not the Lernzeit
// until 14:30.
func TestRosterImpact_JahrgangWithoutClassList_ReportsOnlyUncarriedBlock(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	ganztag := f.offering("Ganztag bis 14.30", "14:30", true)
	early := f.template("Jahrgang 1 Mittag", carryFriday, "11:45", "12:45", "")
	lernzeit := f.template("Lernzeit JG 1", carryFriday, "13:45", "14:30", timetable.ListKindLearningTime)
	f.target(early, timetable.TargetGroupTypeGrade, nil, testpkg.Int16Ptr(1))
	f.target(lernzeit, timetable.TargetGroupTypeGrade, nil, testpkg.Int16Ptr(1))
	requestID, childID, studentID := f.child("1c", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"}, ganztag: {"fri"},
	})
	f.plan(studentID, early)
	f.plan(studentID, lernzeit)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{randstunde: {"fri"}}, false)

	assert.Equal(t, map[int64][]int{lernzeit: {carryFriday}}, reported)
}

// A child without any booking left keeps no Regeltermin carried.
func TestRosterImpact_ChildWithoutAnyBooking_ReportsEveryHandRegeltermin(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	classRandstunde := f.template("RDST 2c", carryMonday, "11:45", "12:45", timetable.ListKindEdgeHours)
	handList := f.template("Hausaufgaben", carryFriday, "13:00", "14:00", "")
	requestID, childID, studentID := f.child("2c", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon", "fri"},
	})
	f.plan(studentID, classRandstunde)
	f.plan(studentID, handList)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{}, true)

	assert.Equal(t, map[int64][]int{classRandstunde: {carryMonday}, handList: {carryFriday}}, reported)
}

// A school without pickup times (missing data): the end of care is unknown.
// A Friday without any booking is reported; a switch between two offerings
// on Monday is no net loss and keeps the Gruppe list, as before.
func TestRosterImpact_WithoutPickupTimes_ReportsOnlyDaysWithoutBooking(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	afternoon := f.offering("Nachmittagsbetreuung", "", true)
	ganztag := f.offering("Ganztag", "", true)
	groupList := f.template("Mittagessen Gruppe", carryMonday, "13:00", "14:00", timetable.ListKindMensa)
	f.target(groupList, timetable.TargetGroupTypeEducationGroup, nil, nil)
	fridayList := f.template("Hausaufgaben", carryFriday, "13:00", "14:45", "")
	requestID, childID, studentID := f.child("3", map[*offeringContract.CareOfferingRecord][]string{
		afternoon: {"mon", "fri"},
	})
	f.plan(studentID, groupList)
	f.plan(studentID, fridayList)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{ganztag: {"mon"}}, false)

	assert.Equal(t, map[int64][]int{fridayList: {carryFriday}}, reported)
}

// A Regeltermin whose schedule has no time cannot be checked against a
// pickup time: the day keeps the earlier rule. Dropping Ganztag beside the
// Randstunde is a net loss and is reported.
func TestRosterImpact_RegelterminWithoutTime_KeepsTheNetLossRule(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstunde", "12:45", true)
	ganztag := f.offering("Ganztag bis 16 Uhr", "16:00", true)
	untimed := f.template("Freitag ohne Uhrzeit", carryFriday, "", "", "")
	requestID, childID, studentID := f.child("1b", map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"}, ganztag: {"fri"},
	})
	f.plan(studentID, untimed)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{randstunde: {"fri"}}, false)

	assert.Equal(t, map[int64][]int{untimed: {carryFriday}}, reported)
}

// A school with a Randstunde and Ganztag but without pickup times (missing
// data, production pattern of another school): the times cannot decide, so
// the earlier rule stays. Dropping Ganztag beside the Randstunde reports the
// afternoon Lernzeit; dropping the Randstunde beside Ganztag reports the
// class list, as before. Nothing that was reported before is lost.
func TestRosterImpact_RandstundeAndGanztagWithoutPickupTimes_KeepTheNetLossRule(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	randstunde := f.offering("Randstundenbetreuung", "", true)
	ganztag := f.offering("Ganztagsbetreuung bis 16 Uhr", "", true)
	lernzeit := f.template("Lernzeit", carryFriday, "14:30", "15:30", timetable.ListKindLearningTime)
	classList := f.template("Klasse 1a Mittag", carryMonday, "11:45", "12:45", "")
	f.target(classList, timetable.TargetGroupTypeSchoolClass, testpkg.StrPtr("1a"), nil)
	both := map[*offeringContract.CareOfferingRecord][]string{randstunde: {"mon", "fri"}, ganztag: {"mon", "fri"}}

	requestID, childID, studentID := f.child("1a", both)
	f.plan(studentID, lernzeit)
	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"mon", "fri"}, ganztag: {"mon"},
	}, false)
	assert.Equal(t, map[int64][]int{lernzeit: {carryFriday}}, reported, "Ganztag dropped on Friday")

	requestID, childID, studentID = f.child("1a", both)
	f.plan(studentID, classList)
	reported = f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{
		randstunde: {"fri"}, ganztag: {"mon", "fri"},
	}, false)
	assert.Equal(t, map[int64][]int{classList: {carryMonday}}, reported, "Randstunde dropped on Monday")
}

// An offering that does not count as care carries no care Regeltermin: the
// child keeps only a course on Friday, so the care block is reported.
func TestRosterImpact_OnlyCourseLeft_ReportsCareRegeltermin(t *testing.T) {
	t.Parallel()
	f := newCarryFixture(t)
	ganztag := f.offering("Ganztag bis 16 Uhr", "16:00", true)
	course := f.offering("Schwimmkurs", "", false)
	careBlock := f.template("Freitag GT", carryFriday, "13:30", "14:30", "")
	requestID, childID, studentID := f.child("2b", map[*offeringContract.CareOfferingRecord][]string{
		ganztag: {"fri"}, course: {"fri"},
	})
	f.plan(studentID, careBlock)

	reported := f.change(requestID, childID, map[*offeringContract.CareOfferingRecord][]string{course: {"fri"}}, false)

	assert.Equal(t, map[int64][]int{careBlock: {carryFriday}}, reported)
}
