package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Closing days in booked holiday weeks (#6434). The week of newPlacesWeek
// runs from Monday 2027-07-05 to Friday 2027-07-09; the tests run on a fixed
// day before it.

const closureToday = calendar.Date("2027-06-01")

// capturedMails is the Träger's outbox of a test.
type capturedMails struct{ mails []enrollment.TraegerMail }

func (c *capturedMails) EnqueueTraegerMail(_ context.Context, mail enrollment.TraegerMail) error {
	c.mails = append(c.mails, mail)
	return nil
}

// closureCalendar names the same days without care at every host school.
type closureCalendar []enrollment.HolidayNoCareDay

func (c closureCalendar) HolidayNoCareDays(_ context.Context, schoolIDs []int64, from, to calendar.Date) (map[int64][]enrollment.HolidayNoCareDay, error) {
	result := map[int64][]enrollment.HolidayNoCareDay{}
	for _, school := range schoolIDs {
		for _, day := range c {
			if !day.Date.Before(from) && !day.Date.After(to) {
				result[school] = append(result[school], day)
			}
		}
	}
	return result, nil
}

type closureTraeger struct{}

func (closureTraeger) HolidayTraeger(context.Context, int64) (enrollment.HolidayTraeger, error) {
	return enrollment.HolidayTraeger{Name: "Träger Ferien"}, nil
}

func newClosures(mail *capturedMails, days closureCalendar) enrollment.HolidayClosures {
	return compose.NewHolidayClosures(days, closureTraeger{}, mail, func() calendar.Date { return closureToday })
}

func hostSchoolOf(t *testing.T, db *bun.DB, week int64) int64 {
	t.Helper()
	var school int64
	require.NoError(t, db.NewRaw(`SELECT host_school_id FROM enrollment.holiday_period_weeks WHERE id = ?`, week).Scan(context.Background(), &school))
	return school
}

// addSibling books a second child of the family of email into week.
func addSibling(t *testing.T, db *bun.DB, w placesWeek, email, placement string) int64 {
	t.Helper()
	ctx := context.Background()
	var registration, child, model int64
	require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.holiday_week_time_models WHERE holiday_period_week_id = ? LIMIT 1`, w.week).Scan(ctx, &model))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registrations (organization_id, holiday_period_id, guardian_first_name,
			guardian_last_name, guardian_email, guardian_phone, privacy_consent_at)
		VALUES (?, ?, 'Eva', 'Platz', ?, '0123', NOW()) RETURNING id`, w.organization, w.period, email).Scan(ctx, &registration))
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 1, 'Geschwister', 'Platz', '2019-05-06') RETURNING id`, w.organization, w.period, registration).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement) VALUES (?, ?, ?, ?, ?, ?)`,
		w.organization, w.period, child, w.week, model, placement)
	require.NoError(t, err)
	return registration
}

func withinTraeger(t *testing.T, db *bun.DB, organization int64, fn func(ctx context.Context)) {
	t.Helper()
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	require.NoError(t, testpkg.WithinTraeger(ctx, organization, func(ctx context.Context) error {
		fn(ctx)
		return nil
	}))
}

func placements(t *testing.T, db *bun.DB, week int64) []string {
	t.Helper()
	var result []string
	require.NoError(t, db.NewRaw(`SELECT placement FROM enrollment.holiday_registration_bookings WHERE holiday_period_week_id = ? ORDER BY id`, week).
		Scan(context.Background(), &result))
	return result
}

