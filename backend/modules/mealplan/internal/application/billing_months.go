package application

import (
	"context"
	"errors"
	"fmt"
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

// BillingMonths serves mealplan.BillingMonths.
type BillingMonths struct {
	store    ports.BillingMonthStore
	weeks    WeekReader
	calendar mealplan.CalendarFinder
	presence mealplan.PresenceFinder
	tx       func(context.Context, func(context.Context) error) error
	today    func() calendar.Date
}

var _ mealplan.BillingMonths = (*BillingMonths)(nil)

// NewBillingMonths builds the Mensa-Monatsfreigabe.
func NewBillingMonths(store ports.BillingMonthStore, weeks WeekReader, calendarFinder mealplan.CalendarFinder,
	presence mealplan.PresenceFinder, tx func(context.Context, func(context.Context) error) error, today func() calendar.Date,
) *BillingMonths {
	if store == nil || weeks == nil || calendarFinder == nil || presence == nil || tx == nil || today == nil {
		panic("meal plan billing months: all dependencies are required")
	}
	return &BillingMonths{store: store, weeks: weeks, calendar: calendarFinder, presence: presence, tx: tx, today: today}
}

func monthStart(day calendar.Date) calendar.Date {
	return calendar.NewDate(day.Year(), day.Month(), 1)
}

func monthLast(first calendar.Date) calendar.Date {
	return calendar.NewDate(first.Year(), first.Month()+1, 1).AddDays(-1)
}

// parseBillingMonth accepts the first day of the current month or of one of
// the eleven before it.
func (b *BillingMonths) parseBillingMonth(value mealplan.Date) (calendar.Date, error) {
	month, err := calendar.ParseDate(string(value))
	current := monthStart(b.today())
	earliest := calendar.NewDate(current.Year(), current.Month()-(billingMonthsBack-1), 1)
	if err != nil || month.Day() != 1 || month.After(current) || month.Before(earliest) {
		return "", mealplan.ErrInvalidBillingMonth
	}
	return month, nil
}

// BillingMonthOverview lists the current month and the eleven before it,
// the latest first.
func (b *BillingMonths) BillingMonthOverview(ctx context.Context) ([]mealplan.BillingMonthSummary, error) {
	current := monthStart(b.today())
	earliest := calendar.NewDate(current.Year(), current.Month()-(billingMonthsBack-1), 1)
	var stored []domain.StoredBillingMonth
	err := b.tx(ctx, func(txCtx context.Context) error {
		var err error
		stored, err = b.store.ReleasedMonths(txCtx, earliest, current)
		return err
	})
	if err != nil {
		return nil, err
	}
	released := make(map[calendar.Date]domain.StoredBillingMonth, len(stored))
	for _, month := range stored {
		released[month.Month] = month
	}
	result := make([]mealplan.BillingMonthSummary, 0, billingMonthsBack)
	for month := current; !month.Before(earliest); month = monthStart(month.AddDays(-1)) {
		summary := mealplan.BillingMonthSummary{Month: mealplan.Date(month.String()), Status: b.statusOf(month)}
		if entry, ok := released[month]; ok {
			releasedAt, children, meals := entry.ReleasedAt, entry.ChildCount, entry.MealCount
			summary.Status, summary.ReleasedAt, summary.ChildCount, summary.MealCount = mealplan.BillingMonthReleased, &releasedAt, &children, &meals
		}
		result = append(result, summary)
	}
	return result, nil
}

// statusOf is running until the last day of the month has passed.
func (b *BillingMonths) statusOf(month calendar.Date) mealplan.BillingMonthStatus {
	if monthLast(month).Before(b.today()) {
		return mealplan.BillingMonthOpen
	}
	return mealplan.BillingMonthRunning
}

// BillingMonth shows a released month as stored, any other month as
// computed now.
func (b *BillingMonths) BillingMonth(ctx context.Context, value mealplan.Date) (mealplan.BillingMonth, error) {
	month, err := b.parseBillingMonth(value)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	var result mealplan.BillingMonth
	err = b.tx(ctx, func(txCtx context.Context) error {
		stored, released, err := b.store.ReleasedMonth(txCtx, month)
		if err != nil {
			return err
		}
		if released {
			result = releasedView(stored)
			return nil
		}
		result, err = b.compute(txCtx, month)
		return err
	})
	return result, err
}

// ReleaseBillingMonth computes the month again and stores it when it still
// shows the children and meals the OGS-Leitung saw.
func (b *BillingMonths) ReleaseBillingMonth(ctx context.Context, release mealplan.BillingMonthRelease) (mealplan.BillingMonth, error) {
	month, err := b.parseBillingMonth(release.Month)
	if err != nil {
		return mealplan.BillingMonth{}, err
	}
	if release.AccountID <= 0 {
		return mealplan.BillingMonth{}, errors.New("meal plan billing months: the releasing account is required")
	}
	if b.statusOf(month) != mealplan.BillingMonthOpen {
		return mealplan.BillingMonth{}, mealplan.ErrBillingMonthRunning
	}
	var result mealplan.BillingMonth
	err = b.tx(ctx, func(txCtx context.Context) error {
		if _, released, err := b.store.ReleasedMonth(txCtx, month); err != nil || released {
			if err != nil {
				return err
			}
			return mealplan.ErrBillingMonthReleased
		}
		organizationID, ok, err := b.store.SchoolOrganization(txCtx)
		if err != nil {
			return err
		}
		if !ok {
			return mealplan.ErrBillingMonthNoTraeger
		}
		computed, err := b.compute(txCtx, month)
		if err != nil {
			return err
		}
		if computed.ChildCount != release.ChildCount || computed.MealCount != release.MealCount {
			return mealplan.ErrBillingMonthChanged
		}
		stored := domain.StoredBillingMonth{Month: month, ChildCount: computed.ChildCount, MealCount: computed.MealCount,
			Children: make([]domain.BillingChild, len(computed.Children))}
		for i, child := range computed.Children {
			stored.Children[i] = domain.BillingChild{StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
				SchoolClass: child.SchoolClass, RegisteredDays: child.RegisteredDays, MealDays: child.MealDays}
		}
		releasedAt, err := b.store.InsertRelease(txCtx, organizationID, release.AccountID, stored)
		if err != nil {
			return err
		}
		stored.ReleasedAt = releasedAt
		result = releasedView(stored)
		result.SchoolDays = computed.SchoolDays
		return nil
	})
	return result, err
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
// up to until, in its Träger transaction.
func (b TraegerMealMonths) ReleasedMealMonths(ctx context.Context, organizationID int64, until calendar.Date) ([]mealplan.ReleasedMealMonth, error) {
	if organizationID <= 0 {
		return nil, errors.New("meal plan billing months: the Träger is required")
	}
	return b.store.TraegerMonths(ctx, organizationID, until)
}

func releasedView(stored domain.StoredBillingMonth) mealplan.BillingMonth {
	releasedAt := stored.ReleasedAt
	children := make([]mealplan.BillingMonthChild, len(stored.Children))
	for i, child := range stored.Children {
		children[i] = mealplan.BillingMonthChild{StudentID: child.StudentID, FirstName: child.FirstName, LastName: child.LastName,
			SchoolClass: child.SchoolClass, RegisteredDays: child.RegisteredDays, MealDays: child.MealDays, AbsentDates: []mealplan.Date{}}
	}
	view := mealplan.BillingMonth{Month: mealplan.Date(stored.Month.String()), Status: mealplan.BillingMonthReleased,
		ReleasedAt: &releasedAt, ChildCount: stored.ChildCount, MealCount: stored.MealCount, Children: children}
	for _, child := range children {
		view.AbsenceCount += child.RegisteredDays - child.MealDays
	}
	return view
}

// compute reads the kitchen lists, the school holidays and the attendance
// of the month and counts them.
func (b *BillingMonths) compute(ctx context.Context, month calendar.Date) (mealplan.BillingMonth, error) {
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
	return result, nil
}

// countBillingMonth counts the month (#4686): a school day is a day of the
// month in a kitchen list that is not closed (closing day, public holiday)
// and not in a school holiday. A child registered for lunch on a school day
// after every cancellation has a registered day; with attendance on file
// that day it is a meal, without it an absence.
func countBillingMonth(month calendar.Date, days []mealplan.DailyList, holidays []mealplan.HolidayPeriod,
	present map[calendar.Date]map[int64]bool,
) mealplan.BillingMonth {
	last := monthLast(month)
	children := map[int64]*mealplan.BillingMonthChild{}
	result := mealplan.BillingMonth{Month: mealplan.Date(month.String()), Children: []mealplan.BillingMonthChild{}}
	for _, day := range days {
		date, err := calendar.ParseDate(string(day.Date))
		if err != nil || date.Before(month) || date.After(last) || day.Closed || inHoliday(date, holidays) {
			continue
		}
		result.SchoolDays++
		for _, participant := range day.Participants {
			child, ok := children[participant.StudentID]
			if !ok {
				child = &mealplan.BillingMonthChild{StudentID: participant.StudentID, AbsentDates: []mealplan.Date{}}
				children[participant.StudentID] = child
			}
			child.FirstName, child.LastName, child.SchoolClass = participant.FirstName, participant.LastName, participant.SchoolClass
			child.RegisteredDays++
			if present[date][participant.StudentID] {
				child.MealDays++
			} else {
				child.AbsentDates = append(child.AbsentDates, day.Date)
			}
		}
	}
	for _, child := range children {
		result.Children = append(result.Children, *child)
		result.MealCount += child.MealDays
		result.AbsenceCount += len(child.AbsentDates)
	}
	result.ChildCount = len(result.Children)
	slices.SortFunc(result.Children, func(a, b mealplan.BillingMonthChild) int {
		if c := strings.Compare(strings.ToLower(a.LastName), strings.ToLower(b.LastName)); c != 0 {
			return c
		}
		if c := strings.Compare(strings.ToLower(a.FirstName), strings.ToLower(b.FirstName)); c != 0 {
			return c
		}
		return int(a.StudentID - b.StudentID)
	})
	return result
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
