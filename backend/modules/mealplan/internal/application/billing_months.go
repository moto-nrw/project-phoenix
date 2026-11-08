package application

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// billingMonthsBack is how many months back the Mensa-Monatsfreigabe goes:
// the current month and the eleven before it.
const billingMonthsBack = 12

// WeekReader reads the kitchen list of the week of a Monday: who is
// registered for lunch on each weekday after every cancellation.
type WeekReader func(ctx context.Context, monday mealplan.Date) (mealplan.WeeklyList, error)

// BillingMonths serves mealplan.BillingMonths. Every operation runs in the
// caller's school transaction, which the route opens.
type BillingMonths struct {
	store         ports.BillingMonthStore
	weeks         WeekReader
	calendar      mealplan.CalendarFinder
	presence      mealplan.PresenceFinder
	availableFrom mealplan.AttendanceAvailableFrom
	today         func() calendar.Date
}

var _ mealplan.BillingMonths = (*BillingMonths)(nil)

// NewBillingMonths builds the Mensa-Monatsfreigabe.
func NewBillingMonths(store ports.BillingMonthStore, weeks WeekReader, calendarFinder mealplan.CalendarFinder,
	presence mealplan.PresenceFinder, availableFrom mealplan.AttendanceAvailableFrom, today func() calendar.Date,
) *BillingMonths {
	if store == nil || weeks == nil || calendarFinder == nil || presence == nil || availableFrom == nil || today == nil {
		panic("meal plan billing months: all dependencies are required")
	}
	return &BillingMonths{store: store, weeks: weeks, calendar: calendarFinder, presence: presence, availableFrom: availableFrom, today: today}
}

func monthStart(day calendar.Date) calendar.Date {
	return calendar.NewDate(day.Year(), day.Month(), 1)
}

func monthLast(first calendar.Date) calendar.Date {
	return calendar.NewDate(first.Year(), first.Month()+1, 1).AddDays(-1)
}

// window is the current month and the first of the eleven before it.
func (b *BillingMonths) window() (earliest, current calendar.Date) {
	current = monthStart(b.today())
	return calendar.NewDate(current.Year(), current.Month()-(billingMonthsBack-1), 1), current
}

// parseBillingMonth accepts the first day of the current month or of one of
// the eleven before it.
func (b *BillingMonths) parseBillingMonth(value mealplan.Date) (calendar.Date, error) {
	month, err := calendar.ParseDate(string(value))
	earliest, current := b.window()
	if err != nil || month.Day() != 1 || month.After(current) || month.Before(earliest) {
		return "", mealplan.ErrInvalidBillingMonth
	}
	return month, nil
}

// statusOf is running until the last day of the month has passed.
func (b *BillingMonths) statusOf(month calendar.Date) mealplan.BillingMonthStatus {
	if monthLast(month).Before(b.today()) {
		return mealplan.BillingMonthOpen
	}
	return mealplan.BillingMonthRunning
}

// BillingMonthOverview lists the current month and the eleven before it,
// the latest first.
func (b *BillingMonths) BillingMonthOverview(ctx context.Context) ([]mealplan.BillingMonthSummary, error) {
	earliest, current := b.window()
	stored, err := b.store.ReleasedMonths(ctx, earliest, current)
	if err != nil {
		return nil, err
	}
	released := make(map[calendar.Date]domain.StoredBillingMonth, len(stored))
	for _, month := range stored {
		released[month.Month] = month
	}
	availableFrom, err := b.attendanceFrom(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]mealplan.BillingMonthSummary, 0, billingMonthsBack)
	for month := current; !month.Before(earliest); month = monthStart(month.AddDays(-1)) {
		summary := mealplan.BillingMonthSummary{Month: mealplan.Date(month.String()), Status: b.statusOf(month)}
		if month.Before(availableFrom) {
			summary.Status = mealplan.BillingMonthExpired
		}
		if entry, ok := released[month]; ok {
			releasedAt, children, meals := entry.ReleasedAt, entry.ChildCount, entry.MealCount
			summary.Status, summary.ReleasedAt, summary.ChildCount, summary.MealCount = mealplan.BillingMonthReleased, &releasedAt, &children, &meals
		}
		result = append(result, summary)
	}
	return result, nil
}

