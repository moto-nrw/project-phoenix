package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gofrs/uuid"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days in booked holiday weeks (#6434, ADR 0196); the rules are on
// enrollment.HolidayClosures. Preview, count and notice share one
// computation (changes), so what the Büro or the school saw is what the
// parents are told.

// HolidayClosureStore reads the bookings of a closure and records what the
// parents were told.
type HolidayClosureStore interface {
	LockHolidayClosureNotices(ctx context.Context, organizationID int64) error
	HolidayClosureBookings(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date) ([]enrollment.HolidayClosureBookingRecord, error)
	HolidayClosureNotices(ctx context.Context, organizationID int64, registrationIDs []int64) ([]enrollment.HolidayClosureNoticeRecord, error)
	SetHolidayClosureNotices(ctx context.Context, organizationID int64, operationID string, notices []enrollment.HolidayClosureNoticeRecord) error
	// HolidayClosureFamilyKey returns the Träger's stored key, storing
	// candidate first when it has none.
	HolidayClosureFamilyKey(ctx context.Context, organizationID int64, candidate []byte) ([]byte, error)
	RecordHolidayClosureSends(ctx context.Context, organizationID int64, sends []enrollment.HolidayClosureSendRecord) error
	HolidayClosureSendKeys(ctx context.Context, organizationID int64, operationID string) ([]string, error)
}

// errHolidayClosureMailRequired reports a notice without the outbox.
var errHolidayClosureMailRequired = errors.New("enrollment: telling the parents about a closing day needs the mail outbox")

// HolidayClosures serves enrollment.HolidayClosures.
type HolidayClosures struct {
	store     HolidayClosureStore
	calendar  enrollment.HolidaySchoolCalendar
	operation enrollment.HolidayClosureCalendar
	traeger   enrollment.HolidayTraegerDirectory
	mail      enrollment.TraegerMailDelivery
	random    func([]byte) error
	keyed     func(key, content []byte) string
	today     func() calendar.Date
}

var _ enrollment.HolidayClosures = (*HolidayClosures)(nil)

