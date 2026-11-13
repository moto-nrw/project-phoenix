package compose_test

import (
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/appointments"
	calendarSvc "github.com/moto-ogs/project-phoenix/modules/schoolcalendar/portal"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func eventStatuses(events []calendarSvc.Event) map[string]string {
	statuses := map[string]string{}
	for _, event := range events {
		if event.Source == calendarSvc.EventSourceAppointment && event.ResponseStatus != nil {
			statuses[event.StartDate] = *event.ResponseStatus
		}
	}
	return statuses
}

// A parent who can come to the Elterncafé on one date but not on the next
// answers each date on its own; the OGS counts per date (#6987).
func TestCalendarServiceIntegration_AnswersForSingleDatesOfASeries(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Occurrence", "Invitee")
	parentChain := testpkg.CreateTestParentGuardianChain(t, db)

	endsOn := portalDate(2026, 12, 2)
	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:              "Elterncafé",
		StartDate:          portalDate(2026, 11, 18), // Wednesday
		EndDate:            portalDate(2026, 11, 18),
		StartTime:          wallClock(15, 0),
		EndTime:            wallClock(16, 0),
		DeliveryMode:       appointments.DeliveryModeRSVPRequired,
		OverviewVisibility: appointments.OverviewVisibilityAll,
		Recurrence: &calendarSvc.RecurrenceRequest{
			Frequency: appointments.RecurrenceFrequencyWeekly, IntervalCount: 1,
			Weekdays: []string{"wednesday"}, EndsOn: &endsOn,
		},
		Targets: []calendarSvc.AppointmentTarget{
			{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID},
			{Type: appointments.TargetTypeGuardianProfile, ID: &parentChain.GuardianProfileID},
		},
	})
	require.NoError(t, err)
	appointmentID := detail.Appointment.ID
	staffRecipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	guardianRecipientID := findRecipientByGuardian(t, detail, parentChain.GuardianProfileID)
	staffCtx := calendarContext(t, invitedAccount.ID)
	parentCtx := testpkg.Ctx(t)
	accepted, declined, pending := appointments.ResponseStatusAccepted, appointments.ResponseStatusDeclined, appointments.ResponseStatusPending

	// The series answer holds for every date, then one date gets its own.
	require.NoError(t, service.RespondToParentInvitation(parentCtx, parentChain.AccountID, guardianRecipientID, calendarSvc.InvitationAnswer{Status: accepted}))
	require.NoError(t, service.RespondToParentInvitation(parentCtx, parentChain.AccountID, guardianRecipientID, calendarSvc.InvitationAnswer{Status: declined, OccurrenceDate: portalDate(2026, 11, 25)}))
	require.NoError(t, testpkg.TenantCommand2(t, staffCtx, db, service.RespondToStaffInvitation, staffRecipientID, calendarSvc.InvitationAnswer{Status: accepted, OccurrenceDate: portalDate(2026, 12, 2)}))

	parentEvents, err := service.ListMyParentEvents(parentCtx, parentChain.AccountID, portalDate(2026, 11, 18), portalDate(2026, 12, 2))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"2026-11-18": accepted, "2026-11-25": declined, "2026-12-02": accepted}, eventStatuses(parentEvents))
	for _, event := range parentEvents {
		require.NotNil(t, event.SeriesResponseStatus)
		assert.Equal(t, accepted, *event.SeriesResponseStatus, "every date carries the series answer")
		assert.True(t, event.CanRespond, "a date with its own answer can be answered again")
	}
	staffEvents, err := testpkg.TenantQuery2(t, staffCtx, db, service.ListMyStaffEvents, portalDate(2026, 11, 18), portalDate(2026, 12, 2))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"2026-11-18": pending, "2026-11-25": pending, "2026-12-02": accepted}, eventStatuses(staffEvents))

	// The overview counts the answers of the date asked for.
	overview, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentOverview, appointmentID, portalDate(2026, 11, 25))
	require.NoError(t, err)
	assert.Equal(t, calendarSvc.AppointmentResponseCounts{Declined: 1, Pending: 1}, overview.Counts)
	assert.Equal(t, portalDate(2026, 11, 25), overview.OccurrenceDate)
	overview, err = testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentOverview, appointmentID, portalDate(2026, 12, 2))
	require.NoError(t, err)
	assert.Equal(t, calendarSvc.AppointmentResponseCounts{Accepted: 2}, overview.Counts)
	parentOverview, err := service.GetParentAppointmentOverview(parentCtx, parentChain.AccountID, appointmentID, portalDate(2026, 11, 25))
	require.NoError(t, err)
	assert.Equal(t, calendarSvc.AppointmentResponseCounts{Declined: 1, Pending: 1}, parentOverview.Counts)
	seriesOverview, err := testpkg.TenantQuery2(t, calendarContext(t, organizerAccount.ID), db, service.GetStaffAppointmentOverview, appointmentID, calendarSvc.Date(""))
	require.NoError(t, err)
	assert.Equal(t, calendarSvc.AppointmentResponseCounts{Accepted: 1, Pending: 1}, seriesOverview.Counts, "without a date the overview counts the series answers")

	// A date the series does not have, or a cancelled one, takes no answer.
	err = service.RespondToParentInvitation(parentCtx, parentChain.AccountID, guardianRecipientID, calendarSvc.InvitationAnswer{Status: accepted, OccurrenceDate: portalDate(2026, 11, 19)})
	assert.ErrorIs(t, err, calendarSvc.ErrInvalidRequest)
	require.NoError(t, testpkg.TenantCommand2(t, calendarContext(t, organizerAccount.ID), db, service.CancelStaffAppointmentOccurrence, appointmentID, portalDate(2026, 12, 2)))
	err = testpkg.TenantCommand2(t, staffCtx, db, service.RespondToStaffInvitation, staffRecipientID, calendarSvc.InvitationAnswer{Status: declined, OccurrenceDate: portalDate(2026, 12, 2)})
	assert.ErrorIs(t, err, calendarSvc.ErrInvalidRequest)

	// A new answer for the whole series holds for every date again.
	require.NoError(t, service.RespondToParentInvitation(parentCtx, parentChain.AccountID, guardianRecipientID, calendarSvc.InvitationAnswer{Status: declined}))
	parentEvents, err = service.ListMyParentEvents(parentCtx, parentChain.AccountID, portalDate(2026, 11, 18), portalDate(2026, 11, 25))
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"2026-11-18": declined, "2026-11-25": declined}, eventStatuses(parentEvents))
}

