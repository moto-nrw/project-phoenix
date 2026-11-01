package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- master data ---

func (s *Service) FindStaffMasterData(ctx context.Context, staffID int64) (workforce.StaffMasterData, error) {
	if staffID <= 0 {
		return workforce.StaffMasterData{}, invalidStaffRecord("staff ID is required")
	}
	var result domain.StaffMasterData
	var err error
	err = s.run(ctx, "find_staff_master_data", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffMasterData(ctx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffMasterDataNotFound
		}
		return err
	})
	return masterDataToPublic(result), mapStaffRecordError(err)
}

func (s *Service) CreateStaffMasterData(ctx context.Context, value workforce.StaffMasterData) (workforce.StaffMasterData, error) {
	storedValue := masterDataToDomain(value)
	var result domain.StaffMasterData
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return masterDataToPublic(domain.StaffMasterData{}), mapStaffRecordError(validationErr)
	}
	err = s.run(ctx, "create_staff_master_data", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffMasterData(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return masterDataToPublic(result), mapStaffRecordError(err)
}

func (s *Service) UpdateStaffMasterData(ctx context.Context, value workforce.StaffMasterData) (workforce.StaffMasterData, error) {
	if value.ID <= 0 {
		return workforce.StaffMasterData{}, invalidStaffRecord("staff master data ID is required")
	}
	storedValue := masterDataToDomain(value)
	var result domain.StaffMasterData
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return masterDataToPublic(domain.StaffMasterData{}), mapStaffRecordError(validationErr)
	}
	err = s.run(ctx, "update_staff_master_data", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffMasterData(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffMasterDataNotFound
			}
			return err
		})
	})
	return masterDataToPublic(result), mapStaffRecordError(err)
}

// --- qualifications ---

func (s *Service) ListStaffQualifications(ctx context.Context, staffID int64) ([]workforce.StaffQualification, error) {
	if staffID <= 0 {
		return nil, invalidStaffRecord("staff ID is required")
	}
	var result []domain.StaffQualification
	var err error
	err = s.run(ctx, "list_staff_qualifications", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffQualifications(ctx, staffID)
		stats.Add(queryStats)
		return err
	})
	return qualificationsToPublic(result), mapStaffRecordError(err)
}

// ReplaceStaffQualifications makes the submitted list the live qualification
// list of one staff member in one unit of work, so a rejected row leaves the
// previous list in place. Removing retires instead of deleting (ADR 0021).
// A submitted row continues the live row with the same name: an equal row is
// left untouched, changed dates update it in place. Live rows no submitted row
// continues are retired, and the remaining submitted rows are inserted. Saving
// the same list again therefore writes nothing, and the list keeps its order.
// The result follows the submitted order.
func (s *Service) ReplaceStaffQualifications(ctx context.Context, staffID int64, values []workforce.StaffQualification) ([]workforce.StaffQualification, error) {
	if staffID <= 0 {
		return nil, invalidStaffRecord("staff ID is required")
	}
	rows, err := qualificationRowsFor(staffID, values)
	if err != nil {
		return qualificationsToPublic(nil), mapStaffRecordError(err)
	}
	var result []domain.StaffQualification
	err = s.run(ctx, "replace_staff_qualifications", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if lockErr := s.transaction.LockStaffQualifications(txCtx, staffID); lockErr != nil {
				return lockErr
			}
			live, listStats, listErr := s.store.ListStaffQualifications(txCtx, staffID)
			stats.Add(listStats)
			if listErr != nil {
				return listErr
			}
			if validationErr := validateQualificationIDs(rows, live); validationErr != nil {
				return validationErr
			}
			resolved, resolveErr := s.resolveQualificationTypes(txCtx, rows, live, stats)
			if resolveErr != nil {
				return resolveErr
			}
			result, err = s.applyQualificationPlan(txCtx, staffID, planQualificationReplace(live, resolved), stats)
			return err
		})
	})
	return qualificationsToPublic(result), mapStaffRecordError(err)
}

