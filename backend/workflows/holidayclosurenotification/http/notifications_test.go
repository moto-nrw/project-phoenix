package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	schoolCalendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	notification "github.com/moto-nrw/project-phoenix/workflows/holidayclosurenotification"
)

// The notices after a school closing day changed (#6442, #6457, ADR 0196).
// The booked week of bookHolidayWeek runs from Monday 2030-07-08 to Friday
// 2030-07-12, so every day of these tests is in the future.

// bookHolidayWeek books one child of the family email into a week of a
// published period of the Träger at school.
func bookHolidayWeek(t *testing.T, db *bun.DB, organization, school int64, email string) {
	t.Helper()
	ctx := context.Background()
	var period, week, model, registration, child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_periods (organization_id, name, registration_opens_on,
			registration_closes_on, status, published_at) VALUES (?, 'Sommerferien', '2030-01-01', '2030-06-01', 'published', NOW())
		RETURNING id`, organization).Scan(ctx, &period))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_period_weeks (organization_id, holiday_period_id, starts_on, ends_on, host_school_id)
		VALUES (?, ?, '2030-07-08', '2030-07-12', ?) RETURNING id`, organization, period, school).Scan(ctx, &week))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_week_time_models (organization_id, holiday_period_week_id, name, starts_at, ends_at, price_cents)
		VALUES (?, ?, 'Ganztag', '08:00', '16:00', 4500) RETURNING id`, organization, week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Ferien', ?, '0123', NOW()) RETURNING id`, organization, period, email).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Mia', 'Ferien', '2019-03-04') RETURNING id`, organization, period, registration).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id) VALUES (?, ?, ?, ?, ?)`,
		organization, period, child, week, model)
	require.NoError(t, err)
}

// closureWorld is a Träger with one school, two families who booked its
// holiday week, the school's calendar and the workflow over real owners.
type closureWorld struct {
	db           *bun.DB
	ctx          context.Context
	organization int64
	school       int64
	records      schoolcalendar.Days
	notices      notification.Notifications
	closures     enrollment.HolidayClosures
	mail         owners.HolidayClosureMail
}

// newClosureWorld builds the world on the package's shared database. A test
// that runs the Worker's ProcessDue, which leases the due commands of every
// Träger, builds it on a clone of its own, so it takes no other test's
// command away.
func newClosureWorld(t *testing.T, name string) closureWorld {
	t.Helper()
	return newClosureWorldIn(t, name, testpkg.SetupTestDB(t))
}

