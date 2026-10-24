package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- staff absences ---

func (s *Service) FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error) {
	if id <= 0 {
		return workforce.StaffAbsence{}, invalidAbsence("staff absence ID is required")
	}
	var result domain.StaffAbsence
	var err error
	err = s.run(ctx, "find_staff_absence", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffAbsence(ctx, id)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffAbsenceNotFound
		}
		return err
	})
	return AbsenceToPublic(result), MapError(err)
}

func (s *Service) ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	storedFilter := absenceFilterToDomain(filter)
	var result []domain.StaffAbsence
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return absencesToPublic(nil), MapError(validationErr)
	}
	err = s.run(ctx, "list_staff_absences", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsences(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return absencesToPublic(result), MapError(err)
}

func (s *Service) CountStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) (int, error) {
	storedFilter := absenceFilterToDomain(filter)
	var result int
	var err error
	if validationErr := storedFilter.Validate(); validationErr != nil {
		return 0, MapError(validationErr)
	}
	err = s.run(ctx, "count_staff_absences", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountStaffAbsences(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return result, MapError(err)
}

func (s *Service) ListStaffAbsenceRequests(ctx context.Context, filter workforce.StaffAbsenceRequestFilter) ([]workforce.StaffAbsence, error) {
	if len(filter.Statuses) == 0 {
		return nil, invalidAbsence("at least one status is required")
	}
	if filter.FilterSubjects && len(filter.SubjectStaffIDs) == 0 {
		return []workforce.StaffAbsence{}, nil
	}
	storedFilter := domain.StaffAbsenceRequestFilter{
		Statuses: filter.Statuses, Types: filter.Types, FilterSubjects: filter.FilterSubjects,
		SubjectStaffIDs: filter.SubjectStaffIDs, Limit: filter.Limit, Decided: filter.Decided,
	}
	var result []domain.StaffAbsence
	var err error
	err = s.run(ctx, "list_staff_absence_requests", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsenceRequests(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	return absencesToPublic(result), MapError(err)
}

// StaffAbsenceMapForDate returns staff ID -> canonical type of the winning
// effective absence on the day.
func (s *Service) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	var result map[int64]string
	var err error
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return nil, MapError(validationErr)
	}
	err = s.run(ctx, "staff_absence_map_for_date", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return result, MapError(err)
}

// StaffAbsenceTypeIDMapForDate returns staff ID -> school-defined type of the
// same winning absence StaffAbsenceMapForDate picks; only winners carrying
// one appear. Both read the same ordered query so they agree under ties.
func (s *Service) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	var result map[int64]int64
	var err error
	if validationErr := domain.ValidateDate(date, "date"); validationErr != nil {
		return nil, MapError(validationErr)
	}
	err = s.run(ctx, "staff_absence_type_id_map_for_date", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return result, MapError(err)
}

func (s *Service) OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, error) {
	var result string
	var err error
	if validationErr := domain.ValidateDateColumn(column); validationErr != nil {
		return "", MapError(validationErr)
	}
	if validationErr := domain.ValidateDate(before, "before"); validationErr != nil {
		return "", MapError(validationErr)
	}
	err = s.run(ctx, "oldest_staff_absence_date", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.OldestStaffAbsenceDate(ctx, column, before)
		stats.Add(queryStats)
		return err
	})
	return result, MapError(err)
}

// LockStaffAbsenceWrites takes the shared staff balance lock and then the
// absence lock, in that order, inside the caller's ambient transaction.
func (s *Service) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return invalidAbsence("staff ID is required")
	}
	return MapError(s.observeOperation("lock_staff_absence_writes", func(*domain.OperationStats) error {
		if err := s.transaction.LockStaffBalance(ctx, staffID); err != nil {
			return err
		}
		return s.transaction.LockStaffAbsence(ctx, staffID)
	}))
}

