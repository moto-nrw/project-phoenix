package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- master data ---

func (s *Service) FindStaffMasterData(ctx context.Context, staffID int64) (result domain.StaffMasterData, err error) {
	err = s.run("find_staff_master_data", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffMasterData(ctx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffMasterDataNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) CreateStaffMasterData(ctx context.Context, value domain.StaffMasterData) (result domain.StaffMasterData, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffMasterData{}, validationErr
	}
	err = s.run("create_staff_master_data", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffMasterData(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffMasterData(ctx context.Context, value domain.StaffMasterData) (result domain.StaffMasterData, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffMasterData{}, validationErr
	}
	err = s.run("update_staff_master_data", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffMasterData(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffMasterDataNotFound
			}
			return err
		})
	})
	return result, err
}

// --- qualifications ---

func (s *Service) ListStaffQualifications(ctx context.Context, staffID int64) (result []domain.StaffQualification, err error) {
	err = s.run("list_staff_qualifications", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffQualifications(ctx, staffID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// ReplaceStaffQualifications makes the submitted list the live qualification
// list of one staff member in one unit of work, so a rejected row leaves the
// previous list in place. Removing retires instead of deleting (ADR 0021).
// A submitted row continues the live row with the same name: an equal row is
// left untouched, changed dates update it in place. Live rows no submitted row
// continues are retired, and the remaining submitted rows are inserted. Saving
// the same list again therefore writes nothing, and the list keeps its order.
// The result follows the submitted order.
func (s *Service) ReplaceStaffQualifications(ctx context.Context, staffID int64, values []domain.StaffQualification) (result []domain.StaffQualification, err error) {
	if staffID <= 0 {
		return nil, &domain.InvalidStaffRecordError{Reason: "staff_id is required"}
	}
	rows := make([]domain.StaffQualification, 0, len(values))
	for _, value := range values {
		value.StaffID = staffID
		if validationErr := value.Validate(); validationErr != nil {
			return nil, validationErr
		}
		rows = append(rows, value)
	}
	err = s.run("replace_staff_qualifications", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			live, listStats, listErr := s.store.ListStaffQualifications(txCtx, staffID)
			stats.Add(listStats)
			if listErr != nil {
				return listErr
			}
			result, err = s.applyQualificationPlan(txCtx, staffID, planQualificationReplace(live, rows), stats)
			return err
		})
	})
	return result, err
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
			// A concurrent replace retired the row since the list was read;
			// the submitted row then starts a new one.
			pending = append(pending, pendingInsert{index: index, value: domain.StaffQualification{
				StaffID: staffID, Name: row.value.Name, AcquiredOn: row.value.AcquiredOn, ExpiresOn: row.value.ExpiresOn,
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
	pair := func(match func(live, submitted domain.StaffQualification) bool, action qualificationAction) {
		for index, row := range submitted {
			if plan.rows[index].action != qualificationInsert {
				continue
			}
			if candidate, ok := firstUnpaired(live, paired, row, match); ok {
				paired[candidate.ID] = true
				candidate.AcquiredOn, candidate.ExpiresOn = row.AcquiredOn, row.ExpiresOn
				plan.rows[index] = plannedQualification{action: action, value: candidate}
			}
		}
	}
	pair(sameQualification, qualificationKeep)
	pair(sameQualificationName, qualificationUpdate)
	for index, row := range submitted {
		if plan.rows[index].action == qualificationInsert {
			plan.rows[index].value = row
		}
	}
	for _, row := range live {
		if !paired[row.ID] {
			plan.retire = append(plan.retire, row.ID)
		}
	}
	return plan
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

func sameQualification(live, submitted domain.StaffQualification) bool {
	return sameQualificationName(live, submitted) && live.AcquiredOn == submitted.AcquiredOn && live.ExpiresOn == submitted.ExpiresOn
}

// --- financial data ---

func (s *Service) FindStaffFinancialData(ctx context.Context, staffID int64) (result domain.StaffFinancialData, err error) {
	err = s.run("find_staff_financial_data", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffFinancialData(ctx, staffID)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffFinancialDataNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) CreateStaffFinancialData(ctx context.Context, value domain.StaffFinancialData) (result domain.StaffFinancialData, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffFinancialData{}, validationErr
	}
	err = s.run("create_staff_financial_data", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffFinancialData(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) UpdateStaffFinancialData(ctx context.Context, value domain.StaffFinancialData) (result domain.StaffFinancialData, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffFinancialData{}, validationErr
	}
	err = s.run("update_staff_financial_data", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var found bool
			var writeStats domain.OperationStats
			result, found, writeStats, err = s.store.UpdateStaffFinancialData(txCtx, value)
			stats.Add(writeStats)
			if err == nil && !found {
				return domain.ErrStaffFinancialDataNotFound
			}
			return err
		})
	})
	return result, err
}

// --- documents ---

func (s *Service) CreateStaffDocument(ctx context.Context, value domain.StaffDocument) (result domain.StaffDocument, err error) {
	if validationErr := value.Validate(); validationErr != nil {
		return domain.StaffDocument{}, validationErr
	}
	err = s.run("create_staff_document", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffDocument(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) FindStaffDocument(ctx context.Context, staffID, documentID int64, includeDeleted bool) (result domain.StaffDocument, err error) {
	err = s.run("find_staff_document", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindStaffDocument(ctx, staffID, documentID, includeDeleted)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrStaffDocumentNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListStaffDocuments(ctx context.Context, filter domain.StaffDocumentFilter) (result []domain.StaffDocument, err error) {
	err = s.run("list_staff_documents", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffDocuments(ctx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) SoftDeleteStaffDocument(ctx context.Context, id, deletedBy int64, at time.Time) (result int64, err error) {
	if deletedBy <= 0 {
		return 0, &domain.InvalidStaffRecordError{Reason: "deleted_by is required"}
	}
	err = s.run("soft_delete_staff_document", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.SoftDeleteStaffDocument(txCtx, id, deletedBy, at)
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

func (s *Service) MarkStaffDocumentFileDeleted(ctx context.Context, id int64, at time.Time) error {
	return s.run("mark_staff_document_file_deleted", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.MarkStaffDocumentFileDeleted(txCtx, id, at)
			stats.Add(writeStats)
			return err
		})
	})
}

// --- file cleanup intents ---

func (s *Service) QueueStaffDocumentFileCleanup(ctx context.Context, value domain.StaffDocumentFileCleanup) error {
	if validationErr := value.Validate(); validationErr != nil {
		return validationErr
	}
	return s.run("queue_staff_document_file_cleanup", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.QueueStaffDocumentFileCleanup(txCtx, value)
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) ListQueuedStaffDocumentFileCleanups(ctx context.Context, staffID int64) (result []domain.StaffDocumentFileCleanup, err error) {
	err = s.run("list_queued_staff_document_file_cleanups", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListQueuedStaffDocumentFileCleanups(ctx, staffID, s.clock.Now())
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CompleteStaffDocumentFileCleanup(ctx context.Context, id int64) error {
	return s.run("complete_staff_document_file_cleanup", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CompleteStaffDocumentFileCleanup(txCtx, id, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) CompleteStaffDocumentFileCleanupByFilename(ctx context.Context, filename string) error {
	if filename == "" {
		return &domain.InvalidStaffRecordError{Reason: "filename_stored is required"}
	}
	return s.run("complete_staff_document_file_cleanup_by_filename", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.CompleteStaffDocumentFileCleanupByFilename(txCtx, filename, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	})
}

func (s *Service) ActivateStaffDocumentFileCleanup(ctx context.Context, filename string) error {
	if filename == "" {
		return &domain.InvalidStaffRecordError{Reason: "filename_stored is required"}
	}
	return s.run("activate_staff_document_file_cleanup", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			writeStats, err := s.store.ActivateStaffDocumentFileCleanup(txCtx, filename, s.clock.Now())
			stats.Add(writeStats)
			return err
		})
	})
}