func newClosureWorldIn(t *testing.T, name string, db *bun.DB) closureWorld {
	t.Helper()
	database, unit := testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: name, Slug: fmt.Sprintf("schliesstag-hinweis-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	testpkg.OwnTraegerHolidayPeriods(t, db, organization)
	testpkg.OwnTraegerClosingDays(t, db, organization)
	school := testpkg.CreateTestSchoolInOrganization(t, db, organization, name+" OGS")
	bookHolidayWeek(t, db, organization, school, "familie.eins@example.org")
	bookHolidayWeek(t, db, organization, school, "familie.zwei@example.org")
	settings, err := owners.NewSettingsTestModule(database, unit)
	require.NoError(t, err)
	records, err := owners.BuildSchoolCalendarRecords(database, func(schoolCalendarCompose.Observation) {}, settings.Settings)
	require.NoError(t, err)
	organizations, err := owners.NewOrganizationTenancy(database)
	require.NoError(t, err)
	delivery, err := deliveryCompose.NewModule(deliveryCompose.ModuleDependencies{
		DB: db, People: owners.EnrollmentFlowGuardianDirectory{}, Observe: func(deliveryContract.Observation) {},
	})
	require.NoError(t, err)
	mail := owners.HolidayClosureMail{Delivery: delivery.Module, TraegerMails: delivery.TraegerMails}
	closures := owners.NewHolidayClosures(owners.NewHolidaySchoolCalendar(settings.Settings, records, organizations), records,
		owners.NewHolidayOfficeTraeger(organizations), mail)
	return closureWorld{
		db: db, ctx: tenant.WithUnitOfWork(context.Background(), unit), organization: organization, school: school, records: records,
		notices: owners.NewHolidayClosureNotifications(closures, organizations, mail, slog.Default()), closures: closures, mail: mail,
	}
}

func (w closureWorld) schoolCtx() context.Context { return tenant.WithTenantID(w.ctx, w.school) }

func (w closureWorld) createClosingDay(t *testing.T, start, end string) schoolcalendar.ClosingDay {
	t.Helper()
	day, err := w.records.CreateClosingDay(w.schoolCtx(), schoolcalendar.CreateClosingDay{ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: start, EndDate: end, Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	return day
}

func (w closureWorld) schoolNotices(t *testing.T) []notification.Notice {
	t.Helper()
	notices, err := w.notices.SchoolNotices(w.ctx, w.school)
	require.NoError(t, err)
	return notices
}

// mails reads the closing day mails of the Träger with the days of each
// section, ordered by recipient.
type closureMailRow struct {
	Recipient string          `bun:"recipient"`
	Status    string          `bun:"status"`
	Payload   json.RawMessage `bun:"payload"`
}

func (w closureWorld) mails(t *testing.T) []closureMailRow {
	t.Helper()
	var rows []closureMailRow
	require.NoError(t, w.db.NewRaw(`SELECT payload->>'recipient_email' AS recipient, status, payload FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_closure_notice' ORDER BY payload->>'recipient_email', id`, w.organization).Scan(context.Background(), &rows))
	return rows
}

func (w closureWorld) events(t *testing.T) []string {
	t.Helper()
	var states []string
	require.NoError(t, w.db.NewRaw(`SELECT state FROM calendar.holiday_closure_events WHERE organization_id = ? ORDER BY id`, w.organization).
		Scan(context.Background(), &states))
	return states
}

// Invariants 1 to 3 of ADR 0196: saving a school closing day sends nothing,
// neither does the worker; the school sees the proposal with the number of
// families a click tells, and nothing else of them; one click tells each
// family once, a click of the Büro afterwards finds the same command.
func TestSchoolClosingDayTellsParentsOnlyAfterOneClick(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Hinweis Klick", testpkg.SetupIsolatedTestDB(t)) // ProcessDue sweeps every Träger
	w.createClosingDay(t, "2030-08-20", "2030-08-20")
	assert.Empty(t, w.schoolNotices(t), "no booked week, no family: no proposal")
	w.createClosingDay(t, "2030-07-10", "2030-07-10")
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Empty(t, w.mails(t), "neither the save nor the worker sends without a click")
	assert.Empty(t, w.events(t), "a proposal is no command")

	notices := w.schoolNotices(t)
	require.Len(t, notices, 1)
	notice := notices[0]
	assert.Equal(t, string(schoolcalendar.HolidayClosureSuggested), notice.Status)
	assert.True(t, notice.CanNotify)
	assert.Equal(t, 2, notice.Families, "the school sees how many families a click tells")
	raw, err := json.Marshal(notice)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	assert.ElementsMatch(t, []string{"id", "school_id", "closing_day_id", "reason", "previous", "current", "revision", "status", "changed",
		"can_notify", "families", "token", "confirmed_at", "updated_at"}, keysOf(fields), "the school gets no contact, name or child of the Träger")
	assert.NotContains(t, string(raw), "example.org")

	confirmed, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	assert.NotEqual(t, string(schoolcalendar.HolidayClosureSuggested), confirmed.Status)
	mails := w.mails(t)
	require.Len(t, mails, 2, "one mail per family")
	assert.Equal(t, notice.Families, len(mails), "the count the school saw is the number of mails")

	_, err = w.notices.NotifyFromTraeger(w.ctx, w.organization, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Len(t, w.mails(t), 2, "a second click of the other place sends nothing twice")
	assert.Len(t, w.events(t), 1, "both clicks share one command")

	// An overlapping closing day tells only the day nobody told yet.
	w.createClosingDay(t, "2030-07-10", "2030-07-11")
	var overlap notification.Notice
	for _, notice := range w.schoolNotices(t) {
		if notice.Status == string(schoolcalendar.HolidayClosureSuggested) {
			overlap = notice
		}
	}
	require.NotZero(t, overlap.ID)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, overlap.ID, overlap.Revision, overlap.Token)
	require.NoError(t, err)
	mails = w.mails(t)
	require.Len(t, mails, 4)
	var payload enrollment.HolidayClosureMail
	require.NoError(t, json.Unmarshal(mails[1].Payload, &payload))
	require.Len(t, payload.Children, 1)
	assert.Equal(t, []string{"Donnerstag, 11.07.2030"}, payload.Children[0].Weeks[0].ClosedDays, "a day told before is not told again")
}

// Invariant 1 under load: the school's and the Büro's click at the same
// time write one command and one mail per family.
func TestConcurrentClicksOfSchoolAndBueroSendOneMailPerFamily(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Hinweis gleichzeitig", testpkg.SetupIsolatedTestDB(t)) // ProcessDue sweeps every Träger
	w.createClosingDay(t, "2030-07-09", "2030-07-10")
	notice := w.schoolNotices(t)[0]
	var wg sync.WaitGroup
	errs := make([]error, 4)
	for i := range 4 {
		wg.Go(func() {
			if i%2 == 0 {
				_, errs[i] = w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
			} else {
				_, errs[i] = w.notices.NotifyFromTraeger(w.ctx, w.organization, 0, notice.ID, notice.Revision, notice.Token)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		require.NoError(t, err)
	}
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Len(t, w.events(t), 1, "one command")
	assert.Len(t, w.mails(t), 2, "one mail per family, however many clicks")
}

// Invariant 4: a known transport failure shows as failed at both places; a
// click takes the same mail up again and leaves the family that got its
// mail alone. An unknown outcome is not repeated blindly.
func TestFailedNoticeIsTakenUpAgainWithoutASecondMail(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Hinweis nachholen", testpkg.SetupIsolatedTestDB(t)) // ProcessDue sweeps every Träger
	w.createClosingDay(t, "2030-07-11", "2030-07-11")
	notice := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	mails := w.mails(t)
	require.Len(t, mails, 2)
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW()
		WHERE organization_id = ? AND kind = 'holiday_closure_notice' AND payload->>'recipient_email' = ?`, w.organization, mails[0].Recipient)
	require.NoError(t, err)
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter', dead_letter_at = NOW(),
			last_error = 'mailbox unavailable'
		WHERE organization_id = ? AND kind = 'holiday_closure_notice' AND payload->>'recipient_email' = ?`, w.organization, mails[1].Recipient)
	require.NoError(t, err)

	require.NoError(t, w.notices.ProcessDue(w.ctx))
	failed := w.schoolNotices(t)
	require.Len(t, failed, 1)
	assert.Equal(t, string(schoolcalendar.HolidayClosureFailed), failed[0].Status, "the school sees the failure")
	assert.True(t, failed[0].CanNotify, "and can take it up again")
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	require.Len(t, pending.Notices, 1)
	assert.Equal(t, string(schoolcalendar.HolidayClosureFailed), pending.Notices[0].Status, "so does the Büro")

	_, err = w.notices.NotifyFromTraeger(w.ctx, w.organization, 0, notice.ID, notice.Revision, notice.Token)
	require.NoError(t, err)
	after := w.mails(t)
	require.Len(t, after, 2, "the repetition is the same mail, not a new one")
	assert.Equal(t, "sent", after[0].Status, "the family that got its mail is left alone")
	assert.Equal(t, "pending", after[1].Status, "the failed mail is due again")

	// An unknown outcome stays: no blind repetition.
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter', dead_letter_at = NOW(), last_error = ?
		WHERE organization_id = ? AND kind = 'holiday_closure_notice' AND payload->>'recipient_email' = ?`,
		deliveryContract.DispatchOutcomeUnknown, w.organization, mails[1].Recipient)
	require.NoError(t, err)
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	unknown := w.schoolNotices(t)
	require.Len(t, unknown, 1)
	assert.Equal(t, string(schoolcalendar.HolidayClosureUnknown), unknown[0].Status)
	assert.False(t, unknown[0].CanNotify, "an unknown outcome is checked first")
}

// #6457 and invariant 5: moving a told closing day proposes one mail per
// family with the days closed now and the days open again; deleting it
// proposes the days open again. Only the click sends.
func TestMovedAndDeletedClosingDayTellOnceWithBothSections(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Hinweis verschoben")
	day := w.createClosingDay(t, "2030-07-10", "2030-07-10")
	first := w.schoolNotices(t)[0]
	_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, first.ID, first.Revision, first.Token)
	require.NoError(t, err)
	require.Len(t, w.mails(t), 2)

	_, err = w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-11", EndDate: "2030-07-11", Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	assert.Len(t, w.mails(t), 2, "the change itself sends nothing")
	var moved notification.Notice
	for _, notice := range w.schoolNotices(t) {
		if notice.Status == string(schoolcalendar.HolidayClosureSuggested) {
			moved = notice
		}
	}
	require.NotZero(t, moved.ID)
	require.NotNil(t, moved.Previous)
	require.NotNil(t, moved.Current)
	assert.Equal(t, "2030-07-10", moved.Previous.StartDate.String())
	assert.Equal(t, "2030-07-11", moved.Current.StartDate.String())
	assert.Equal(t, 2, moved.Families)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, moved.ID, moved.Revision, moved.Token)
	require.NoError(t, err)
	mails := w.mails(t)
	require.Len(t, mails, 4, "one more mail per family, not two")
	var payload enrollment.HolidayClosureMail
	require.NoError(t, json.Unmarshal(mails[1].Payload, &payload))
	require.Len(t, payload.Children, 1)
	require.Len(t, payload.ReopenedChildren, 1)
	assert.Equal(t, []string{"Donnerstag, 11.07.2030"}, payload.Children[0].Weeks[0].ClosedDays, "Diese Tage sind geschlossen")
	assert.Equal(t, []string{"Mittwoch, 10.07.2030"}, payload.ReopenedChildren[0].Weeks[0].ClosedDays, "Diese Tage sind doch geöffnet")

	require.NoError(t, w.records.DeleteClosingDay(w.schoolCtx(), day.ID))
	var deleted notification.Notice
	for _, notice := range w.schoolNotices(t) {
		if notice.Status == string(schoolcalendar.HolidayClosureSuggested) {
			deleted = notice
		}
	}
	require.NotZero(t, deleted.ID, "a deleted closing day keeps its proposal")
	assert.Nil(t, deleted.Current)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, deleted.ID, deleted.Revision, deleted.Token)
	require.NoError(t, err)
	mails = w.mails(t)
	require.Len(t, mails, 6)
	require.NoError(t, json.Unmarshal(mails[2].Payload, &payload))
	assert.Empty(t, payload.Children)
	require.Len(t, payload.ReopenedChildren, 1)
	assert.Equal(t, []string{"Donnerstag, 11.07.2030"}, payload.ReopenedChildren[0].Weeks[0].ClosedDays)

	// Closing the day once more after it opened again is a new notice.
	w.createClosingDay(t, "2030-07-11", "2030-07-11")
	again := w.schoolNotices(t)
	var reclosed notification.Notice
	for _, notice := range again {
		if notice.Status == string(schoolcalendar.HolidayClosureSuggested) {
			reclosed = notice
		}
	}
	assert.Equal(t, 2, reclosed.Families, "the old notice does not stop a new one")
}

// A click on a proposal whose closing day changed since the caller saw it
// confirms nothing; the caller reloads.
func TestStaleClickConfirmsNothing(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Hinweis veraltet")
	day := w.createClosingDay(t, "2030-07-10", "2030-07-10")
	seen := w.schoolNotices(t)[0]
	_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-09", EndDate: "2030-07-10", Reason: "Pädagogische Tage",
	}})
	require.NoError(t, err)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, seen.ID, seen.Revision, seen.Token)
	require.ErrorIs(t, err, notification.ErrNoticeChanged)
	assert.Empty(t, w.events(t))
	assert.Empty(t, w.mails(t))
}

// A Büro of another Träger neither sees nor confirms the school's notice.
func TestForeignBueroCannotConfirmANotice(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Hinweis eigen")
	other := newClosureWorld(t, "Träger Hinweis fremd")
	w.createClosingDay(t, "2030-07-10", "2030-07-10")
	notice := w.schoolNotices(t)[0]
	pending, err := other.notices.PendingHolidayClosures(other.ctx, other.organization)
	require.NoError(t, err)
	assert.Empty(t, pending.Notices)
	_, err = w.notices.NotifyFromTraeger(w.ctx, other.organization, 0, notice.ID, notice.Revision, notice.Token)
	require.ErrorIs(t, err, notification.ErrNoticeNotFound)
	assert.Empty(t, w.events(t))
}

// Point 25 of the brief rules: every Träger and school filter of the
// notices holds beneath row security. In the units of A, as the owner of the
// tables, each statement is asked for B's rows; each assertion goes red when
// its explicit filter is dropped.
func TestHolidayClosureNoticesKeepTheirFiltersBeneathRLS(t *testing.T) {
	t.Parallel()
	own := newClosureWorld(t, "Träger Hinweis RLS A")
	foreign := newClosureWorld(t, "Träger Hinweis RLS B")
	own.createClosingDay(t, "2030-07-10", "2030-07-10")
	foreign.createClosingDay(t, "2030-07-10", "2030-07-10")
	foreignNotice := foreign.schoolNotices(t)[0]
	_, err := foreign.notices.NotifyFromSchool(foreign.ctx, foreign.school, 0, foreignNotice.ID, foreignNotice.Revision, foreignNotice.Token)
	require.NoError(t, err)
	require.NotEmpty(t, foreign.mails(t))
	_, err = foreign.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter', dead_letter_at = NOW(),
			last_error = 'mailbox unavailable' WHERE organization_id = ? AND kind = 'holiday_closure_notice'`, foreign.organization)
	require.NoError(t, err)
	_, err = foreign.db.ExecContext(context.Background(), `UPDATE calendar.holiday_closure_events SET state = 'pending', next_attempt_at = NOW()
		WHERE organization_id = ?`, foreign.organization)
	require.NoError(t, err)
	var foreignOperation string
	var foreignKeys []string
	require.NoError(t, foreign.db.NewRaw(`SELECT operation_id::text FROM calendar.holiday_closure_events WHERE organization_id = ?`, foreign.organization).
		Scan(context.Background(), &foreignOperation))
	require.NoError(t, foreign.db.NewRaw(`SELECT idempotency_key FROM enrollment.holiday_closure_sends WHERE organization_id = ?`, foreign.organization).
		Scan(context.Background(), &foreignKeys))
	events := schoolCalendarCompose.NewHolidayClosureEvents()

	ownSchool, err := tenant.NewTenantID(own.school)
	require.NoError(t, err)
	require.NoError(t, tenant.WithinTenant(own.ctx, ownSchool, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		notices, err := events.SchoolHolidayClosureNotices(ctx)
		require.NoError(t, err)
		for _, notice := range notices {
			assert.Equal(t, own.school, notice.SchoolID, "the school reads no notice of another school")
		}
		_, err = events.ConfirmHolidayClosureNotice(ctx, foreignNotice.ID, foreignNotice.Revision,
			schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool})
		require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureNoticeNotFound, "nor confirms one")
		return nil
	}))
	require.NoError(t, testpkg.WithinTraeger(own.ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		notices, err := events.TraegerHolidayClosureNotices(ctx, own.organization)
		require.NoError(t, err)
		require.NotEmpty(t, notices)
		for _, notice := range notices {
			assert.Equal(t, own.organization, notice.OrganizationID, "A reads no notice of B")
		}
		_, leased, err := events.LeaseHolidayClosureEvent(ctx, own.organization, time.Minute)
		require.NoError(t, err)
		assert.False(t, leased, "A leases no command of B")
		calendars, err := events.HolidayClosureCalendars(ctx, own.organization, []int64{foreign.school}, calendar.NewDate(2030, 7, 8), calendar.NewDate(2030, 7, 12))
		require.NoError(t, err)
		assert.Empty(t, calendars, "A reads no calendar of B's school")
		retired, err := events.RetireHolidayClosureEvents(ctx, own.organization)
		require.NoError(t, err)
		assert.Zero(t, retired)
		delivery, err := own.closures.HolidayClosureOperationsDelivery(ctx, own.organization, []string{foreignOperation})
		require.NoError(t, err)
		assert.Zero(t, delivery[foreignOperation], "A reads no send record of B")
		retried, err := own.mail.RetryFailedTraegerEmails(ctx, own.organization, foreignKeys)
		require.NoError(t, err)
		assert.Zero(t, retried, "A takes up no mail of B")
		statuses, err := own.mail.TraegerEmailStatusesByKey(ctx, own.organization, foreignKeys)
		require.NoError(t, err)
		assert.Empty(t, statuses, "A reads no mail of B")
		return nil
	}))
	var states []string
	require.NoError(t, foreign.db.NewRaw(`SELECT state FROM calendar.holiday_closure_events WHERE organization_id = ?`, foreign.organization).
		Scan(context.Background(), &states))
	assert.Equal(t, []string{"pending"}, states, "B's command stays untouched")
	var dead int
	require.NoError(t, foreign.db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE organization_id = ? AND status = 'dead_letter'`, foreign.organization).
		Scan(context.Background(), &dead))
	assert.Equal(t, len(foreignKeys), dead, "B's failed mails stay as they are")
	require.NotEmpty(t, foreignOperation)
}

func keysOf(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

// A different source may change the calendar without superseding the proposal.
func TestConfirmedNoticeRequiresTheSameCalendarRevision(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Kalenderstand bestätigen", testpkg.SetupIsolatedTestDB(t))
	day := w.createClosingDay(t, "2030-07-08", "2030-07-08")
	// Widen the command's span by moving Monday to Friday before confirmation.
	_, err := w.records.UpdateClosingDay(w.schoolCtx(), schoolcalendar.UpdateClosingDay{ID: day.ID, ClosingDayFields: schoolcalendar.ClosingDayFields{
		StartDate: "2030-07-12", EndDate: "2030-07-12", Reason: "Pädagogischer Tag",
	}})
	require.NoError(t, err)
	seen := w.schoolNotices(t)[0]
	proposals := schoolCalendarCompose.NewHolidayClosureEvents()
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := proposals.ConfirmHolidayClosureNotice(ctx, seen.ID, seen.Revision,
			schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: seen.Token})
		return err
	}))
	w.createClosingDay(t, "2030-07-09", "2030-07-09")
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Empty(t, w.mails(t), "a confirmed Friday must not approve an unconfirmed Tuesday")
	assert.Equal(t, []string{schoolcalendar.HolidayClosureEventNeedsConfirmation}, w.events(t))
	notices := w.schoolNotices(t)
	require.Len(t, notices, 2, "the stale command must remain available for renewed confirmation")
	var renewed notification.Notice
	for _, notice := range notices {
		if notice.ID == seen.ID {
			renewed = notice
		}
	}
	require.Equal(t, "suggested", renewed.Status)
	assert.True(t, renewed.Changed)
	// Model a membership change between the Büro's check and the school unit.
	other := testpkg.CreateTestOrganization(t, w.db, &testpkg.Organization{
		Name: "Neuer Träger", Slug: fmt.Sprintf("neuer-traeger-%d", w.school), Active: true,
	})
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, other.ID, w.school)
	require.NoError(t, err)
	err = tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := proposals.ConfirmHolidayClosureNotice(ctx, renewed.ID, renewed.Revision,
			schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelTraeger, OrganizationID: w.organization, ImpactToken: renewed.Token})
		return err
	})
	require.ErrorIs(t, err, schoolcalendar.ErrHolidayClosureNoticeNotFound, "the old Büro must not confirm a replacement stamped for the new Träger")
	_, err = w.db.ExecContext(context.Background(), `UPDATE platform.schools SET organization_id = ? WHERE id = ?`, w.organization, w.school)
	require.NoError(t, err)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, renewed.ID, renewed.Revision, renewed.Token)
	require.NoError(t, err)
	require.Len(t, w.mails(t), 2, "renewed confirmation can prepare the current calendar")
}

func TestIndividualNoticeRejectsChangedRecipients(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Empfänger bestätigen")
	w.createClosingDay(t, "2030-07-10", "2030-07-10")
	seen := w.schoolNotices(t)[0]
	_, err := w.db.ExecContext(context.Background(), `UPDATE enrollment.holiday_registrations
  SET guardian_email = 'andere.familie@example.org' WHERE organization_id = ? AND guardian_email = 'familie.eins@example.org'`, w.organization)
	require.NoError(t, err)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, seen.ID, seen.Revision, seen.Token)
	require.ErrorIs(t, err, notification.ErrNoticeChanged)
	_, err = w.notices.NotifyFromTraeger(w.ctx, w.organization, 0, seen.ID, seen.Revision, seen.Token)
	require.ErrorIs(t, err, notification.ErrNoticeChanged)
	assert.Empty(t, w.events(t), "neither click confirms changed recipients")
	assert.Empty(t, w.mails(t))
}

// The school's list plans its notices that wait for a click together: more
// waiting notices read no further statement.
func TestSchoolNoticesPlanWaitingNoticesTogether(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Hinweis Budget")
	w.createClosingDay(t, "2030-07-09", "2030-07-09")
	counter := testpkg.CaptureQueriesForContext(t, w.db)
	read := func(want int) testpkg.QueryMeasurement {
		t.Helper()
		counter.Reset()
		notices, err := w.notices.SchoolNotices(counter.Context(w.ctx), w.school)
		require.NoError(t, err)
		require.Len(t, notices, want)
		return counter.Measurement(counter.Queries())
	}
	one := read(1)
	w.createClosingDay(t, "2030-07-10", "2030-07-10")
	w.createClosingDay(t, "2030-07-12", "2030-07-12")
	three := read(3)
	assert.Len(t, three.Statements, len(one.Statements), "three waiting notices read the statements of one")
	t.Logf("school notices: %d statements", len(three.Statements))
	querybudgets.AssertQueryBudget(t, "workflows.holidayclosurenotification.school_notices", three)
}

// The Büro's one click for all waiting notices confirms each notice of the
// list it saw once, as its own command; a list that changed since confirms
// nothing.
func TestBueroConfirmsAllWaitingNoticesOnce(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger Hinweis Alle")
	w.createClosingDay(t, "2030-07-09", "2030-07-09")
	w.createClosingDay(t, "2030-07-11", "2030-07-11")
	pending, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	require.Len(t, pending.Notices, 2)
	stale := pending.Token

	w.createClosingDay(t, "2030-07-12", "2030-07-12")
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, stale)
	require.ErrorIs(t, err, notification.ErrNoticeChanged, "a changed list confirms nothing")
	assert.Empty(t, w.events(t))

	pending, err = w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	result, err := w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.NoError(t, err)
	assert.Equal(t, notification.NotifiedPending{Notices: 3, Families: 6}, result)
	assert.Len(t, w.events(t), 3, "one command per notice")
	// A later notice to the same family waits for the earlier mail, so the
	// mails of the second and third command follow once it is sent.
	sent := len(w.mails(t))
	assert.Positive(t, sent, "the click starts the sending")
	assert.LessOrEqual(t, sent, 6, "at most one mail per family and command")

	again, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	for _, notice := range again.Notices {
		assert.NotEqual(t, string(schoolcalendar.HolidayClosureSuggested), notice.Status, "nothing waits any more")
	}
	_, err = w.notices.NotifyPendingHolidayClosures(w.ctx, w.organization, 0, pending.Token)
	require.ErrorIs(t, err, notification.ErrNoticeChanged, "the same click again confirms nothing twice")
	assert.Len(t, w.events(t), 3)
	assert.Len(t, w.mails(t), sent, "and sends nothing twice")
}

// Following prepared mail must cost the same statements for one or many commands.
func TestWaitingHolidayClosureDeliveryIsBatched(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger Zustellstand gesammelt", testpkg.SetupIsolatedTestDB(t))
	counter := testpkg.CaptureQueries(t, w.db)
	var counts []int
	for _, date := range []string{"2030-07-08", "2030-07-09", "2030-07-10"} {
		w.createClosingDay(t, date, date)
		for _, notice := range w.schoolNotices(t) {
			if notice.Status == "suggested" {
				_, err := w.notices.NotifyFromSchool(w.ctx, w.school, 0, notice.ID, notice.Revision, notice.Token)
				require.NoError(t, err)
			}
		}
		counter.Reset()
		require.NoError(t, w.notices.ProcessDue(w.ctx))
		counts = append(counts, len(counter.Queries()))
	}
	assert.Equal(t, counts[0], counts[2], "waiting commands must share the send-key and transport reads")
	querybudgets.AssertQueryBudget(t, "workflows.holidayclosurenotification.waiting_delivery", counter.Measurement(counter.Queries()))
	_, err := w.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW() WHERE organization_id = ?`, w.organization)
	require.NoError(t, err)
	counter.Reset()
	require.NoError(t, w.notices.ProcessDue(w.ctx))
	assert.Equal(t, counts[0]+1, len(counter.Queries()), "changed delivery states add a single bulk update")
	assert.Equal(t, []string{"completed", "completed", "completed"}, w.events(t))
}