// NewHolidayClosures builds the notices over store. calendar names the
// holidays and closing days of the host schools, traeger the Träger and its
// schools in the mail, mail the outbox; random draws a Träger's family key
// and keyed derives a family fingerprint with it; today defaults to the
// Berlin day.
func NewHolidayClosures(store HolidayClosureStore, schoolCalendar enrollment.HolidaySchoolCalendar, operationCalendar enrollment.HolidayClosureCalendar,
	traeger enrollment.HolidayTraegerDirectory, mail enrollment.TraegerMailDelivery, random func([]byte) error, keyed func(key, content []byte) string,
	today func() calendar.Date,
) *HolidayClosures {
	if store == nil || schoolCalendar == nil || operationCalendar == nil || traeger == nil || mail == nil || random == nil || keyed == nil {
		panic("enrollment: the holiday closure store, the host schools' calendar, the Träger directory, the mail outbox and the key functions are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayClosures{store: store, calendar: schoolCalendar, operation: operationCalendar, traeger: traeger, mail: mail, random: random, keyed: keyed, today: today}
}

// changedBooking is a booking with the days its parents learn about now:
// newly closed ones and ones open again.
type changedBooking struct {
	enrollment.HolidayClosureBookingRecord
	closed   []enrollment.HolidayClosedDay
	reopened []enrollment.HolidayClosedDay
}

// closedOn names whether day is closed at school by a closing day, with its
// reason, and whether it is a statutory holiday, about which nobody is told.
type closedOn func(schoolID int64, day calendar.Date) (closed bool, reason string, holiday bool)

// HolidayClosureImpact previews the bookings closure affects.
func (h *HolidayClosures) HolidayClosureImpact(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure) (enrollment.HolidayClosureImpact, error) {
	bookings, err := h.traegerChanges(ctx, organizationID, closure)
	if err != nil {
		return enrollment.HolidayClosureImpact{}, err
	}
	return holidayClosureImpact(bookings), nil
}

// NotifyHolidayClosure tells the parents of the bookings closure affects.
func (h *HolidayClosures) NotifyHolidayClosure(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure, token string) (enrollment.HolidayClosureNotice, error) {
	operationID, err := uuid.NewV4()
	if err != nil {
		return enrollment.HolidayClosureNotice{}, fmt.Errorf("enrollment: name the closure operation: %w", err)
	}
	if err := h.store.LockHolidayClosureNotices(ctx, organizationID); err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	bookings, err := h.traegerChanges(ctx, organizationID, closure)
	if err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	if len(bookings) == 0 {
		return enrollment.HolidayClosureNotice{}, nil
	}
	if token != holidayClosureToken(bookings) {
		return enrollment.HolidayClosureNotice{}, enrollment.ErrHolidayClosureChanged
	}
	return h.notify(ctx, organizationID, operationID.String(), bookings)
}

// PlanHolidayClosureOperation previews the notice of a school closing day
// change without writing anything.
func (h *HolidayClosures) PlanHolidayClosureOperation(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (enrollment.HolidayClosureImpact, error) {
	bookings, err := h.operationChanges(ctx, organizationID, operation)
	if err != nil {
		return enrollment.HolidayClosureImpact{}, err
	}
	return holidayClosureImpact(bookings), nil
}

// NotifyHolidayClosureOperation tells the parents of a confirmed school
// closing day change and takes the operation's failed mails up again.
func (h *HolidayClosures) NotifyHolidayClosureOperation(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (enrollment.HolidayClosureOperationResult, error) {
	if _, err := uuid.FromString(operation.OperationID); err != nil {
		return enrollment.HolidayClosureOperationResult{}, fmt.Errorf("enrollment: the closure operation is required: %w", err)
	}
	if err := h.store.LockHolidayClosureNotices(ctx, organizationID); err != nil {
		return enrollment.HolidayClosureOperationResult{}, err
	}
	bookings, err := h.operationChanges(ctx, organizationID, operation)
	if err != nil {
		return enrollment.HolidayClosureOperationResult{}, err
	}
	var result enrollment.HolidayClosureOperationResult
	if len(bookings) > 0 {
		notice, err := h.notify(ctx, organizationID, operation.OperationID, bookings)
		if err != nil {
			return enrollment.HolidayClosureOperationResult{}, err
		}
		result.Families = notice.Families
	}
	keys, err := h.store.HolidayClosureSendKeys(ctx, organizationID, operation.OperationID)
	if err != nil {
		return enrollment.HolidayClosureOperationResult{}, err
	}
	result.Mails = len(keys)
	if len(keys) > 0 {
		if result.Retried, err = h.mail.RetryFailedTraegerEmails(ctx, organizationID, keys); err != nil {
			return enrollment.HolidayClosureOperationResult{}, fmt.Errorf("enrollment: take the failed closure mails up again: %w", err)
		}
	}
	return result, nil
}

// HolidayClosureOperationDelivery sums the operation's mails up.
func (h *HolidayClosures) HolidayClosureOperationDelivery(ctx context.Context, organizationID int64, operationID string) (enrollment.HolidayClosureDelivery, error) {
	keys, err := h.store.HolidayClosureSendKeys(ctx, organizationID, operationID)
	if err != nil || len(keys) == 0 {
		return enrollment.HolidayClosureDelivery{}, err
	}
	states, err := h.mail.TraegerMailStates(ctx, organizationID, keys)
	if err != nil {
		return enrollment.HolidayClosureDelivery{}, fmt.Errorf("enrollment: read the closure mails: %w", err)
	}
	var delivery enrollment.HolidayClosureDelivery
	seen := map[string]bool{}
	for _, state := range states {
		seen[state.IdempotencyKey] = true
		switch state.Outcome {
		case enrollment.TraegerMailSent:
			delivery.Sent++
		case enrollment.TraegerMailFailed:
			delivery.Failed++
		case enrollment.TraegerMailUnknown:
			delivery.Unknown++
		default:
			delivery.Queued++
		}
	}
	// A recorded mail the outbox no longer holds is a known failure: it can
	// only have been deleted, never accepted.
	for _, key := range keys {
		if !seen[key] {
			delivery.Failed++
		}
	}
	return delivery, nil
}

// traegerChanges are the changes a Träger closing day makes: its stored
// version no longer counts, the new one counts from the save on.
func (h *HolidayClosures) traegerChanges(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure) ([]changedBooking, error) {
	if organizationID <= 0 {
		return nil, errors.New("enrollment: the Träger is required")
	}
	schools, from, to := closureSpan(closure)
	if len(schools) == 0 {
		return nil, nil
	}
	start := max(from, h.today())
	if to.Before(start) {
		// Authorize the schools all the same: a refusal must not depend on
		// the dates.
		_, err := h.calendar.HolidayNoCareDays(ctx, schools, from, from)
		return nil, err
	}
	// The calendar authorizes every school before it reads anything.
	noCare, err := h.calendar.HolidayNoCareDays(ctx, schools, start, to)
	if err != nil {
		return nil, err
	}
	return h.changes(ctx, organizationID, schools, start, to, traegerClosedOn(closure, noCareIndex(noCare, closure.TraegerClosingDayID)))
}

// closureSpan names the schools and days a Träger closing day touches: its
// new version unless it is deleted, and its stored version.
func closureSpan(closure enrollment.HolidayClosure) ([]int64, calendar.Date, calendar.Date) {
	var schools []int64
	var from, to calendar.Date
	widen := func(ids []int64, start, end calendar.Date) {
		if start.IsZero() || end.IsZero() || end.Before(start) {
			return
		}
		schools = append(schools, ids...)
		if from.IsZero() || start.Before(from) {
			from = start
		}
		if to.IsZero() || end.After(to) {
			to = end
		}
	}
	if !closure.Deleted {
		widen(closure.SchoolIDs, closure.StartsOn, closure.EndsOn)
	}
	if closure.Previous != nil {
		widen(closure.Previous.SchoolIDs, closure.Previous.StartsOn, closure.Previous.EndsOn)
	}
	return slices.Compact(slices.Sorted(slices.Values(schools))), from, to
}

// traegerClosedOn closes the days of the saved Träger closing day on top of
// the calendar, which still holds its stored version.
func traegerClosedOn(closure enrollment.HolidayClosure, index closedOn) closedOn {
	reason := strings.TrimSpace(closure.Reason)
	return func(schoolID int64, day calendar.Date) (bool, string, bool) {
		closed, name, holiday := index(schoolID, day)
		if holiday {
			return false, "", true
		}
		if !closure.Deleted && slices.Contains(closure.SchoolIDs, schoolID) && !day.Before(closure.StartsOn) && !day.After(closure.EndsOn) {
			return true, reason, false
		}
		return closed, name, false
	}
}

// operationChanges are the changes of a school closing day operation
// against the calendar the caller read.
func (h *HolidayClosures) operationChanges(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) ([]changedBooking, error) {
	if organizationID <= 0 || operation.SchoolID <= 0 {
		return nil, errors.New("enrollment: the Träger and the school are required")
	}
	from, to := max(operation.From, h.today()), operation.To
	if to.IsZero() || to.Before(from) {
		return nil, nil
	}
	noCare, err := h.operation.HolidayClosureNoCareDays(ctx, organizationID, operation.SchoolID, from, to)
	if err != nil {
		return nil, err
	}
	index := noCareIndex(map[int64][]enrollment.HolidayNoCareDay{operation.SchoolID: noCare}, 0)
	return h.changes(ctx, organizationID, []int64{operation.SchoolID}, from, to, index)
}

// noCareIndex answers closedOn from the days without care; a Träger closing
// day named by exclude does not count.
func noCareIndex(noCare map[int64][]enrollment.HolidayNoCareDay, exclude int64) closedOn {
	type key struct {
		school int64
		day    calendar.Date
	}
	days := map[key]enrollment.HolidayNoCareDay{}
	for school, list := range noCare {
		for _, day := range list {
			days[key{school, day.Date}] = day
		}
	}
	return func(schoolID int64, date calendar.Date) (bool, string, bool) {
		day, ok := days[key{schoolID, date}]
		switch {
		case !ok:
			return false, "", false
		case day.Kind == enrollment.HolidayNoCareHoliday:
			return false, "", true
		case len(day.ClosingDays) == 0:
			return true, day.Name, false
		}
		for _, ref := range day.ClosingDays {
			if !ref.Traeger || ref.ID != exclude {
				return true, day.Name, false
			}
		}
		return false, "", false
	}
}

// changes compares, for every booked care day of the bookings at schools in
// [from, to], what the calendar says with what the parents were last told.
func (h *HolidayClosures) changes(ctx context.Context, organizationID int64, schools []int64, from, to calendar.Date, closed closedOn) ([]changedBooking, error) {
	records, err := h.store.HolidayClosureBookings(ctx, organizationID, schools, from, to)
	if err != nil || len(records) == 0 {
		return nil, err
	}
	told, err := h.told(ctx, organizationID, records)
	if err != nil {
		return nil, err
	}
	var result []changedBooking
	for _, record := range records {
		if booking := bookingChanges(record, from, to, closed, told); len(booking.closed) > 0 || len(booking.reopened) > 0 {
			result = append(result, booking)
		}
	}
	return result, nil
}

// bookingChanges compares the booked care days of one booking in [from, to]
// with what its parents were last told.
func bookingChanges(record enrollment.HolidayClosureBookingRecord, from, to calendar.Date, closed closedOn, told map[string]string) changedBooking {
	booking := changedBooking{HolidayClosureBookingRecord: record}
	for day := max(from, record.StartsOn); !day.After(min(to, record.EndsOn)); day = day.AddDays(1) {
		if !isCareWeekday(day) || !bookedDay(record, day) {
			continue
		}
		isClosed, reason, holiday := closed(record.HostSchoolID, day)
		state := told[noticeKey(record, day)]
		switch {
		case holiday:
		case isClosed && state != enrollment.HolidayClosureDayClosed:
			booking.closed = append(booking.closed, enrollment.HolidayClosedDay{Date: day, Reason: reason})
		case !isClosed && state == enrollment.HolidayClosureDayClosed:
			booking.reopened = append(booking.reopened, enrollment.HolidayClosedDay{Date: day})
		}
	}
	return booking
}

// told are the states the parents were last told, keyed by registration,
// week and day.
func (h *HolidayClosures) told(ctx context.Context, organizationID int64, records []enrollment.HolidayClosureBookingRecord) (map[string]string, error) {
	var registrations []int64
	for _, record := range records {
		registrations = append(registrations, record.RegistrationID)
	}
	notices, err := h.store.HolidayClosureNotices(ctx, organizationID, slices.Compact(slices.Sorted(slices.Values(registrations))))
	if err != nil {
		return nil, err
	}
	told := make(map[string]string, len(notices))
	for _, notice := range notices {
		told[fmt.Sprintf("%d:%d:%s", notice.RegistrationID, notice.WeekID, notice.ClosedOn)] = notice.State
	}
	return told, nil
}

func noticeKey(record enrollment.HolidayClosureBookingRecord, day calendar.Date) string {
	return fmt.Sprintf("%d:%d:%s", record.RegistrationID, record.WeekID, day)
}

// isCareWeekday reports a day from Monday to Friday: holiday care has no
// weekend.
func isCareWeekday(day calendar.Date) bool {
	weekday := day.Weekday()
	return weekday != time.Saturday && weekday != time.Sunday
}

// bookedDay reports whether the booking books day: every care day of a
// booking of the whole week, only its days for a booking of days (#4701).
func bookedDay(record enrollment.HolidayClosureBookingRecord, day calendar.Date) bool {
	return len(record.Days) == 0 || slices.Contains(record.Days, day)
}

// familyOf is the recipient address that groups the bookings of one family.
func familyOf(booking changedBooking) string {
	return strings.ToLower(strings.TrimSpace(booking.GuardianEmail))
}

// holidayClosureImpact sums the bookings up per week and names the token of
// exactly these bookings and days.
func holidayClosureImpact(bookings []changedBooking) enrollment.HolidayClosureImpact {
	impact := enrollment.HolidayClosureImpact{Weeks: []enrollment.HolidayClosureWeek{}, Token: holidayClosureToken(bookings),
		KeepsContribution: enrollment.HolidayClosureKeepsContribution}
	weeks := map[int64]int{}
	weekFamilies := map[int64]map[string]bool{}
	children, families := map[int64]bool{}, map[string]bool{}
	addDays := func(into *[]enrollment.HolidayClosedDay, days []enrollment.HolidayClosedDay) {
		for _, day := range days {
			if !slices.ContainsFunc(*into, func(d enrollment.HolidayClosedDay) bool { return d.Date == day.Date }) {
				*into = append(*into, day)
			}
		}
	}
	for _, booking := range bookings {
		i, ok := weeks[booking.WeekID]
		if !ok {
			i = len(impact.Weeks)
			weeks[booking.WeekID] = i
			weekFamilies[booking.WeekID] = map[string]bool{}
			impact.Weeks = append(impact.Weeks, enrollment.HolidayClosureWeek{
				WeekID: booking.WeekID, PeriodID: booking.PeriodID, PeriodName: booking.PeriodName, StartsOn: booking.StartsOn,
				EndsOn: booking.EndsOn, HostSchoolID: booking.HostSchoolID, ClosedDays: []enrollment.HolidayClosedDay{},
				ReopenedDays: []enrollment.HolidayClosedDay{},
			})
		}
		week := &impact.Weeks[i]
		week.Children++
		if booking.Placement == enrollment.HolidayPlacementWaitlisted {
			week.Waitlisted++
		}
		addDays(&week.ClosedDays, booking.closed)
		addDays(&week.ReopenedDays, booking.reopened)
		weekFamilies[booking.WeekID][familyOf(booking)] = true
		week.Families = len(weekFamilies[booking.WeekID])
		children[booking.ChildID] = true
		families[familyOf(booking)] = true
	}
	byDate := func(a, b enrollment.HolidayClosedDay) int { return a.Date.Compare(b.Date) }
	for i := range impact.Weeks {
		slices.SortFunc(impact.Weeks[i].ClosedDays, byDate)
		slices.SortFunc(impact.Weeks[i].ReopenedDays, byDate)
	}
	impact.Children, impact.Families = len(children), len(families)
	return impact
}

// holidayClosureToken fingerprints the bookings with their placement and
// changed days; any change of who is told what changes it. It detects a
// change between preview and save and guards nothing secret, so a 64-bit
// FNV hash suffices.
func holidayClosureToken(bookings []changedBooking) string {
	lines := make([]string, 0, len(bookings))
	dates := func(days []enrollment.HolidayClosedDay) string {
		list := make([]string, 0, len(days))
		for _, day := range days {
			list = append(list, day.Date.String())
		}
		return strings.Join(list, ",")
	}
	for _, booking := range bookings {
		lines = append(lines, fmt.Sprintf("%d:%d:%d:%s:%q:%s:%s", booking.RegistrationID, booking.ChildID, booking.WeekID, booking.Placement,
			familyOf(booking), dates(booking.closed), dates(booking.reopened)))
	}
	slices.Sort(lines)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(strings.Join(lines, "\n")))
	return strconv.FormatUint(hash.Sum64(), 16)
}

// notify enqueues one mail per family under operationID, records it and
// records every told day; the caller holds the notice lock.
func (h *HolidayClosures) notify(ctx context.Context, organizationID int64, operationID string, bookings []changedBooking) (enrollment.HolidayClosureNotice, error) {
	if h.mail == nil {
		return enrollment.HolidayClosureNotice{}, errHolidayClosureMailRequired
	}
	traeger, err := h.traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayClosureNotice{}, fmt.Errorf("enrollment: read the Träger of a closure notice: %w", err)
	}
	candidate := make([]byte, 32)
	if err := h.random(candidate); err != nil {
		return enrollment.HolidayClosureNotice{}, fmt.Errorf("enrollment: draw the family key: %w", err)
	}
	secret, err := h.store.HolidayClosureFamilyKey(ctx, organizationID, candidate)
	if err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	order, byFamily, notices, children := groupByFamily(bookings)
	var sends []enrollment.HolidayClosureSendRecord
	for _, family := range order {
		familyKey := h.keyed(secret, []byte(strconv.FormatInt(organizationID, 10)+":"+family))
		key := "holiday-closure-op:" + operationID + ":" + familyKey
		if err := h.enqueue(ctx, organizationID, traeger, key, byFamily[family]); err != nil {
			return enrollment.HolidayClosureNotice{}, err
		}
		for _, booking := range byFamily[family] {
			sends = append(sends, enrollment.HolidayClosureSendRecord{OperationID: operationID, FamilyKey: familyKey,
				RegistrationID: booking.RegistrationID, IdempotencyKey: key})
		}
	}
	if err := h.store.RecordHolidayClosureSends(ctx, organizationID, sends); err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	if err := h.store.SetHolidayClosureNotices(ctx, organizationID, operationID, notices); err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	return enrollment.HolidayClosureNotice{Families: len(order), Children: children}, nil
}

// groupByFamily orders the families as their bookings come, collects each
// family's bookings and the told days, and counts the children.
func groupByFamily(bookings []changedBooking) ([]string, map[string][]changedBooking, []enrollment.HolidayClosureNoticeRecord, int) {
	var order []string
	byFamily := map[string][]changedBooking{}
	var notices []enrollment.HolidayClosureNoticeRecord
	children := map[int64]bool{}
	for _, booking := range bookings {
		family := familyOf(booking)
		if _, seen := byFamily[family]; !seen {
			order = append(order, family)
		}
		byFamily[family] = append(byFamily[family], booking)
		children[booking.ChildID] = true
		notices = append(notices, dayNotices(booking, booking.closed, enrollment.HolidayClosureDayClosed)...)
		notices = append(notices, dayNotices(booking, booking.reopened, enrollment.HolidayClosureDayOpen)...)
	}
	return order, byFamily, uniqueNotices(notices), len(children)
}

// uniqueNotices keeps one state per registration, week and day: two
// children of a registration in one week share it.
func uniqueNotices(notices []enrollment.HolidayClosureNoticeRecord) []enrollment.HolidayClosureNoticeRecord {
	seen := make(map[enrollment.HolidayClosureNoticeRecord]bool, len(notices))
	result := notices[:0]
	for _, notice := range notices {
		if !seen[notice] {
			seen[notice] = true
			result = append(result, notice)
		}
	}
	return result
}

func dayNotices(booking changedBooking, days []enrollment.HolidayClosedDay, state string) []enrollment.HolidayClosureNoticeRecord {
	notices := make([]enrollment.HolidayClosureNoticeRecord, 0, len(days))
	for _, day := range days {
		notices = append(notices, enrollment.HolidayClosureNoticeRecord{RegistrationID: booking.RegistrationID, WeekID: booking.WeekID,
			ClosedOn: day.Date, State: state})
	}
	return notices
}

// enqueue writes the one mail of a family: its children with their weeks,
// the days closed and the days open again. It preserves every registration
// for erasure; key makes the mail of an operation and family unique.
func (h *HolidayClosures) enqueue(ctx context.Context, organizationID int64, traeger enrollment.HolidayTraeger, key string, bookings []changedBooking) error {
	schools := map[int64]string{}
	for _, school := range traeger.Schools {
		schools[school.ID] = school.Name
	}
	first := bookings[0]
	related := first.RegistrationID
	var registrations []int64
	var reasons []string
	closed, reopened := mailChildren{}, mailChildren{}
	for _, booking := range bookings {
		related = min(related, booking.RegistrationID)
		registrations = append(registrations, booking.RegistrationID)
		line := holidayClosureWeekLine(booking, schools)
		waitlisted := booking.Placement == enrollment.HolidayPlacementWaitlisted
		closed.add(booking, line, waitlisted, booking.closed)
		reopened.add(booking, line, waitlisted, booking.reopened)
		for _, day := range booking.closed {
			if day.Reason != "" && !slices.Contains(reasons, day.Reason) {
				reasons = append(reasons, day.Reason)
			}
		}
	}
	payload, err := json.Marshal(enrollment.HolidayClosureMail{
		Recipient: first.GuardianEmail, GuardianName: first.GuardianFirstName + " " + first.GuardianLastName, TraegerName: traeger.Name,
		RegistrationIDs: slices.Compact(slices.Sorted(slices.Values(registrations))),
		Reasons:         reasons, Children: closed.children, ReopenedChildren: reopened.children,
		KeepsContribution: enrollment.HolidayClosureKeepsContribution,
	})
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday closure mail: %w", err)
	}
	if err := h.mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: organizationID, Kind: enrollment.MailKindHolidayClosureNotice,
		Recipient: first.GuardianEmail, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: related,
		IdempotencyKey: key,
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday closure mail: %w", err)
	}
	return nil
}

