package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// The Dienstplan rows (#2689): concrete shifts, their recurrence rules, the
// removed occurrences and the tenant's shift types. Every write runs on the
// caller's ambient tenant transaction when one exists and otherwise opens one,
// so a multi-row plan change composed by the caller stays one unit of work.

// --- staff shifts ---

// ReadShift owns the tenant-scoped read and returns the calculation record
// directly to planning. The capability maps it only at its outer boundary.
func (s *Service) ReadShift(ctx context.Context, id int64) (*workforce.ShiftRecord, error) {
	if id <= 0 {
		return nil, &workforce.InvalidStaffShiftError{Reason: "staff shift ID is required"}
	}
	var value domain.StaffShift
	err := s.run(ctx, "find_staff_shift", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		var err error
		value, found, queryStats, err = s.store.FindStaffShift(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffShiftNotFound
		}
		return err
	})
	if err != nil {
		if errors.Is(err, domain.ErrStaffShiftNotFound) {
			return nil, transactionruntime.RecordErrors{}.MissingRecordError("find by id")
		}
		return nil, mapShiftError(err)
	}
	return shiftReadRecord(value), nil
}

// ReadShifts owns filter validation, observation and the batched store read.
func (s *Service) ReadShifts(ctx context.Context, filter workforce.StaffShiftFilter) ([]*workforce.ShiftRecord, error) {
	selection := shiftFilterToDomain(filter)
	if err := selection.Validate(); err != nil {
		return nil, mapShiftError(err)
	}
	var values []domain.StaffShift
	err := s.run(ctx, "list_staff_shifts", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		values, queryStats, err = s.store.ListStaffShifts(ctx, selection)
		stats.Add(queryStats)
		return err
	})
	if values == nil {
		return nil, mapShiftError(err)
	}
	result := make([]*workforce.ShiftRecord, 0, len(values))
	for _, value := range values {
		result = append(result, shiftReadRecord(value))
	}
	return result, mapShiftError(err)
}

func (s *Service) UsedStaffShiftWeeks(ctx context.Context, from, to string) ([]string, error) {
	var result []string
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return nil, mapShiftError(validationErr)
	}
	if validationErr := domain.ValidateDate(to, "to"); validationErr != nil {
		return nil, mapShiftError(validationErr)
	}
	err = s.run(ctx, "used_staff_shift_weeks", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UsedStaffShiftWeeks(ctx, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, mapShiftError(err)
}

func (s *Service) CreateStaffShift(ctx context.Context, shift *workforce.StaffShift) (workforce.StaffShift, error) {
	if shift == nil {
		return workforce.StaffShift{}, errors.New("workforce.StaffShift cannot be nil or zero value")
	}
	storedShift := shiftToDomain(*shift)
	var result domain.StaffShift
	var err error
	if validationErr := domain.ValidateStaffShift(storedShift); validationErr != nil {
		return shiftToPublic(domain.StaffShift{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "create_staff_shift", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if storedShift.Date >= s.clock.Today() {
				if err := s.lockStaffAssignment(txCtx, storedShift.StaffID); err != nil {
					return err
				}
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShift(txCtx, storedShift)
			stats.Add(writeStats)
			return err
		})
	})
	return shiftToPublic(result), mapShiftError(err)
}

// CreateStaffShifts materializes many rows in one statement; the series
// engine calls it once per plan instead of once per occurrence.
func (s *Service) CreateStaffShifts(ctx context.Context, shifts []workforce.StaffShift) ([]workforce.StaffShift, error) {
	values := make([]domain.StaffShift, 0, len(shifts))
	for _, shift := range shifts {
		values = append(values, shiftToDomain(shift))
	}
	storedShifts := values
	var result []domain.StaffShift
	var err error
	for _, shift := range storedShifts {
		if validationErr := domain.ValidateStaffShift(shift); validationErr != nil {
			return shiftsToPublic(nil), mapShiftError(validationErr)
		}
	}
	if len(storedShifts) == 0 {
		return shiftsToPublic([]domain.StaffShift{}), mapShiftError(nil)
	}
	err = s.run(ctx, "create_staff_shifts", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			ids := make([]int64, 0, len(storedShifts))
			for _, shift := range storedShifts {
				if shift.Date >= s.clock.Today() {
					ids = append(ids, shift.StaffID)
				}
			}
			if err := s.lockAssignmentStaff(txCtx, ids); err != nil {
				return err
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShifts(txCtx, storedShifts)
			stats.Add(writeStats)
			return err
		})
	})
	return shiftsToPublic(result), mapShiftError(err)
}

