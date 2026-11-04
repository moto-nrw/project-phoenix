package application

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

func (s *Service) FindCareExits(ctx context.Context, studentIDs []int64) (map[int64]careplan.CareExit, error) {
	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "find_care_exits", func(ctx context.Context) (map[int64]domain.CareExit, domain.OperationStats, error) {
		return s.store.FindCareExits(ctx, studentIDs)
	})
	return values, err
}

func (s *Service) UpsertCareExit(ctx context.Context, value careplan.CareExit) error {
	if err := normalizeCareExit(&value); err != nil {
		return err
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "upsert_care_exit", func(ctx context.Context) (domain.OperationStats, error) { return s.store.UpsertCareExit(ctx, value) })
	})
}

func (s *Service) DeleteCareExits(ctx context.Context, studentIDs []int64) error {
	studentIDs = uniquePositive(studentIDs)
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "delete_care_exits", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.DeleteCareExits(ctx, studentIDs)
		})
	})
}

func (s *Service) ListCompanionEdges(ctx context.Context, studentID int64) ([]careplan.CompanionEdge, error) {
	if studentID <= 0 {
		return nil, invalid(careplan.ErrInvalidCompanion, "student ID is required")
	}

	values, err := runValue(ctx, s, "list_companion_edges", func(ctx context.Context) ([]domain.CompanionEdge, domain.OperationStats, error) {
		return s.store.ListCompanionEdges(ctx, studentID)
	})
	return values, err
}

func (s *Service) ListCompanionLinks(ctx context.Context, studentIDs []int64) (map[int64][]careplan.CompanionLink, error) {
	studentIDs = uniquePositive(studentIDs)
	links, err := runValue(ctx, s, "list_companion_links", func(ctx context.Context) (map[int64][]domain.CompanionLink, domain.OperationStats, error) {
		return s.store.ListCompanionLinks(ctx, studentIDs)
	})
	if err != nil {
		return nil, err
	}
	companionIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, values := range links {
		for _, link := range values {
			if !seen[link.CompanionStudentID] {
				seen[link.CompanionStudentID] = true
				companionIDs = append(companionIDs, link.CompanionStudentID)
			}
		}
	}
	if len(companionIDs) == 0 {
		return links, nil
	}
	names, err := s.people.ListStudentNamesByID(ctx, companionIDs)
	if err != nil {
		return nil, err
	}
	nameByStudent := make(map[int64][2]string, len(names))
	for _, name := range names {
		nameByStudent[name.StudentID] = [2]string{name.FirstName, name.LastName}
	}
	for studentID, values := range links {
		for i := range values {
			name := nameByStudent[values[i].CompanionStudentID]
			values[i].FirstName, values[i].LastName = name[0], name[1]
		}
		links[studentID] = values
	}
	return links, nil
}

func (s *Service) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "count_companions_excluding", func(ctx context.Context) (map[int64]int, domain.OperationStats, error) {
		return s.store.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	})
	return values, err
}

func (s *Service) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "list_companion_days_excluding", func(ctx context.Context) (map[int64]map[string]bool, domain.OperationStats, error) {
		return s.store.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	})
	return values, err
}

func (s *Service) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	if weekday < 1 || weekday > 5 {
		return nil, invalid(careplan.ErrInvalidCompanion, "companion weekday must be between 1 and 5")
	}

	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "list_companion_group", func(ctx context.Context) (map[int64][]int64, domain.OperationStats, error) {
		return s.store.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	})
	return values, err
}

func (s *Service) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	if studentID <= 0 {
		return nil, invalid(careplan.ErrInvalidCompanion, "student ID is required")
	}

	values, err := runValue(ctx, s, "list_companion_weekdays", func(ctx context.Context) ([]int, domain.OperationStats, error) {
		return s.store.CompanionWeekdays(ctx, studentID)
	})
	return values, err
}

func (s *Service) CountCompanionLinks(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalid(careplan.ErrInvalidCompanion, "student ID is required")
	}

	value, err := runValue(ctx, s, "count_companion_links", func(ctx context.Context) (int, domain.OperationStats, error) {
		return s.store.CountCompanionLinks(ctx, studentID)
	})
	return value, err
}

func (s *Service) ReplaceCompanionEdges(ctx context.Context, studentID int64, edges []careplan.CompanionEdge) error {
	if studentID <= 0 {
		return invalid(careplan.ErrInvalidCompanion, "student ID is required")
	}
	for i := range edges {
		if edges[i].StudentLowID <= 0 || edges[i].StudentHighID <= 0 || edges[i].StudentLowID >= edges[i].StudentHighID || edges[i].Weekday < 1 || edges[i].Weekday > 5 {
			return invalid(careplan.ErrInvalidCompanion, "companion edge is invalid")
		}
		if edges[i].StudentLowID != studentID && edges[i].StudentHighID != studentID {
			return invalid(careplan.ErrInvalidCompanion, "companion edge does not contain the student")
		}
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "replace_companion_edges", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.ReplaceCompanionEdges(ctx, studentID, edges)
		})
	})
}