// TestHolidayClosureTellsEachFamilyOnceAndOnlyAboutNewDays pins invariants
// 1 and 2 of #6434: every affected family gets exactly one mail per notice,
// with all its children and closed days, and nobody gets a mail without an
// affected booking: not at another school, not on a weekend or a statutory
// holiday, and not for a day told before, so saving again sends nothing. The
// bookings stay as they were.
func TestHolidayClosureTellsEachFamilyOnceAndOnlyAboutNewDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag A")
	// The family of the two bookings of newPlacesWeek books a third child,
	// with its address written differently: still one family.
	addSibling(t, db, w, "Eva.Platz@Example.org ", "placed")
	school := hostSchoolOf(t, db, w.week)
	before := placements(t, db, w.week)
	mail := &capturedMails{}
	holidays := closureCalendar{{Date: calendar.NewDate(2027, 7, 8), Kind: enrollment.HolidayNoCareHoliday, Name: "Feiertag"}}
	closures := newClosures(mail, holidays)
	// Tuesday to Sunday: Thursday is a statutory holiday, the weekend has
	// no care.
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 7, 6), EndsOn: calendar.NewDate(2027, 7, 11), Reason: "Betriebsausflug"}

	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		require.Len(t, impact.Weeks, 1)
		assert.Equal(t, 3, impact.Weeks[0].Children)
		assert.Equal(t, 1, impact.Weeks[0].Waitlisted)
		assert.Equal(t, 1, impact.Families, "one address, however it is written, is one family")
		assert.Equal(t, []enrollment.HolidayClosedDay{
			{Date: calendar.NewDate(2027, 7, 6), Reason: "Betriebsausflug"},
			{Date: calendar.NewDate(2027, 7, 7), Reason: "Betriebsausflug"},
			{Date: calendar.NewDate(2027, 7, 9), Reason: "Betriebsausflug"},
		}, impact.Weeks[0].ClosedDays, "no weekend, no statutory holiday")
		assert.True(t, impact.KeepsContribution)

		notice, err := closures.NotifyHolidayClosure(ctx, w.organization, closure, impact.Token)
		require.NoError(t, err)
		assert.Equal(t, enrollment.HolidayClosureNotice{Families: 1, Children: 3}, notice)
		require.Len(t, mail.mails, 1, "one mail for the family, not one per child, week or day")
		assert.Equal(t, enrollment.MailKindHolidayClosureNotice, mail.mails[0].Kind)
		var payload enrollment.HolidayClosureMail
		require.NoError(t, json.Unmarshal(mail.mails[0].Payload, &payload))
		assert.Len(t, payload.Children, 3)
		var registrations []int64
		require.NoError(t, db.NewRaw(`SELECT id FROM enrollment.holiday_registrations WHERE organization_id = ? ORDER BY id`, w.organization).Scan(ctx, &registrations))
		assert.Equal(t, registrations, payload.RegistrationIDs, "erasure must find every registration named in a grouped mail")
		assert.Equal(t, []string{"Dienstag, 06.07.2027", "Mittwoch, 07.07.2027", "Freitag, 09.07.2027"}, payload.Children[0].Weeks[0].ClosedDays)
		assert.Equal(t, []string{"Betriebsausflug"}, payload.Reasons)

		// Saving again tells nobody: every day is told.
		again, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		assert.Empty(t, again.Weeks)
		notice, err = closures.NotifyHolidayClosure(ctx, w.organization, closure, "")
		require.NoError(t, err)
		assert.Zero(t, notice.Families)
		assert.Len(t, mail.mails, 1, "an unchanged save sends no second mail")

		// One more day: only the new day is told.
		longer := closure
		longer.StartsOn = calendar.NewDate(2027, 7, 5)
		impact, err = closures.HolidayClosureImpact(ctx, w.organization, longer)
		require.NoError(t, err)
		require.Len(t, impact.Weeks, 1)
		assert.Equal(t, []enrollment.HolidayClosedDay{{Date: calendar.NewDate(2027, 7, 5), Reason: "Betriebsausflug"}}, impact.Weeks[0].ClosedDays)

		// Another school or a weekend affects no booking.
		for name, other := range map[string]enrollment.HolidayClosure{
			"other school": {SchoolIDs: []int64{school + 1000000}, StartsOn: closure.StartsOn, EndsOn: closure.EndsOn, Reason: "Fremd"},
			"weekend":      {SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 7, 10), EndsOn: calendar.NewDate(2027, 7, 11), Reason: "Wochenende"},
			"past":         {SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 5, 3), EndsOn: calendar.NewDate(2027, 5, 7), Reason: "Vorbei"},
		} {
			notice, err := closures.NotifyHolidayClosure(ctx, w.organization, other, "")
			require.NoError(t, err, name)
			assert.Zero(t, notice.Families, name)
		}
		assert.Len(t, mail.mails, 1, "no mail without an affected booking")
	})
	assert.Equal(t, before, placements(t, db, w.week), "no booking changes its place, nobody moves up")
}

