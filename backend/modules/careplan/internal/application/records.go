package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func runValue[T any](s *Service, operation string, call func() (T, domain.OperationStats, error)) (result T, err error) {
	err = s.run(operation, func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = call()
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func runCommand(s *Service, operation string, call func() (domain.OperationStats, error)) error {
	return s.run(operation, func(stats *domain.OperationStats) error {
		queryStats, err := call()
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) FindCareExits(ctx context.Context, studentIDs []int64) (map[int64]domain.CareExit, error) {
	return runValue(s, "find_care_exits", func() (map[int64]domain.CareExit, domain.OperationStats, error) {
		return s.store.FindCareExits(ctx, studentIDs)
	})
}

func (s *Service) UpsertCareExit(ctx context.Context, value domain.CareExit) error {
	return runCommand(s, "upsert_care_exit", func() (domain.OperationStats, error) { return s.store.UpsertCareExit(ctx, value) })
}

func (s *Service) DeleteCareExits(ctx context.Context, studentIDs []int64) error {
	return runCommand(s, "delete_care_exits", func() (domain.OperationStats, error) { return s.store.DeleteCareExits(ctx, studentIDs) })
}

func (s *Service) ListCompanionEdges(ctx context.Context, studentID int64) ([]domain.CompanionEdge, error) {
	return runValue(s, "list_companion_edges", func() ([]domain.CompanionEdge, domain.OperationStats, error) {
		return s.store.ListCompanionEdges(ctx, studentID)
	})
}

func (s *Service) ListCompanionLinks(ctx context.Context, studentIDs []int64) (map[int64][]domain.CompanionLink, error) {
	return runValue(s, "list_companion_links", func() (map[int64][]domain.CompanionLink, domain.OperationStats, error) {
		return s.store.ListCompanionLinks(ctx, studentIDs)
	})
}

func (s *Service) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	return runValue(s, "count_companions_excluding", func() (map[int64]int, domain.OperationStats, error) {
		return s.store.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	})
}

func (s *Service) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	return runValue(s, "list_companion_days_excluding", func() (map[int64]map[string]bool, domain.OperationStats, error) {
		return s.store.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	})
}

func (s *Service) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	return runValue(s, "list_companion_group", func() (map[int64][]int64, domain.OperationStats, error) {
		return s.store.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	})
}

func (s *Service) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	return runValue(s, "list_companion_weekdays", func() ([]int, domain.OperationStats, error) { return s.store.CompanionWeekdays(ctx, studentID) })
}

func (s *Service) CountCompanionLinks(ctx context.Context, studentID int64) (int, error) {
	return runValue(s, "count_companion_links", func() (int, domain.OperationStats, error) { return s.store.CountCompanionLinks(ctx, studentID) })
}

func (s *Service) ReplaceCompanionEdges(ctx context.Context, studentID int64, edges []domain.CompanionEdge) error {
	return runCommand(s, "replace_companion_edges", func() (domain.OperationStats, error) { return s.store.ReplaceCompanionEdges(ctx, studentID, edges) })
}

func (s *Service) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	return runCommand(s, "delete_companion_edges", func() (domain.OperationStats, error) { return s.store.DeleteCompanionEdges(ctx, edgeIDs) })
}

func (s *Service) FindCareDocument(ctx context.Context, studentID, documentID int64, includeDeleted bool) (result domain.CareDocument, err error) {
	err = s.run("find_care_document", func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCareDocument(ctx, studentID, documentID, includeDeleted)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrCareDocumentNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListCareDocuments(ctx context.Context, studentID int64, categories []string) ([]domain.CareDocument, error) {
	return runValue(s, "list_care_documents", func() ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListCareDocuments(ctx, studentID, categories)
	})
}

func (s *Service) ListPendingCareDocumentCleanup(ctx context.Context, studentID int64) ([]domain.CareDocument, error) {
	return runValue(s, "list_pending_care_document_cleanup", func() ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListPendingCareDocumentCleanup(ctx, studentID)
	})
}

