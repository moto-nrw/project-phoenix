package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- work sessions ---

func (s *Service) FindWorkSession(ctx context.Context, id int64) (result domain.WorkSession, err error) {
	err = s.run("find_work_session", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindWorkSession(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) LockOpenWorkSession(ctx context.Context, id int64) (result domain.WorkSession, err error) {
	err = s.run("lock_open_work_session", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.LockOpenWorkSession(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (result domain.WorkSession, err error) {
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return domain.WorkSession{}, &domain.InvalidWorkSessionError{Reason: validationErr.Error()}
	}
	err = s.run("open_work_session_on", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.OpenWorkSessionOn(ctx, staffID, date, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

// TodayOpenWorkSession resolves the running block filed on the module clock's
// calendar day.
func (s *Service) TodayOpenWorkSession(ctx context.Context, staffID int64) (result domain.WorkSession, err error) {
	err = s.run("today_open_work_session", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.OpenWorkSessionOn(ctx, staffID, s.clock.Today(), false)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

// LatestOpenWorkSession resolves the running block of a staff member inside
// the live window as of the module clock.
func (s *Service) LatestOpenWorkSession(ctx context.Context, staffID int64) (result domain.WorkSession, err error) {
	err = s.run("latest_open_work_session", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.LatestOpenWorkSession(ctx, staffID, s.clock.Today(), s.clock.Now())
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListWorkSessions(ctx context.Context, filter domain.WorkSessionFilter) (result []domain.WorkSession, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_work_sessions", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkSessions(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) (result []domain.WorkSession, err error) {
	if len(staffIDs) == 0 {
		return []domain.WorkSession{}, nil
	}
	err = s.run("list_overlapping_work_sessions", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountWorkSessions(ctx context.Context, filter domain.WorkSessionFilter) (result int, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("count_work_sessions", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountWorkSessions(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) OldestWorkSessionDate(ctx context.Context, column, before string) (result string, err error) {
	if validationErr := domain.ValidateWorkSessionDateColumn(column); validationErr != nil {
		return "", validationErr
	}
	if validationErr := domain.ValidateDate(before, "before"); validationErr != nil {
		return "", &domain.InvalidWorkSessionError{Reason: validationErr.Error()}
	}
	err = s.run("oldest_work_session_date", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.OldestWorkSessionDate(ctx, before)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (result int64, err error) {
	if validationErr := domain.ValidateWorkSessionDateColumn(column); validationErr != nil {
		return 0, validationErr
	}
	if cutoff == "" {
		return 0, &domain.InvalidWorkSessionError{Reason: "cutoff is required"}
	}
	if validationErr := domain.ValidateDate(cutoff, "cutoff"); validationErr != nil {
		return 0, &domain.InvalidWorkSessionError{Reason: validationErr.Error()}
	}
	err = s.run("delete_work_sessions_older_than", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteWorkSessionsOlderThan(txCtx, cutoff)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) WorkPresenceMap(ctx context.Context) (result map[int64]string, err error) {
	err = s.run("work_presence_map", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.WorkPresenceMap(ctx, s.clock.Today(), s.clock.Now())
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateWorkSession(ctx context.Context, value domain.WorkSession) (result domain.WorkSession, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.WorkSession{}, validationErr
	}
	err = s.run("create_work_session", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateWorkSession(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateWorkSession(ctx context.Context, value domain.WorkSession) (result domain.WorkSession, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.WorkSession{}, validationErr
	}
	err = s.run("update_work_session", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateWorkSession(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrWorkSessionNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteWorkSession(ctx context.Context, id int64) error {
	return s.run("delete_work_session", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteWorkSession(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (result int64, err error) {
	if minutes < 0 {
		return 0, &domain.InvalidWorkSessionError{Reason: "break minutes cannot be negative"}
	}
	err = s.run("set_work_session_break_minutes", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetWorkSessionBreakMinutes(txCtx, id, minutes)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (result bool, err error) {
	if checkOut.IsZero() {
		return false, &domain.InvalidWorkSessionError{Reason: "check-out time is required"}
	}
	err = s.run("close_work_session", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CloseWorkSession(txCtx, id, checkOut, autoCheckedOut)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// --- breaks ---

func (s *Service) FindWorkSessionBreak(ctx context.Context, id int64) (result domain.WorkSessionBreak, err error) {
	err = s.run("find_work_session_break", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindWorkSessionBreak(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrWorkSessionBreakNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListWorkSessionBreaks(ctx context.Context, filter domain.WorkSessionBreakFilter) (result []domain.WorkSessionBreak, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_work_session_breaks", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListWorkSessionBreaks(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) (result []domain.WorkSessionBreak, err error) {
	err = s.run("expired_work_session_breaks", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ExpiredWorkSessionBreaks(ctx, before)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateWorkSessionBreak(ctx context.Context, value domain.WorkSessionBreak) (result domain.WorkSessionBreak, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.WorkSessionBreak{}, validationErr
	}
	err = s.run("create_work_session_break", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateWorkSessionBreak(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateWorkSessionBreak(ctx context.Context, value domain.WorkSessionBreak) (result domain.WorkSessionBreak, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.WorkSessionBreak{}, validationErr
	}
	err = s.run("update_work_session_break", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateWorkSessionBreak(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrWorkSessionBreakNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteWorkSessionBreak(ctx context.Context, id int64) error {
	return s.run("delete_work_session_break", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteWorkSessionBreak(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (result int64, err error) {
	if endedAt.IsZero() {
		return 0, &domain.InvalidWorkSessionError{Reason: "ended_at is required"}
	}
	if durationMinutes < 0 {
		return 0, &domain.InvalidWorkSessionError{Reason: "duration_minutes cannot be negative"}
	}
	err = s.run("end_work_session_break", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.EndWorkSessionBreak(txCtx, id, endedAt, durationMinutes)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (result int64, err error) {
	if durationMinutes < 0 {
		return 0, &domain.InvalidWorkSessionError{Reason: "duration_minutes cannot be negative"}
	}
	err = s.run("set_work_session_break_duration", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetWorkSessionBreakDuration(txCtx, id, durationMinutes, endedAt)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// --- staff balance adjustments ---

func (s *Service) FindStaffBalanceAdjustment(ctx context.Context, id int64) (result domain.StaffBalanceAdjustment, err error) {
	err = s.run("find_staff_balance_adjustment", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffBalanceAdjustment(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffBalanceAdjustmentNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffBalanceAdjustments(ctx context.Context, filter domain.StaffBalanceAdjustmentFilter) (result []domain.StaffBalanceAdjustment, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_staff_balance_adjustments", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffBalanceAdjustments(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaffBalanceAdjustment(ctx context.Context, value domain.StaffBalanceAdjustment) (result domain.StaffBalanceAdjustment, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffBalanceAdjustment{}, validationErr
	}
	err = s.run("create_staff_balance_adjustment", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffBalanceAdjustment(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffBalanceAdjustment(ctx context.Context, value domain.StaffBalanceAdjustment) (result domain.StaffBalanceAdjustment, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffBalanceAdjustment{}, validationErr
	}
	err = s.run("update_staff_balance_adjustment", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffBalanceAdjustment(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffBalanceAdjustmentNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	return s.run("delete_staff_balance_adjustment", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffBalanceAdjustment(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

// --- staff vacation openings ---

func (s *Service) FindStaffVacationOpening(ctx context.Context, id int64) (result domain.StaffVacationOpening, err error) {
	err = s.run("find_staff_vacation_opening", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffVacationOpening(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffVacationOpeningNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffVacationOpenings(ctx context.Context, filter domain.StaffVacationFilter) (result []domain.StaffVacationOpening, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_staff_vacation_openings", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffVacationOpenings(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaffVacationOpening(ctx context.Context, value domain.StaffVacationOpening) (result domain.StaffVacationOpening, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffVacationOpening{}, validationErr
	}
	err = s.run("create_staff_vacation_opening", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffVacationOpening(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffVacationOpening(ctx context.Context, value domain.StaffVacationOpening) (result domain.StaffVacationOpening, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffVacationOpening{}, validationErr
	}
	err = s.run("update_staff_vacation_opening", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffVacationOpening(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffVacationOpeningNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffVacationOpening(ctx context.Context, id int64) error {
	return s.run("delete_staff_vacation_opening", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffVacationOpening(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

// --- staff vacation quota ---

func (s *Service) FindStaffVacationQuota(ctx context.Context, id int64) (result domain.StaffVacationQuota, err error) {
	err = s.run("find_staff_vacation_quota", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffVacationQuota(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffVacationQuotaNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffVacationQuotas(ctx context.Context, filter domain.StaffVacationFilter) (result []domain.StaffVacationQuota, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_staff_vacation_quotas", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffVacationQuotas(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) (result domain.StaffVacationQuota, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffVacationQuota{}, validationErr
	}
	err = s.run("create_staff_vacation_quota", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffVacationQuota(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) (result domain.StaffVacationQuota, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffVacationQuota{}, validationErr
	}
	err = s.run("update_staff_vacation_quota", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffVacationQuota(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffVacationQuotaNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffVacationQuota(ctx context.Context, id int64) error {
	return s.run("delete_staff_vacation_quota", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffVacationQuota(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) UpsertStaffVacationQuota(ctx context.Context, value domain.StaffVacationQuota) error {
	if validationErr := value.Validate(); validationErr != nil {
		return validationErr
	}
	return s.run("upsert_staff_vacation_quota", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.UpsertStaffVacationQuota(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
}

// LockStaffBalanceWrites serializes every writer that changes a staff member's
// Stundenkonto inputs on the ambient transaction.
func (s *Service) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return s.run("lock_staff_balance_writes", func(*domain.OperationStats) error {
		return s.transaction.LockStaffBalance(ctx, staffID)
	})
}
