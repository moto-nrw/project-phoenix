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
	HolidayClosureSendKeys(ctx context.Context, organizationID int64, operationIDs []string) (map[string][]string, error)
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

// PlanHolidayClosureOperations previews the notices of school closing day
// changes without writing anything, in a constant number of statements for
// any number of operations; the impacts follow the operations' order.
func (h *HolidayClosures) PlanHolidayClosureOperations(ctx context.Context, organizationID int64, operations []enrollment.HolidayClosureOperation) ([]enrollment.HolidayClosureImpact, error) {
	today := h.today()
	clipped := make([]enrollment.HolidayClosureOperation, len(operations))
	for i, operation := range operations {
		if !operation.PreserveSpan {
			operation.From = max(operation.From, today)
		}
		clipped[i] = operation
	}
	changes, err := h.operationsChanges(ctx, organizationID, clipped)
	if err != nil {
		return nil, err
	}
	impacts := make([]enrollment.HolidayClosureImpact, len(changes))
	for i, plan := range changes {
		impacts[i] = holidayClosureImpact(plan.bookings)
		impacts[i].Token = plan.token
	}
	return impacts, nil
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
	byOperation, err := h.store.HolidayClosureSendKeys(ctx, organizationID, []string{operation.OperationID})
	keys := byOperation[operation.OperationID]
	if err != nil {
		return enrollment.HolidayClosureOperationResult{}, err
	}
	// Recorded mails and day states committed together. Recovery only
	// retries that content; it never reads or approves a newer calendar.
	if len(keys) > 0 {
		retried, err := h.mail.RetryFailedTraegerEmails(ctx, organizationID, keys)
		return enrollment.HolidayClosureOperationResult{Mails: len(keys), Retried: retried}, err
	}
	plan, err := h.operationChanges(ctx, organizationID, operation)
	if err != nil {
		return enrollment.HolidayClosureOperationResult{}, err
	}
	if operation.Token != "" && operation.Token != plan.token {
		return enrollment.HolidayClosureOperationResult{}, enrollment.ErrHolidayClosureChanged
	}
	var result enrollment.HolidayClosureOperationResult
	if len(plan.bookings) > 0 {
		notice, err := h.notify(ctx, organizationID, operation.OperationID, plan.bookings)
		if err != nil {
			return enrollment.HolidayClosureOperationResult{}, err
		}
		result.Families, result.Mails = notice.Families, notice.Families
	}
	return result, nil
}

// HolidayClosureOperationsDelivery reads all send keys and transport states
// together. A missing recorded mail is failed, never assumed accepted.
func (h *HolidayClosures) HolidayClosureOperationsDelivery(ctx context.Context, organizationID int64, operationIDs []string) (map[string]enrollment.HolidayClosureDelivery, error) {
	result := make(map[string]enrollment.HolidayClosureDelivery, len(operationIDs))
	if len(operationIDs) == 0 {
		return result, nil
	}
	byOperation, err := h.store.HolidayClosureSendKeys(ctx, organizationID, operationIDs)
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, operationKeys := range byOperation {
		keys = append(keys, operationKeys...)
	}
	if len(keys) == 0 {
		return result, nil
	}
	states, err := h.mail.TraegerMailStates(ctx, organizationID, keys)
	if err != nil {
		return nil, fmt.Errorf("enrollment: read the closure mails: %w", err)
	}
	outcomes := make(map[string]string, len(states))
	for _, state := range states {
		outcomes[state.IdempotencyKey] = state.Outcome
	}
	for operationID, operationKeys := range byOperation {
		var delivery enrollment.HolidayClosureDelivery
		for _, key := range operationKeys {
			switch outcome, found := outcomes[key]; {
			case !found, outcome == enrollment.TraegerMailFailed:
				delivery.Failed++
			case outcome == enrollment.TraegerMailSent:
				delivery.Sent++
			case outcome == enrollment.TraegerMailUnknown:
				delivery.Unknown++
			default:
				delivery.Queued++
			}
		}
		result[operationID] = delivery
	}
	return result, nil
}

// operationChanges are the changes of a school closing day operation
// against the calendar the caller read.
// closurePlan binds an approval to eligible bookings and calendar days,
// independently of notices already prepared by another approved operation.
type closurePlan struct {
	bookings []changedBooking
	token    string
}

