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

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days in booked holiday weeks (#6434); the rules are on
// enrollment.HolidayClosures. Preview and notice share one computation, so
// what the Büro saw is what the parents are told: the save confirms the
// token of the preview and is refused when the bookings moved in between.

// HolidayClosureStore reads the bookings of a closure and records the
// closed days the parents were told about.
type HolidayClosureStore interface {
	LockHolidayClosureNotices(ctx context.Context, organizationID int64) error
	HolidayClosureBookings(ctx context.Context, organizationID int64, schoolIDs []int64, from, to calendar.Date) ([]enrollment.HolidayClosureBookingRecord, error)
	HolidayClosureNotices(ctx context.Context, organizationID int64, registrationIDs []int64) ([]enrollment.HolidayClosureNoticeRecord, error)
	RecordHolidayClosureNotices(ctx context.Context, organizationID int64, notices []enrollment.HolidayClosureNoticeRecord) error
}

// errHolidayClosureMailRequired reports a notice without the outbox.
var errHolidayClosureMailRequired = errors.New("enrollment: telling the parents about a closing day needs the mail outbox")

// HolidayClosures serves enrollment.HolidayClosures.
type HolidayClosures struct {
	store    HolidayClosureStore
	calendar enrollment.HolidaySchoolCalendar
	traeger  enrollment.HolidayTraegerDirectory
	mail     enrollment.TraegerMailOutbox
	today    func() calendar.Date
}

var _ enrollment.HolidayClosures = (*HolidayClosures)(nil)

// NewHolidayClosures builds the notices over store. calendar names the
// holidays and closing days of the host schools, traeger the Träger and its
// schools in the mail, mail the outbox; today defaults to the Berlin day.
func NewHolidayClosures(store HolidayClosureStore, schoolCalendar enrollment.HolidaySchoolCalendar, traeger enrollment.HolidayTraegerDirectory,
	mail enrollment.TraegerMailOutbox, today func() calendar.Date,
) *HolidayClosures {
	if store == nil || schoolCalendar == nil || traeger == nil || mail == nil {
		panic("enrollment: the holiday closure store, the host schools' calendar, the Träger directory and the mail outbox are required")
	}
	if today == nil {
		today = calendar.TodayDate
	}
	return &HolidayClosures{store: store, calendar: schoolCalendar, traeger: traeger, mail: mail, today: today}
}

// closedBooking is a booking with its closed days nobody told its parents.
type closedBooking struct {
	enrollment.HolidayClosureBookingRecord
	closed []enrollment.HolidayClosedDay
}

// HolidayClosureImpact previews the bookings closure affects.
func (h *HolidayClosures) HolidayClosureImpact(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure) (enrollment.HolidayClosureImpact, error) {
	bookings, err := h.proposed(ctx, organizationID, closure)
	if err != nil {
		return enrollment.HolidayClosureImpact{}, err
	}
	return holidayClosureImpact(bookings), nil
}

// NotifyHolidayClosure tells the parents of the bookings closure affects.
func (h *HolidayClosures) NotifyHolidayClosure(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure, token string) (enrollment.HolidayClosureNotice, error) {
	return h.notify(ctx, organizationID, token, func() ([]closedBooking, error) {
		return h.proposed(ctx, organizationID, closure)
	})
}

// PendingHolidayClosures lists the closed days of booked weeks nobody told.
func (h *HolidayClosures) PendingHolidayClosures(ctx context.Context, organizationID int64) (enrollment.HolidayClosureImpact, error) {
	bookings, err := h.pending(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayClosureImpact{}, err
	}
	return holidayClosureImpact(bookings), nil
}

// NotifyPendingHolidayClosures tells the parents of the pending days.
func (h *HolidayClosures) NotifyPendingHolidayClosures(ctx context.Context, organizationID int64, token string) (enrollment.HolidayClosureNotice, error) {
	return h.notify(ctx, organizationID, token, func() ([]closedBooking, error) {
		return h.pending(ctx, organizationID)
	})
}

// proposed are the bookings at the closure's schools with a day of the
// closure in their week that is a weekday from today on, no statutory
// holiday of the host school and not told before.
func (h *HolidayClosures) proposed(ctx context.Context, organizationID int64, closure enrollment.HolidayClosure) ([]closedBooking, error) {
	if organizationID <= 0 {
		return nil, errors.New("enrollment: the Träger is required")
	}
	schools := slices.Compact(slices.Sorted(slices.Values(closure.SchoolIDs)))
	if len(schools) == 0 || closure.StartsOn.IsZero() || closure.EndsOn.IsZero() || closure.EndsOn.Before(closure.StartsOn) {
		return nil, nil
	}
	from := max(closure.StartsOn, h.today())
	if closure.EndsOn.Before(from) {
		// Authorize the schools all the same: a refusal must not depend on
		// the dates.
		_, err := h.calendar.HolidayNoCareDays(ctx, schools, closure.StartsOn, closure.StartsOn)
		return nil, err
	}
	// The calendar authorizes every school before it reads anything.
	noCare, err := h.calendar.HolidayNoCareDays(ctx, schools, from, closure.EndsOn)
	if err != nil {
		return nil, err
	}
	records, err := h.store.HolidayClosureBookings(ctx, organizationID, schools, from, closure.EndsOn)
	if err != nil {
		return nil, err
	}
	told, err := h.told(ctx, organizationID, records)
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(closure.Reason)
	return closedBookings(records, func(record enrollment.HolidayClosureBookingRecord) []enrollment.HolidayClosedDay {
		var days []enrollment.HolidayClosedDay
		for day := max(from, record.StartsOn); !day.After(min(closure.EndsOn, record.EndsOn)); day = day.AddDays(1) {
			if isCareWeekday(day) && !isHoliday(noCare[record.HostSchoolID], day) && !told[noticeKey(record, day)] {
				days = append(days, enrollment.HolidayClosedDay{Date: day, Reason: reason})
			}
		}
		return days
	}), nil
}

// pending are the bookings whose week has a closing day of its host school,
// from today on, that nobody told the parents about.
func (h *HolidayClosures) pending(ctx context.Context, organizationID int64) ([]closedBooking, error) {
	if organizationID <= 0 {
		return nil, errors.New("enrollment: the Träger is required")
	}
	today := h.today()
	records, err := h.store.HolidayClosureBookings(ctx, organizationID, nil, today, calendar.Date("9999-12-31"))
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	spans := make([]holidayWeekSpan, 0, len(records))
	for _, record := range records {
		spans = append(spans, holidayWeekSpan{StartsOn: max(record.StartsOn, today), EndsOn: record.EndsOn, HostSchoolID: record.HostSchoolID})
	}
	noCare, err := holidayNoCareDays(ctx, h.calendar, spans)
	if err != nil {
		return nil, err
	}
	told, err := h.told(ctx, organizationID, records)
	if err != nil {
		return nil, err
	}
	return closedBookings(records, func(record enrollment.HolidayClosureBookingRecord) []enrollment.HolidayClosedDay {
		var days []enrollment.HolidayClosedDay
		for _, day := range noCare[record.HostSchoolID] {
			if day.Kind == enrollment.HolidayNoCareClosingDay && !day.Date.Before(max(record.StartsOn, today)) && !day.Date.After(record.EndsOn) &&
				isCareWeekday(day.Date) && !told[noticeKey(record, day.Date)] {
				days = append(days, enrollment.HolidayClosedDay{Date: day.Date, Reason: day.Name})
			}
		}
		return days
	}), nil
}

func closedBookings(records []enrollment.HolidayClosureBookingRecord, closed func(enrollment.HolidayClosureBookingRecord) []enrollment.HolidayClosedDay) []closedBooking {
	var result []closedBooking
	for _, record := range records {
		if days := closed(record); len(days) > 0 {
			result = append(result, closedBooking{HolidayClosureBookingRecord: record, closed: days})
		}
	}
	return result
}

// told are the days told before, keyed by registration, week and day.
func (h *HolidayClosures) told(ctx context.Context, organizationID int64, records []enrollment.HolidayClosureBookingRecord) (map[string]bool, error) {
	var registrations []int64
	for _, record := range records {
		registrations = append(registrations, record.RegistrationID)
	}
	notices, err := h.store.HolidayClosureNotices(ctx, organizationID, slices.Compact(slices.Sorted(slices.Values(registrations))))
	if err != nil {
		return nil, err
	}
	told := make(map[string]bool, len(notices))
	for _, notice := range notices {
		told[fmt.Sprintf("%d:%d:%s", notice.RegistrationID, notice.WeekID, notice.ClosedOn)] = true
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

func isHoliday(days []enrollment.HolidayNoCareDay, day calendar.Date) bool {
	return slices.ContainsFunc(days, func(none enrollment.HolidayNoCareDay) bool {
		return none.Date == day && none.Kind == enrollment.HolidayNoCareHoliday
	})
}

// familyOf is the recipient address that groups the bookings of one family.
func familyOf(booking closedBooking) string {
	return strings.ToLower(strings.TrimSpace(booking.GuardianEmail))
}

// holidayClosureImpact sums the bookings up per week and names the token of
// exactly these bookings and days.
func holidayClosureImpact(bookings []closedBooking) enrollment.HolidayClosureImpact {
	impact := enrollment.HolidayClosureImpact{Weeks: []enrollment.HolidayClosureWeek{}, Token: holidayClosureToken(bookings),
		KeepsContribution: enrollment.HolidayClosureKeepsContribution}
	weeks := map[int64]int{}
	weekFamilies := map[int64]map[string]bool{}
	children, families := map[int64]bool{}, map[string]bool{}
	for _, booking := range bookings {
		i, ok := weeks[booking.WeekID]
		if !ok {
			i = len(impact.Weeks)
			weeks[booking.WeekID] = i
			weekFamilies[booking.WeekID] = map[string]bool{}
			impact.Weeks = append(impact.Weeks, enrollment.HolidayClosureWeek{
				WeekID: booking.WeekID, PeriodID: booking.PeriodID, PeriodName: booking.PeriodName, StartsOn: booking.StartsOn,
				EndsOn: booking.EndsOn, HostSchoolID: booking.HostSchoolID, ClosedDays: []enrollment.HolidayClosedDay{},
			})
		}
		week := &impact.Weeks[i]
		week.Children++
		if booking.Placement == enrollment.HolidayPlacementWaitlisted {
			week.Waitlisted++
		}
		for _, day := range booking.closed {
			if !slices.ContainsFunc(week.ClosedDays, func(d enrollment.HolidayClosedDay) bool { return d.Date == day.Date }) {
				week.ClosedDays = append(week.ClosedDays, day)
			}
		}
		weekFamilies[booking.WeekID][familyOf(booking)] = true
		week.Families = len(weekFamilies[booking.WeekID])
		children[booking.ChildID] = true
		families[familyOf(booking)] = true
	}
	for i := range impact.Weeks {
		slices.SortFunc(impact.Weeks[i].ClosedDays, func(a, b enrollment.HolidayClosedDay) int { return a.Date.Compare(b.Date) })
	}
	impact.Children, impact.Families = len(children), len(families)
	return impact
}

// holidayClosureToken fingerprints the bookings with their placement and
// closed days; any change of who is told what changes it. It detects a
// change between preview and save and guards nothing secret, so a 64-bit
// FNV hash suffices.
func holidayClosureToken(bookings []closedBooking) string {
	lines := make([]string, 0, len(bookings))
	for _, booking := range bookings {
		days := make([]string, 0, len(booking.closed))
		for _, day := range booking.closed {
			days = append(days, day.Date.String())
		}
		lines = append(lines, fmt.Sprintf("%d:%d:%d:%s:%q:%s", booking.RegistrationID, booking.ChildID, booking.WeekID, booking.Placement, familyOf(booking), strings.Join(days, ",")))
	}
	slices.Sort(lines)
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(strings.Join(lines, "\n")))
	return strconv.FormatUint(hash.Sum64(), 16)
}

// notify checks token against the bookings, enqueues one mail per family
// and records every told day.
func (h *HolidayClosures) notify(ctx context.Context, organizationID int64, token string, load func() ([]closedBooking, error)) (enrollment.HolidayClosureNotice, error) {
	// Hold the owner-local gate until the caller's transaction ends. Read
	// told days only after earlier notifications have committed.
	if err := h.store.LockHolidayClosureNotices(ctx, organizationID); err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	bookings, err := load()
	if err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	if len(bookings) == 0 {
		return enrollment.HolidayClosureNotice{}, nil
	}
	if token != holidayClosureToken(bookings) {
		return enrollment.HolidayClosureNotice{}, enrollment.ErrHolidayClosureChanged
	}
	if h.mail == nil {
		return enrollment.HolidayClosureNotice{}, errHolidayClosureMailRequired
	}
	traeger, err := h.traeger.HolidayTraeger(ctx, organizationID)
	if err != nil {
		return enrollment.HolidayClosureNotice{}, fmt.Errorf("enrollment: read the Träger of a closure notice: %w", err)
	}
	var order []string
	byFamily := map[string][]closedBooking{}
	var notices []enrollment.HolidayClosureNoticeRecord
	children := map[int64]bool{}
	for _, booking := range bookings {
		family := familyOf(booking)
		if _, seen := byFamily[family]; !seen {
			order = append(order, family)
		}
		byFamily[family] = append(byFamily[family], booking)
		children[booking.ChildID] = true
		for _, day := range booking.closed {
			notices = append(notices, enrollment.HolidayClosureNoticeRecord{RegistrationID: booking.RegistrationID, WeekID: booking.WeekID, ClosedOn: day.Date})
		}
	}
	for _, family := range order {
		if err := h.enqueue(ctx, organizationID, traeger, byFamily[family]); err != nil {
			return enrollment.HolidayClosureNotice{}, err
		}
	}
	if err := h.store.RecordHolidayClosureNotices(ctx, organizationID, notices); err != nil {
		return enrollment.HolidayClosureNotice{}, err
	}
	return enrollment.HolidayClosureNotice{Families: len(order), Children: len(children)}, nil
}

// enqueue writes the one mail of a family: its children with their weeks
// and closed days. It preserves every registration for erasure, and its key
// the bookings and days, so the same notice never goes out twice.
func (h *HolidayClosures) enqueue(ctx context.Context, organizationID int64, traeger enrollment.HolidayTraeger, bookings []closedBooking) error {
	schools := map[int64]string{}
	for _, school := range traeger.Schools {
		schools[school.ID] = school.Name
	}
	first := bookings[0]
	related := first.RegistrationID
	var registrations []int64
	var reasons []string
	var mailChildren []enrollment.HolidayClosureMailChild
	index := map[int64]int{}
	for _, booking := range bookings {
		related = min(related, booking.RegistrationID)
		registrations = append(registrations, booking.RegistrationID)
		i, ok := index[booking.ChildID]
		if !ok {
			i = len(mailChildren)
			index[booking.ChildID] = i
			mailChildren = append(mailChildren, enrollment.HolidayClosureMailChild{Name: booking.FirstName + " " + booking.LastName, Guest: booking.Guest})
		}
		week := enrollment.HolidayClosureMailWeek{Week: holidayClosureWeekLine(booking, schools), Waitlisted: booking.Placement == enrollment.HolidayPlacementWaitlisted}
		for _, day := range booking.closed {
			week.ClosedDays = append(week.ClosedDays, germanDay(day.Date))
			if day.Reason != "" && !slices.Contains(reasons, day.Reason) {
				reasons = append(reasons, day.Reason)
			}
		}
		mailChildren[i].Weeks = append(mailChildren[i].Weeks, week)
	}
	payload, err := json.Marshal(enrollment.HolidayClosureMail{
		Recipient: first.GuardianEmail, GuardianName: first.GuardianFirstName + " " + first.GuardianLastName, TraegerName: traeger.Name,
		RegistrationIDs: slices.Compact(slices.Sorted(slices.Values(registrations))),
		Reasons:         reasons, Children: mailChildren, KeepsContribution: enrollment.HolidayClosureKeepsContribution,
	})
	if err != nil {
		return fmt.Errorf("enrollment: encode holiday closure mail: %w", err)
	}
	if err := h.mail.EnqueueTraegerMail(ctx, enrollment.TraegerMail{
		OrganizationID: organizationID, Kind: enrollment.MailKindHolidayClosureNotice,
		Recipient: first.GuardianEmail, Payload: payload,
		RelatedType: enrollment.MailRelatedHolidayRegistration, RelatedID: related,
		IdempotencyKey: "holiday-closure:" + strconv.FormatInt(related, 10) + ":" + holidayClosureToken(bookings),
	}); err != nil {
		return fmt.Errorf("enrollment: enqueue holiday closure mail: %w", err)
	}
	return nil
}

// holidayClosureWeekLine names a booked week in the mail like the other
// holiday mails: dates, host school and time model.
func holidayClosureWeekLine(booking closedBooking, schools map[int64]string) string {
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