// BillingMonth shows a released month as stored, any other month as
// computed now.
func (b *BillingMonths) BillingMonth(ctx context.Context, value mealplan.Date) (mealplan.BillingMonth, error) {
	month, err := b.parseBillingMonth(value)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	return b.monthView(ctx, month)
}

// monthView is the stored release of a month or, before it, the month as
// computed now.
func (b *BillingMonths) monthView(ctx context.Context, month calendar.Date) (mealplan.BillingMonth, error) {
	stored, released, err := b.store.ReleasedMonth(ctx, month)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	if released {
		return releasedView(stored), nil
	}
	availableFrom, err := b.attendanceFrom(ctx)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	if month.Before(availableFrom) {
		return mealplan.BillingMonth{Month: mealplan.Date(month.String()), Status: mealplan.BillingMonthExpired,
			Children: []mealplan.BillingMonthChild{}}, nil
	}
	return b.compute(ctx, month)
}

// attendanceFrom is the earliest day whose attendance the school still
// keeps. Expired evidence and absence look the same, so a month that starts
// before it is not counted, even when the cleanup has not run yet.
func (b *BillingMonths) attendanceFrom(ctx context.Context) (calendar.Date, error) {
	availableFrom, err := b.availableFrom(ctx)
	if err != nil {
		return "", fmt.Errorf("meal plan billing months: read the attendance window: %w", err)
	}
	if availableFrom == "" {
		return "", errors.New("meal plan billing months: the attendance window is empty")
	}
	return availableFrom, nil
}

// ReleaseBillingMonth computes the month again and stores it when it still
// shows the children and meals the OGS-Leitung saw.
func (b *BillingMonths) ReleaseBillingMonth(ctx context.Context, release mealplan.BillingMonthRelease) (mealplan.BillingMonth, error) {
	month, organizationID, err := b.releasable(ctx, release)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	computed, err := b.compute(ctx, month)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	if computed.ChildCount != release.ChildCount || computed.MealCount != release.MealCount ||
		computed.Fingerprint != release.Fingerprint {
		return mealplan.BillingMonth{}, mealplan.ErrBillingMonthChanged
	}
	stored := storedMonth(month, computed)
	if stored.ReleasedAt, err = b.store.InsertRelease(ctx, organizationID, release.AccountID, stored); err != nil {
		return mealplan.BillingMonth{}, err
	}
	result := releasedView(stored)
	result.SchoolDays = computed.SchoolDays
	return result, nil
}

// releasable checks a release before it counts: a month of the window that
// has ended, is not released yet, of a school with a Träger.
func (b *BillingMonths) releasable(ctx context.Context, release mealplan.BillingMonthRelease) (calendar.Date, int64, error) {
	month, err := b.parseBillingMonth(release.Month)
	switch {
	case err != nil:
		return "", 0, err
	case release.AccountID <= 0:
		return "", 0, errors.New("meal plan billing months: the releasing account is required")
	case b.statusOf(month) != mealplan.BillingMonthOpen:
		return "", 0, mealplan.ErrBillingMonthRunning
	}
	_, released, err := b.store.ReleasedMonth(ctx, month)
	if err != nil {
		return "", 0, err
	}
	if released {
		return "", 0, mealplan.ErrBillingMonthReleased
	}
	organizationID, ok, err := b.store.SchoolOrganization(ctx)
	if err != nil {
		return "", 0, err
	}
	if !ok {
		return "", 0, mealplan.ErrBillingMonthNoTraeger
	}
	return month, organizationID, nil
}

