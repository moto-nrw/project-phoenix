// End-to-end scenarios for "Betreuung beenden" (#2487) in BOTH operating
// modes the platform runs today.
//
// The two schools in production are deliberately different, and the exit has
// to behave the same in both:
//
//   - Altenberge: binary attendance with NFC, OGS groups, arrival and pickup
//     times. No care plan, no offerings, no parent accounts.
//   - Am Berg: the full setup — materialized timetable blocks with roster
//     rows, care offerings, detailed room presence and parent accounts.
package users_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newActiveService wires the presence service the way the server does, minus
// the broadcaster (these tests assert on data, not on SSE).
func newActiveService(t *testing.T, db *bun.DB) activeService.Service {
	t.Helper()
	repos := repositories.NewFactory(db)
	return activeService.NewService(activeService.ServiceDependencies{
		GroupRepo:          repos.ActiveGroup,
		VisitRepo:          repos.ActiveVisit,
		SupervisorRepo:     repos.GroupSupervisor,
		CombinedGroupRepo:  repos.CombinedGroup,
		GroupMappingRepo:   repos.GroupMapping,
		AttendanceRepo:     repos.Attendance,
		StudentRepo:        repos.Student,
		PersonRepo:         repos.Person,
		TeacherRepo:        repos.Teacher,
		StaffRepo:          repos.Staff,
		RoomRepo:           repos.Room,
		ActivityGroupRepo:  repos.ActivityGroup,
		ActivityCatRepo:    repos.ActivityCategory,
		EducationGroupRepo: repos.Group,
		DeviceRepo:         repos.Device,
		UsersService: userService.NewPersonService(userService.PersonServiceDependencies{
			PersonRepo:  repos.Person,
			RFIDRepo:    repos.RFIDCard,
			AccountRepo: repos.Account,
			StudentRepo: repos.Student,
			StaffRepo:   repos.Staff,
			TeacherRepo: repos.Teacher,
			DB:          db,
		}),
		DB:     db,
		Logger: slog.Default(),
	})
}

// makeExitEffective moves the recorded last care day into the past, which is
// what the calendar does overnight. The product refuses a retroactive exit, so
// this is the only honest way to test the day AFTER one.
func makeExitEffective(t *testing.T, db *bun.DB, studentID int64, lastCareDay timezone.Date) {
	t.Helper()
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("enrolled_until = ?", lastCareDay).
		Where("id = ?", studentID).
		Exec(context.Background())
	require.NoError(t, err)
}

