package compose_test

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Elternsprechtag (#4801). The calendar test clock stands on 2026-01-01;
// the day lies on 2026-03-04, well outside the 24-hour change window.

func setupParentConferenceService(t *testing.T, db *bun.DB, now time.Time) calendarSvc.FullService {
	t.Helper()
	cfg := calendarTestConfig(t, db)
	_, _, reposCalendar, _, _, _, _ := calendarTestRepositories(t, db)
	cfg.SchoolDays = reposCalendar.SchoolCalendar()
	cfg.ParentConferences = owners.BuildServeParentConferences(testpkg.NewDatabase(t, db))
	require.NotNil(t, cfg.ParentConferences)
	if !now.IsZero() {
		cfg.Now = func() time.Time { return now }
	}
	return owners.NewCalendarPortal(cfg)
}

// conferenceDay is 14:00 to 15:00 in 15-minute slots with a break from
// 14:30 to 14:45: the slots start at 14:00, 14:15 and 14:45.
func conferenceDay(class string) calendarSvc.ParentConferenceRequest {
	room := "Raum 12"
	return calendarSvc.ParentConferenceRequest{
		SchoolClass: class,
		Date:        calendar.NewDate(2026, 3, 4),
		StartTime:   "14:00",
		EndTime:     "15:00",
		SlotMinutes: 15,
		Breaks:      []calendarSvc.ParentConferenceWindow{{StartTime: "14:30", EndTime: "14:45"}},
		Location:    &room,
	}
}

func conferenceChild(chain testpkg.ParentChain) calendarSvc.ParentConferenceChild {
	return calendarSvc.ParentConferenceChild{TenantID: chain.TenantID, StudentID: chain.StudentID, GuardianProfileID: chain.GuardianProfileID}
}

func slotStarts(windows []calendarSvc.ParentConferenceWindow) []string {
	starts := make([]string, 0, len(windows))
	for _, window := range windows {
		starts = append(starts, window.StartTime)
	}
	return starts
}

func offerConference(conferenceID string) func(calendarSvc.ParentConferenceOffer) bool {
	return func(offer calendarSvc.ParentConferenceOffer) bool { return offer.ID == conferenceID }
}

func createConference(t *testing.T, db *bun.DB, service calendarSvc.FullService, accountID int64, req calendarSvc.ParentConferenceRequest) *calendarSvc.ParentConferenceView {
	t.Helper()
	view, err := testpkg.TenantQuery1(t, calendarContext(t, accountID), db, service.CreateSchoolParentConference, req)
	require.NoError(t, err)
	return view
}

func conferenceID(t *testing.T, view *calendarSvc.ParentConferenceView) int64 {
	t.Helper()
	id, err := parseInt64(view.ID)
	require.NoError(t, err)
	return id
}

func TestParentConferenceIntegration_LehrkraftOffersADayForOwnClassesOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Sprech", "Tag")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")

	view := createConference(t, db, service, account.ID, conferenceDay("1a"))
	assert.Equal(t, "1a", view.SchoolClass)
	require.Len(t, view.Slots, 3, "the break leaves three slots")
	assert.Equal(t, []string{"14:00", "14:15", "14:45"}, []string{view.Slots[0].StartTime, view.Slots[1].StartTime, view.Slots[2].StartTime})
	assert.Equal(t, "15:00", view.Slots[2].EndTime)

	_, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateSchoolParentConference, conferenceDay("2b"))
	require.ErrorIs(t, err, calendarSvc.ErrForbidden, "a class the Lehrkraft does not teach")

	past := conferenceDay("1a")
	past.Date = calendar.NewDate(2025, 12, 1)
	_, err = testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.CreateSchoolParentConference, past)
	require.ErrorIs(t, err, calendarSvc.ErrInvalidRequest, "a day in the past")

	_, otherAccount := createTestLehrkraft(t, db, "Andere", "Lehrkraft")
	_, err = testpkg.TenantQuery1(t, calendarContext(t, otherAccount.ID), db, service.GetSchoolParentConference, conferenceID(t, view))
	require.ErrorIs(t, err, calendarSvc.ErrNotFound, "another Lehrkraft's day is not hers to see")
}