func storedMonth(month calendar.Date, computed mealplan.BillingMonth) domain.StoredBillingMonth {
	stored := domain.StoredBillingMonth{Month: month, ChildCount: computed.ChildCount, MealCount: computed.MealCount,
		Children: make([]domain.BillingChild, len(computed.Children))}
	for i, child := range computed.Children {
		stored.Children[i] = domain.BillingChild{StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, RegisteredDays: child.RegisteredDays, MealDays: child.MealDays}
	}
	return stored
}

func releasedView(stored domain.StoredBillingMonth) mealplan.BillingMonth {
	releasedAt := stored.ReleasedAt
	children := make([]mealplan.BillingMonthChild, len(stored.Children))
	view := mealplan.BillingMonth{Month: mealplan.Date(stored.Month.String()), Status: mealplan.BillingMonthReleased,
		ReleasedAt: &releasedAt, ChildCount: stored.ChildCount, MealCount: stored.MealCount, Children: children}
	for i, child := range stored.Children {
		children[i] = mealplan.BillingMonthChild{StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, RegisteredDays: child.RegisteredDays, MealDays: child.MealDays, AbsentDates: []mealplan.Date{}}
		view.AbsenceCount += child.RegisteredDays - child.MealDays
	}
	return view
}

// compute reads the kitchen lists, the school holidays and the attendance
// of the month and counts them.
func (b *BillingMonths) compute(ctx context.Context, month calendar.Date) (mealplan.BillingMonth, error) {
	availableFrom, err := b.attendanceFrom(ctx)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	if month.Before(availableFrom) {
		return mealplan.BillingMonth{}, mealplan.ErrBillingMonthAttendanceExpired
	}
	last := monthLast(month)
	days := []mealplan.DailyList{}
	for monday := month.StartOfISOWeek(); !monday.After(last); monday = monday.AddDays(7) {
		week, err := b.weeks(ctx, mealplan.Date(monday.String()))
		if err != nil {
			return mealplan.BillingMonth{}, err
		}
		days = append(days, week.Days...)
	}
	_, holidays, err := b.calendar(ctx, month.String(), last.String())
	if err != nil {
		return mealplan.BillingMonth{}, fmt.Errorf("meal plan billing months: read the school holidays: %w", err)
	}
	present, err := b.presence(ctx, month, last)
	if err != nil {
		return mealplan.BillingMonth{}, fmt.Errorf("meal plan billing months: read the attendance: %w", err)
	}
	result := countBillingMonth(month, days, holidays, present)
	result.Status = b.statusOf(month)
	result.Fingerprint = monthFingerprint(result)
	return result, nil
}

// monthFingerprint names a computed month as the OGS-Leitung sees it: each
// child with its class, its registered days, its meals and its absences.
// The release compares it, so the totals alone cannot hide a meal that
// moved from one child to another.
func monthFingerprint(month mealplan.BillingMonth) string {
	state, err := json.Marshal(struct {
		Month    mealplan.Date                `json:"month"`
		Children []mealplan.BillingMonthChild `json:"children"`
	}{month.Month, month.Children})
	if err != nil {
		// A month always marshals; no fingerprint matches no release.
		return ""
	}
	// A change detector, not a secret: as the monthly run's review (#5916).
	hash := fnv.New128a()
	_, _ = hash.Write(state)
	return hex.EncodeToString(hash.Sum(nil))
}

// countBillingMonth counts the month (#4686): a school day is a day of the
// month in a kitchen list that is not closed (closing day, public holiday)
// and not in a school holiday. A child registered for lunch on a school day
// after every cancellation has a registered day; with attendance on file
// that day it is a meal, without it an absence.
func countBillingMonth(month calendar.Date, days []mealplan.DailyList, holidays []mealplan.HolidayPeriod,
	present map[calendar.Date]map[int64]bool,
) mealplan.BillingMonth {
	children := map[int64]*mealplan.BillingMonthChild{}
	result := mealplan.BillingMonth{Month: mealplan.Date(month.String()), Children: []mealplan.BillingMonthChild{}}
	for _, day := range days {
		date, ok := schoolDay(month, day, holidays)
		if !ok {
			continue
		}
		result.SchoolDays++
		for _, participant := range day.Participants {
			countDay(children, participant, day.Date, present[date][participant.StudentID])
		}
	}
	for _, child := range children {
		result.Children = append(result.Children, *child)
		result.MealCount += child.MealDays
		result.AbsenceCount += len(child.AbsentDates)
	}
	result.ChildCount = len(result.Children)
	slices.SortFunc(result.Children, compareChildren)
	return result
}

