package application

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/domain"
)

func (s *Service) FindCalendarPeriod(ctx context.Context, id int64) (result schoolcalendar.CalendarPeriod, err error) {
	if id <= 0 {
		return schoolcalendar.CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	err = s.run(ctx, "find_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCalendarPeriod(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolcalendar.ErrCalendarPeriodNotFound
		}
		return err
	})
	return result, err
}
func (s *Service) ListCalendarPeriods(ctx context.Context, filter schoolcalendar.CalendarPeriodFilter) (result []schoolcalendar.CalendarPeriod, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	if filter.PeriodType != "" && !schoolcalendar.IsValidPeriodType(filter.PeriodType) {
		return nil, invalidPeriod("invalid period type")
	}
	if err := validateWindow(filter.OverlappingFrom, filter.OverlappingTo, invalidPeriod); err != nil {
		return nil, err
	}
	if filter.ExcludeID < 0 {
		return nil, invalidPeriod("exclude ID must not be negative")
	}
	err = s.run(ctx, "list_calendar_periods", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListCalendarPeriods(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, err
}
func (s *Service) CreateCalendarPeriod(ctx context.Context, input schoolcalendar.CreateCalendarPeriod) (result schoolcalendar.CalendarPeriod, err error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return schoolcalendar.CalendarPeriod{}, err
	}

	fields := input.CalendarPeriodFields
	ifAbsent := false
	operation := "create_calendar_period"
	if ifAbsent {
		operation = "create_calendar_period_if_absent"
	}
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, _, createStats, err = s.store.CreateCalendarPeriod(ctx, fields, ifAbsent)
		stats.Add(createStats)
		return err
	})
	return result, err
}
func (s *Service) CreateCalendarPeriodIfAbsent(ctx context.Context, input schoolcalendar.CreateCalendarPeriod) (result schoolcalendar.CalendarPeriod, created bool, err error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return schoolcalendar.CalendarPeriod{}, false, err
	}

	fields := input.CalendarPeriodFields
	ifAbsent := true
	operation := "create_calendar_period"
	if ifAbsent {
		operation = "create_calendar_period_if_absent"
	}
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, created, createStats, err = s.store.CreateCalendarPeriod(ctx, fields, ifAbsent)
		stats.Add(createStats)
		return err
	})
	return result, created, err
}
func (s *Service) UpdateCalendarPeriod(ctx context.Context, input schoolcalendar.UpdateCalendarPeriod) (result schoolcalendar.CalendarPeriod, err error) {
	if input.ID <= 0 {
		return schoolcalendar.CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return schoolcalendar.CalendarPeriod{}, err
	}

	fields := input.CalendarPeriodFields
	id := input.ID
	err = s.run(ctx, "update_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateCalendarPeriod(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}
func (s *Service) DeleteCalendarPeriod(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidPeriod("calendar period ID is required")
	}
	return s.run(ctx, "delete_calendar_period", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteCalendarPeriod(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}
func (s *Service) FindClosingDay(ctx context.Context, id int64) (result schoolcalendar.ClosingDay, err error) {
	if id <= 0 {
		return schoolcalendar.ClosingDay{}, invalidClosingDay("closing day ID is required")
	}
	err = s.run(ctx, "find_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindClosingDay(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolcalendar.ErrClosingDayNotFound
		}
		return err
	})
	return result, err
}
func (s *Service) ListClosingDays(ctx context.Context, filter schoolcalendar.ClosingDayFilter) (result []schoolcalendar.ClosingDay, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	if err := validateWindow(filter.OverlappingFrom, filter.OverlappingTo, invalidClosingDay); err != nil {
		return nil, err
	}
	err = s.run(ctx, "list_closing_days", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClosingDays(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, err
}
func (s *Service) CreateClosingDay(ctx context.Context, input schoolcalendar.CreateClosingDay) (result schoolcalendar.ClosingDay, err error) {
	if err := validateClosingDayFields(&input.ClosingDayFields); err != nil {
		return schoolcalendar.ClosingDay{}, err
	}

	fields := input.ClosingDayFields
	err = s.run(ctx, "create_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClosingDay(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}
func (s *Service) UpdateClosingDay(ctx context.Context, input schoolcalendar.UpdateClosingDay) (result schoolcalendar.ClosingDay, err error) {
	if input.ID <= 0 {
		return schoolcalendar.ClosingDay{}, invalidClosingDay("closing day ID is required")
	}
	if err := validateClosingDayFields(&input.ClosingDayFields); err != nil {
		return schoolcalendar.ClosingDay{}, err
	}

	fields := input.ClosingDayFields
	id := input.ID
	err = s.run(ctx, "update_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClosingDay(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}
func (s *Service) DeleteClosingDay(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidClosingDay("closing day ID is required")
	}
	return s.run(ctx, "delete_closing_day", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteClosingDay(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}
func (s *Service) FindDateframe(ctx context.Context, id int64) (result schoolcalendar.Dateframe, err error) {
	if id <= 0 {
		return schoolcalendar.Dateframe{}, invalidDateframe("dateframe ID is required")
	}
	err = s.run(ctx, "find_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindDateframe(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return schoolcalendar.ErrDateframeNotFound
		}
		return err
	})
	return result, err
}
func (s *Service) ListDateframes(ctx context.Context, filter schoolcalendar.DateframeFilter) (result []schoolcalendar.Dateframe, err error) {
	filter.IDs = uniquePositive(filter.IDs)
	if (filter.OverlappingFrom == nil) != (filter.OverlappingTo == nil) {
		return nil, invalidDateframe("overlap window needs both from and to instants")
	}
	if filter.OverlappingFrom != nil && filter.OverlappingTo.Before(*filter.OverlappingFrom) {
		return nil, invalidDateframe("overlap window to must not be before from")
	}
	if filter.Limit < 0 || filter.Offset < 0 {
		return nil, invalidDateframe("limit and offset must not be negative")
	}
	for _, sort := range filter.Sort {
		switch sort.Field {
		case schoolcalendar.DateframeSortID, schoolcalendar.DateframeSortName, schoolcalendar.DateframeSortStartDate, schoolcalendar.DateframeSortEndDate:
		default:
			return nil, invalidDateframe("unsupported dateframe sort field " + sort.Field)
		}
	}
	err = s.run(ctx, "list_dateframes", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListDateframes(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, err
}
func (s *Service) CreateDateframe(ctx context.Context, input schoolcalendar.CreateDateframe) (result schoolcalendar.Dateframe, err error) {
	if err := validateDateframeFields(input.DateframeFields); err != nil {
		return schoolcalendar.Dateframe{}, err
	}

	fields := input.DateframeFields
	err = s.run(ctx, "create_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateDateframe(ctx, fields)
		stats.Add(createStats)
		return err
	})
	return result, err
}
func (s *Service) UpdateDateframe(ctx context.Context, input schoolcalendar.UpdateDateframe) (result schoolcalendar.Dateframe, err error) {
	if input.ID <= 0 {
		return schoolcalendar.Dateframe{}, invalidDateframe("dateframe ID is required")
	}
	if err := validateDateframeFields(input.DateframeFields); err != nil {
		return schoolcalendar.Dateframe{}, err
	}

	fields := input.DateframeFields
	id := input.ID
	err = s.run(ctx, "update_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateDateframe(ctx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}
func (s *Service) DeleteDateframe(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidDateframe("dateframe ID is required")
	}
	return s.run(ctx, "delete_dateframe", func(ctx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteDateframe(ctx, id)
		stats.Add(deleteStats)
		return err
	})
}
func (s *Service) ListHolidays(ctx context.Context, region, from, to string) (result []schoolcalendar.Holiday, err error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	err = s.observeOperation("list_holidays", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = s.algorithms.ListHolidays(ctx, region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}
func (s *Service) HolidayDates(ctx context.Context, region, from, to string) (result map[string]bool, err error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	err = s.observeOperation("holiday_dates", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = s.algorithms.HolidayDates(ctx, region, from, to)
		stats.Rows = int64(len(result))
		return err
	})
	return result, err
}
func (s *Service) RenderCalendar(ctx context.Context, name string, events []schoolcalendar.CalendarEvent) (result string, err error) {
	for _, event := range events {
		if err := validateCalendarEvent(event); err != nil {
			return "", err
		}
	}
	err = s.observeOperation("render_calendar", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = s.algorithms.RenderCalendar(ctx, name, events)
		stats.Rows = int64(len(events))
		return err
	})
	return result, err
}
func (s *Service) RenderCalendarObject(ctx context.Context, event schoolcalendar.CalendarEvent) (result string, err error) {
	if err := validateCalendarEvent(event); err != nil {
		return "", err
	}
	err = s.observeOperation("render_calendar_object", func(ctx context.Context, stats *domain.OperationStats) error {
		result, err = s.algorithms.RenderCalendarObject(ctx, event)
		stats.Rows = 1
		return err
	})
	return result, err
}
func (s *Service) AddCalendarPeriod(ctx context.Context, input schoolcalendar.CreateCalendarPeriod) (result schoolcalendar.CalendarPeriod, err error) {
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return schoolcalendar.CalendarPeriod{}, err
	}

	fields := input.CalendarPeriodFields
	err = s.observeAdministration("add_calendar_period", func(_ context.Context, stats *domain.OperationStats) error {
		return s.underRecurrenceGate(ctx, func(txCtx context.Context) error {
			if err := s.ensureUniqueName(txCtx, fields.Name, 0, stats); err != nil {
				return err
			}
			if err := s.ensureNoActiveSameTypeOverlap(txCtx, fields, 0, stats); err != nil {
				return err
			}
			var createStats domain.OperationStats
			var createErr error
			result, _, createStats, createErr = s.store.CreateCalendarPeriod(txCtx, fields, false)
			stats.Add(createStats)
			return createErr
		})
	})
	return result, err
}
func (s *Service) ChangeCalendarPeriod(ctx context.Context, input schoolcalendar.UpdateCalendarPeriod) (result schoolcalendar.CalendarPeriod, err error) {
	if input.ID <= 0 {
		return schoolcalendar.CalendarPeriod{}, invalidPeriod("calendar period ID is required")
	}
	if err := validateCalendarPeriodFields(&input.CalendarPeriodFields); err != nil {
		return schoolcalendar.CalendarPeriod{}, err
	}

	fields := input.CalendarPeriodFields
	id := input.ID
	err = s.observeAdministration("change_calendar_period", func(_ context.Context, stats *domain.OperationStats) error {
		return s.underRecurrenceGate(ctx, func(txCtx context.Context) error {
			if err := s.ensureChangeAllowed(txCtx, id, fields, stats); err != nil {
				return err
			}
			var updateStats domain.OperationStats
			var updateErr error
			result, updateStats, updateErr = s.store.UpdateCalendarPeriod(txCtx, id, fields)
			stats.Add(updateStats)
			return updateErr
		})
	})
	return result, err
}
func (s *Service) RemoveCalendarPeriod(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidPeriod("calendar period ID is required")
	}
	return s.observeAdministration("remove_calendar_period", func(_ context.Context, stats *domain.OperationStats) error {
		return s.underRecurrenceGate(ctx, func(txCtx context.Context) error {
			if s.administration.CareOfferingGuard != nil {
				if err := s.administration.CareOfferingGuard(txCtx, id, nil); err != nil {
					return fmt.Errorf("validate linked care offerings: %w", err)
				}
			}
			deleteStats, err := s.store.DeleteCalendarPeriod(txCtx, id)
			stats.Add(deleteStats)
			return err
		})
	})
}
func (s *Service) EnsureDefaultSchoolYear(ctx context.Context) (periods []schoolcalendar.CalendarPeriod, created bool, err error) {
	err = s.observeAdministration("ensure_default_school_year", func(_ context.Context, stats *domain.OperationStats) error {
		return s.underRecurrenceGate(ctx, func(txCtx context.Context) error {
			var listStats domain.OperationStats
			var listErr error
			periods, listStats, listErr = s.store.ListCalendarPeriods(txCtx, schoolcalendar.CalendarPeriodFilter{})
			stats.Add(listStats)
			if listErr != nil {
				return listErr
			}
			if len(periods) > 0 {
				return nil
			}
			fields := defaultSchoolYear(s.administration.Today())
			if err := s.ensureNoActiveSameTypeOverlap(txCtx, fields, 0, stats); err != nil {
				return err
			}
			_, inserted, createStats, createErr := s.store.CreateCalendarPeriod(txCtx, fields, true)
			stats.Add(createStats)
			if createErr != nil {
				return createErr
			}
			created = inserted
			periods, listStats, listErr = s.store.ListCalendarPeriods(txCtx, schoolcalendar.CalendarPeriodFilter{})
			stats.Add(listStats)
			return listErr
		})
	})
	if err != nil {
		return nil, false, err
	}
	return periods, created, nil
}
func defaultSchoolYear(today string) schoolcalendar.CalendarPeriodFields {
	name, start, end := schoolcalendar.DefaultSchoolYear(today)
	return schoolcalendar.CalendarPeriodFields{Name: name, PeriodType: schoolcalendar.PeriodTypeSchoolYear, StartDate: start, EndDate: end, WeekCycleLength: 1, IsActive: true}
}

func (s *Service) ListActiveOverlaps(ctx context.Context, period schoolcalendar.CalendarPeriod) ([]schoolcalendar.CalendarPeriod, error) {
	if !period.IsActive {
		return nil, nil
	}
	return s.ListCalendarPeriods(ctx, schoolcalendar.CalendarPeriodFilter{
		ActiveOnly: true, OverlappingFrom: period.StartDate, OverlappingTo: period.EndDate, ExcludeID: period.ID,
	})
}
func validateHolidayWindow(from, to string) error {
	invalid := func(reason string) error { return fmt.Errorf("%w: %s", schoolcalendar.ErrInvalidHolidayRange, reason) }
	if from == "" || to == "" {
		return invalid("holiday range needs both from and to dates")
	}
	return validateWindow(from, to, invalid)
}

func validateCalendarEvent(event schoolcalendar.CalendarEvent) error {
	if err := validateDate(event.StartDate, "calendar event start date", invalidPeriod); err != nil {
		return err
	}
	if err := validateDate(event.EndDate, "calendar event end date", invalidPeriod); err != nil {
		return err
	}
	if event.StartDate == "" || event.EndDate == "" {
		return invalidPeriod("calendar event dates are required")
	}
	if event.Recurrence != nil && event.Recurrence.Until != "" {
		if err := validateDate(event.Recurrence.Until, "calendar recurrence until", invalidPeriod); err != nil {
			return err
		}
	}
	for _, date := range event.ExDates {
		if err := validateDate(date, "calendar exception date", invalidPeriod); err != nil {
			return err
		}
	}
	return nil
}

// --- validation ---

// schoolcalendar.IsValidPeriodType reports whether t is one of the known period types.

// validateCalendarPeriodFields mirrors the legacy model rules so a caller
// that skipped the model validation gets the same messages.
func validateCalendarPeriodFields(fields *schoolcalendar.CalendarPeriodFields) error {
	if fields.Name == "" {
		return invalidPeriod("name is required")
	}
	if len(fields.Name) > schoolcalendar.CalendarPeriodNameMaxLength {
		return invalidPeriod("name cannot exceed 255 characters")
	}
	if !schoolcalendar.IsValidPeriodType(fields.PeriodType) {
		return invalidPeriod("invalid period type")
	}
	if fields.StartDate == "" {
		return invalidPeriod("start_date is required")
	}
	if fields.EndDate == "" {
		return invalidPeriod("end_date is required")
	}
	if err := validateDate(fields.StartDate, "start_date", invalidPeriod); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end_date", invalidPeriod); err != nil {
		return err
	}
	if fields.EndDate <= fields.StartDate {
		return invalidPeriod("end_date must be after start_date")
	}
	if fields.WeekCycleLength < 1 {
		return invalidPeriod("week_cycle_length must be at least 1")
	}
	if fields.WeekCycleLength > 1 && fields.WeekCycleAnchor == "" {
		return invalidPeriod("week_cycle_anchor is required when week_cycle_length > 1")
	}
	return validateDate(fields.WeekCycleAnchor, "week_cycle_anchor", invalidPeriod)
}

func validateClosingDayFields(fields *schoolcalendar.ClosingDayFields) error {
	if strings.TrimSpace(fields.Reason) == "" {
		return invalidClosingDay("reason is required")
	}
	if utf8.RuneCountInString(strings.TrimSpace(fields.Reason)) > schoolcalendar.ClosingDayReasonMaxLength {
		return invalidClosingDay("reason cannot exceed 255 characters")
	}
	if fields.StartDate == "" {
		return invalidClosingDay("start_date is required")
	}
	if fields.EndDate == "" {
		return invalidClosingDay("end_date is required")
	}
	if err := validateDate(fields.StartDate, "start_date", invalidClosingDay); err != nil {
		return err
	}
	if err := validateDate(fields.EndDate, "end_date", invalidClosingDay); err != nil {
		return err
	}
	if fields.EndDate < fields.StartDate {
		return invalidClosingDay("end_date must not be before start_date")
	}
	return nil
}

// validateWindow accepts an unset window or a complete one; from must not
// come after to. schoolcalendar.DateLayout strings compare lexically as dates.
func validateWindow(from, to string, invalid func(string) error) error {
	if from == "" && to == "" {
		return nil
	}
	if from == "" || to == "" {
		return invalid("overlap window needs both from and to dates")
	}
	if err := validateDate(from, "overlap window from", invalid); err != nil {
		return err
	}
	if err := validateDate(to, "overlap window to", invalid); err != nil {
		return err
	}
	if to < from {
		return invalid("overlap window to must not be before from")
	}
	return nil
}

func validateDate(value, label string, invalid func(string) error) error {
	if value == "" {
		return nil
	}
	if _, err := time.Parse(schoolcalendar.DateLayout, value); err != nil {
		return invalid(label + " must be a calendar date in YYYY-MM-DD format")
	}
	return nil
}

func uniquePositive(ids []int64) []int64 {
	if ids == nil {
		return nil
	}
	result := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

// validateDateframeFields mirrors the legacy model rules: both instants are
// required and the range may not end before it starts.
func validateDateframeFields(fields schoolcalendar.DateframeFields) error {
	if fields.StartDate.IsZero() {
		return invalidDateframe("start date is required")
	}
	if fields.EndDate.IsZero() {
		return invalidDateframe("end date is required")
	}
	if fields.EndDate.Before(fields.StartDate) {
		return invalidDateframe("end date must be on or after start date")
	}
	return nil
}

func invalidPeriod(reason string) error {
	return &schoolcalendar.InvalidCalendarPeriodError{Reason: reason}
}
func invalidClosingDay(reason string) error {
	return &schoolcalendar.InvalidClosingDayError{Reason: reason}
}
func invalidDateframe(reason string) error {
	return &schoolcalendar.InvalidDateframeError{Reason: reason}
}

func (s *Service) TenantHolidays(ctx context.Context, from, to string) ([]schoolcalendar.Holiday, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	region, err := s.tenantRegion(ctx)
	if err != nil {
		return nil, err
	}
	return s.ListHolidays(ctx, region, from, to)
}

func (s *Service) TenantHolidayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	if err := validateHolidayWindow(from, to); err != nil {
		return nil, err
	}
	region, err := s.tenantRegion(ctx)
	if err != nil {
		return nil, err
	}
	return s.HolidayDates(ctx, region, from, to)
}

func (s *Service) ClosingDayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	if from == "" || to == "" {
		return nil, invalidClosingDay("closing day window needs both from and to dates")
	}
	if err := validateDate(from, "closing day window from", invalidClosingDay); err != nil {
		return nil, err
	}
	if err := validateDate(to, "closing day window to", invalidClosingDay); err != nil {
		return nil, err
	}
	set := make(map[string]bool)
	if to < from {
		return set, nil
	}
	ranges, err := s.ListClosingDays(ctx, schoolcalendar.ClosingDayFilter{OverlappingFrom: from, OverlappingTo: to})
	if err != nil {
		return nil, err
	}
	for _, closing := range ranges {
		start := max(closing.StartDate, from)
		end := min(closing.EndDate, to)
		for day := start; day <= end; day = nextDay(day) {
			set[day] = true
		}
	}
	return set, nil
}

func (s *Service) NonWorkingDayDates(ctx context.Context, from, to string) (map[string]bool, error) {
	set, err := s.TenantHolidayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	closing, err := s.ClosingDayDates(ctx, from, to)
	if err != nil {
		return nil, err
	}
	for day := range closing {
		set[day] = true
	}
	return set, nil
}

// tenantRegion resolves the tenant's federal state and checks that it names
// a supported holiday region; the setting is the single source of truth.
func (s *Service) tenantRegion(ctx context.Context) (string, error) {
	region, err := s.administration.FederalState(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %w", schoolcalendar.ErrFederalStateUnavailable, err)
	}
	if !s.ValidHolidayRegion(region) {
		return "", fmt.Errorf("%w: unsupported region %q", schoolcalendar.ErrFederalStateUnavailable, region)
	}
	return region, nil
}

// nextDay advances a schoolcalendar.DateLayout day by one; the input was validated, so a
// parse failure cannot happen and would only end the loop early.
func nextDay(day string) string {
	value, err := time.Parse(schoolcalendar.DateLayout, day)
	if err != nil {
		return "9999-12-31"
	}
	return value.AddDate(0, 0, 1).Format(schoolcalendar.DateLayout)
}