// TestHolidayClosureRefusesANoticeTheBueroDidNotSee pins invariant 3 of
// #6434: a notice is sent only for the bookings of the preview the Büro
// confirmed. A booking added after the preview changes the token, the notice
// is refused and nothing is sent or recorded.
func TestHolidayClosureRefusesANoticeTheBueroDidNotSee(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag B")
	school := hostSchoolOf(t, db, w.week)
	mail := &capturedMails{}
	closures := newClosures(mail, nil)
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 7, 9), EndsOn: calendar.NewDate(2027, 7, 9), Reason: "Brückentag"}

	var token string
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		token = impact.Token
	})
	addSibling(t, db, w, "neu@example.org", "waitlisted")
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		_, err := closures.NotifyHolidayClosure(ctx, w.organization, closure, token)
		require.ErrorIs(t, err, enrollment.ErrHolidayClosureChanged)
		_, err = closures.NotifyHolidayClosure(ctx, w.organization, closure, "")
		require.ErrorIs(t, err, enrollment.ErrHolidayClosureChanged, "a save without a confirmed preview tells nobody")
	})
	assert.Empty(t, mail.mails)
	var told int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM enrollment.holiday_closure_notices WHERE organization_id = ?`, w.organization).
		Scan(context.Background(), &told))
	assert.Zero(t, told)
}

// TestHolidayClosurePendingListsTheSchoolsOwnClosingDays pins the safety net
// of #6434: a closing day the school entered itself, which its transaction
// cannot tell the Träger's families about, stays pending for the Büro until
// it tells them, and then is gone.
func TestHolidayClosurePendingListsTheSchoolsOwnClosingDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag C")
	mail := &capturedMails{}
	closures := newClosures(mail, closureCalendar{
		{Date: calendar.NewDate(2027, 7, 7), Kind: enrollment.HolidayNoCareClosingDay, Name: "Pädagogischer Tag"},
		{Date: calendar.NewDate(2027, 7, 8), Kind: enrollment.HolidayNoCareHoliday, Name: "Feiertag"},
	})
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		pending, err := closures.PendingHolidayClosures(ctx, w.organization)
		require.NoError(t, err)
		require.Len(t, pending.Weeks, 1)
		assert.Equal(t, []enrollment.HolidayClosedDay{{Date: calendar.NewDate(2027, 7, 7), Reason: "Pädagogischer Tag"}}, pending.Weeks[0].ClosedDays)

		notice, err := closures.NotifyPendingHolidayClosures(ctx, w.organization, pending.Token)
		require.NoError(t, err)
		assert.Equal(t, 1, notice.Families)
		pending, err = closures.PendingHolidayClosures(ctx, w.organization)
		require.NoError(t, err)
		assert.Empty(t, pending.Weeks)
	})
	assert.Len(t, mail.mails, 1)
}

// TestHolidayClosuresKeepTheirTraegerBeneathRLS pins the Träger filter of
// every statement of the closure notices (#6434) beneath RLS, as #5952 does:
// in the Träger transaction of A, as the owner of the tables, each statement
// is asked for B's school, bookings and notices. Each assertion goes red
// when its organization_id filter is dropped.
func TestHolidayClosuresKeepTheirTraegerBeneathRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newPlacesWeek(t, db, "Träger Schließtag RLS A")
	foreign := newPlacesWeek(t, db, "Träger Schließtag RLS B")
	foreignSchool := hostSchoolOf(t, db, foreign.week)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_closure_notices (organization_id, holiday_registration_id, holiday_period_week_id, closed_on)
		VALUES (?, ?, ?, '2027-07-07')`, foreign.organization, foreign.waiting, foreign.week)
	require.NoError(t, err)
	store := postgres.NewHolidayRegistrationStore()
	withinTraeger(t, db, own.organization, func(ctx context.Context) {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		all, err := store.HolidayClosureBookings(ctx, own.organization, nil, calendar.NewDate(2027, 7, 5), calendar.NewDate(2027, 7, 9))
		require.NoError(t, err)
		require.Len(t, all, 2, "A reads its own two bookings")
		for _, booking := range all {
			assert.Equal(t, own.week, booking.WeekID, "A reads no booking of B")
		}
		atForeign, err := store.HolidayClosureBookings(ctx, own.organization, []int64{foreignSchool}, calendar.NewDate(2027, 7, 5), calendar.NewDate(2027, 7, 9))
		require.NoError(t, err)
		assert.Empty(t, atForeign, "A reads no booking at B's school")
		told, err := store.HolidayClosureNotices(ctx, own.organization, []int64{foreign.waiting})
		require.NoError(t, err)
		assert.Empty(t, told, "A reads no notice of B")
	})
	assert.Equal(t, fmt.Sprint([]string{"placed", "waitlisted"}), fmt.Sprint(placements(t, db, foreign.week)))
}

