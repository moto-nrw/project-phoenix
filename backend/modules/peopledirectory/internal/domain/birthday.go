package domain

import (
	"errors"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrBirthdayStaffNotFound reports an account without a staff record.
var ErrBirthdayStaffNotFound = errors.New("staff not found")

// BirthdayKind separates the two populations a birthday display mixes.
type BirthdayKind string

const (
	BirthdayKindStudent BirthdayKind = "student"
	BirthdayKindStaff   BirthdayKind = "staff"
)

// MonthDay is an annually recurring calendar day.
type MonthDay struct {
	Month time.Month
	Day   int
}

func MonthDayOf(d calendar.Date) MonthDay { return MonthDay{Month: d.Month(), Day: d.Day()} }

// BirthdayEntry is one person having a birthday on one of the queried days.
// Birthday carries the full stored date because the age is derived from it for
// children; staff-facing consumers drop the year before it reaches a screen.
type BirthdayEntry struct {
	Kind        BirthdayKind
	ID          int64
	FirstName   string
	LastName    string
	Birthday    calendar.Date
	GroupName   string
	SchoolClass string
}

func (e BirthdayEntry) FullName() string {
	if e.FirstName == "" {
		return e.LastName
	}
	if e.LastName == "" {
		return e.FirstName
	}
	return e.FirstName + " " + e.LastName
}

// BirthdayStaff is the staff row behind an account.
type BirthdayStaff struct {
	ID     int64
	OptOut bool
}

// BirthdayCelebration is one person celebrating on one concrete calendar day.
// Date is the day the celebration is SHOWN on in the viewed week, which is not
// always the stored birth date: 29 February falls on 1 March in a common year.
type BirthdayCelebration struct {
	Kind        BirthdayKind
	ID          int64
	Name        string
	GroupName   string
	SchoolClass string
	Date        calendar.Date
	// Age is the age reached, in completed years. Only children carry it: a
	// colleague's age is not published to the team, which is what the opt-out
	// exists to prevent. Zero means "not disclosed".
	Age int
	// IsToday separates today's birthdays from the rest of the week, so the UI
	// can highlight them without re-deriving it.
	IsToday bool
}

// BirthdayVisibility decides whether the caller may see children at all.
type BirthdayVisibility interface{ HasFullAccess() bool }

// VisibleBirthdayStudents drops every child the caller may not see.
func VisibleBirthdayStudents(entries []BirthdayEntry, visibility BirthdayVisibility) []BirthdayEntry {
	visible := make([]BirthdayEntry, 0, len(entries))
	if visibility == nil || !visibility.HasFullAccess() {
		return visible
	}
	return append(visible, entries...)
}

// BirthdayWeekReach is how many weeks the display may step back or ahead of
// the current one (#3777). The OGS celebrates after the actual day, so the
// previous weeks matter as much as the coming ones; beyond a month the card
// would turn into a year-round staff birthday calendar, which is what the
// administrative export exists for.
const BirthdayWeekReach = 4

// ErrBirthdayWeekOutOfRange reports a requested week beyond BirthdayWeekReach.
var ErrBirthdayWeekOutOfRange = errors.New("birthday week out of range")

// BirthdayWeek is the Monday-to-Sunday week one birthday view speaks for. The
// weekend belongs to it: nobody opens the app on Saturday, and a child born on
// a Saturday is celebrated during the week around it.
type BirthdayWeek struct {
	Start calendar.Date
	End   calendar.Date
}

// WeekStartOf returns the Monday of the week containing date.
func WeekStartOf(date calendar.Date) calendar.Date {
	sinceMonday := (int(date.Weekday()) + 6) % 7
	return date.AddDays(-sinceMonday)
}

// ResolveBirthdayWeek picks the week of weekOf (nil means the current week)
// and rejects one further than BirthdayWeekReach weeks from today's week.
func ResolveBirthdayWeek(today calendar.Date, weekOf *calendar.Date) (BirthdayWeek, error) {
	current := WeekStartOf(today)
	start := current
	if weekOf != nil {
		start = WeekStartOf(*weekOf)
	}
	earliest, latest := BirthdayWeekBounds(today)
	if start.Before(earliest) || start.After(latest) {
		return BirthdayWeek{}, ErrBirthdayWeekOutOfRange
	}
	return BirthdayWeek{Start: start, End: start.AddDays(6)}, nil
}

// BirthdayWeekBounds returns the Mondays of the earliest and latest week a
// view may show.
func BirthdayWeekBounds(today calendar.Date) (calendar.Date, calendar.Date) {
	current := WeekStartOf(today)
	return current.AddDays(-7 * BirthdayWeekReach), current.AddDays(7 * BirthdayWeekReach)
}

// CelebrationDates are the calendar days one week view speaks for.
func CelebrationDates(week BirthdayWeek) []calendar.Date {
	dates := make([]calendar.Date, 0, 7)
	for date := week.Start; !date.After(week.End); date = date.AddDays(1) {
		dates = append(dates, date)
	}
	return dates
}

// MonthDaysFor maps a shown date to the recurring birth days it stands for.
func MonthDaysFor(date calendar.Date) []MonthDay {
	days := []MonthDay{{Month: date.Month(), Day: date.Day()}}
	if date.Month() == time.March && date.Day() == 1 && !isLeapYear(date.Year()) {
		days = append(days, MonthDay{Month: time.February, Day: 29})
	}
	return days
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}

// BirthdayWindow returns the recurring days a week view speaks for and the
// date each one is shown on.
func BirthdayWindow(week BirthdayWeek) ([]MonthDay, map[MonthDay]calendar.Date) {
	dates := CelebrationDates(week)
	byMonthDay := make(map[MonthDay]calendar.Date, len(dates)*2)
	days := make([]MonthDay, 0, len(dates)*2)
	for _, date := range dates {
		for _, day := range MonthDaysFor(date) {
			if _, seen := byMonthDay[day]; seen {
				continue
			}
			byMonthDay[day] = date
			days = append(days, day)
		}
	}
	return days, byMonthDay
}

func BuildCelebrations(entries []BirthdayEntry, byMonthDay map[MonthDay]calendar.Date, today calendar.Date) []BirthdayCelebration {
	celebrations := make([]BirthdayCelebration, 0, len(entries))
	for _, entry := range entries {
		shownOn, ok := byMonthDay[MonthDayOf(entry.Birthday)]
		if !ok {
			// The query only returns the days we asked for; a miss would mean
			// a mismatch between filter and mapping, and inventing a date here
			// would hide it.
			continue
		}
		celebration := BirthdayCelebration{
			Kind: entry.Kind, ID: entry.ID, Name: entry.FullName(),
			GroupName: entry.GroupName, SchoolClass: entry.SchoolClass,
			Date: shownOn, IsToday: shownOn == today,
		}
		if entry.Kind == BirthdayKindStudent {
			celebration.Age = shownOn.Year() - entry.Birthday.Year()
		}
		celebrations = append(celebrations, celebration)
	}
	sort.SliceStable(celebrations, func(i, j int) bool {
		a, b := celebrations[i], celebrations[j]
		// Calendar order: the card reads as the week from Monday to Sunday.
		if a.Date != b.Date {
			return a.Date.Before(b.Date)
		}
		if a.Kind != b.Kind {
			return a.Kind == BirthdayKindStudent
		}
		return a.Name < b.Name
	})
	return celebrations
}

// StaffBirthdayList filters the staff by birth month (empty = all) and sorts
// them as a calendar.
func StaffBirthdayList(persons []BirthdayEntry, months map[time.Month]bool) []BirthdayEntry {
	filtered := persons[:0]
	for _, person := range persons {
		if len(months) > 0 && !months[person.Birthday.Month()] {
			continue
		}
		filtered = append(filtered, person)
	}
	sort.SliceStable(filtered, func(i, j int) bool {
		a, b := filtered[i], filtered[j]
		if a.Birthday.Month() != b.Birthday.Month() {
			return a.Birthday.Month() < b.Birthday.Month()
		}
		if a.Birthday.Day() != b.Birthday.Day() {
			return a.Birthday.Day() < b.Birthday.Day()
		}
		return a.FullName() < b.FullName()
	})
	return filtered
}
