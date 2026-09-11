package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// --- class list entries ---

func (s *Service) FindClassListEntry(ctx context.Context, id int64, lock string) (result domain.ClassListEntry, err error) {
	run := s.runRead
	if lock != "" {
		run = s.runWrite
	}
	err = run(ctx, "find_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindClassListEntry(txCtx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			result = domain.ClassListEntry{}
			return domain.ErrClassListEntryNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListClassListEntries(ctx context.Context, filter domain.ClassListEntryFilter) (result []domain.ClassListEntry, err error) {
	err = s.runRead(ctx, "list_class_list_entries", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListClassListEntries(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateClassListEntry(ctx context.Context, fields domain.ClassListEntryFields, createdBy *int64) (result domain.ClassListEntry, err error) {
	err = s.runWrite(ctx, "create_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClassListEntry(txCtx, fields, createdBy)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) UpdateClassListEntry(ctx context.Context, id int64, fields domain.ClassListEntryFields) (result domain.ClassListEntry, err error) {
	err = s.runWrite(ctx, "update_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		_, found, queryStats, err := s.store.FindClassListEntry(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrClassListEntryNotFound
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.UpdateClassListEntry(txCtx, id, fields)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteClassListEntry(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_class_list_entry", func(txCtx context.Context, stats *domain.OperationStats) error {
		deleteStats, err := s.store.DeleteClassListEntry(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}