func (s *Service) applyQualificationPlan(ctx context.Context, staffID int64, plan qualificationReplacePlan, stats *domain.OperationStats) ([]domain.StaffQualification, error) {
	retireStats, err := s.store.RetireStaffQualifications(ctx, staffID, plan.retire)
	stats.Add(retireStats)
	if err != nil {
		return nil, err
	}
	type pendingInsert struct {
		index int
		value domain.StaffQualification
	}
	result := make([]domain.StaffQualification, len(plan.rows))
	var pending []pendingInsert
	for index, row := range plan.rows {
		switch row.action {
		case qualificationKeep:
			result[index] = row.value
		case qualificationUpdate:
			updated, found, updateStats, updateErr := s.store.UpdateStaffQualification(ctx, row.value)
			stats.Add(updateStats)
			if updateErr != nil {
				return nil, updateErr
			}
			if found {
				result[index] = updated
				continue
			}
			// A writer outside this replacement path may have retired the row.
			pending = append(pending, pendingInsert{index: index, value: domain.StaffQualification{
				StaffID: staffID, SortOrder: row.value.SortOrder, TypeID: row.value.TypeID, Name: row.value.Name,
				AcquiredOn: row.value.AcquiredOn, ExpiresOn: row.value.ExpiresOn,
			}})
		case qualificationInsert:
			pending = append(pending, pendingInsert{index: index, value: row.value})
		}
	}
	inserts := make([]domain.StaffQualification, 0, len(pending))
	for _, insert := range pending {
		inserts = append(inserts, insert.value)
	}
	inserted, insertStats, err := s.store.InsertStaffQualifications(ctx, inserts)
	stats.Add(insertStats)
	if err != nil {
		return nil, err
	}
	for position, insert := range pending {
		result[insert.index] = inserted[position]
	}
	return result, nil
}

type qualificationAction int

const (
	qualificationInsert qualificationAction = iota
	qualificationKeep
	qualificationUpdate
)

type plannedQualification struct {
	action qualificationAction
	value  domain.StaffQualification
}

func (row plannedQualification) atPosition(index int) plannedQualification {
	if row.action == qualificationKeep && row.value.SortOrder != index {
		row.action = qualificationUpdate
	}
	row.value.SortOrder = index
	return row
}

// qualificationReplacePlan holds one planned write per submitted row, in
// submitted order, and the live rows to retire.
type qualificationReplacePlan struct {
	rows   []plannedQualification
	retire []int64
}

// planQualificationReplace pairs submitted rows with live rows. Equal rows
// pair first, so a list holding the same name twice keeps both rows; a row
// whose name is left pairs with the oldest live row of that name.
func planQualificationReplace(live, submitted []domain.StaffQualification) qualificationReplacePlan {
	plan := qualificationReplacePlan{rows: make([]plannedQualification, len(submitted))}
	paired := make(map[int64]bool, len(live))
	// A submitted row naming a live row continues it, so assigning a kind to
	// a free-text entry keeps its identity (#4808).
	plan.pair(live, submitted, paired, sameQualificationID, qualificationKeep)
	plan.pair(live, submitted, paired, sameQualification, qualificationKeep)
	plan.pair(live, submitted, paired, sameQualificationName, qualificationUpdate)
	for index, row := range submitted {
		if plan.rows[index].action == qualificationInsert {
			plan.rows[index].value = row
		}
		plan.rows[index] = plan.rows[index].atPosition(index)
	}
	for _, row := range live {
		if !paired[row.ID] {
			plan.retire = append(plan.retire, row.ID)
		}
	}
	return plan
}

// pair lets every still unpaired submitted row continue the first unpaired
// live row match accepts. A kept row that differs becomes an update.
func (plan *qualificationReplacePlan) pair(live, submitted []domain.StaffQualification, paired map[int64]bool, match func(live, submitted domain.StaffQualification) bool, action qualificationAction) {
	for index, row := range submitted {
		if plan.rows[index].action != qualificationInsert {
			continue
		}
		candidate, ok := firstUnpaired(live, paired, row, match)
		if !ok {
			continue
		}
		paired[candidate.ID] = true
		rowAction := action
		if rowAction == qualificationKeep && !sameQualification(candidate, row) {
			rowAction = qualificationUpdate
		}
		candidate.TypeID, candidate.Name = row.TypeID, row.Name
		candidate.AcquiredOn, candidate.ExpiresOn = row.AcquiredOn, row.ExpiresOn
		plan.rows[index] = plannedQualification{action: rowAction, value: candidate}
	}
}

func firstUnpaired(live []domain.StaffQualification, paired map[int64]bool, submitted domain.StaffQualification, match func(live, submitted domain.StaffQualification) bool) (domain.StaffQualification, bool) {
	for _, candidate := range live {
		if !paired[candidate.ID] && match(candidate, submitted) {
			return candidate, true
		}
	}
	return domain.StaffQualification{}, false
}

func sameQualificationName(live, submitted domain.StaffQualification) bool {
	return live.Name == submitted.Name
}

func sameQualificationID(live, submitted domain.StaffQualification) bool {
	return submitted.ID > 0 && live.ID == submitted.ID
}

func sameQualification(live, submitted domain.StaffQualification) bool {
	return sameQualificationName(live, submitted) && sameQualificationType(live.TypeID, submitted.TypeID) &&
		live.AcquiredOn == submitted.AcquiredOn && live.ExpiresOn == submitted.ExpiresOn
}