func (s *Service) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	edgeIDs = uniquePositive(edgeIDs)
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "delete_companion_edges", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.DeleteCompanionEdges(ctx, edgeIDs)
		})
	})
}

func (s *Service) FindCareDocument(ctx context.Context, studentID, documentID int64, includeDeleted bool) (result careplan.CareDocument, err error) {
	if studentID <= 0 || documentID <= 0 {
		return careplan.CareDocument{}, invalid(careplan.ErrInvalidCareDocument, "student and document IDs are required")
	}
	err = s.run(ctx, "find_care_document", func(ctx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindCareDocument(ctx, studentID, documentID, includeDeleted)
		stats.Add(queryStats)
		if err == nil && !found {
			return careplan.ErrCareDocumentNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListCareDocuments(ctx context.Context, studentID int64, categories []string) ([]careplan.CareDocument, error) {
	if studentID <= 0 {
		return nil, invalid(careplan.ErrInvalidCareDocument, "student ID is required")
	}

	values, err := runValue(ctx, s, "list_care_documents", func(ctx context.Context) ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListCareDocuments(ctx, studentID, categories)
	})
	return values, err
}

func (s *Service) ListPendingCareDocumentCleanup(ctx context.Context, studentID int64) ([]careplan.CareDocument, error) {
	if studentID <= 0 {
		return nil, invalid(careplan.ErrInvalidCareDocument, "student ID is required")
	}

	values, err := runValue(ctx, s, "list_pending_care_document_cleanup", func(ctx context.Context) ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListPendingCareDocumentCleanup(ctx, studentID)
	})
	return values, err
}

func (s *Service) ListDeletedCareDocuments(ctx context.Context, studentID int64, categories []string) ([]careplan.CareDocument, error) {
	if studentID < 0 {
		return nil, invalid(careplan.ErrInvalidCareDocument, "student ID is invalid")
	}

	values, err := runValue(ctx, s, "list_deleted_care_documents", func(ctx context.Context) ([]domain.CareDocument, domain.OperationStats, error) {
		return s.store.ListDeletedCareDocuments(ctx, studentID, categories)
	})
	return values, err
}

func (s *Service) ListCareDocumentCleanups(ctx context.Context, studentID *int64) ([]careplan.CareDocumentCleanup, error) {
	if studentID != nil && *studentID <= 0 {
		return nil, invalid(careplan.ErrInvalidCareDocument, "student ID is invalid")
	}

	values, err := runValue(ctx, s, "list_care_document_cleanups", func(ctx context.Context) ([]domain.CareDocumentCleanup, domain.OperationStats, error) {
		return s.store.ListCareDocumentCleanups(ctx, studentID)
	})
	return values, err
}

func (s *Service) CreateCareDocument(ctx context.Context, value careplan.CareDocument) (result careplan.CareDocument, err error) {
	if err := careplan.ValidateCareDocument(value); err != nil {
		return careplan.CareDocument{}, err
	}

	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = runValue(txCtx, s, "create_care_document", func(ctx context.Context) (domain.CareDocument, domain.OperationStats, error) {
			return s.store.CreateCareDocument(ctx, value)
		})
		return err
	})
	return result, err
}

// UpdateOpenCareAccidentReport validates the changed report row and writes
// it only while the report is open. It runs in the accident report command's
// tenant unit, so the report, its signature and its change-history row
// commit together.
func (s *Service) UpdateOpenCareAccidentReport(ctx context.Context, value careplan.CareDocument) (careplan.CareDocument, error) {
	if value.ID <= 0 || value.AccidentReport == nil {
		return careplan.CareDocument{}, invalid(careplan.ErrInvalidCareDocument, "an accident report document is required")
	}
	if err := careplan.ValidateCareDocument(value); err != nil {
		return careplan.CareDocument{}, err
	}
	// A report is signed exactly when a digital signature or a paper copy
	// signs it, and a signed report names who signed it and when.
	signedBy := value.AccidentSignatureID != nil || strings.TrimSpace(value.FilenameStored) != ""
	if signedBy != (value.AccidentSignedAt != nil) || (value.AccidentSignedAt == nil) != (value.AccidentSignedBy == nil) {
		return careplan.CareDocument{}, invalid(careplan.ErrInvalidCareDocument, "an accident report is signed by its signature or its paper copy")
	}
	if value.AccidentSignatureID != nil && strings.TrimSpace(value.FilenameStored) != "" {
		return careplan.CareDocument{}, invalid(careplan.ErrInvalidCareDocument, "an accident report is signed either digitally or on paper")
	}
	return runValue(ctx, s, "update_open_care_accident_report", func(ctx context.Context) (domain.CareDocument, domain.OperationStats, error) {
		return s.store.UpdateOpenCareAccidentReport(ctx, value)
	})
}

