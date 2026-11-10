package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
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
type capturedMails struct {
	mails    []enrollment.TraegerMail
	outcomes map[string]string
	retried  []string
}

func (c *capturedMails) EnqueueTraegerMail(_ context.Context, mail enrollment.TraegerMail) error {
	c.mails = append(c.mails, mail)
	return nil
}

// TraegerMailStates reports every captured mail as outcome, or as queued.
func (c *capturedMails) TraegerMailStates(_ context.Context, _ int64, keys []string) ([]enrollment.TraegerMailState, error) {
	var states []enrollment.TraegerMailState
	for _, mail := range c.mails {
		if slices.Contains(keys, mail.IdempotencyKey) {
			states = append(states, enrollment.TraegerMailState{IdempotencyKey: mail.IdempotencyKey, Outcome: c.outcome(mail.IdempotencyKey)})
		}
	}
	return states, nil
}

func (c *capturedMails) outcome(key string) string {
	if outcome, ok := c.outcomes[key]; ok {
		return outcome
	}
	return enrollment.TraegerMailQueued
}

// RetryFailedTraegerEmails counts the failed mails of keys as retried.
func (c *capturedMails) RetryFailedTraegerEmails(_ context.Context, _ int64, keys []string) (int64, error) {
	var retried int64
	for _, key := range keys {
		if c.outcomes[key] == enrollment.TraegerMailFailed {
			c.outcomes[key] = enrollment.TraegerMailQueued
			c.retried = append(c.retried, key)
			retried++
		}
	}
	return retried, nil
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

// closureKeyBytes and closureKeyed stand in for Security Runtime's random
// source and keyed fingerprint: deterministic, and still keyed.
func closureKeyBytes(b []byte) error {
	for i := range b {
		b[i] = byte(i + 1)
	}
	return nil
}

func closureKeyed(key, content []byte) string {
	return fmt.Sprintf("%064x", fnvOf(append(slices.Clone(key), content...)))
}

func fnvOf(b []byte) uint64 {
	var h uint64 = 14695981039346656037
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

// closureOperation is a school operation over the week of newPlacesWeek.
func closureOperation(school int64) enrollment.HolidayClosureOperation {
	return enrollment.HolidayClosureOperation{
		OperationID: "8c1b0c2e-6a59-4c3f-9a65-3c5f0d0a2b11", SchoolID: school,
		From: calendar.NewDate(2027, 7, 5), To: calendar.NewDate(2027, 7, 9),
	}
}

// HolidayClosureNoCareDays answers the operation's read channel with the
// same days.
func (c closureCalendar) HolidayClosureNoCareDays(ctx context.Context, _ int64, school int64, from, to calendar.Date) ([]enrollment.HolidayNoCareDay, error) {
	byDay, err := c.HolidayNoCareDays(ctx, []int64{school}, from, to)
	return byDay[school], err
}

func newClosures(mail *capturedMails, days closureCalendar) enrollment.HolidayClosures {
	if mail.outcomes == nil {
		mail.outcomes = map[string]string{}
	}
	return compose.NewHolidayClosures(days, days, closureTraeger{}, mail, closureKeyBytes, closureKeyed, func() calendar.Date { return closureToday })
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

// TestHolidayClosureOperationTellsTheSchoolsOwnClosingDays pins the school
// path of ADR 0196: a confirmed operation of a school closing day tells the
// parents of the closed days nobody told, once; the plan the school and
// the Büro see counts exactly the families the notice tells.
func TestHolidayClosureOperationTellsTheSchoolsOwnClosingDays(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag C")
	addSibling(t, db, w, "zweite@example.org", "placed")
	mail := &capturedMails{}
	closures := newClosures(mail, closureCalendar{
		{Date: calendar.NewDate(2027, 7, 7), Kind: enrollment.HolidayNoCareClosingDay, Name: "Pädagogischer Tag"},
		{Date: calendar.NewDate(2027, 7, 8), Kind: enrollment.HolidayNoCareHoliday, Name: "Feiertag"},
	})
	operation := closureOperation(hostSchoolOf(t, db, w.week))
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		plan, err := closures.PlanHolidayClosureOperation(ctx, w.organization, operation)
		require.NoError(t, err)
		require.Len(t, plan.Weeks, 1)
		assert.Equal(t, []enrollment.HolidayClosedDay{{Date: calendar.NewDate(2027, 7, 7), Reason: "Pädagogischer Tag"}}, plan.Weeks[0].ClosedDays)
		assert.Empty(t, mail.mails, "a plan sends nothing")

		result, err := closures.NotifyHolidayClosureOperation(ctx, w.organization, operation)
		require.NoError(t, err)
		assert.Equal(t, plan.Families, result.Families, "the count the school sees is the families the click tells")
		assert.Equal(t, 2, result.Families)
		assert.Len(t, mail.mails, plan.Families, "one mail per counted family")

		again, err := closures.NotifyHolidayClosureOperation(ctx, w.organization, operation)
		require.NoError(t, err)
		assert.Zero(t, again.Families, "a second click of the same operation tells nobody again")
		assert.Equal(t, 2, again.Mails, "but still follows the operation's two mails")
		plan, err = closures.PlanHolidayClosureOperation(ctx, w.organization, operation)
		require.NoError(t, err)
		assert.Zero(t, plan.Families, "nothing waits any more")
	})
	assert.Len(t, mail.mails, 2)
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

func (m *heldClosureMail) TraegerMailStates(context.Context, int64, []string) ([]enrollment.TraegerMailState, error) {
	return nil, nil
}

func (m *heldClosureMail) RetryFailedTraegerEmails(context.Context, int64, []string) (int64, error) {
	return 0, nil
}

// TestHolidayClosureSerializesOverlappingNotices pins the lock of the
// notices: a school operation that overlaps a running Träger save waits
// before it reads what was told, so it tells only the day the save did not.
func TestHolidayClosureSerializesOverlappingNotices(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag gleichzeitig")
	mail := &heldClosureMail{entered: make(chan struct{}), release: make(chan struct{})}
	// The calendar after the save: the Träger's closing day on Tuesday and
	// Wednesday, the school's on Wednesday and Thursday.
	days := closureCalendar{
		{Date: calendar.NewDate(2027, 7, 6), Kind: enrollment.HolidayNoCareClosingDay},
		{Date: calendar.NewDate(2027, 7, 7), Kind: enrollment.HolidayNoCareClosingDay},
		{Date: calendar.NewDate(2027, 7, 8), Kind: enrollment.HolidayNoCareClosingDay},
	}
	closures := compose.NewHolidayClosures(days, days, closureTraeger{}, mail, closureKeyBytes, closureKeyed, func() calendar.Date { return closureToday })
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{hostSchoolOf(t, db, w.week)}, StartsOn: calendar.NewDate(2027, 7, 6), EndsOn: calendar.NewDate(2027, 7, 7)}
	ctx, cancel := context.WithTimeout(testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db)), 10*time.Second)
	defer cancel()
	var token string
	require.NoError(t, testpkg.WithinTraeger(ctx, w.organization, func(ctx context.Context) error {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		token = impact.Token
		return err
	}))
	operation := closureOperation(hostSchoolOf(t, db, w.week))
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
			_, err = closures.NotifyHolidayClosureOperation(ctx, w.organization, operation)
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
	require.NoError(t, <-secondDone)
	var told []string
	require.NoError(t, db.NewRaw(`SELECT closed_on::text || ':' || state FROM enrollment.holiday_closure_notices
		WHERE organization_id = ? ORDER BY closed_on, holiday_registration_id`, w.organization).Scan(ctx, &told))
	assert.Equal(t, []string{"2027-07-06:closed", "2027-07-06:closed", "2027-07-07:closed", "2027-07-07:closed", "2027-07-08:closed", "2027-07-08:closed"}, told)
	assert.EqualValues(t, 2, mail.calls.Load(), "the waiting operation tells only the day the save did not: one more mail, not a duplicate")
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