func sameQualificationType(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// validateQualificationIDs keeps submitted row IDs scoped to the staff
// member whose qualification list is being replaced. Insert rows must leave
// their ID empty so PostgreSQL assigns it from the sequence.
func validateQualificationIDs(submitted, live []domain.StaffQualification) error {
	liveIDs := make(map[int64]struct{}, len(live))
	for _, row := range live {
		liveIDs[row.ID] = struct{}{}
	}
	for _, row := range submitted {
		if row.ID <= 0 {
			continue
		}
		if _, ok := liveIDs[row.ID]; !ok {
			return invalidStaffRecord("qualification ID does not belong to staff member")
		}
	}
	return nil
}

// qualificationRowsFor converts and validates the submitted rows of one staff
// member. A row of a kind takes the kind's name in the unit of work.
func qualificationRowsFor(staffID int64, values []workforce.StaffQualification) ([]domain.StaffQualification, error) {
	rows := make([]domain.StaffQualification, 0, len(values))
	for _, value := range values {
		row := domain.StaffQualification{
			ID: value.ID, TenantID: value.TenantID, StaffID: staffID, TypeID: value.TypeID, Name: value.Name,
			AcquiredOn: value.AcquiredOn, ExpiresOn: value.ExpiresOn, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		}
		check := row
		if check.TypeID != nil && check.Name == "" {
			check.Name = "-"
		}
		if err := check.Validate(); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *Service) resolveQualificationTypes(ctx context.Context, rows, live []domain.StaffQualification, stats *domain.OperationStats) ([]domain.StaffQualification, error) {
	types, typeStats, err := s.store.ListQualificationTypes(ctx)
	stats.Add(typeStats)
	if err != nil {
		return nil, err
	}
	return resolveQualificationTypes(rows, live, types)
}

// resolveQualificationTypes checks the kind of every submitted row against
// the school's kinds and gives it the kind's current name. An archived kind
// is offered no longer; a row that already had it may keep it.
func resolveQualificationTypes(rows, live []domain.StaffQualification, types []domain.QualificationType) ([]domain.StaffQualification, error) {
	byID := make(map[int64]domain.QualificationType, len(types))
	for _, kind := range types {
		byID[kind.ID] = kind
	}
	had := make(map[int64]*int64, len(live))
	for _, row := range live {
		had[row.ID] = row.TypeID
	}
	resolved := make([]domain.StaffQualification, len(rows))
	for index, row := range rows {
		if row.TypeID != nil {
			kind, ok := byID[*row.TypeID]
			if !ok {
				return nil, fmt.Errorf("%w: unknown qualification type", domain.ErrStaffStammdatenInvalid)
			}
			keepsArchived := row.ID > 0 && sameQualificationType(had[row.ID], row.TypeID)
			if kind.Archived() && !keepsArchived {
				return nil, fmt.Errorf("%w: qualification type is archived", domain.ErrStaffStammdatenInvalid)
			}
			row.Name = kind.Name
		}
		resolved[index] = row
	}
	return resolved, nil
}

// --- financial data ---

func (s *Service) FindStaffFinancialData(ctx context.Context, staffID int64) (workforce.StaffFinancialData, error) {
	if staffID <= 0 {
		return workforce.StaffFinancialData{}, invalidStaffRecord("staff ID is required")
	}
	var result domain.StaffFinancialData
	var err error
	err = s.run(ctx, "find_staff_financial_data", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffFinancialData(ctx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffFinancialDataNotFound
		}
		return err
	})
	return financialToPublic(result), mapStaffRecordError(err)
}

func (s *Service) CreateStaffFinancialData(ctx context.Context, value workforce.StaffFinancialData) (workforce.StaffFinancialData, error) {
	storedValue := financialToDomain(value)
	var result domain.StaffFinancialData
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return financialToPublic(domain.StaffFinancialData{}), mapStaffRecordError(validationErr)
	}
	err = s.run(ctx, "create_staff_financial_data", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffFinancialData(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return financialToPublic(result), mapStaffRecordError(err)
}

func (s *Service) UpdateStaffFinancialData(ctx context.Context, value workforce.StaffFinancialData) (workforce.StaffFinancialData, error) {
	if value.ID <= 0 {
		return workforce.StaffFinancialData{}, invalidStaffRecord("staff financial data ID is required")
	}
	storedValue := financialToDomain(value)
	var result domain.StaffFinancialData
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return financialToPublic(domain.StaffFinancialData{}), mapStaffRecordError(validationErr)
	}
	err = s.run(ctx, "update_staff_financial_data", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffFinancialData(txCtx, storedValue)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffFinancialDataNotFound
			}
			return err
		})
	})
	return financialToPublic(result), mapStaffRecordError(err)
}

// --- documents ---

