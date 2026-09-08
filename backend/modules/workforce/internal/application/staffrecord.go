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

// ReplaceStaffQualifications rewrites the qualification list of one staff
// member atomically: the delete and the insert share one unit of work, so a
// rejected row leaves the previous list in place.
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
			deleteStats, deleteErr := s.store.DeleteStaffQualifications(txCtx, staffID)
			stats.Add(deleteStats)
			if deleteErr != nil {
				return deleteErr
			}
			var insertStats domain.OperationStats
			result, insertStats, err = s.store.InsertStaffQualifications(txCtx, rows)
			stats.Add(insertStats)
			return err
		})
	})
	return result, err
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