// An appointment that does not recur takes its own date as an answer for the
// whole appointment and rejects any other date (#6987).
func TestCalendarServiceIntegration_SingleAppointmentAnswersWithItsDate(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	service := setupCalendarService(t, db)
	_, organizerAccount := testpkg.CreateTestCalendarStaff(t, db, "Single", "Organizer")
	invitedStaff, invitedAccount := testpkg.CreateTestCalendarStaff(t, db, "Single", "Invitee")

	detail, err := testpkg.TenantQuery1(t, calendarContext(t, organizerAccount.ID), db, service.CreateStaffAppointment, calendarSvc.CreateAppointmentRequest{
		Title:        "Teamsitzung",
		StartDate:    portalDate(2026, 11, 18),
		EndDate:      portalDate(2026, 11, 18),
		StartTime:    wallClock(15, 0),
		EndTime:      wallClock(16, 0),
		DeliveryMode: appointments.DeliveryModeRSVPRequired,
		Targets:      []calendarSvc.AppointmentTarget{{Type: appointments.TargetTypeStaff, ID: &invitedStaff.ID}},
	})
	require.NoError(t, err)
	recipientID := findRecipientByStaff(t, detail, invitedStaff.ID)
	staffCtx := calendarContext(t, invitedAccount.ID)

	err = testpkg.TenantCommand2(t, staffCtx, db, service.RespondToStaffInvitation, recipientID, calendarSvc.InvitationAnswer{Status: appointments.ResponseStatusDeclined, OccurrenceDate: portalDate(2026, 11, 25)})
	assert.ErrorIs(t, err, calendarSvc.ErrInvalidRequest)
	require.NoError(t, testpkg.TenantCommand2(t, staffCtx, db, service.RespondToStaffInvitation, recipientID, calendarSvc.InvitationAnswer{Status: appointments.ResponseStatusDeclined, OccurrenceDate: portalDate(2026, 11, 18)}))

	events, err := testpkg.TenantQuery2(t, staffCtx, db, service.ListMyStaffEvents, portalDate(2026, 11, 18), portalDate(2026, 11, 18))
	require.NoError(t, err)
	require.Len(t, events, 1)
	assert.Equal(t, appointments.ResponseStatusDeclined, *events[0].ResponseStatus)
	assert.Equal(t, appointments.ResponseStatusDeclined, *events[0].SeriesResponseStatus, "the answer holds for the appointment itself")
}