func (s *Service) CreateStaffDocument(ctx context.Context, value workforce.StaffDocument) (workforce.StaffDocument, error) {
	storedValue := documentToDomain(value)
	var result domain.StaffDocument
	var err error
	if validationErr := storedValue.Validate(); validationErr != nil {
		return documentToPublic(domain.StaffDocument{}), mapStaffRecordError(validationErr)
	}
	err = s.run(ctx, "create_staff_document", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffDocument(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	})
	return documentToPublic(result), mapStaffRecordError(err)
}

func (s *Service) FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (workforce.StaffDocument, error) {
	if staffID <= 0 || documentID <= 0 {
		return workforce.StaffDocument{}, invalidStaffRecord("staff ID and document ID are required")
	}
	var result domain.StaffDocument
	var err error
	err = s.run(ctx, "find_staff_document", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffDocumentNotFound
		}
		return err
	})
	return documentToPublic(result), mapStaffRecordError(err)
}

func (s *Service) ListStaffDocuments(ctx context.Context, filter workforce.StaffDocumentFilter) ([]workforce.StaffDocument, error) {
	storedFilter := domain.StaffDocumentFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Categories: filter.Categories,
		IncludeDeleted: filter.IncludeDeleted, DeletedOnly: filter.DeletedOnly, FilePending: filter.FilePending,
	}
	var result []domain.StaffDocument
	var err error
	err = s.run(ctx, "list_staff_documents", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffDocuments(ctx, storedFilter)
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, mapStaffRecordError(storedErr)
	}
	storedResult := make([]workforce.StaffDocument, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, documentToPublic(value))
	}
	return storedResult, mapStaffRecordError(storedErr)
}

func (s *Service) SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (int64, error) {
	if id <= 0 {
		return 0, invalidStaffRecord("staff document ID is required")
	}
	var result int64
	var err error
	if deletedBy <= 0 {
		return 0, mapStaffRecordError(&domain.InvalidStaffRecordError{Reason: "deleted_by is required"})
	}
	err = s.run(ctx, "soft_delete_staff_document", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SoftDeleteStaffDocument(txCtx, id, deletedBy, at)
			stats.Add(writeStats)
			return err
		})
	})
	return result, mapStaffRecordError(err)
}

func (s *Service) MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error {
	if id <= 0 {
		return invalidStaffRecord("staff document ID is required")
	}
	return mapStaffRecordError(s.run(ctx, "mark_staff_document_file_deleted", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.MarkStaffDocumentFileDeleted(txCtx, id, at)
			stats.Add(writeStats)
			return err
		})
	}))
}

// --- file cleanup intents ---

func (s *Service) QueueStaffDocumentFileCleanup(ctx context.Context, value workforce.StaffDocumentFileCleanup) error {
	storedValue := domain.StaffDocumentFileCleanup{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, FilenameStored: value.FilenameStored,
		RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
	if validationErr := storedValue.Validate(); validationErr != nil {
		return mapStaffRecordError(validationErr)
	}
	return mapStaffRecordError(s.run(ctx, "queue_staff_document_file_cleanup", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.QueueStaffDocumentFileCleanup(txCtx, storedValue)
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) ([]workforce.StaffDocumentFileCleanup, error) {
	var result []domain.StaffDocumentFileCleanup
	var err error
	err = s.run(ctx, "list_queued_staff_document_file_cleanups", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListQueuedStaffDocumentFileCleanups(ctx, staffID, s.clock.Now())
		stats.Add(queryStats)
		return err
	})
	values, storedErr := result, err
	if values == nil {
		return nil, mapStaffRecordError(storedErr)
	}
	storedResult := make([]workforce.StaffDocumentFileCleanup, 0, len(values))
	for _, value := range values {
		storedResult = append(storedResult, workforce.StaffDocumentFileCleanup{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, FilenameStored: value.FilenameStored,
			RetryAfter: value.RetryAfter, CleanedAt: value.CleanedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return storedResult, mapStaffRecordError(storedErr)
}

func (s *Service) CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidStaffRecord("staff document file cleanup ID is required")
	}
	return mapStaffRecordError(s.run(ctx, "complete_staff_document_file_cleanup", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CompleteStaffDocumentFileCleanup(txCtx, id, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error {
	if filename == "" {
		return mapStaffRecordError(&domain.InvalidStaffRecordError{Reason: "filename_stored is required"})
	}
	return mapStaffRecordError(s.run(ctx, "complete_staff_document_file_cleanup_by_filename", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CompleteStaffDocumentFileCleanupByFilename(txCtx, filename, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	}))
}

func (s *Service) ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error {
	if filename == "" {
		return mapStaffRecordError(&domain.InvalidStaffRecordError{Reason: "filename_stored is required"})
	}
	return mapStaffRecordError(s.run(ctx, "activate_staff_document_file_cleanup", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.ActivateStaffDocumentFileCleanup(txCtx, filename, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	}))
}