func (s *Service) SoftDeleteCareDocument(ctx context.Context, documentID, deletedBy int64) (result time.Time, err error) {
	if documentID <= 0 || deletedBy <= 0 {
		return time.Time{}, invalid(careplan.ErrInvalidCareDocument, "document and actor IDs are required")
	}

	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = runValue(txCtx, s, "soft_delete_care_document", func(ctx context.Context) (time.Time, domain.OperationStats, error) {
			return s.store.SoftDeleteCareDocument(ctx, documentID, deletedBy)
		})
		return err
	})
	return result, err
}

func (s *Service) MarkCareDocumentFileDeleted(ctx context.Context, documentID int64) error {
	if documentID <= 0 {
		return invalid(careplan.ErrInvalidCareDocument, "document ID is required")
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "mark_care_document_file_deleted", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.MarkCareDocumentFileDeleted(ctx, documentID)
		})
	})
}

func (s *Service) QueueCareDocumentCleanup(ctx context.Context, value careplan.CareDocumentCleanup) (result careplan.CareDocumentCleanup, err error) {
	if value.OwnerID <= 0 || strings.TrimSpace(value.FilenameStored) == "" || value.RetryAfter.IsZero() {
		return careplan.CareDocumentCleanup{}, invalid(careplan.ErrInvalidCareDocument, "care document cleanup is invalid")
	}

	err = s.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = runValue(txCtx, s, "queue_care_document_cleanup", func(ctx context.Context) (domain.CareDocumentCleanup, domain.OperationStats, error) {
			return s.store.QueueCareDocumentCleanup(ctx, value)
		})
		return err
	})
	return result, err
}

func (s *Service) CompleteCareDocumentCleanup(ctx context.Context, cleanupID int64) error {
	if cleanupID <= 0 {
		return invalid(careplan.ErrInvalidCareDocument, "cleanup ID is required")
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "complete_care_document_cleanup", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.CompleteCareDocumentCleanup(ctx, cleanupID)
		})
	})
}

func (s *Service) CompleteCareDocumentCleanupByFilename(ctx context.Context, filename string) error {
	if strings.TrimSpace(filename) == "" {
		return invalid(careplan.ErrInvalidCareDocument, "stored filename is required")
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "complete_care_document_cleanup_by_filename", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.CompleteCareDocumentCleanupByFilename(ctx, filename)
		})
	})
}

func (s *Service) ActivateCareDocumentCleanup(ctx context.Context, filename string) error {
	if strings.TrimSpace(filename) == "" {
		return invalid(careplan.ErrInvalidCareDocument, "stored filename is required")
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "activate_care_document_cleanup", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.ActivateCareDocumentCleanup(ctx, filename)
		})
	})
}

func (s *Service) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitRemoval, error) {
	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "list_care_exit_removals", func(ctx context.Context) ([]domain.CareExitRemoval, domain.OperationStats, error) {
		return s.store.ListCareExitRemovals(ctx, studentIDs)
	})
	return values, err
}

func (s *Service) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitSourceRemoval, error) {
	studentIDs = uniquePositive(studentIDs)
	values, err := runValue(ctx, s, "list_care_exit_source_removals", func(ctx context.Context) ([]domain.CareExitSourceRemoval, domain.OperationStats, error) {
		return s.store.ListCareExitSourceRemovals(ctx, studentIDs)
	})
	return values, err
}

func (s *Service) RecordCareExitRemovals(ctx context.Context, values []careplan.CareExitRemoval) error {
	for _, value := range values {
		if !validCareExitRemoval(value) {
			return invalid(careplan.ErrInvalidCareExit, "care exit removal is invalid")
		}
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "record_care_exit_removals", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.RecordCareExitRemovals(ctx, values)
		})
	})
}

func (s *Service) RecordCareExitSourceRemovals(ctx context.Context, values []careplan.CareExitSourceRemoval) error {
	for _, value := range values {
		if value.StudentID <= 0 || value.SourceRowID <= 0 || !validCareExitSource(value.Kind) || len(value.Snapshot) == 0 || !json.Valid(value.Snapshot) {
			return invalid(careplan.ErrInvalidCareExit, "care exit source removal is invalid")
		}
	}

	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "record_care_exit_source_removals", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.RecordCareExitSourceRemovals(ctx, values)
		})
	})
}

func (s *Service) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	studentIDs = uniquePositive(studentIDs)
	return s.withinTenant(ctx, func(txCtx context.Context) error {
		return runCommand(txCtx, s, "discard_care_exit_removals", func(ctx context.Context) (domain.OperationStats, error) {
			return s.store.DiscardCareExitRemovals(ctx, studentIDs)
		})
	})
}