func TestHolidayClosureRefusesChangedRecipients(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag Adresse")
	mail := &capturedMails{}
	closures := newClosures(mail, nil)
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{hostSchoolOf(t, db, w.week)}, StartsOn: calendar.NewDate(2027, 7, 9), EndsOn: calendar.NewDate(2027, 7, 9)}
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET guardian_email = ' Eva.Platz@Example.org ' WHERE organization_id = ? AND id = ?`, w.organization, w.waiting)
		require.NoError(t, err)
		normalized, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		assert.Equal(t, impact.Token, normalized.Token, "the grouping uses normalized addresses")
		_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registrations SET guardian_email = 'andere@example.org' WHERE organization_id = ? AND id = ?`, w.organization, w.waiting)
		require.NoError(t, err)
		_, err = closures.NotifyHolidayClosure(ctx, w.organization, closure, impact.Token)
		require.ErrorIs(t, err, enrollment.ErrHolidayClosureChanged)
	})
	assert.Empty(t, mail.mails)
}

// heldClosureMail pauses the first notification inside its transaction.
type heldClosureMail struct {
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (m *heldClosureMail) EnqueueTraegerMail(ctx context.Context, _ enrollment.TraegerMail) error {
	if m.calls.Add(1) == 1 {
		close(m.entered)
		select {
		case <-m.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestHolidayClosureSerializesOverlappingPendingNotices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag gleichzeitig")
	mail := &heldClosureMail{entered: make(chan struct{}), release: make(chan struct{})}
	days := closureCalendar{
		{Date: calendar.NewDate(2027, 7, 7), Kind: enrollment.HolidayNoCareClosingDay},
		{Date: calendar.NewDate(2027, 7, 8), Kind: enrollment.HolidayNoCareClosingDay},
	}
	closures := compose.NewHolidayClosures(days, closureTraeger{}, mail, func() calendar.Date { return closureToday })
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{hostSchoolOf(t, db, w.week)}, StartsOn: calendar.NewDate(2027, 7, 6), EndsOn: calendar.NewDate(2027, 7, 7)}
	ctx, cancel := context.WithTimeout(testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db)), 10*time.Second)
	defer cancel()
	var token, pendingToken string
	require.NoError(t, testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		if err != nil {
			return err
		}
		token = impact.Token
		pending, err := closures.PendingHolidayClosures(ctx, w.organization)
		pendingToken = pending.Token
		return err
	}))
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
			_, err := closures.NotifyHolidayClosure(ctx, w.organization, closure, token)
			return err
		})
	}()
	select {
	case <-mail.entered:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	secondDone := make(chan error, 1)
	pidReady := make(chan int, 1)
	go func() {
		secondDone <- testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
			var pid int
			tx, err := postgres.NewDatabase()(ctx)
			if err != nil {
				return err
			}
			if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
				return err
			}
			pidReady <- pid
			_, err = closures.NotifyPendingHolidayClosures(ctx, w.organization, pendingToken)
			return err
		})
	}()
	var pid int
	select {
	case pid = <-pidReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	// Wait for either the blocked advisory lock or the buggy second enqueue.
	assert.Eventually(t, func() bool {
		if mail.calls.Load() > 1 {
			return true
		}
		var waiting bool
		err := db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = ? AND locktype = 'advisory' AND NOT granted)`, pid).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond)
	assert.EqualValues(t, 1, mail.calls.Load(), "an overlapping notification must wait before reading unnotified days")
	close(mail.release)
	require.NoError(t, <-firstDone)
	require.ErrorIs(t, <-secondDone, enrollment.ErrHolidayClosureChanged)
	assert.EqualValues(t, 1, mail.calls.Load(), "a stale overlapping preview cannot enqueue duplicate mail")
}

// TestHolidayClosureTellsABookingOfDaysOnlyAboutItsBookedDays pins the join
// with the booking of days (#4701): a closing day affects a booking of days
// only on a day it booked, so a closure of other days of the week tells
// nobody.
func TestHolidayClosureTellsABookingOfDaysOnlyAboutItsBookedDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag Tage")
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `UPDATE enrollment.holiday_period_weeks SET booking_unit = 'day' WHERE id = ?`, w.week)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_bookings SET days = ARRAY['2027-07-05', '2027-07-06']::date[]
		WHERE holiday_period_week_id = ?`, w.week)
	require.NoError(t, err)
	school := hostSchoolOf(t, db, w.week)
	mail := &capturedMails{}
	closures := newClosures(mail, nil)
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		other, err := closures.HolidayClosureImpact(ctx, w.organization, enrollment.HolidayClosure{
			SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 7, 7), EndsOn: calendar.NewDate(2027, 7, 9), Reason: "Teamtage",
		})
		require.NoError(t, err)
		assert.Empty(t, other.Weeks, "no booked day is closed")

		booked, err := closures.HolidayClosureImpact(ctx, w.organization, enrollment.HolidayClosure{
			SchoolIDs: []int64{school}, StartsOn: calendar.NewDate(2027, 7, 6), EndsOn: calendar.NewDate(2027, 7, 7), Reason: "Teamtage",
		})
		require.NoError(t, err)
		require.Len(t, booked.Weeks, 1)
		assert.Equal(t, []enrollment.HolidayClosedDay{{Date: calendar.NewDate(2027, 7, 6), Reason: "Teamtage"}}, booked.Weeks[0].ClosedDays,
			"only the booked Tuesday, not the unbooked Wednesday")
	})
}

