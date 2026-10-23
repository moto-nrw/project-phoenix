package application

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// maxOrderWindowDays bounds one read of the orders: a month and a few days.
const maxOrderWindowDays = 42

func (s *Service) Week(ctx context.Context, weekStart mealplan.Date) ([]mealplan.Entry, error) {
	date, err := calendar.ParseDate(string(weekStart))
	if err != nil {
		return nil, mealplan.ErrInvalidMealDate
	}
	entries, err := s.weekEntries(ctx, date.StartOfISOWeek())
	if err != nil {
		return nil, mapPublicError(err)
	}
	result := make([]mealplan.Entry, 0, len(entries))
	for _, entry := range entries {
		result = append(result, mealplan.Entry{Date: mealplan.Date(entry.Date.String()), Position: entry.Position, Dish: entry.Dish, Note: entry.Note})
	}
	return result, nil
}

func (s *Service) ReplaceDay(ctx context.Context, command mealplan.ReplaceDay) error {
	date, err := calendar.ParseDate(string(command.Date))
	if err != nil {
		return mealplan.ErrInvalidMealDate
	}
	if date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return mealplan.ErrInvalidMealDate
	}

	dishes := make([]mealplan.Dish, 0, len(command.Dishes))
	for _, input := range command.Dishes {
		dish := strings.TrimSpace(input.Dish)
		if dish == "" {
			continue
		}
		dishes = append(dishes, mealplan.Dish{Dish: dish, Note: trimOptional(input.Note)})
	}
	return mapPublicError(s.replace(ctx, date, dishes))
}

func (s *Service) ClearDay(ctx context.Context, date mealplan.Date) error {
	parsed, err := calendar.ParseDate(string(date))
	if err != nil {
		return mealplan.ErrInvalidMealDate
	}
	return mapPublicError(s.clear(ctx, parsed))
}

func (s *Service) RegistrationAvailable(ctx context.Context) (bool, error) {
	available, err := s.registrationAvailable(ctx)
	return available, mapPublicError(err)
}

func (s *Service) Participation(ctx context.Context, studentID int64, from, to mealplan.Date) (mealplan.ParticipationPlan, error) {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
	if studentID <= 0 || startErr != nil || endErr != nil || end.Before(start) || start.DaysUntil(end) > 31 {
		return mealplan.ParticipationPlan{}, mealplan.ErrInvalidParticipation
	}
	plan, err := s.participationPlan(ctx, studentID, start, end)
	if err != nil {
		return mealplan.ParticipationPlan{}, mapPublicError(err)
	}
	return publicParticipationPlan(plan), nil
}

func (s *Service) ReplaceParticipationSchedule(ctx context.Context, command mealplan.ReplaceParticipationSchedule) (mealplan.Date, error) {
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 {
		return "", mealplan.ErrInvalidParticipation
	}
	weekdays, ok := normalizeWeekdays(command.Weekdays)
	if !ok {
		return "", mealplan.ErrInvalidParticipation
	}
	return s.publicSchedule(ctx, command.StudentID, command.GuardianAccountID, weekdays, "")
}

// ReplaceScheduleBySchool sets a child's regular days for the school.
func (s *Service) ReplaceScheduleBySchool(ctx context.Context, command mealplan.SchoolSchedule) (mealplan.Date, error) {
	weekdays, ok := normalizeWeekdays(command.Weekdays)
	effectiveFrom, err := calendar.ParseDate(string(command.EffectiveFrom))
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || !ok || err != nil {
		return "", mealplan.ErrInvalidParticipation
	}
	return s.publicSchedule(ctx, command.StudentID, command.StaffAccountID, weekdays, effectiveFrom)
}

// publicSchedule replaces the regular days. The empty effective date is a
// parent's change; a set one is the school's.
func (s *Service) publicSchedule(ctx context.Context, studentID, accountID int64, weekdays []mealplan.Weekday, schoolFrom domain.Date) (mealplan.Date, error) {
	values := make([]domain.Weekday, 0, len(weekdays))
	for _, weekday := range weekdays {
		values = append(values, domain.Weekday(weekday))
	}
	from, err := s.replaceSchedule(ctx, studentID, accountID, values, schoolFrom)
	return mealplan.Date(from.String()), mapPublicError(err)
}

// SetParticipationBySchool registers or cancels a day for the school.
func (s *Service) SetParticipationBySchool(ctx context.Context, command mealplan.SchoolChange) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}
	return mapPublicError(s.setParticipationDay(ctx, command.StudentID, command.StaffAccountID, date, command.Participating, true))
}

// ClearParticipationBySchool drops a day's exception for the school, so the
// regular days apply again.
func (s *Service) ClearParticipationBySchool(ctx context.Context, command mealplan.SchoolChange) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.StaffAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}
	return mapPublicError(s.clearParticipationDay(ctx, command.StudentID, command.StaffAccountID, date, true))
}