func TestParentConferenceIntegration_FamilyBooksAFreeSlotThatShowsInBothCalendars(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Buch", "Ung")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	view := createConference(t, db, service, account.ID, conferenceDay("1a"))

	offers, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, service.ListChildParentConferences, conferenceChild(family))
	require.NoError(t, err)
	offer := findOffer(t, offers, view.ID)
	assert.Equal(t, []string{"14:00", "14:15", "14:45"}, slotStarts(offer.FreeSlots))
	assert.Equal(t, "Buch Ung", offer.TeacherName)
	assert.Nil(t, offer.Booking)

	booked, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), conferenceID(t, view), "14:15")
	require.NoError(t, err)
	require.NotNil(t, booked.Booking)
	assert.Equal(t, "14:15", booked.Booking.StartTime)
	assert.True(t, booked.CanChange)
	assert.Equal(t, []string{"14:00", "14:45"}, slotStarts(booked.FreeSlots), "a booked slot is gone from the free slots")

	occupancy, err := testpkg.TenantQuery1(t, calendarContext(t, account.ID), db, service.GetSchoolParentConference, conferenceID(t, view))
	require.NoError(t, err)
	assert.Equal(t, 1, occupancy.BookedCount)
	require.True(t, occupancy.Slots[1].Booked)
	require.NotNil(t, occupancy.Slots[1].StudentName)
	assert.Equal(t, "Felix Schneider", *occupancy.Slots[1].StudentName, "the Lehrkraft sees whom she meets")

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), family.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	parentEvent := findConferenceEvent(parentEvents)
	require.NotNil(t, parentEvent, "the booking shows in the parents' calendar")
	assert.Equal(t, "14:15", parentEvent.StartTime)
	assert.Equal(t, "14:30", parentEvent.EndTime)
	assert.Equal(t, "Elternsprechtag Klasse 1a", parentEvent.Title)

	schoolView, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.ListSchoolCalendar, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	schoolEvent := findConferenceEvent(schoolView.Events)
	require.NotNil(t, schoolEvent, "the booking shows in moto schule")
	assert.False(t, schoolEvent.CanEdit, "the booking changes through the Sprechtag only")
	_, err = testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.UpdateSchoolAppointment, mustParseInt64(t, *schoolEvent.AppointmentID), schoolTrip("Umgehung", calendarSvc.SchoolParentsNone))
	require.ErrorIs(t, err, calendarSvc.ErrForbidden)
}