func (s *Service) UpdateStaffShift(ctx context.Context, shift workforce.StaffShift) (workforce.StaffShift, error) {
	if shift.ID <= 0 {
		return workforce.StaffShift{}, invalidShift("staff shift ID is required")
	}
	storedShift := shiftToDomain(shift)
	var result domain.StaffShift
	var err error
	if validationErr := domain.ValidateStaffShift(storedShift); validationErr != nil {
		return shiftToPublic(domain.StaffShift{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "update_staff_shift", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if storedShift.Date >= s.clock.Today() {
				if err := s.lockStaffAssignment(txCtx, storedShift.StaffID); err != nil {
					return err
				}
			}
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffShift(txCtx, storedShift)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffShiftNotFound
			}
			return err
		})
	})
	return shiftToPublic(result), mapShiftError(err)
}

func (s *Service) SetStaffShiftSickAbsence(ctx context.Context, shiftID int64, absenceID *int64) (int64, error) {
	if shiftID <= 0 {
		return 0, invalidShift("staff shift ID is required")
	}
	if absenceID != nil && *absenceID <= 0 {
		return 0, invalidShift("sick absence ID must be positive")
	}
	var result int64
	var err error
	err = s.run(ctx, "set_staff_shift_sick_absence", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SetStaffShiftSickAbsence(txCtx, shiftID, absenceID)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

func (s *Service) DeleteStaffShift(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidShift("staff shift ID is required")
	}
	return mapShiftError(s.run(ctx, "delete_staff_shift", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffShift(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) DeleteUpcomingStaffShifts(ctx context.Context, staffID int64, from string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidShift("staff ID is required")
	}
	var result int64
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, mapShiftError(validationErr)
	}
	err = s.run(ctx, "delete_upcoming_staff_shifts", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteUpcomingStaffShifts(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

func (s *Service) DeleteRegenerableSeriesShifts(ctx context.Context, seriesID int64, from string) (int64, error) {
	if seriesID <= 0 {
		return 0, invalidSeries("series ID is required")
	}
	var result int64
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, mapShiftError(validationErr)
	}
	err = s.run(ctx, "delete_regenerable_series_shifts", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteRegenerableSeriesShifts(txCtx, seriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

func (s *Service) RepointDetachedSeriesShifts(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	if fromSeriesID <= 0 || toSeriesID <= 0 {
		return 0, invalidSeries("series IDs are required")
	}
	var result int64
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, mapShiftError(validationErr)
	}
	err = s.run(ctx, "repoint_detached_series_shifts", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RepointDetachedSeriesShifts(txCtx, fromSeriesID, toSeriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

// --- shift series ---

func (s *Service) FindStaffShiftSeries(ctx context.Context, id int64) (workforce.StaffShiftSeries, error) {
	if id <= 0 {
		return workforce.StaffShiftSeries{}, invalidSeries("series ID is required")
	}
	var result domain.StaffShiftSeries
	var err error
	err = s.run(ctx, "find_staff_shift_series", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffShiftSeries(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftSeriesNotFound
		}
		return err
	})
	return seriesToPublic(result), mapShiftError(err)
}

// FindOverlappingSeriesInLineage reports found=false through the not-found
// sentinel; callers that treat "no successor" as a normal outcome translate
// it back.
func (s *Service) FindOverlappingSeriesInLineage(ctx context.Context, rootID, excludeID int64, from string) (workforce.StaffShiftSeries, error) {
	if rootID <= 0 {
		return workforce.StaffShiftSeries{}, invalidSeries("series root ID is required")
	}
	var result domain.StaffShiftSeries
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return seriesToPublic(domain.StaffShiftSeries{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "find_overlapping_series_in_lineage", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindOverlappingSeriesInLineage(ctx, rootID, excludeID, from)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftSeriesNotFound
		}
		return err
	})
	return seriesToPublic(result), mapShiftError(err)
}

func (s *Service) CreateStaffShiftSeries(ctx context.Context, series workforce.StaffShiftSeries) (workforce.StaffShiftSeries, error) {
	storedSeries := seriesToDomain(series)
	var result domain.StaffShiftSeries
	var err error
	if validationErr := domain.ValidateStaffShiftSeries(storedSeries); validationErr != nil {
		return seriesToPublic(domain.StaffShiftSeries{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "create_staff_shift_series", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if storedSeries.ValidUntil == "" || storedSeries.ValidUntil > s.clock.Today() {
				if err := s.lockStaffAssignment(txCtx, storedSeries.StaffID); err != nil {
					return err
				}
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffShiftSeries(txCtx, storedSeries)
			stats.Add(writeStats)
			return err
		})
	})
	return seriesToPublic(result), mapShiftError(err)
}

func (s *Service) UpdateStaffShiftSeries(ctx context.Context, series workforce.StaffShiftSeries) (workforce.StaffShiftSeries, error) {
	if series.ID <= 0 {
		return workforce.StaffShiftSeries{}, invalidSeries("series ID is required")
	}
	storedSeries := seriesToDomain(series)
	var result domain.StaffShiftSeries
	var err error
	if validationErr := domain.ValidateStaffShiftSeries(storedSeries); validationErr != nil {
		return seriesToPublic(domain.StaffShiftSeries{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "update_staff_shift_series", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if storedSeries.ValidUntil == "" || storedSeries.ValidUntil > s.clock.Today() {
				if err := s.lockStaffAssignment(txCtx, storedSeries.StaffID); err != nil {
					return err
				}
			}
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffShiftSeries(txCtx, storedSeries)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrShiftSeriesNotFound
			}
			return err
		})
	})
	return seriesToPublic(result), mapShiftError(err)
}

func (s *Service) DeleteStaffShiftSeries(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidSeries("series ID is required")
	}
	return mapShiftError(s.run(ctx, "delete_staff_shift_series", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffShiftSeries(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) CapStaffShiftSeries(ctx context.Context, id int64, until string) error {
	if id <= 0 {
		return invalidSeries("series ID is required")
	}
	if until == "" {
		return mapShiftError(&domain.InvalidShiftSeriesError{Reason: "until is required"})
	}
	if validationErr := domain.ValidateDate(until, "until"); validationErr != nil {
		return mapShiftError(validationErr)
	}
	return mapShiftError(s.run(ctx, "cap_staff_shift_series", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CapStaffShiftSeries(txCtx, id, until)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) CapStaffShiftSeriesForStaff(ctx context.Context, staffID int64, until string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidSeries("staff ID is required")
	}
	var result int64
	var err error
	if until == "" {
		return 0, mapShiftError(&domain.InvalidShiftSeriesError{Reason: "until is required"})
	}
	if validationErr := domain.ValidateDate(until, "until"); validationErr != nil {
		return 0, mapShiftError(validationErr)
	}
	err = s.run(ctx, "cap_staff_shift_series_for_staff", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CapStaffShiftSeriesForStaff(txCtx, staffID, until)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

// --- series exceptions ---

func (s *Service) RecordSeriesException(ctx context.Context, exception workforce.StaffShiftSeriesException) error {
	storedException := domain.StaffShiftSeriesException{
		ID: exception.ID, TenantID: exception.TenantID, SeriesID: exception.SeriesID,
		Date: exception.Date, CreatedBy: exception.CreatedBy, CreatedAt: exception.CreatedAt,
	}
	if validationErr := domain.ValidateStaffShiftSeriesException(storedException); validationErr != nil {
		return mapShiftError(validationErr)
	}
	return mapShiftError(s.run(ctx, "record_series_exception", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.RecordSeriesException(txCtx, storedException)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) SeriesExceptionDates(ctx context.Context, seriesID int64) ([]string, error) {
	if seriesID <= 0 {
		return nil, invalidSeries("series ID is required")
	}
	var result []string
	var err error
	err = s.run(ctx, "series_exception_dates", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.SeriesExceptionDates(ctx, seriesID)
		stats.Add(queryStats)
		return err
	})
	return result, mapShiftError(err)
}

func (s *Service) RepointSeriesExceptions(ctx context.Context, fromSeriesID, toSeriesID int64, from string) (int64, error) {
	if fromSeriesID <= 0 || toSeriesID <= 0 {
		return 0, invalidSeries("series IDs are required")
	}
	var result int64
	var err error
	if validationErr := domain.ValidateDate(from, "from"); validationErr != nil {
		return 0, mapShiftError(validationErr)
	}
	err = s.run(ctx, "repoint_series_exceptions", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RepointSeriesExceptions(txCtx, fromSeriesID, toSeriesID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapShiftError(err)
}

// --- shift types ---

func (s *Service) ListShiftTypes(ctx context.Context) ([]workforce.ShiftType, error) {
	var result []domain.ShiftType
	var err error
	err = s.run(ctx, "list_shift_types", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListShiftTypes(ctx)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if storedErr != nil {
		return nil, mapShiftError(storedErr)
	}
	storedResult := make([]workforce.ShiftType, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, shiftTypeToPublic(value))
	}
	return storedResult, nil
}

func (s *Service) FindShiftType(ctx context.Context, id int64) (workforce.ShiftType, error) {
	if id <= 0 {
		return workforce.ShiftType{}, workforce.ErrShiftTypeNotFound
	}
	var result domain.ShiftType
	var err error
	err = s.run(ctx, "find_shift_type", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindShiftType(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrShiftTypeNotFound
		}
		return err
	})
	return shiftTypeToPublic(result), mapShiftError(err)
}

func (s *Service) CreateShiftType(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, error) {
	storedShiftType := shiftTypeToDomain(shiftType)
	var result domain.ShiftType
	var err error
	normalized, validationErr := domain.NormalizeShiftType(storedShiftType)
	if validationErr != nil {
		return shiftTypeToPublic(domain.ShiftType{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "create_shift_type", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateShiftType(txCtx, normalized)
			stats.Add(writeStats)
			return err
		})
	})
	return shiftTypeToPublic(result), mapShiftError(err)
}

func (s *Service) CreateShiftTypeIfAbsent(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, bool, error) {
	storedShiftType := shiftTypeToDomain(shiftType)
	var result domain.ShiftType
	var created bool
	var err error
	normalized, validationErr := domain.NormalizeShiftType(storedShiftType)
	if validationErr != nil {
		return shiftTypeToPublic(domain.ShiftType{}), false, mapShiftError(validationErr)
	}
	err = s.run(ctx, "create_shift_type_if_absent", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, created, writeStats, err = s.store.CreateShiftTypeIfAbsent(txCtx, normalized)
			stats.Add(writeStats)
			return err
		})
	})
	return shiftTypeToPublic(result), created, mapShiftError(err)
}

func (s *Service) UpdateShiftType(ctx context.Context, shiftType workforce.ShiftType) (workforce.ShiftType, error) {
	if shiftType.ID <= 0 {
		return workforce.ShiftType{}, workforce.ErrShiftTypeNotFound
	}
	storedShiftType := shiftTypeToDomain(shiftType)
	var result domain.ShiftType
	var err error
	normalized, validationErr := domain.NormalizeShiftType(storedShiftType)
	if validationErr != nil {
		return shiftTypeToPublic(domain.ShiftType{}), mapShiftError(validationErr)
	}
	err = s.run(ctx, "update_shift_type", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return shiftTypeToPublic(result), mapShiftError(err)
}

func (s *Service) DeleteShiftType(ctx context.Context, id int64) error {
	if id <= 0 {
		return workforce.ErrShiftTypeNotFound
	}
	return mapShiftError(s.run(ctx, "delete_shift_type", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteShiftType(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}
