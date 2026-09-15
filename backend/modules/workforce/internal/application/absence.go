package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
			if value.DateEnd >= s.clock.Today() || value.Status == domain.AbsenceStatusRequested || value.Status == domain.AbsenceStatusQuestion {
				if err := s.lockStaffAssignment(txCtx, value.StaffID); err != nil {
					return err
				}
			}
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
			if value.DateEnd >= s.clock.Today() || value.Status == domain.AbsenceStatusRequested || value.Status == domain.AbsenceStatusQuestion {
				if err := s.lockStaffAssignment(txCtx, value.StaffID); err != nil {
					return err
				}
			}
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

// CreateAbsenceType applies administration rules before the storage write.
// The unique index remains the final arbiter when concurrent names race.
func (s *Service) CreateAbsenceType(ctx context.Context, fields domain.StaffAbsenceTypeFields) (domain.StaffAbsenceType, error) {
	fields.BaseType, fields.IsActive = domain.AbsenceTypeOther, true
	if err := fields.Normalize(); err != nil {
		return domain.StaffAbsenceType{}, err
	}
	if err := s.checkAbsenceTypeName(ctx, fields.Name, 0); err != nil {
		return domain.StaffAbsenceType{}, err
	}
	created, err := s.CreateStaffAbsenceType(ctx, fields)
	if err != nil {
		return domain.StaffAbsenceType{}, fmt.Errorf("database error during create: %w", err)
	}
	return created, nil
}

func (s *Service) checkAbsenceTypeName(ctx context.Context, candidate string, excludeID int64) error {
	name := strings.ToLower(strings.TrimSpace(candidate))
	switch name {
	case "urlaub", "krank", "krankmeldung", "fortbildung", "sonstige", "sonstiges", "freizeitausgleich", "sonstige abwesenheit":
		return domain.ErrAbsenceTypeNameReserved
	}
	existing, err := s.ListStaffAbsenceTypes(ctx)
	if err != nil {
		return fmt.Errorf("database error during list all staff absence types: %w", err)
	}
	for _, value := range existing {
		if value.ID != excludeID && strings.ToLower(strings.TrimSpace(value.Name)) == name {
			return domain.ErrAbsenceTypeNameTaken
		}
	}
	return nil
}

func (s *Service) UpdateAbsenceType(ctx context.Context, id int64, name *string, isActive, allowanceEnabled *bool, overrunPolicy *string) (domain.StaffAbsenceType, error) {
	if id <= 0 {
		return domain.StaffAbsenceType{}, domain.ErrAbsenceTypeNotFound
	}
	existing, err := s.LockStaffAbsenceType(ctx, id)
	if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
		return domain.StaffAbsenceType{}, domain.ErrAbsenceTypeNotFound
	}
	if err != nil {
		return domain.StaffAbsenceType{}, fmt.Errorf("lock absence type: database error during lock staff absence type: %w", err)
	}
	fields := domain.StaffAbsenceTypeFields{
		Name: existing.Name, BaseType: existing.BaseType, IsActive: existing.IsActive,
		AllowanceEnabled: existing.AllowanceEnabled, OverrunPolicy: existing.OverrunPolicy,
	}
	if name != nil {
		fields.Name = *name
		if err := fields.Normalize(); err != nil {
			return domain.StaffAbsenceType{}, err
		}
		if err := s.checkAbsenceTypeName(ctx, fields.Name, id); err != nil {
			return domain.StaffAbsenceType{}, err
		}
		if fields.Name != existing.Name {
			inUse, err := s.StaffAbsenceTypeInUse(ctx, id)
			if err != nil {
				return domain.StaffAbsenceType{}, fmt.Errorf("check absence type usage: database error during check staff absence type usage: %w", err)
			}
			if inUse {
				return domain.StaffAbsenceType{}, domain.ErrAbsenceTypeInUse
			}
		}
	}
	if isActive != nil {
		fields.IsActive = *isActive
	}
	if allowanceEnabled != nil {
		fields.AllowanceEnabled = *allowanceEnabled
	}
	if overrunPolicy != nil {
		fields.OverrunPolicy = *overrunPolicy
	}
	if err := fields.Normalize(); err != nil {
		return domain.StaffAbsenceType{}, err
	}
	existing.Name, existing.IsActive = fields.Name, fields.IsActive
	existing.AllowanceEnabled, existing.OverrunPolicy = fields.AllowanceEnabled, fields.OverrunPolicy
	updated, err := s.UpdateStaffAbsenceType(ctx, existing)
	if err != nil {
		if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
			// A row disappearing after the lock was a failed write, not a
			// missing-resource response, in the retained administration API.
			err = errors.New("expected 1 rows affected, got 0")
		}
		return domain.StaffAbsenceType{}, fmt.Errorf("database error during update staff absence type: %w", err)
	}
	return updated, nil
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