func (h *HolidayClosures) operationChanges(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (closurePlan, error) {
	changes, err := h.operationsChanges(ctx, organizationID, []enrollment.HolidayClosureOperation{operation})
	if err != nil {
		return closurePlan{}, err
	}
	return changes[0], nil
}

// operationsChanges are the changes of school closing day operations, per
// operation: one calendar read, one booking read and one read of the told
// days for all of them over the span they cover together.
func (h *HolidayClosures) operationsChanges(ctx context.Context, organizationID int64, operations []enrollment.HolidayClosureOperation) ([]closurePlan, error) {
	result := make([]closurePlan, len(operations))
	span, schools, revisions, err := operationsSpan(organizationID, operations)
	if err != nil || len(schools) == 0 {
		return result, err
	}
	noCare, err := h.operation.HolidayClosureNoCareDays(ctx, organizationID, schools, span.from, span.to, revisions)
	if err != nil {
		return nil, err
	}
	all, scope, err := h.changes(ctx, organizationID, schools, span.from, span.to, noCareIndex(noCare, 0))
	if err != nil {
		return nil, err
	}
	for i, operation := range operations {
		if _, ok := noCare[operation.SchoolID]; ok && !operation.To.IsZero() && !operation.To.Before(operation.From) {
			result[i] = closurePlan{bookings: changesWithin(all, operation), token: holidayClosureToken(changesWithin(scope, operation))}
		}
	}
	return result, nil
}

type dateSpan struct{ from, to calendar.Date }

// operationsSpan is the span the operations cover together, their schools
// and the calendar revisions they require. An operation without days adds
// nothing.
func operationsSpan(organizationID int64, operations []enrollment.HolidayClosureOperation) (dateSpan, []int64, map[int64]int64, error) {
	var span dateSpan
	var schools []int64
	revisions := map[int64]int64{}
	if organizationID <= 0 {
		return span, nil, nil, errors.New("enrollment: the Träger is required")
	}
	for _, operation := range operations {
		if operation.SchoolID <= 0 {
			return span, nil, nil, errors.New("enrollment: the Träger and the school are required")
		}
		if operation.To.IsZero() || operation.To.Before(operation.From) {
			continue
		}
		if len(schools) == 0 || operation.From.Before(span.from) {
			span.from = operation.From
		}
		if len(schools) == 0 || operation.To.After(span.to) {
			span.to = operation.To
		}
		schools = append(schools, operation.SchoolID)
		if operation.Revision != nil {
			revisions[operation.SchoolID] = *operation.Revision
		}
	}
	return span, slices.Compact(slices.Sorted(slices.Values(schools))), revisions, nil
}

// changesWithin narrows the changes to the operation's school and days.
func changesWithin(all []changedBooking, operation enrollment.HolidayClosureOperation) []changedBooking {
	var result []changedBooking
	for _, booking := range all {
		if booking.HostSchoolID != operation.SchoolID {
			continue
		}
		narrowed := booking
		narrowed.closed = daysWithinOperation(booking.closed, operation)
		narrowed.reopened = daysWithinOperation(booking.reopened, operation)
		if len(narrowed.closed) > 0 || len(narrowed.reopened) > 0 {
			result = append(result, narrowed)
		}
	}
	return result
}

func daysWithinOperation(days []enrollment.HolidayClosedDay, operation enrollment.HolidayClosureOperation) []enrollment.HolidayClosedDay {
	var result []enrollment.HolidayClosedDay
	for _, day := range days {
		if day.Date.Before(operation.From) || day.Date.After(operation.To) {
			continue
		}
		if (!operation.ConstrainRanges && len(operation.Ranges) == 0) || slices.ContainsFunc(operation.Ranges, func(r enrollment.HolidayClosureRange) bool {
			return !day.Date.Before(r.From) && !day.Date.After(r.To)
		}) {
			result = append(result, day)
		}
	}
	return result
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
func (h *HolidayClosures) changes(ctx context.Context, organizationID int64, schools []int64, from, to calendar.Date, closed closedOn) ([]changedBooking, []changedBooking, error) {
	records, err := h.store.HolidayClosureBookings(ctx, organizationID, schools, from, to)
	if err != nil || len(records) == 0 {
		return nil, nil, err
	}
	told, err := h.told(ctx, organizationID, records)
	if err != nil {
		return nil, nil, err
	}
	var result, scope []changedBooking
	for _, record := range records {
		if booking := bookingChanges(record, from, to, closed, told); len(booking.closed) > 0 || len(booking.reopened) > 0 {
			result = append(result, booking)
		}
		// An approval covers both calendar states of every eligible booked day.
		// Later writes to the told-day ledger may reduce the mail, never widen it.
		approved := bookingChanges(record, from, to, closed, nil)
		for day := max(from, record.StartsOn); !day.After(min(to, record.EndsOn)); day = day.AddDays(1) {
			isClosed, _, holiday := closed(record.HostSchoolID, day)
			if isCareWeekday(day) && bookedDay(record, day) && !isClosed && !holiday {
				approved.reopened = append(approved.reopened, enrollment.HolidayClosedDay{Date: day})
			}
		}
		scope = append(scope, approved)
	}
	return result, scope, nil
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
		// A booking the Büro has not decided on by criteria has no place
		// either (#4700).
		if booking.Placement != enrollment.HolidayPlacementPlaced {
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
		closed.add(booking, line, booking.closed)
		reopened.add(booking, line, booking.reopened)
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
		KeepsContribution: enrollment.HolidayClosureKeepsContribution, ContainsPastDays: hasPastClosureDays(bookings, h.today()),
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

func (m *mailChildren) add(booking changedBooking, line string, days []enrollment.HolidayClosedDay) {
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
	week := enrollment.HolidayClosureMailWeek{Week: line, Waitlisted: booking.Placement == enrollment.HolidayPlacementWaitlisted,
		Pending: booking.Placement == enrollment.HolidayPlacementPending}
	for _, day := range days {
		week.ClosedDays = append(week.ClosedDays, germanDay(day.Date))
		week.Dates = append(week.Dates, day.Date)
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

// hasPastClosureDays marks a notice prepared after one of its confirmed days.
func hasPastClosureDays(bookings []changedBooking, today calendar.Date) bool {
	for _, booking := range bookings {
		for _, days := range [][]enrollment.HolidayClosedDay{booking.closed, booking.reopened} {
			for _, day := range days {
				if day.Date.Before(today) {
					return true
				}
			}
		}
	}
	return false
}