func (s *Service) CreateStaffAbsence(ctx context.Context, absence *workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	if absence == nil {
		return workforce.StaffAbsence{}, errors.New("workforce.StaffAbsence cannot be nil or zero value")
	}
	value := absenceToDomain(*absence)
	var result domain.StaffAbsence
	var err error
	if validationErr := value.Validate(); validationErr != nil {
		return AbsenceToPublic(domain.StaffAbsence{}), MapError(validationErr)
	}
	err = s.run(ctx, "create_staff_absence", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return AbsenceToPublic(result), MapError(err)
}

func (s *Service) UpdateStaffAbsence(ctx context.Context, absence workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	if absence.ID <= 0 {
		return workforce.StaffAbsence{}, invalidAbsence("staff absence ID is required")
	}
	value := absenceToDomain(absence)
	var result domain.StaffAbsence
	var err error
	if validationErr := value.Validate(); validationErr != nil {
		return AbsenceToPublic(domain.StaffAbsence{}), MapError(validationErr)
	}
	err = s.run(ctx, "update_staff_absence", func(ctx context.Context, stats *domain.OperationStats) error {
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
	return AbsenceToPublic(result), MapError(err)
}

func (s *Service) DeleteStaffAbsence(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidAbsence("staff absence ID is required")
	}
	return MapError(s.run(ctx, "delete_staff_absence", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.DeleteStaffAbsence(txCtx, id)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, error) {
	if staffID <= 0 {
		return 0, invalidAbsence("staff ID is required")
	}
	var result int64
	var err error
	if validationErr := requireDate(from, "from"); validationErr != nil {
		return 0, MapError(validationErr)
	}
	err = s.run(ctx, "delete_non_historical_staff_absences", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteNonHistoricalStaffAbsences(txCtx, staffID, from)
			stats.Add(writeStats)
			return err
		})
	})
	return result, MapError(err)
}

func (s *Service) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	var result int64
	var err error
	if validationErr := domain.ValidateDateColumn(column); validationErr != nil {
		return 0, MapError(validationErr)
	}
	if validationErr := requireDate(cutoff, "cutoff"); validationErr != nil {
		return 0, MapError(validationErr)
	}
	err = s.run(ctx, "delete_staff_absences_older_than", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.DeleteStaffAbsencesOlderThan(txCtx, column, cutoff)
			stats.Add(writeStats)
			return err
		})
	})
	return result, MapError(err)
}

// requireDate is ValidateDate for a field a write cannot do without.
func requireDate(value, field string) error {
	if value == "" {
		return &domain.InvalidStaffAbsenceError{Reason: field + " is required"}
	}
	return domain.ValidateDate(value, field)
}

// --- absence types ---

func (s *Service) ListStaffAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	var result []domain.StaffAbsenceType
	var err error
	err = s.run(ctx, "list_staff_absence_types", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffAbsenceTypes(ctx)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, MapError(storedErr)
	}
	storedResult := make([]workforce.StaffAbsenceType, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, absenceTypeToPublic(value))
	}
	return storedResult, MapError(storedErr)
}

func (s *Service) FindStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	if id <= 0 {
		return workforce.StaffAbsenceType{}, workforce.ErrAbsenceTypeNotFound
	}
	value, err := s.findStaffAbsenceType(ctx, "find_staff_absence_type", id, false)
	return absenceTypeToPublic(value), MapError(err)
}

func (s *Service) LockStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	if id <= 0 {
		return workforce.StaffAbsenceType{}, workforce.ErrAbsenceTypeNotFound
	}
	value, err := s.findStaffAbsenceType(ctx, "lock_staff_absence_type", id, true)
	return absenceTypeToPublic(value), MapError(err)
}