// TestCareExit_BinarySchoolWithNfcAndGroups is the Altenberge shape.
func TestCareExit_BinarySchoolWithNfcAndGroups(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	svc := newCareLifecycleService(t, db)
	presence := newActiveService(t, db)
	actorID := careActor(t, db)

	group := testpkg.CreateTestEducationGroup(t, db, "Bienen")
	student := testpkg.CreateTestStudent(t, db, "Leon", "Altmann", "2a")
	_, err := db.NewUpdate().
		TableExpr("users.students").
		Set("group_id = ?", group.ID).
		Where("id = ?", student.ID).
		Exec(context.Background())
	require.NoError(t, err)

	// The bracelet the school hands back when a child leaves.
	card := testpkg.CreateTestRFIDCard(t, db, "AABBCCDD")
	_, err = db.NewUpdate().
		TableExpr("users.persons").
		Set("tag_id = ?", card.ID).
		Where("id = ?", student.PersonID).
		Exec(context.Background())
	require.NoError(t, err)

	staff := testpkg.CreateTestStaff(t, db, "Petra", "Sommer")
	device := testpkg.CreateTestDevice(t, db, "kiosk-altenberge")
	today := timezone.TodayDate()

	// The child is at the OGS on their last care day, and is still checked in
	// when the day ends — the case the effect pass has to close cleanly.
	testpkg.CreateTestAttendance(t, db, student.ID, staff.ID, device.ID, time.Now().Add(-3*time.Hour), nil)

	endCare(t, ctx, svc, actorID, userService.CareExitInput{
		StudentIDs:  []int64{student.ID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonMovedAway,
	})

	// STILL the last care day: everything keeps working.
	assert.False(t, loadStudent(t, db, ctx, student.ID).CareEndedOn(today))

	// --- the next morning -------------------------------------------------
	makeExitEffective(t, db, student.ID, today.AddDays(-1))
	applied, err := svc.ApplyDueEffects(ctx, today)
	require.NoError(t, err)
	assert.Equal(t, 1, applied)

	t.Run("the open attendance is closed, not deleted", func(t *testing.T) {
		rows, err := repos.Attendance.GetTodayByStudentIDs(ctx, []int64{student.ID})
		require.NoError(t, err)
		record := rows[student.ID]
		require.NotNil(t, record, "the day that happened stays in the history")
		assert.NotNil(t, record.CheckOutTime, "but it is no longer an open day")
	})

	t.Run("the bracelet is free again", func(t *testing.T) {
		person, err := repos.Person.FindByID(ctx, student.PersonID)
		require.NoError(t, err)
		assert.Nil(t, person.TagID,
			"the physical bracelet goes back into the box and can be reissued")
	})

	t.Run("web and kiosk check-in are refused", func(t *testing.T) {
		_, err := presence.ToggleStudentAttendance(ctx, student.ID, staff.ID, device.ID, true)
		require.ErrorIs(t, err, activeService.ErrStudentCareEnded)
	})

	t.Run("the child is gone from the group roster reads", func(t *testing.T) {
		personSvc := userService.NewPersonService(userService.PersonServiceDependencies{
			PersonRepo:  repos.Person,
			RFIDRepo:    repos.RFIDCard,
			AccountRepo: repos.Account,
			StudentRepo: repos.Student,
			StaffRepo:   repos.Staff,
			TeacherRepo: repos.Teacher,
			DB:          db,
		})
		eligible, err := personSvc.GetEligibleStudentsByGroupIDsOnDate(
			ctx, []int64{group.ID}, today, today)
		require.NoError(t, err)
		for _, row := range eligible {
			assert.NotEqual(t, student.ID, row.ID,
				"a departed child is not on today's group roster any more")
		}
	})

	t.Run("running the effect pass twice changes nothing more", func(t *testing.T) {
		// Status has not flipped yet in this test (no scheduler), so the pass
		// still finds the child — and finds nothing left to do.
		_, err := svc.ApplyDueEffects(ctx, today)
		require.NoError(t, err)
		person, err := repos.Person.FindByID(ctx, student.PersonID)
		require.NoError(t, err)
		assert.Nil(t, person.TagID)
	})
}

// TestCareExit_FullSchoolWithPlanOfferingsAndParents is the am-Berg shape.
func TestCareExit_FullSchoolWithPlanOfferingsAndParents(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(db)
	svc := newCareLifecycleService(t, db)
	actorID := careActor(t, db)

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	studentID := chain.StudentID
	today := timezone.TodayDate()

	room := testpkg.CreateTestRoom(t, db, "Werkraum")
	activityGroup := testpkg.CreateTestActivityGroup(t, db, "Töpfern")

	// Two materialized blocks: one BEFORE the exit (history) and one after it.
	pastInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(-7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	openInstance := testpkg.CreateTestActivityInstance(t, db, today, room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	futureInstance := testpkg.CreateTestActivityInstance(t, db, today.AddDays(7), room.ID,
		testpkg.ActivityInstanceOpts{ActivityGroupID: &activityGroup.ID})
	testpkg.CreateTestInstanceStudent(t, db, pastInstance.ID, studentID, scheduleModels.AttendanceStatusPresent)
	checkedInAt := time.Now().Add(-time.Hour)
	testpkg.CreateTestInstanceStudent(t, db, openInstance.ID, studentID,
		scheduleModels.AttendanceStatusPresent, testpkg.InstanceStudentOpts{CheckedInAt: &checkedInAt})
	testpkg.CreateTestInstanceStudent(t, db, futureInstance.ID, studentID, scheduleModels.AttendanceStatusExpected)

	// A care offering that runs open-ended.
	booking := &activityModels.StudentEnrollment{
		StudentID:       studentID,
		ActivityGroupID: activityGroup.ID,
		ValidFrom:       today.AddDays(-60),
	}
	booking.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repos.StudentEnrollment.Create(ctx, booking))

	// A parent request nobody has decided yet.
	request := &activeModels.ExcusedAbsenceRequest{
		StudentID:     studentID,
		SubmittedBy:   chain.AccountID,
		Dates:         []timezone.Date{today.AddDays(3)},
		Note:          "Arzttermin",
		AbsenceStatus: "excused",
		Status:        activeModels.ExcusedRequestStatusPending,
	}
	request.SetTenantID(testpkg.Tenant(t))
	_, err := db.NewInsert().
		Model(request).
		ModelTableExpr("active.excused_absence_requests").
		Exec(ctx)
	require.NoError(t, err)

	preview, err := svc.Preview(ctx, userService.CareExitInput{
		StudentIDs:  []int64{studentID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonOther,
		ReasonNote:  "Wechsel an eine andere Schule",
	})
	require.NoError(t, err)
	require.Len(t, preview.Students, 1)
	impact := preview.Students[0]

	t.Run("the preview names every consequence before it happens", func(t *testing.T) {
		assert.Equal(t, 1, impact.PlannedRosterRows,
			"only the block AFTER the last care day counts")
		assert.Equal(t, 1, impact.ActivityBookings)
		assert.Equal(t, 1, impact.OpenParentRequests)
		assert.True(t, impact.CurrentlyPresent,
			"an open roster check-in is closed when the exit takes effect")
	})

	_, err = svc.Confirm(ctx, preview.Token, userService.CareExitInput{
		StudentIDs:  []int64{studentID},
		LastCareDay: today,
		Reason:      userModels.CareExitReasonOther,
		ReasonNote:  "Wechsel an eine andere Schule",
	}, actorID)
	require.NoError(t, err)

	t.Run("future roster rows go, past ones stay", func(t *testing.T) {
		future, err := repos.InstanceStudent.FindByInstanceID(ctx, futureInstance.ID)
		require.NoError(t, err)
		assert.Empty(t, future, "the departed child is off the upcoming block")

		past, err := repos.InstanceStudent.FindByInstanceID(ctx, pastInstance.ID)
		require.NoError(t, err)
		require.Len(t, past, 1, "the day that happened is frozen history")
		assert.Equal(t, studentID, past[0].StudentID)
	})

	t.Run("the offering ends at the last care day", func(t *testing.T) {
		stillToday, err := repos.StudentEnrollment.FindActiveByStudentIDs(ctx, []int64{studentID}, today)
		require.NoError(t, err)
		assert.Len(t, stillToday, 1)

		tomorrow, err := repos.StudentEnrollment.FindActiveByStudentIDs(ctx, []int64{studentID}, today.AddDays(1))
		require.NoError(t, err)
		assert.Empty(t, tomorrow)
	})

	t.Run("the open request survives until the exit takes effect", func(t *testing.T) {
		assert.Equal(t, activeModels.ExcusedRequestStatusPending,
			reloadRequestStatus(t, db, request.ID),
			"a request about a day the child is still in care stays decidable")
	})

	// --- the next morning -------------------------------------------------
	makeExitEffective(t, db, studentID, today.AddDays(-1))
	_, err = svc.ApplyDueEffects(ctx, today)
	require.NoError(t, err)

	t.Run("the open request is closed with its own outcome", func(t *testing.T) {
		assert.Equal(t, activeModels.ExcusedRequestStatusCareEnded,
			reloadRequestStatus(t, db, request.ID),
			"neither approved nor rejected — the care simply ended")
	})

	t.Run("the open roster check-in is closed", func(t *testing.T) {
		var checkedOutAt *time.Time
		err := db.NewSelect().
			TableExpr("schedule.instance_students").
			ColumnExpr("checked_out_at").
			Where("instance_id = ? AND student_id = ?", openInstance.ID, studentID).
			Scan(ctx, &checkedOutAt)
		require.NoError(t, err)
		assert.NotNil(t, checkedOutAt)
	})

	t.Run("the family keeps the request in their history", func(t *testing.T) {
		var count int
		err := db.NewSelect().
			TableExpr("active.excused_absence_requests").
			ColumnExpr("COUNT(*)").
			Where("id = ?", request.ID).
			Scan(context.Background(), &count)
		require.NoError(t, err)
		assert.Equal(t, 1, count, "nothing is deleted, only closed")
	})

	t.Run("the archive holds the child with its reason", func(t *testing.T) {
		rows, total, err := svc.ListEnded(ctx, userModels.CareExitListFilter{})
		require.NoError(t, err)
		require.Equal(t, 1, total)
		require.NotNil(t, rows[0].Reason)
		assert.Equal(t, userModels.CareExitReasonOther, *rows[0].Reason)
		require.NotNil(t, rows[0].ReasonNote)
		assert.Equal(t, "Wechsel an eine andere Schule", *rows[0].ReasonNote)
	})
}

func reloadRequestStatus(t *testing.T, db *bun.DB, requestID int64) string {
	t.Helper()
	var status string
	err := db.NewSelect().
		TableExpr("active.excused_absence_requests").
		ColumnExpr("status").
		Where("id = ?", requestID).
		Scan(context.Background(), &status)
	require.NoError(t, err)
	return status
}
