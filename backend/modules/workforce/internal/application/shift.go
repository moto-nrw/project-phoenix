package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// The Dienstplan rows (#2689): concrete shifts, their recurrence rules, the
// removed occurrences and the tenant's shift types. Every write runs on the
// caller's ambient tenant transaction when one exists and otherwise opens one,
// so a multi-row plan change composed by the caller stays one unit of work.

// --- staff shifts ---

func (s *Service) FindStaffShift(ctx context.Context, id int64) (result domain.StaffShift, err error) {
	err = s.run("find_staff_shift", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffShift(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffShiftNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffShifts(ctx context.Context, filter domain.StaffShiftFilter) (result []domain.StaffShift, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_staff_shifts", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffShifts(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) UsedStaffShiftWeeks(ctx context.Context, from, to string) (result []string, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return nil, validationErr
	}
	if validationErr := domain.ValidateDate(to, "to"); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("used_staff_shift_weeks", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UsedStaffShiftWeeks(ctx, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaffShift(ctx context.Context, shift domain.StaffShift) (result domain.StaffShift, err error) {
	if validationErr := domain.ValidateStaffShift(shift); validationErr != nil {
		return domain.StaffShift{}, validationErr
	}
	err = s.run("create_staff_shift", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShift(txCtx, shift)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// CreateStaffShifts materializes many rows in one statement; the series
// engine calls it once per plan instead of once per occurrence.
func (s *Service) CreateStaffShifts(ctx context.Context, shifts []domain.StaffShift) (result []domain.StaffShift, err error) {
	for _, shift := range shifts {
		if validationErr := domain.ValidateStaffShift(shift); validationErr != nil {
			return nil, validationErr
		}
	}
	if len(shifts) == 0 {
		return []domain.StaffShift{}, nil
	}
	err = s.run("create_staff_shifts", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShifts(txCtx, shifts)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffShift(ctx context.Context, shift domain.StaffShift) (result domain.StaffShift, err error) {
	if validationErr := domain.ValidateStaffShift(shift); validationErr != nil {
		return domain.StaffShift{}, validationErr
	}
	err = s.run("update_staff_shift", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffShift(txCtx, shift)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffShiftNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (result int64, err error) {
	err = s.run("set_staff_shift_sick_absence", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetStaffShiftSickAbsence(txCtx, shiftID, absenceID)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffShift(ctx context.Context, id int64) error {
	return s.run("delete_staff_shift", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffShift(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (result int64, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("delete_upcoming_staff_shifts", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteUpcomingStaffShifts(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (result int64, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("delete_regenerable_series_shifts", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteRegenerableSeriesShifts(txCtx, seriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (result int64, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("repoint_detached_series_shifts", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RepointDetachedSeriesShifts(txCtx, fromSeriesID, toSeriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// --- shift series ---

func (s *Service) FindStaffShiftSeries(ctx context.Context, id int64) (result domain.StaffShiftSeries, err error) {
	err = s.run("find_staff_shift_series", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffShiftSeries(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftSeriesNotFound
		}
		return err
	})
	return result, err
}

// FindOverlappingSeriesInLineage reports found=false through the not-found
// sentinel; callers that treat "no successor" as a normal outcome translate
// it back.
func (s *Service) FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (result domain.StaffShiftSeries, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return domain.StaffShiftSeries{}, validationErr
	}
	err = s.run("find_overlapping_series_in_lineage", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindOverlappingSeriesInLineage(ctx, rootID, excludeID, from)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftSeriesNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) CreateStaffShiftSeries(ctx context.Context, series domain.StaffShiftSeries) (result domain.StaffShiftSeries, err error) {
	if validationErr := domain.ValidateStaffShiftSeries(series); validationErr != nil {
		return domain.StaffShiftSeries{}, validationErr
	}
	err = s.run("create_staff_shift_series", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShiftSeries(txCtx, series)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffShiftSeries(ctx context.Context, series domain.StaffShiftSeries) (result domain.StaffShiftSeries, err error) {
	if validationErr := domain.ValidateStaffShiftSeries(series); validationErr != nil {
		return domain.StaffShiftSeries{}, validationErr
	}
	err = s.run("update_staff_shift_series", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffShiftSeries(txCtx, series)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrShiftSeriesNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffShiftSeries(ctx context.Context, id int64) error {
	return s.run("delete_staff_shift_series", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffShiftSeries(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) CapStaffShiftSeries(ctx context.Context, id int64, until string) error {
	if until == "" {
		return &domain.InvalidShiftSeriesError{Reason: "until is required"}
	}
	if validationErr := domain.ValidateDate(until, "until"); validationErr != nil {
		return validationErr
	}
	return s.run("cap_staff_shift_series", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CapStaffShiftSeries(txCtx, id, until)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (result int64, err error) {
	if until == "" {
		return 0, &domain.InvalidShiftSeriesError{Reason: "until is required"}
	}
	if validationErr := domain.ValidateDate(until, "until"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("cap_staff_shift_series_for_staff", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CapStaffShiftSeriesForStaff(txCtx, staffID, until)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// --- series exceptions ---

func (s *Service) RecordSeriesException(ctx context.Context, exception domain.StaffShiftSeriesException) error {
	if validationErr := domain.ValidateStaffShiftSeriesException(exception); validationErr != nil {
		return validationErr
	}
	return s.run("record_series_exception", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.RecordSeriesException(txCtx, exception)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) SeriesExceptionDates(ctx context.Context, seriesID int64) (result []string, err error) {
	err = s.run("series_exception_dates", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.SeriesExceptionDates(ctx, seriesID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (result int64, err error) {
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("repoint_series_exceptions", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RepointSeriesExceptions(txCtx, fromSeriesID, toSeriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// --- shift types ---

func (s *Service) ListShiftTypes(ctx context.Context) (result []domain.ShiftType, err error) {
	err = s.run("list_shift_types", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListShiftTypes(ctx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindShiftType(ctx context.Context, id int64) (result domain.ShiftType, err error) {
	err = s.run("find_shift_type", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindShiftType(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftTypeNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) CreateShiftType(ctx context.Context, shiftType domain.ShiftType) (result domain.ShiftType, err error) {
	normalized, validationErr := domain.NormalizeShiftType(shiftType)
	if validationErr != nil {
		return domain.ShiftType{}, validationErr
	}
	err = s.run("create_shift_type", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateShiftType(txCtx, normalized)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) CreateShiftTypeIfAbsent(ctx context.Context, shiftType domain.ShiftType) (result domain.ShiftType, created bool, err error) {
	normalized, validationErr := domain.NormalizeShiftType(shiftType)
	if validationErr != nil {
		return domain.ShiftType{}, false, validationErr
	}
	err = s.run("create_shift_type_if_absent", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, created, writeStats, err = s.store.CreateShiftTypeIfAbsent(txCtx, normalized)
			stats.Add(writeStats)
			return err
		})
	})
	return result, created, err
}

func (s *Service) UpdateShiftType(ctx context.Context, shiftType domain.ShiftType) (result domain.ShiftType, err error) {
	normalized, validationErr := domain.NormalizeShiftType(shiftType)
	if validationErr != nil {
		return domain.ShiftType{}, validationErr
	}
	err = s.run("update_shift_type", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateShiftType(txCtx, normalized)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrShiftTypeNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteShiftType(ctx context.Context, id int64) error {
	return s.run("delete_shift_type", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteShiftType(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}