func (s *Service) findStaffAbsenceType(ctx context.Context, operation string, id int64, lock bool) (result domain.StaffAbsenceType, err error) {
	err = s.run(ctx, operation, func(ctx context.Context, stats *domain.OperationStats) error {
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

func (s *Service) StaffAbsenceTypeInUse(ctx context.Context, id int64) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	var result bool
	var err error
	err = s.run(ctx, "staff_absence_type_in_use", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffAbsenceTypeInUse(ctx, id)
		stats.Add(queryStats)
		return err
	})
	return result, MapError(err)
}

// CreateAbsenceType applies administration rules before the storage write.
// The unique index remains the final arbiter when concurrent names race.
func (s *Service) CreateAbsenceType(ctx context.Context, input workforce.CreateAbsenceType) (workforce.StaffAbsenceType, error) {
	fields := domain.StaffAbsenceTypeFields{
		Name: input.Name, AllowanceEnabled: input.AllowanceEnabled, CarryoverUntil: input.CarryoverUntil,
	}
	fields.BaseType, fields.IsActive = domain.AbsenceTypeOther, true
	if err := fields.Normalize(); err != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(err)
	}
	if err := s.checkAbsenceTypeName(ctx, fields.Name, 0); err != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(err)
	}
	created, err := s.CreateStaffAbsenceType(ctx, workforce.StaffAbsenceTypeFields(fields))
	if err != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(fmt.Errorf("database error during create: %w", err))
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

func (s *Service) UpdateAbsenceType(ctx context.Context, input workforce.UpdateAbsenceType) (workforce.StaffAbsenceType, error) {
	id := input.ID
	name := input.Name
	isActive := input.IsActive
	allowanceEnabled := input.AllowanceEnabled
	carryoverUntil := input.CarryoverUntil
	if id <= 0 {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(domain.ErrAbsenceTypeNotFound)
	}
	existing, err := s.LockStaffAbsenceType(ctx, id)
	if errors.Is(err, workforce.ErrAbsenceTypeNotFound) {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(domain.ErrAbsenceTypeNotFound)
	}
	if err != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(fmt.Errorf("lock absence type: database error during lock staff absence type: %w", err))
	}
	fields := domain.StaffAbsenceTypeFields{
		Name: existing.Name, BaseType: existing.BaseType, IsActive: existing.IsActive,
		AllowanceEnabled: existing.AllowanceEnabled, CarryoverUntil: existing.CarryoverUntil,
	}
	if name != nil {
		if err := s.renameAbsenceType(ctx, id, existing.Name, *name, &fields); err != nil {
			return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(err)
		}
	}
	applyAbsenceTypeSettings(&fields, isActive, allowanceEnabled, carryoverUntil)
	if err := fields.Normalize(); err != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(err)
	}
	existing.Name, existing.IsActive = fields.Name, fields.IsActive
	existing.AllowanceEnabled, existing.CarryoverUntil = fields.AllowanceEnabled, fields.CarryoverUntil
	updated, err := s.UpdateStaffAbsenceType(ctx, existing)
	if err != nil {
		if errors.Is(err, workforce.ErrAbsenceTypeNotFound) {
			// A row disappearing after the lock was a failed write, not a
			// missing-resource response, in the retained administration API.
			err = errors.New("expected 1 rows affected, got 0")
		}
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(fmt.Errorf("database error during update staff absence type: %w", err))
	}
	return updated, nil
}

// renameAbsenceType sets the normalized new name on fields. The name must be
// free and not reserved; a changed name additionally requires that no absence
// uses the type yet.
func (s *Service) renameAbsenceType(ctx context.Context, id int64, currentName, name string, fields *domain.StaffAbsenceTypeFields) error {
	fields.Name = name
	if err := fields.Normalize(); err != nil {
		return err
	}
	if err := s.checkAbsenceTypeName(ctx, fields.Name, id); err != nil {
		return err
	}
	if fields.Name == currentName {
		return nil
	}
	inUse, err := s.StaffAbsenceTypeInUse(ctx, id)
	if err != nil {
		return fmt.Errorf("check absence type usage: database error during check staff absence type usage: %w", err)
	}
	if inUse {
		return domain.ErrAbsenceTypeInUse
	}
	return nil
}

// applyAbsenceTypeSettings overwrites the settings the update supplied.
func applyAbsenceTypeSettings(fields *domain.StaffAbsenceTypeFields, isActive, allowanceEnabled *bool, carryoverUntil *string) {
	if isActive != nil {
		fields.IsActive = *isActive
	}
	if allowanceEnabled != nil {
		fields.AllowanceEnabled = *allowanceEnabled
	}
	if carryoverUntil != nil {
		fields.CarryoverUntil = *carryoverUntil
	}
}