// schoolDay is the day of a kitchen list when it is a school day of the
// month.
func schoolDay(month calendar.Date, day mealplan.DailyList, holidays []mealplan.HolidayPeriod) (calendar.Date, bool) {
	date, err := calendar.ParseDate(string(day.Date))
	if err != nil || date.Before(month) || date.After(monthLast(month)) || day.Closed || inHoliday(date, holidays) {
		return "", false
	}
	return date, true
}

// countDay counts one registered school day of a child.
func countDay(children map[int64]*mealplan.BillingMonthChild, participant mealplan.DailyParticipant, date mealplan.Date, present bool) {
	child, ok := children[participant.StudentID]
	if !ok {
		child = &mealplan.BillingMonthChild{StudentID: participant.StudentID, AbsentDates: []mealplan.Date{}}
		children[participant.StudentID] = child
	}
	child.FirstName, child.LastName, child.SchoolClass = participant.FirstName, participant.LastName, participant.SchoolClass
	child.RegisteredDays++
	if present {
		child.MealDays++
		return
	}
	child.AbsentDates = append(child.AbsentDates, date)
}

func compareChildren(a, b mealplan.BillingMonthChild) int {
	if c := strings.Compare(strings.ToLower(a.LastName), strings.ToLower(b.LastName)); c != 0 {
		return c
	}
	if c := strings.Compare(strings.ToLower(a.FirstName), strings.ToLower(b.FirstName)); c != 0 {
		return c
	}
	return int(a.StudentID - b.StudentID)
}

func inHoliday(day calendar.Date, holidays []mealplan.HolidayPeriod) bool {
	for _, period := range holidays {
		start, startErr := calendar.ParseDate(period.Start)
		end, endErr := calendar.ParseDate(period.End)
		if startErr == nil && endErr == nil && !day.Before(start) && !day.After(end) {
			return true
		}
	}
	return false
}

// TraegerMealMonths serves mealplan.TraegerMealMonths.
type TraegerMealMonths struct{ store ports.BillingMonthStore }

var _ mealplan.TraegerMealMonths = TraegerMealMonths{}

// NewTraegerMealMonths builds the Träger's read of the released months.
func NewTraegerMealMonths(store ports.BillingMonthStore) TraegerMealMonths {
	if store == nil {
		panic("meal plan billing months: the store is required")
	}
	return TraegerMealMonths{store: store}
}

// ReleasedMealMonths reads the released months of the schools of the Träger
// up to until, in its Träger transaction, and groups the children per
// school and month; a release without children is a month without meals.
func (m TraegerMealMonths) ReleasedMealMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]mealplan.ReleasedMealMonth, error) {
	if organizationID <= 0 {
		return nil, errors.New("meal plan billing months: the Träger is required")
	}
	rows, err := m.store.TraegerMonths(ctx, organizationID, until)
	if err != nil {
		return nil, err
	}
	result := []mealplan.ReleasedMealMonth{}
	for _, row := range rows {
		last := len(result) - 1
		if last < 0 || result[last].SchoolID != row.SchoolID || result[last].Month != row.Month {
			result = append(result, mealplan.ReleasedMealMonth{SchoolID: row.SchoolID, Month: row.Month,
				ReleasedAt: row.ReleasedAt, Children: []mealplan.ReleasedMealChild{}})
			last++
		}
		if row.Child != nil {
			result[last].Children = append(result[last].Children, mealplan.ReleasedMealChild(*row.Child))
		}
	}
	return result, nil
}