func (s *Service) SetParticipationForDay(ctx context.Context, command mealplan.SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}
	return mapPublicError(s.setParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date, command.Participating, false))
}

func (s *Service) SetParticipationForWeek(ctx context.Context, command mealplan.SetParticipationWeek) (mealplan.WeekChange, error) {
	date, err := calendar.ParseDate(string(command.WeekOf))
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return mealplan.WeekChange{}, mealplan.ErrInvalidParticipation
	}
	changed, locked, err := s.setParticipationWeek(ctx, command.StudentID, command.GuardianAccountID, date.StartOfISOWeek(), command.Participating)
	if err != nil {
		return mealplan.WeekChange{}, mapPublicError(err)
	}
	return mealplan.WeekChange{Changed: publicDates(changed), Locked: publicDates(locked)}, nil
}

func (s *Service) ClearParticipationForDay(ctx context.Context, command mealplan.SetParticipationDay) error {
	date, err := participationDate(command.Date)
	if command.StudentID <= 0 || command.GuardianAccountID <= 0 || err != nil {
		return mealplan.ErrInvalidParticipation
	}
	return mapPublicError(s.clearParticipationDay(ctx, command.StudentID, command.GuardianAccountID, date, false))
}

func (s *Service) DailyList(ctx context.Context, date mealplan.Date) (mealplan.DailyList, error) {
	parsed, err := participationDate(date)
	if err != nil {
		return mealplan.DailyList{}, mealplan.ErrInvalidParticipation
	}
	list, err := s.dailyParticipants(ctx, parsed)
	if err != nil {
		return mealplan.DailyList{}, mapPublicError(err)
	}
	result := mealplan.DailyList{Date: mealplan.Date(list.Date.String()), CutoffTime: list.CutoffTime, Deadline: list.Deadline, Closed: list.Closed, Participants: make([]mealplan.DailyParticipant, 0, len(list.Participants))}
	for _, participant := range list.Participants {
		result.Participants = append(result.Participants, mealplan.DailyParticipant(participant))
	}
	for _, other := range list.Others {
		result.Others = append(result.Others, mealplan.DailyParticipant(other))
	}
	return result, nil
}

func (s *Service) SettleOrders(ctx context.Context) error {
	return mapPublicError(s.settleDueOrders(ctx))
}

func (s *Service) Orders(ctx context.Context, from, to mealplan.Date) (mealplan.OrderWeek, error) {
	start, end, err := readWindow(from, to)
	if err != nil {
		return mealplan.OrderWeek{}, err
	}
	view, err := s.orderView(ctx, start, end)
	if err != nil {
		return mealplan.OrderWeek{}, mapPublicError(err)
	}
	return publicOrderWeek(view), nil
}

func (s *Service) StartSupply(ctx context.Context, catererID int64, deliveryAddress string, accountID int64) error {
	if catererID <= 0 {
		return mealplan.ErrInvalidParticipation
	}
	return s.startSupply(ctx, catererID, strings.TrimSpace(deliveryAddress), accountID)
}

func (s *Service) CatererOrders(ctx context.Context, catererID int64, from, to mealplan.Date) (mealplan.OrderWeek, error) {
	start, end, err := readWindow(from, to)
	if err != nil {
		return mealplan.OrderWeek{}, err
	}
	return s.catererOrders(ctx, catererID, start.String(), end.String())
}

func (s *Service) ConfirmOrder(ctx context.Context, catererID, orderID, accountID int64) error {
	if orderID <= 0 {
		return mealplan.ErrOrderNotFound
	}
	return s.confirmOrder(ctx, catererID, orderID, accountID)
}

// readWindow parses one read of the orders, at most maxOrderWindowDays long.
func readWindow(from, to mealplan.Date) (calendar.Date, calendar.Date, error) {
	start, startErr := calendar.ParseDate(string(from))
	end, endErr := calendar.ParseDate(string(to))
	if startErr != nil || endErr != nil || end.Before(start) || start.DaysUntil(end) >= maxOrderWindowDays {
		return "", "", mealplan.ErrInvalidParticipation
	}
	return start, end, nil
}

func participationDate(value mealplan.Date) (calendar.Date, error) {
	date, err := calendar.ParseDate(string(value))
	if err != nil || date.Weekday() == time.Saturday || date.Weekday() == time.Sunday {
		return "", mealplan.ErrInvalidParticipation
	}
	return date, nil
}

func normalizeWeekdays(values []mealplan.Weekday) ([]mealplan.Weekday, bool) {
	seen := make(map[mealplan.Weekday]struct{}, len(values))
	result := make([]mealplan.Weekday, 0, len(values))
	for _, weekday := range values {
		if weekday < mealplan.Monday || weekday > mealplan.Friday {
			return nil, false
		}
		if _, exists := seen[weekday]; exists {
			continue
		}
		seen[weekday] = struct{}{}
		result = append(result, weekday)
	}
	slices.Sort(result)
	return result, true
}

func trimOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func publicDates(dates []domain.Date) []mealplan.Date {
	result := make([]mealplan.Date, 0, len(dates))
	for _, date := range dates {
		result = append(result, mealplan.Date(date.String()))
	}
	return result
}

func publicOrderWeek(view domain.OrderView) mealplan.OrderWeek {
	week := mealplan.OrderWeek{Rhythm: string(view.Rules.Rhythm), Detail: string(view.Rules.Detail), Days: make([]mealplan.OrderDay, 0, len(view.Days))}
	for _, day := range view.Days {
		week.Days = append(week.Days, publicOrderDay(day, view.Names))
	}
	return week
}

func publicOrderDay(day domain.OrderDay, names map[int64]domain.DailyCandidate) mealplan.OrderDay {
	child := func(studentID int64, schoolClass string) mealplan.OrderChild {
		name := names[studentID]
		return mealplan.OrderChild{StudentID: studentID, FirstName: name.FirstName, LastName: name.LastName, SchoolClass: schoolClass}
	}
	counts := day.Children
	if day.Order != nil {
		counts = day.FrozenChildren
	}
	result := mealplan.OrderDay{Date: mealplan.Date(day.Date.String()), Closed: day.Closed, Deadline: day.Deadline, Status: mealplan.OrderOpen, Total: len(counts)}
	if day.Unordered {
		result.Status = mealplan.OrderUnordered
	}
	if day.Order != nil {
		result.Status, result.OrderID, result.ConfirmedAt = mealplan.OrderOrdered, day.Order.ID, day.Order.ConfirmedAt
		if day.Order.ConfirmedAt != nil {
			result.Status = mealplan.OrderConfirmed
		}
	}
	for _, count := range domain.CountByClass(counts) {
		result.Classes = append(result.Classes, mealplan.ClassCount(count))
	}
	for _, eating := range day.Eating {
		result.Children = append(result.Children, mealplan.OrderChild(eating))
	}
	for _, change := range day.Changes {
		result.Changes = append(result.Changes, mealplan.OrderChange{Sequence: change.Sequence, Date: result.Date, Child: child(change.StudentID, change.SchoolClass), Delta: change.Delta, DetectedAt: change.DetectedAt})
	}
	return result
}

func publicParticipationPlan(plan domain.ParticipationPlan) mealplan.ParticipationPlan {
	result := mealplan.ParticipationPlan{EffectiveFrom: mealplan.Date(plan.EffectiveFrom.String()), CutoffTime: plan.CutoffTime, Model: mealplan.RegistrationModel(plan.Model), ScheduleChangeable: plan.ScheduleChangeable, SickCancels: plan.Sick == domain.SickCancelled, Weekdays: make([]mealplan.Weekday, 0, len(plan.Weekdays)), Days: make([]mealplan.ParticipationDay, 0, len(plan.Days))}
	for _, weekday := range plan.Weekdays {
		result.Weekdays = append(result.Weekdays, mealplan.Weekday(weekday))
	}
	for _, day := range plan.Days {
		result.Days = append(result.Days, mealplan.ParticipationDay{
			Date: mealplan.Date(day.Date.String()), Participating: day.Participating, Regular: day.Regular, Source: mealplan.ParticipationSource(day.Source),
			Changeable: day.Changeable(), CanRegister: day.CanRegister, CanCancel: day.CanCancel,
			RegisterUntil: day.RegisterUntil, CancelUntil: day.CancelUntil,
		})
	}
	return result
}

// publicErrors maps the application's errors to the module's, first match
// wins.
var publicErrors = []struct{ from, to error }{
	{ErrDisabled, mealplan.ErrDisabled},
	{ErrRegistrationDisabled, mealplan.ErrRegistrationDisabled},
	{ErrCutoffPassed, mealplan.ErrParticipationCutoff},
	{ErrPastDay, mealplan.ErrPastDay},
	{ErrDayClosed, mealplan.ErrDayClosed},
	{ErrStudentNotFound, mealplan.ErrStudentNotFound},
	{ErrNotEnrolled, mealplan.ErrChildNotEnrolled},
	{ErrInvalidCutoff, mealplan.ErrInvalidParticipation},
	{ErrTooFarAhead, mealplan.ErrInvalidParticipation},
	{ErrSupplyActive, mealplan.ErrSupplyActive},
	{ErrNotSupplied, mealplan.ErrNotSupplied},
	{ErrOrderNotFound, mealplan.ErrOrderNotFound},
}

func mapPublicError(err error) error {
	for _, mapping := range publicErrors {
		if errors.Is(err, mapping.from) {
			return mapping.to
		}
	}
	return err
}
