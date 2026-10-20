package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- work sessions ---

func (s *Service) FindWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	if id <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("work session ID is required")
	}
	var result workforce.WorkSession
	var err error
	err = s.run(ctx, "find_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindWorkSession(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) LockOpenWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	if id <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("work session ID is required")
	}
	var result workforce.WorkSession
	var err error
	err = s.run(ctx, "lock_open_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.LockOpenWorkSession(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) openWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (result workforce.WorkSession, err error) {
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return workforce.WorkSession{}, &workforce.InvalidWorkSessionError{Reason: validationErr.Error()}
	}
	err = s.run(ctx, "open_work_session_on", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.OpenWorkSessionOn(ctx, staffID, date, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

// TodayOpenWorkSession resolves the running block filed on the module clock's
// calendar day.
func (s *Service) TodayOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	if staffID <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("staff ID is required")
	}
	var result workforce.WorkSession
	var err error
	err = s.run(ctx, "today_open_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.OpenWorkSessionOn(ctx, staffID, s.clock.Today(), false)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

// LatestOpenWorkSession resolves the running block of a staff member inside
// the live window as of the module clock.
func (s *Service) LatestOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	if staffID <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("staff ID is required")
	}
	var result workforce.WorkSession
	var err error
	err = s.run(ctx, "latest_open_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.LatestOpenWorkSession(ctx, staffID, s.clock.Today(), s.clock.Now())
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) ListWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) ([]workforce.WorkSession, error) {
	order := make([]workforce.WorkSessionOrder, 0, len(filter.Order))
	for _, entry := range filter.Order {
		order = append(order, workforce.WorkSessionOrder{Field: workforce.WorkSessionOrderField(entry.Field), Descending: entry.Descending})
	}
	storedFilter := workforce.WorkSessionFilter{
		IDs: filter.IDs, StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Date: filter.Date,
		DateFrom: filter.DateFrom, DateTo: filter.DateTo, DateBefore: filter.DateBefore, Open: filter.Open,
		Order: order, Limit: filter.Limit, Offset: filter.Offset,
	}
	var result []workforce.WorkSession
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "list_work_sessions", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkSessions(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	if len(staffIDs) == 0 {
		return []workforce.WorkSession{}, nil
	}
	var result []workforce.WorkSession
	var err error
	err = s.run(ctx, "list_overlapping_work_sessions", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) CountWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	storedFilter := workforce.WorkSessionFilter{
		IDs: filter.IDs, StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Date: filter.Date,
		DateFrom: filter.DateFrom, DateTo: filter.DateTo, DateBefore: filter.DateBefore, Open: filter.Open,
	}
	var result int
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return 0, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "count_work_sessions", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountWorkSessions(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) OldestWorkSessionDate(ctx context.Context, column, before string) (string, error) {
	var result string
	var err error
	if validationErr := validateWorkSessionDateColumn(column); validationErr != nil {
		return "", mapWorkSessionError(validationErr)
	}
	if validationErr := domain.ValidateDate(before, "before"); validationErr != nil {
		return "", mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: validationErr.Error()})
	}
	err = s.run(ctx, "oldest_work_session_date", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.OldestWorkSessionDate(ctx, before)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	var result int64
	var err error
	if validationErr := validateWorkSessionDateColumn(column); validationErr != nil {
		return 0, mapWorkSessionError(validationErr)
	}
	if cutoff == "" {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "cutoff is required"})
	}
	if validationErr := domain.ValidateDate(cutoff, "cutoff"); validationErr != nil {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: validationErr.Error()})
	}
	err = s.run(ctx, "delete_work_sessions_older_than", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteWorkSessionsOlderThan(txCtx, cutoff)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) WorkPresenceMap(ctx context.Context) (map[int64]string, error) {
	var result map[int64]string
	var err error
	err = s.run(ctx, "work_presence_map", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.WorkPresenceMap(ctx, s.clock.Today(), s.clock.Now())
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) CreateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	storedValue := value
	var result workforce.WorkSession
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.WorkSession{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "create_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateWorkSession(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) UpdateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	if value.ID <= 0 {
		return workforce.WorkSession{}, invalidWorkSession("work session ID is required")
	}
	storedValue := value
	var result workforce.WorkSession
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.WorkSession{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "update_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateWorkSession(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return workforce.ErrWorkSessionNotFound
			}
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) DeleteWorkSession(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("work session ID is required")
	}
	return mapWorkSessionError(s.run(ctx, "delete_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteWorkSession(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session ID is required")
	}
	var result int64
	var err error
	if minutes < 0 {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "break minutes cannot be negative"})
	}
	err = s.run(ctx, "set_work_session_break_minutes", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetWorkSessionBreakMinutes(txCtx, id, minutes)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, error) {
	if id <= 0 {
		return false, invalidWorkSession("work session ID is required")
	}
	var result bool
	var err error
	if checkOut.IsZero() {
		return false, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "check-out time is required"})
	}
	err = s.run(ctx, "close_work_session", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CloseWorkSession(txCtx, id, checkOut, autoCheckedOut)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

// --- breaks ---

func (s *Service) FindWorkSessionBreak(ctx context.Context, id int64) (workforce.WorkSessionBreak, error) {
	if id <= 0 {
		return workforce.WorkSessionBreak{}, invalidWorkSession("work session break ID is required")
	}
	var result workforce.WorkSessionBreak
	var err error
	err = s.run(ctx, "find_work_session_break", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindWorkSessionBreak(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrWorkSessionBreakNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) ListWorkSessionBreaks(ctx context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error) {
	storedFilter := workforce.WorkSessionBreakFilter{
		SessionID: filter.SessionID, SessionIDs: filter.SessionIDs, Active: filter.Active, Limit: filter.Limit, Offset: filter.Offset,
	}
	var result []workforce.WorkSessionBreak
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "list_work_session_breaks", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkSessionBreaks(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]workforce.WorkSessionBreak, error) {
	var result []workforce.WorkSessionBreak
	var err error
	err = s.run(ctx, "expired_work_session_breaks", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ExpiredWorkSessionBreaks(ctx, before)
		stats.Add(queryStats)
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) CreateWorkSessionBreak(ctx context.Context, value workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error) {
	storedValue := value
	var result workforce.WorkSessionBreak
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.WorkSessionBreak{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "create_work_session_break", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateWorkSessionBreak(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) UpdateWorkSessionBreak(ctx context.Context, value workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error) {
	if value.ID <= 0 {
		return workforce.WorkSessionBreak{}, invalidWorkSession("work session break ID is required")
	}
	storedValue := value
	var result workforce.WorkSessionBreak
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.WorkSessionBreak{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "update_work_session_break", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateWorkSessionBreak(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return workforce.ErrWorkSessionBreakNotFound
			}
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) DeleteWorkSessionBreak(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("work session break ID is required")
	}
	return mapWorkSessionError(s.run(ctx, "delete_work_session_break", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteWorkSessionBreak(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session break ID is required")
	}
	var result int64
	var err error
	if endedAt.IsZero() {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "ended_at is required"})
	}
	if durationMinutes < 0 {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "duration_minutes cannot be negative"})
	}
	err = s.run(ctx, "end_work_session_break", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.EndWorkSessionBreak(txCtx, id, endedAt, durationMinutes)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error) {
	if id <= 0 {
		return 0, invalidWorkSession("work session break ID is required")
	}
	var result int64
	var err error
	if durationMinutes < 0 {
		return 0, mapWorkSessionError(&workforce.InvalidWorkSessionError{Reason: "duration_minutes cannot be negative"})
	}
	err = s.run(ctx, "set_work_session_break_duration", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetWorkSessionBreakDuration(txCtx, id, durationMinutes, endedAt)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

// --- staff balance adjustments ---

func (s *Service) FindStaffBalanceAdjustment(ctx context.Context, id int64) (workforce.StaffBalanceAdjustment, error) {
	if id <= 0 {
		return workforce.StaffBalanceAdjustment{}, invalidWorkSession("staff balance adjustment ID is required")
	}
	var result workforce.StaffBalanceAdjustment
	var err error
	err = s.run(ctx, "find_staff_balance_adjustment", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffBalanceAdjustment(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrStaffBalanceAdjustmentNotFound
		}
		return err
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) ListStaffBalanceAdjustments(ctx context.Context, filter workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	storedFilter := workforce.StaffBalanceAdjustmentFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Types: filter.Types,
		EffectiveFrom: filter.EffectiveFrom, EffectiveTo: filter.EffectiveTo, Limit: filter.Limit, Offset: filter.Offset,
	}
	var result []workforce.StaffBalanceAdjustment
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "list_staff_balance_adjustments", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffBalanceAdjustments(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, mapWorkSessionError(storedErr)
	}
	storedResult := make([]workforce.StaffBalanceAdjustment, 0, len(values))
	storedResult = append(storedResult, values...)
	return storedResult, mapWorkSessionError(storedErr)
}

func (s *Service) CreateStaffBalanceAdjustment(ctx context.Context, value workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	storedValue := value
	var result workforce.StaffBalanceAdjustment
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.StaffBalanceAdjustment{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "create_staff_balance_adjustment", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffBalanceAdjustment(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) UpdateStaffBalanceAdjustment(ctx context.Context, value workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	if value.ID <= 0 {
		return workforce.StaffBalanceAdjustment{}, invalidWorkSession("staff balance adjustment ID is required")
	}
	storedValue := value
	var result workforce.StaffBalanceAdjustment
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.StaffBalanceAdjustment{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "update_staff_balance_adjustment", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffBalanceAdjustment(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return workforce.ErrStaffBalanceAdjustmentNotFound
			}
			return err
		})
	})
	return result, mapWorkSessionError(err)
}

func (s *Service) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff balance adjustment ID is required")
	}
	return mapWorkSessionError(s.run(ctx, "delete_staff_balance_adjustment", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffBalanceAdjustment(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

// --- staff vacation openings ---

func (s *Service) FindStaffVacationOpening(ctx context.Context, id int64) (workforce.StaffVacationOpening, error) {
	if id <= 0 {
		return workforce.StaffVacationOpening{}, invalidWorkSession("staff vacation opening ID is required")
	}
	var result domain.StaffVacationOpening
	var err error
	err = s.run(ctx, "find_staff_vacation_opening", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffVacationOpening(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrStaffVacationOpeningNotFound
		}
		return err
	})
	return openingToPublic(result), mapWorkSessionError(err)
}

func (s *Service) ListStaffVacationOpenings(ctx context.Context, filter workforce.StaffVacationFilter) ([]workforce.StaffVacationOpening, error) {
	storedFilter := filter
	var result []domain.StaffVacationOpening
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "list_staff_vacation_openings", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffVacationOpenings(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, mapWorkSessionError(storedErr)
	}
	storedResult := make([]workforce.StaffVacationOpening, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, openingToPublic(value))
	}
	return storedResult, mapWorkSessionError(storedErr)
}

func (s *Service) CreateStaffVacationOpening(ctx context.Context, value workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error) {
	storedValue := openingToDomain(value)
	var result domain.StaffVacationOpening
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.StaffVacationOpening{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "create_staff_vacation_opening", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffVacationOpening(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return openingToPublic(result), mapWorkSessionError(err)
}

func (s *Service) UpdateStaffVacationOpening(ctx context.Context, value workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error) {
	if value.ID <= 0 {
		return workforce.StaffVacationOpening{}, invalidWorkSession("staff vacation opening ID is required")
	}
	storedValue := openingToDomain(value)
	var result domain.StaffVacationOpening
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return workforce.StaffVacationOpening{}, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "update_staff_vacation_opening", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffVacationOpening(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return workforce.ErrStaffVacationOpeningNotFound
			}
			return err
		})
	})
	return openingToPublic(result), mapWorkSessionError(err)
}

func (s *Service) DeleteStaffVacationOpening(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff vacation opening ID is required")
	}
	return mapWorkSessionError(s.run(ctx, "delete_staff_vacation_opening", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffVacationOpening(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

// --- staff vacation quota ---

func (s *Service) FindStaffVacationQuota(ctx context.Context, id int64) (workforce.StaffVacationQuota, error) {
	if id <= 0 {
		return workforce.StaffVacationQuota{}, invalidWorkSession("staff vacation quota ID is required")
	}
	var result workforce.StaffVacationQuota
	var err error
	err = s.run(ctx, "find_staff_vacation_quota", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffVacationQuota(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return workforce.ErrStaffVacationQuotaNotFound
		}
		return err
	})
	return quotaToPublic(result), mapWorkSessionError(err)
}

func (s *Service) ListStaffVacationQuotas(ctx context.Context, filter workforce.StaffVacationFilter) ([]workforce.StaffVacationQuota, error) {
	storedFilter := filter
	var result []workforce.StaffVacationQuota
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return nil, mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "list_staff_vacation_quotas", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffVacationQuotas(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, mapWorkSessionError(storedErr)
	}
	storedResult := make([]workforce.StaffVacationQuota, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, quotaToPublic(value))
	}
	return storedResult, mapWorkSessionError(storedErr)
}

func (s *Service) CreateStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) (workforce.StaffVacationQuota, error) {
	storedValue := value
	var result workforce.StaffVacationQuota
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return quotaToPublic(workforce.StaffVacationQuota{}), mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "create_staff_vacation_quota", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffVacationQuota(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return quotaToPublic(result), mapWorkSessionError(err)
}

func (s *Service) UpdateStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) (workforce.StaffVacationQuota, error) {
	if value.ID <= 0 {
		return workforce.StaffVacationQuota{}, invalidWorkSession("staff vacation quota ID is required")
	}
	storedValue := value
	var result workforce.StaffVacationQuota
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return quotaToPublic(workforce.StaffVacationQuota{}), mapWorkSessionError(validationErr)
	}
	err = s.run(ctx, "update_staff_vacation_quota", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffVacationQuota(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return workforce.ErrStaffVacationQuotaNotFound
			}
			return err
		})
	})
	return quotaToPublic(result), mapWorkSessionError(err)
}

func (s *Service) DeleteStaffVacationQuota(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidWorkSession("staff vacation quota ID is required")
	}
	return mapWorkSessionError(s.run(ctx, "delete_staff_vacation_quota", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffVacationQuota(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) UpsertStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) error {
	storedValue := value
	if validationErr := storedValue.Validate(); validationErr != nil {
		return mapWorkSessionError(validationErr)
	}
	return mapWorkSessionError(s.run(ctx, "upsert_staff_vacation_quota", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.UpsertStaffVacationQuota(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	}))
}

// LockStaffBalanceWrites serializes every writer that changes a staff member's
// Stundenkonto inputs on the ambient transaction.
func (s *Service) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	return mapWorkSessionError(s.observeOperation("lock_staff_balance_writes", func(*domain.OperationStats) error {
		return s.transaction.LockStaffBalance(ctx, staffID)
	}))
}