func TestUnresolvedHolidayClosureSurvivesRetirement(t *testing.T) {
	t.Parallel()
	w := newClosureWorld(t, "Träger offene Bestätigung behalten")
	w.createClosingDay(t, "2030-07-10", "2030-07-10")
	notice := w.schoolNotices(t)[0]
	proposals := schoolCalendarCompose.NewHolidayClosureEvents()
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		_, err := proposals.ConfirmHolidayClosureNotice(ctx, notice.ID, notice.Revision,
			schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool, ImpactToken: notice.Token})
		return err
	}))
	_, err := w.db.ExecContext(context.Background(), `UPDATE calendar.holiday_closure_events SET state = 'needs_confirmation',
  processed_at = NOW() - interval '31 days' WHERE organization_id = ?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(w.ctx, w.organization, func(ctx context.Context) error {
		_, err := proposals.RetireHolidayClosureEvents(ctx, w.organization)
		return err
	}))
	assert.Equal(t, []string{"needs_confirmation"}, w.events(t), "age does not resolve an outstanding confirmed notice")
	require.Len(t, w.schoolNotices(t), 1)
}

// A renewed approval still previews and creates the original confirmed days,
// even when the whole booked week has elapsed.
func TestRenewedHolidayClosureConfirmationKeepsElapsedSpan(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger vergangene Bestätigung", testpkg.SetupIsolatedTestDB(t))
	day := w.createClosingDay(t, "2030-07-10", "2030-07-10")
	// Pin an elapsed, originally confirmed Wednesday without depending on midnight.
	for _, query := range []string{
		`UPDATE enrollment.holiday_period_weeks SET starts_on = '2020-07-06', ends_on = '2020-07-10' WHERE organization_id = ?`,
		`UPDATE calendar.holiday_closure_proposals SET span_from = '2020-07-06', span_to = '2020-07-08',
   current_start = '2020-07-08', current_end = '2020-07-08' WHERE organization_id = ?`,
		`INSERT INTO calendar.holiday_closure_events (tenant_id, organization_id, proposal_id, operation_id, source_kind, source_id,
   revision, span_from, span_to, confirmed_channel, state)
   SELECT tenant_id, organization_id, id, operation_id, source_kind, source_id, revision, '2020-07-08', span_to, 'school', 'needs_confirmation'
   FROM calendar.holiday_closure_proposals WHERE organization_id = ?`,
		`UPDATE calendar.holiday_closure_proposals SET state = 'confirmed', confirmed_at = NOW() WHERE organization_id = ?`,
	} {
		_, err := w.db.ExecContext(context.Background(), query, w.organization)
		require.NoError(t, err)
	}
	_, err := w.db.ExecContext(context.Background(), `UPDATE schedule.closing_days SET start_date = '2020-07-08', end_date = '2020-07-08' WHERE tenant_id = ? AND id = ?`, w.school, day.ID)
	require.NoError(t, err)
	school := w.schoolNotices(t)
	require.Len(t, school, 1, "elapsed confirmed work remains reachable in school")
	assert.Equal(t, 2, school[0].Families)
	office, err := w.notices.PendingHolidayClosures(w.ctx, w.organization)
	require.NoError(t, err)
	require.Len(t, office.Notices, 1, "the office sees the same renewed approval")
	assert.Equal(t, school[0].Families, office.Notices[0].Families)
	_, err = w.notices.NotifyFromSchool(w.ctx, w.school, 0, school[0].ID, school[0].Revision, school[0].Token)
	require.NoError(t, err)
	require.Len(t, w.mails(t), 2)
	assert.Equal(t, []string{"prepared"}, w.events(t))
	var span calendar.Date
	require.NoError(t, w.db.NewRaw(`SELECT span_from FROM calendar.holiday_closure_events WHERE organization_id = ?`, w.organization).Scan(context.Background(), &span))
	assert.Equal(t, calendar.NewDate(2020, 7, 8), span)
	var payload enrollment.HolidayClosureMail
	require.NoError(t, json.Unmarshal(w.mails(t)[0].Payload, &payload))
	assert.True(t, payload.ContainsPastDays)
}

// The read-only watchdog sees blocked work without leasing or processing it.
func TestHolidayClosureWatchdogFindsStalledCommands(t *testing.T) {
	t.Parallel()
	w := newClosureWorldIn(t, "Träger unabhängige Überwachung", testpkg.SetupIsolatedTestDB(t))
	events := schoolCalendarCompose.NewHolidayClosureEvents()
	for _, date := range []string{"2030-07-08", "2030-07-09", "2030-07-10", "2030-07-11", "2030-07-12"} {
		w.createClosingDay(t, date, date)
	}
	require.NoError(t, tenant.WithTenantTx(w.ctx, w.db, w.school, func(ctx context.Context, _ bun.Tx) error {
		notices, err := events.SchoolHolidayClosureNotices(ctx)
		require.NoError(t, err)
		for _, notice := range notices {
			_, err := events.ConfirmHolidayClosureNotice(ctx, notice.ProposalID, notice.Revision,
				schoolcalendar.HolidayClosureActor{Channel: schoolcalendar.HolidayClosureChannelSchool})
			require.NoError(t, err)
		}
		_, err = events.OverdueHolidayClosureEvents(ctx, 15*time.Minute)
		require.Error(t, err, "a school must never read the global watchdog")
		return nil
	}))
	_, err := w.db.ExecContext(context.Background(), `UPDATE calendar.holiday_closure_events SET
  state = CASE source_id % 5 WHEN 0 THEN 'blocked' WHEN 1 THEN 'pending' WHEN 2 THEN 'leased' WHEN 3 THEN 'prepared' ELSE 'unknown' END,
  next_attempt_at = NOW() - interval '16 minutes',
  lease_until = CASE WHEN source_id % 5 = 2 THEN NOW() - interval '16 minutes' END,
  lease_token = CASE WHEN source_id % 5 = 2 THEN gen_random_uuid() END,
  processed_at = NOW() - interval '16 minutes' WHERE organization_id = ?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
		count, err := events.OverdueHolidayClosureEvents(ctx, 15*time.Minute)
		require.NoError(t, err)
		assert.Equal(t, 5, count, "pending, expired leases, prepared, unknown and blocked are covered")
		return nil
	}))
	assert.Empty(t, w.mails(t), "observation never processes or confirms work")
	_, err = w.db.ExecContext(context.Background(), `UPDATE calendar.holiday_closure_events SET state = 'needs_confirmation', lease_token = NULL, lease_until = NULL WHERE organization_id = ?`, w.organization)
	require.NoError(t, err)
	require.NoError(t, tenant.WithAdminTx(w.ctx, w.db, func(ctx context.Context, _ bun.Tx) error {
		count, err := events.OverdueHolidayClosureEvents(ctx, 15*time.Minute)
		require.NoError(t, err)
		assert.Zero(t, count, "renewed human approval has no transport deadline")
		return nil
	}))
}