func (s *Service) ListDeletedCareDocuments(ctx context.Context, studentID int64, categories []string) ([]domain.CareDocument, error) {
	return runValue(s, "list_deleted_care_documents", func() ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListDeletedCareDocuments(ctx, studentID, categories)
	})
}

func (s *Service) ListCareDocumentCleanups(ctx context.Context, studentID *int64) ([]domain.CareDocumentCleanup, error) {
	return runValue(s, "list_care_document_cleanups", func() ([]domain.CareDocumentCleanup, domain.OperationStats, error) {
		return s.store.ListCareDocumentCleanups(ctx, studentID)
	})
}

func (s *Service) CreateCareDocument(ctx context.Context, value domain.CareDocument) (domain.CareDocument, error) {
	return runValue(s, "create_care_document", func() (domain.CareDocument, domain.OperationStats, error) {
		return s.store.CreateCareDocument(ctx, value)
	})
}

func (s *Service) SoftDeleteCareDocument(ctx context.Context, documentID, deletedBy int64) (time.Time, error) {
	return runValue(s, "soft_delete_care_document", func() (time.Time, domain.OperationStats, error) {
		return s.store.SoftDeleteCareDocument(ctx, documentID, deletedBy)
	})
}

func (s *Service) MarkCareDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return runCommand(s, "mark_care_document_file_deleted", func() (domain.OperationStats, error) { return s.store.MarkCareDocumentFileDeleted(ctx, documentID) })
}

func (s *Service) QueueCareDocumentCleanup(ctx context.Context, value domain.CareDocumentCleanup) (domain.CareDocumentCleanup, error) {
	return runValue(s, "queue_care_document_cleanup", func() (domain.CareDocumentCleanup, domain.OperationStats, error) {
		return s.store.QueueCareDocumentCleanup(ctx, value)
	})
}

func (s *Service) CompleteCareDocumentCleanup(ctx context.Context, cleanupID int64) error {
	return runCommand(s, "complete_care_document_cleanup", func() (domain.OperationStats, error) { return s.store.CompleteCareDocumentCleanup(ctx, cleanupID) })
}

func (s *Service) CompleteCareDocumentCleanupByFilename(ctx context.Context, filename string) error {
	return runCommand(s, "complete_care_document_cleanup_by_filename", func() (domain.OperationStats, error) {
		return s.store.CompleteCareDocumentCleanupByFilename(ctx, filename)
	})
}

func (s *Service) ActivateCareDocumentCleanup(ctx context.Context, filename string) error {
	return runCommand(s, "activate_care_document_cleanup", func() (domain.OperationStats, error) { return s.store.ActivateCareDocumentCleanup(ctx, filename) })
}

func (s *Service) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]domain.CareExitRemoval, error) {
	return runValue(s, "list_care_exit_removals", func() ([]domain.CareExitRemoval, domain.OperationStats, error) {
		return s.store.ListCareExitRemovals(ctx, studentIDs)
	})
}

func (s *Service) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]domain.CareExitSourceRemoval, error) {
	return runValue(s, "list_care_exit_source_removals", func() ([]domain.CareExitSourceRemoval, domain.OperationStats, error) {
		return s.store.ListCareExitSourceRemovals(ctx, studentIDs)
	})
}

func (s *Service) RecordCareExitRemovals(ctx context.Context, values []domain.CareExitRemoval) error {
	return runCommand(s, "record_care_exit_removals", func() (domain.OperationStats, error) {
		return s.store.RecordCareExitRemovals(ctx, values)
	})
}

func (s *Service) RecordCareExitSourceRemovals(ctx context.Context, values []domain.CareExitSourceRemoval) error {
	return runCommand(s, "record_care_exit_source_removals", func() (domain.OperationStats, error) {
		return s.store.RecordCareExitSourceRemovals(ctx, values)
	})
}

func (s *Service) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	return runCommand(s, "discard_care_exit_removals", func() (domain.OperationStats, error) {
		return s.store.DiscardCareExitRemovals(ctx, studentIDs)
	})
}