func TestParentConferenceIntegration_ABookedSlotIsTakenForOthersAlsoAtTheSameMoment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Doppel", "Buchung")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	view := createConference(t, db, service, account.ID, conferenceDay("1a"))
	id := conferenceID(t, view)

	first := testpkg.CreateTestParentGuardianChain(t, db)
	second := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(first), id, "14:00")
	require.NoError(t, err)
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(second), id, "14:00")
	require.ErrorIs(t, err, calendarSvc.ErrParentConferenceSlotTaken)

	third := testpkg.CreateTestParentGuardianChain(t, db)
	fourth := testpkg.CreateTestParentGuardianChain(t, db)
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i, family := range []testpkg.ParentChain{third, fourth} {
		wg.Add(1)
		go func(i int, family testpkg.ParentChain) {
			defer wg.Done()
			_, results[i] = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:45")
		}(i, family)
	}
	wg.Wait()
	succeeded := 0
	for _, err := range results {
		if err == nil {
			succeeded++
			continue
		}
		require.ErrorIs(t, err, calendarSvc.ErrParentConferenceSlotTaken)
	}
	assert.Equal(t, 1, succeeded, "exactly one family gets the slot")

	var stored int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM calendar.parent_conference_bookings WHERE conference_id = ? AND slot_start_minute = ?`, id, 14*60+45).Scan(context.Background(), &stored))
	assert.Equal(t, 1, stored)
}

func TestParentConferenceIntegration_FamilyMovesAndCancelsUntil24HoursBefore(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Um", "Buchung")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("1a")))

	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:00")
	require.NoError(t, err)
	moved, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:45")
	require.NoError(t, err)
	assert.Equal(t, "14:45", moved.Booking.StartTime, "one slot per child: booking again moves it")
	assert.Equal(t, []string{"14:00", "14:15"}, slotStarts(moved.FreeSlots))

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), family.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	require.NotNil(t, findConferenceEvent(parentEvents))
	assert.Equal(t, "14:45", findConferenceEvent(parentEvents).StartTime, "the calendar follows the move")

	late := setupParentConferenceService(t, db, time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC))
	offers, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, late.ListChildParentConferences, conferenceChild(family))
	require.NoError(t, err)
	offer := findOffer(t, offers, formatInt64(id))
	assert.False(t, offer.CanChange, "less than 24 hours before the slot")
	assert.Empty(t, offer.FreeSlots, "slots within 24 hours are no longer offered")
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, late.BookParentConference, conferenceChild(family), id, "14:00")
	require.ErrorIs(t, err, calendarSvc.ErrParentConferenceChangeClosed)
	require.ErrorIs(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, late.CancelParentConferenceBooking, conferenceChild(family), id), calendarSvc.ErrParentConferenceChangeClosed)

	require.NoError(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, service.CancelParentConferenceBooking, conferenceChild(family), id))
	offers, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, service.ListChildParentConferences, conferenceChild(family))
	require.NoError(t, err)
	offer = findOffer(t, offers, formatInt64(id))
	assert.Nil(t, offer.Booking)
	assert.Len(t, offer.FreeSlots, 3)
	parentEvents, err = service.ListMyParentEvents(testpkg.Ctx(t), family.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	if event := findConferenceEvent(parentEvents); event != nil {
		assert.True(t, event.Cancelled, "a cancelled booking shows as cancelled")
	}
	require.ErrorIs(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, service.CancelParentConferenceBooking, conferenceChild(family), id), calendarSvc.ErrNotFound)
}

func TestParentConferenceIntegration_MoveChecksBothSlotDeadlines(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Frist", "Pruefung")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	child := conferenceChild(family)
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("1a")))

	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, child, id, "14:00")
	require.NoError(t, err)
	// At 13:16 UTC the next day's 14:00 Berlin slot is within 24 hours,
	// while 14:45 is still outside the change window.
	late := setupParentConferenceService(t, db, time.Date(2026, 3, 3, 13, 16, 0, 0, time.UTC))
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, late.BookParentConference, child, id, "14:45")
	require.ErrorIs(t, err, calendarSvc.ErrParentConferenceChangeClosed)

	offers, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, late.ListChildParentConferences, child)
	require.NoError(t, err)
	offer := findOffer(t, offers, formatInt64(id))
	require.NotNil(t, offer.Booking)
	assert.Equal(t, "14:00", offer.Booking.StartTime)
	require.ErrorIs(t, testpkg.TenantCommand2(t, testpkg.Ctx(t), db, late.CancelParentConferenceBooking, child, id), calendarSvc.ErrParentConferenceChangeClosed)
	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), family.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	event := findConferenceEvent(parentEvents)
	require.NotNil(t, event)
	assert.Equal(t, "14:00", event.StartTime)
	assert.False(t, event.Cancelled)

	// A destination inside the window is also forbidden even if the
	// existing booking remains changeable.
	other := testpkg.CreateTestParentGuardianChain(t, db)
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(other), id, "14:45")
	require.NoError(t, err)
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, late.BookParentConference, conferenceChild(other), id, "14:15")
	require.ErrorIs(t, err, calendarSvc.ErrParentConferenceChangeClosed)
}

func TestParentConferenceIntegration_ClassChangeCancelsExistingBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Klassen", "Wechsel")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "2b")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("1a")))
	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:00")
	require.NoError(t, err)

	// Equivalent spelling keeps the class and its booking.
	unchanged, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.UpdateSchoolParentConference, id, conferenceDay(" 1A "))
	require.NoError(t, err)
	assert.Equal(t, 1, unchanged.BookedCount)

	changed, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.UpdateSchoolParentConference, id, conferenceDay("2b"))
	require.NoError(t, err)
	assert.Equal(t, "2b", changed.SchoolClass)
	assert.Zero(t, changed.BookedCount)
	for _, slot := range changed.Slots {
		assert.False(t, slot.Booked)
	}
	var stored int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM calendar.parent_conference_bookings WHERE conference_id = ?`, id).Scan(context.Background(), &stored))
	assert.Zero(t, stored)

	parentEvents, err := service.ListMyParentEvents(testpkg.Ctx(t), family.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	event := findConferenceEvent(parentEvents)
	require.NotNil(t, event)
	assert.True(t, event.Cancelled, "changing the class cancels the old family's appointment")
}

func TestParentConferenceIntegration_AChildOfAnotherClassSeesAndBooksNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Andere", "Klasse")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "2b")
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("2b")))
	family := testpkg.CreateTestParentGuardianChain(t, db) // a child of class 1a

	offers, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, service.ListChildParentConferences, conferenceChild(family))
	require.NoError(t, err)
	for _, offer := range offers {
		assert.NotEqual(t, formatInt64(id), offer.ID, "a day of class 2b is not offered to class 1a")
	}
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:00")
	require.ErrorIs(t, err, calendarSvc.ErrNotFound)
}

