package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- staff absences ---

func (s *Service) FindStaffAbsence(ctx context.Context, id int64) (result domain.StaffAbsence, err error) {
	err = s.run("find_staff_absence", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffAbsence(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffAbsenceNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffAbsences(ctx context.Context, filter domain.StaffAbsenceFilter) (result []domain.StaffAbsence, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("list_staff_absences", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsences(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountStaffAbsences(ctx context.Context, filter domain.StaffAbsenceFilter) (result int, err error) {
	if validationErr := filter.Validate(); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("count_staff_absences", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountStaffAbsences(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListStaffAbsenceRequests(ctx context.Context, filter domain.StaffAbsenceRequestFilter) (result []domain.StaffAbsence, err error) {
	err = s.run("list_staff_absence_requests", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsenceRequests(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// StaffAbsenceMapForDate returns staff ID -> canonical type of the winning
// effective absence on the day.
func (s *Service) StaffAbsenceMapForDate(ctx context.Context, date string) (result map[int64]string, err error) {
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("staff_absence_map_for_date", func(stats *domain.OperationStats) error {
		candidates, queryStats, queryErr := s.store.EffectiveStaffAbsencesOn(ctx, date)
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		winners := domain.WinningAbsences(candidates)
		result = make(map[int64]string, len(winners))
		for staffID, absence := range winners {
			result[staffID] = absence.AbsenceType
		}
		return nil
	})
	return result, err
}

// StaffAbsenceTypeIDMapForDate returns staff ID -> school-defined type of the
// same winning absence StaffAbsenceMapForDate picks; only winners carrying
// one appear. Both read the same ordered query so they agree under ties.
func (s *Service) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (result map[int64]int64, err error) {
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return nil, validationErr
	}
	err = s.run("staff_absence_type_id_map_for_date", func(stats *domain.OperationStats) error {
		candidates, queryStats, queryErr := s.store.EffectiveStaffAbsencesOn(ctx, date)
		stats.Add(queryStats)
		if queryErr != nil {
			return queryErr
		}
		winners := domain.WinningAbsences(candidates)
		result = make(map[int64]int64, len(winners))
		for staffID, absence := range winners {
			if absence.AbsenceTypeID != nil {
				result[staffID] = *absence.AbsenceTypeID
			}
		}
		return nil
	})
	return result, err
}

func (s *Service) OldestStaffAbsenceDate(ctx context.Context, column, before string) (result string, err error) {
	if validationErr := domain.ValidateDateColumn(column); validationErr != nil {
		return "", validationErr
	}
	if validationErr := domain.ValidateDate(before, "before"); validationErr != nil {
		return "", validationErr
	}
	err = s.run("oldest_staff_absence_date", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.OldestStaffAbsenceDate(ctx, column, before)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// LockStaffAbsenceWrites takes the shared staff balance lock and then the
// absence lock, in that order, inside the caller's ambient transaction.
func (s *Service) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	return s.run("lock_staff_absence_writes", func(*domain.OperationStats) error {
		if err := s.transaction.LockStaffBalance(ctx, staffID); err != nil {
			return err
		}
		return s.transaction.LockStaffAbsence(ctx, staffID)
	})
}

func (s *Service) CreateStaffAbsence(ctx context.Context, value domain.StaffAbsence) (result domain.StaffAbsence, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffAbsence{}, validationErr
	}
	err = s.run("create_staff_absence", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffAbsence(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffAbsence(ctx context.Context, value domain.StaffAbsence) (result domain.StaffAbsence, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffAbsence{}, validationErr
	}
	err = s.run("update_staff_absence", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffAbsence(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffAbsenceNotFound
			}
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffAbsence(ctx context.Context, id int64) error {
	return s.run("delete_staff_absence", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffAbsence(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (result int64, err error) {
	if validationErr := requireDate(from, "from"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("delete_non_historical_staff_absences", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteNonHistoricalStaffAbsences(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (result int64, err error) {
	if validationErr := domain.ValidateDateColumn(column); validationErr != nil {
		return 0, validationErr
	}
	if validationErr := requireDate(cutoff, "cutoff"); validationErr != nil {
		return 0, validationErr
	}
	err = s.run("delete_staff_absences_older_than", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteStaffAbsencesOlderThan(txCtx, column, cutoff)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// requireDate is ValidateDate for a field a write cannot do without.
func requireDate(value, field string) error {
	if value == "" {
		return &domain.InvalidStaffAbsenceError{Reason: field + " is required"}
	}
	return domain.ValidateDate(value, field)
}

// --- absence types ---

func (s *Service) ListStaffAbsenceTypes(ctx context.Context) (result []domain.StaffAbsenceType, err error) {
	err = s.run("list_staff_absence_types", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsenceTypes(ctx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindStaffAbsenceType(ctx context.Context, id int64) (domain.StaffAbsenceType, error) {
	return s.findStaffAbsenceType(ctx, "find_staff_absence_type", id, false)
}

func (s *Service) LockStaffAbsenceType(ctx context.Context, id int64) (domain.StaffAbsenceType, error) {
	return s.findStaffAbsenceType(ctx, "lock_staff_absence_type", id, true)
}

func (s *Service) findStaffAbsenceType(ctx context.Context, operation string, id int64, lock bool) (result domain.StaffAbsenceType, err error) {
	err = s.run(operation, func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffAbsenceType(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrAbsenceTypeNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) StaffAbsenceTypeInUse(ctx context.Context, id int64) (result bool, err error) {
	err = s.run("staff_absence_type_in_use", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffAbsenceTypeInUse(ctx, id)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateStaffAbsenceType(ctx context.Context, fields domain.StaffAbsenceTypeFields) (result domain.StaffAbsenceType, err error) {
	if validationErr := fields.Normalize(); validationErr != nil {
		return domain.StaffAbsenceType{}, validationErr
	}
	err = s.run("create_staff_absence_type", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffAbsenceType(txCtx, fields)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffAbsenceType(ctx context.Context, value domain.StaffAbsenceType) (result domain.StaffAbsenceType, err error) {
	fields := domain.StaffAbsenceTypeFields{
		Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, OverrunPolicy: value.OverrunPolicy,
	}
	if validationErr := fields.Normalize(); validationErr != nil {
		return domain.StaffAbsenceType{}, validationErr
	}
	value.Name, value.BaseType, value.OverrunPolicy = fields.Name, fields.BaseType, fields.OverrunPolicy
	err = s.run("update_staff_absence_type", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffAbsenceType(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrAbsenceTypeNotFound
			}
			return err
		})
	})
	return result, err
}

// --- audit ---

func (s *Service) RecordStaffAbsenceAudit(ctx context.Context, value domain.StaffAbsenceAudit) (result domain.StaffAbsenceAudit, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffAbsenceAudit{}, validationErr
	}
	err = s.run("record_staff_absence_audit", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RecordStaffAbsenceAudit(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}