// mailChildren collects the children of one section of the mail.
type mailChildren struct {
	children []enrollment.HolidayClosureMailChild
	index    map[int64]int
}

func (m *mailChildren) add(booking changedBooking, line string, waitlisted bool, days []enrollment.HolidayClosedDay) {
	if len(days) == 0 {
		return
	}
	if m.index == nil {
		m.index = map[int64]int{}
	}
	i, ok := m.index[booking.ChildID]
	if !ok {
		i = len(m.children)
		m.index[booking.ChildID] = i
		m.children = append(m.children, enrollment.HolidayClosureMailChild{Name: booking.FirstName + " " + booking.LastName, Guest: booking.Guest})
	}
	week := enrollment.HolidayClosureMailWeek{Week: line, Waitlisted: waitlisted}
	for _, day := range days {
		week.ClosedDays = append(week.ClosedDays, germanDay(day.Date))
	}
	m.children[i].Weeks = append(m.children[i].Weeks, week)
}

// holidayClosureWeekLine names a booked week in the mail like the other
// holiday mails: dates, host school and time model.
func holidayClosureWeekLine(booking changedBooking, schools map[int64]string) string {
	place := booking.TimeModelName
	if school := schools[booking.HostSchoolID]; school != "" {
		place = school + ", " + booking.TimeModelName
	}
	return fmt.Sprintf("%s bis %s: %s (%s bis %s Uhr)", booking.StartsOn.Format("02.01.2006"), booking.EndsOn.Format("02.01.2006"),
		place, booking.StartsAt, booking.EndsAt)
}

var germanWeekdays = [...]string{"Sonntag", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag", "Samstag"}

// germanDay is a day as the mail names it, "Montag, 03.08.2026".
func germanDay(day calendar.Date) string {
	return germanWeekdays[day.Weekday()] + ", " + day.Format("02.01.2006")
}