func TestParentConferenceIntegration_ChangedOrDeletedDayFollowsItsBookings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Aend", "Erung")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("1a")))
	keeps := testpkg.CreateTestParentGuardianChain(t, db)
	loses := testpkg.CreateTestParentGuardianChain(t, db)
	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(keeps), id, "14:00")
	require.NoError(t, err)
	_, err = testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(loses), id, "14:45")
	require.NoError(t, err)

	shorter := conferenceDay("1a")
	shorter.EndTime = "14:30"
	room := "Aula"
	shorter.Location = &room
	shorter.Breaks = nil
	view, err := testpkg.TenantQuery2(t, calendarContext(t, account.ID), db, service.UpdateSchoolParentConference, id, shorter)
	require.NoError(t, err)
	assert.Equal(t, 1, view.BookedCount, "the 14:45 slot is gone and so is its booking")

	kept, err := service.ListMyParentEvents(testpkg.Ctx(t), keeps.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	keptEvent := findConferenceEvent(kept)
	require.NotNil(t, keptEvent)
	require.NotNil(t, keptEvent.Location)
	assert.Equal(t, "Aula", *keptEvent.Location, "the kept booking moves with the day")
	lost, err := service.ListMyParentEvents(testpkg.Ctx(t), loses.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	if lostEvent := findConferenceEvent(lost); lostEvent != nil {
		assert.True(t, lostEvent.Cancelled, "the removed booking is cancelled")
	}

	require.NoError(t, testpkg.TenantCommand1(t, calendarContext(t, account.ID), db, service.DeleteSchoolParentConference, id))
	afterDelete, err := service.ListMyParentEvents(testpkg.Ctx(t), keeps.AccountID, portalDate(2026, 3, 2), portalDate(2026, 3, 8))
	require.NoError(t, err)
	if event := findConferenceEvent(afterDelete); event != nil {
		assert.True(t, event.Cancelled, "deleting the day cancels its bookings")
	}
	var stored int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM calendar.parent_conference_bookings WHERE conference_id = ?`, id).Scan(context.Background(), &stored))
	assert.Zero(t, stored)
}

func TestParentConferenceIntegration_SchoolYearEndDeletesDaysBookingsAndAppointments(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	service := setupParentConferenceService(t, db, time.Time{})
	lehrkraft, account := createTestLehrkraft(t, db, "Schul", "Jahr")
	testpkg.CreateTestClassTeacher(t, db, lehrkraft.ID, "1a")
	family := testpkg.CreateTestParentGuardianChain(t, db)
	id := conferenceID(t, createConference(t, db, service, account.ID, conferenceDay("1a")))
	_, err := testpkg.TenantQuery3(t, testpkg.Ctx(t), db, service.BookParentConference, conferenceChild(family), id, "14:00")
	require.NoError(t, err)
	var appointmentID int64
	require.NoError(t, db.NewRaw(`SELECT appointment_id FROM calendar.parent_conference_bookings WHERE conference_id = ?`, id).Scan(context.Background(), &appointmentID))

	conferences := owners.BuildServeParentConferences(testpkg.NewDatabase(t, db))
	deleted, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, conferences.DeleteParentConferencesBefore, calendar.NewDate(2026, 3, 4))
	require.NoError(t, err)
	assert.Zero(t, deleted, "a day on the cutoff belongs to the running school year")

	deleted, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, conferences.DeleteParentConferencesBefore, calendar.NewDate(2026, 8, 1))
	require.NoError(t, err)
	assert.Equal(t, 2, deleted, "the day and its booking")

	appointmentOwner, err := owners.BuildServeAppointments(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	deleted, err = testpkg.TenantQuery1(t, testpkg.Ctx(t), db, appointmentOwner.DeleteParentConferenceAppointmentsBefore, appointments.NewDate(2026, 8, 1))
	require.NoError(t, err)
	assert.Equal(t, 1, deleted, "the booking's appointment")
	var remaining int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM calendar.appointments WHERE id = ?`, appointmentID).Scan(context.Background(), &remaining))
	assert.Zero(t, remaining, "the booking's appointment ends with the school year")
}

func findOffer(t *testing.T, offers []calendarSvc.ParentConferenceOffer, id string) calendarSvc.ParentConferenceOffer {
	t.Helper()
	for _, offer := range offers {
		if offerConference(id)(offer) {
			return offer
		}
	}
	require.Failf(t, "offer missing", "no offer for parent conference %s", id)
	return calendarSvc.ParentConferenceOffer{}
}

func findConferenceEvent(events []calendarSvc.Event) *calendarSvc.Event {
	for i := range events {
		if events[i].SchoolEventType != nil && *events[i].SchoolEventType == appointments.SchoolEventTypeParentConference {
			return &events[i]
		}
	}
	return nil
}

func parseInt64(value string) (int64, error) { return strconv.ParseInt(value, 10, 64) }

func mustParseInt64(t *testing.T, value string) int64 {
	t.Helper()
	id, err := parseInt64(value)
	require.NoError(t, err)
	return id
}

func formatInt64(value int64) string { return strconv.FormatInt(value, 10) }