// TestHolidayClosureCountsAPendingBookingWithoutAPlace: a booking that waits
// for the decision of the Büro by criteria (#4700) has no place, so the
// preview counts it with the waiting ones and its mail says so, like a
// waitlisted one. Counted as placed, it would promise the family a place.
func TestHolidayClosureCountsAPendingBookingWithoutAPlace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag offen")
	addSibling(t, db, w, "offen@example.org", "pending")
	mail := &capturedMails{}
	closures := newClosures(mail, nil)
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{hostSchoolOf(t, db, w.week)}, StartsOn: calendar.NewDate(2027, 7, 6), EndsOn: calendar.NewDate(2027, 7, 6), Reason: "Betriebsausflug"}

	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		require.Len(t, impact.Weeks, 1)
		assert.Equal(t, 3, impact.Weeks[0].Children)
		assert.Equal(t, 2, impact.Weeks[0].Waitlisted, "the waitlisted and the pending booking have no place")
		_, err = closures.NotifyHolidayClosure(ctx, w.organization, closure, impact.Token)
		require.NoError(t, err)
		waiting := 0
		for _, sent := range mail.mails {
			var payload enrollment.HolidayClosureMail
			require.NoError(t, json.Unmarshal(sent.Payload, &payload))
			for _, child := range payload.Children {
				for _, week := range child.Weeks {
					if week.Waitlisted {
						waiting++
					}
				}
			}
		}
		assert.Equal(t, 2, waiting, "the mail does not promise the pending child a place")
	})
}