func (s *Service) CreateStaffAbsenceType(ctx context.Context, fields workforce.StaffAbsenceTypeFields) (workforce.StaffAbsenceType, error) {
	storedFields := domain.StaffAbsenceTypeFields{
		Name: fields.Name, BaseType: fields.BaseType, IsActive: fields.IsActive,
		AllowanceEnabled: fields.AllowanceEnabled, CarryoverUntil: fields.CarryoverUntil,
	}
	var result domain.StaffAbsenceType
	var err error
	if validationErr := storedFields.Normalize(); validationErr != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(validationErr)
	}
	err = s.run(ctx, "create_staff_absence_type", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffAbsenceType(txCtx, storedFields)
			stats.Add(writeStats)
			return err
		})
	})
	return absenceTypeToPublic(result), MapError(err)
}

func (s *Service) UpdateStaffAbsenceType(ctx context.Context, value workforce.StaffAbsenceType) (workforce.StaffAbsenceType, error) {
	if value.ID <= 0 {
		return workforce.StaffAbsenceType{}, workforce.ErrAbsenceTypeNotFound
	}
	storedValue := domain.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, CarryoverUntil: value.CarryoverUntil,
	}
	var result domain.StaffAbsenceType
	var err error
	fields := domain.StaffAbsenceTypeFields{
		Name: storedValue.Name, BaseType: storedValue.BaseType, IsActive: storedValue.IsActive,
		AllowanceEnabled: storedValue.AllowanceEnabled, CarryoverUntil: storedValue.CarryoverUntil,
	}
	if validationErr := fields.Normalize(); validationErr != nil {
		return absenceTypeToPublic(domain.StaffAbsenceType{}), MapError(validationErr)
	}
	storedValue.Name, storedValue.BaseType = fields.Name, fields.BaseType
	err = s.run(ctx, "update_staff_absence_type", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffAbsenceType(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrAbsenceTypeNotFound
			}
			return err
		})
	})
	return absenceTypeToPublic(result), MapError(err)
}

// --- audit ---

func (s *Service) RecordStaffAbsenceAudit(ctx context.Context, audit workforce.StaffAbsenceAudit) (workforce.StaffAbsenceAudit, error) {
	value := domain.StaffAbsenceAudit{
		ID: audit.ID, TenantID: audit.TenantID, AbsenceID: audit.AbsenceID, FromStatus: audit.FromStatus,
		ToStatus: audit.ToStatus, ActorID: audit.ActorID, Note: audit.Note, ChangedAt: audit.ChangedAt,
		TypeChange: (*domain.AbsenceTypeChange)(audit.TypeChange),
	}
	var result domain.StaffAbsenceAudit
	var err error
	if validationErr := value.Validate(); validationErr != nil {
		recorded, storedErr := domain.StaffAbsenceAudit{}, validationErr
		return workforce.StaffAbsenceAudit{
			ID: recorded.ID, TenantID: recorded.TenantID, AbsenceID: recorded.AbsenceID, FromStatus: recorded.FromStatus,
			ToStatus: recorded.ToStatus, ActorID: recorded.ActorID, Note: recorded.Note, ChangedAt: recorded.ChangedAt,
			TypeChange: (*workforce.AbsenceTypeChange)(recorded.TypeChange),
		}, MapError(storedErr)
	}
	err = s.run(ctx, "record_staff_absence_audit", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.RecordStaffAbsenceAudit(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	recorded, storedErr := result, err
	return workforce.StaffAbsenceAudit{
		ID: recorded.ID, TenantID: recorded.TenantID, AbsenceID: recorded.AbsenceID, FromStatus: recorded.FromStatus,
		ToStatus: recorded.ToStatus, ActorID: recorded.ActorID, Note: recorded.Note, ChangedAt: recorded.ChangedAt,
		TypeChange: (*workforce.AbsenceTypeChange)(recorded.TypeChange),
	}, MapError(storedErr)
}