// TestHolidayClosureTellsARegistrationWithTwoChildrenInOneWeek pins the
// told days of a registration whose two children booked the same week: the
// day is one state of the registration and week, written once, and the
// family gets one mail.
func TestHolidayClosureTellsARegistrationWithTwoChildrenInOneWeek(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Schließtag Geschwister")
	ctx := context.Background()
	var child int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_registration_children (organization_id, holiday_period_id,
			holiday_registration_id, position, first_name, last_name, birth_date)
		VALUES (?, ?, ?, 2, 'Zweites', 'Platz', '2020-05-06') RETURNING id`, w.organization, w.period, w.waiting).Scan(ctx, &child))
	_, err := db.ExecContext(ctx, `INSERT INTO enrollment.holiday_registration_bookings (organization_id, holiday_period_id,
			holiday_registration_child_id, holiday_period_week_id, holiday_week_time_model_id, placement)
		SELECT organization_id, holiday_period_id, ?, holiday_period_week_id, holiday_week_time_model_id, 'placed'
		FROM enrollment.holiday_registration_bookings WHERE holiday_period_week_id = ? LIMIT 1`, child, w.week)
	require.NoError(t, err)
	mail := &capturedMails{}
	closures := newClosures(mail, nil)
	closure := enrollment.HolidayClosure{SchoolIDs: []int64{hostSchoolOf(t, db, w.week)}, StartsOn: calendar.NewDate(2027, 7, 7), EndsOn: calendar.NewDate(2027, 7, 7), Reason: "Teamtag"}
	withinTraeger(t, db, w.organization, func(ctx context.Context) {
		impact, err := closures.HolidayClosureImpact(ctx, w.organization, closure)
		require.NoError(t, err)
		notice, err := closures.NotifyHolidayClosure(ctx, w.organization, closure, impact.Token)
		require.NoError(t, err)
		assert.Equal(t, 1, notice.Families)
	})
	assert.Len(t, mail.mails, 1)
}
