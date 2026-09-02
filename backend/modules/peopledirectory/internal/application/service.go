package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

type Service struct {
	store   ports.Store
	tx      ports.Transaction
	observe ports.Observer
}

func New(store ports.Store, tx ports.Transaction, observe ports.Observer) *Service {
	if store == nil || tx == nil || observe == nil {
		panic("people directory application: all dependencies are required")
	}
	return &Service{store: store, tx: tx, observe: observe}
}

func (s *Service) Create(ctx context.Context, input domain.CreatePerson) (result domain.Person, err error) {
	err = s.runWrite(ctx, "create_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.Create(txCtx, input)
		stats.Add(createStats)
		return err
	})
	return result, err
}

func (s *Service) Update(ctx context.Context, input domain.UpdatePerson) (result domain.Person, err error) {
	err = s.runWrite(ctx, "update_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, input.ID, stats); err != nil {
			return err
		}
		var updateStats domain.OperationStats
		result, updateStats, err = s.store.Update(txCtx, input)
		stats.Add(updateStats)
		return err
	})
	return result, err
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.runWrite(ctx, "delete_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, id, stats); err != nil {
			return err
		}
		deleteStats, err := s.store.SoftDelete(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

func (s *Service) FindByID(ctx context.Context, id int64, lock string) (result domain.Person, err error) {
	run := s.runRead
	if lock != "" {
		run = s.runWrite
	}
	err = run(ctx, "find_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindByID(txCtx, id, lock)
		stats.Add(queryStats)
		if err == nil && (!found || result.IsDeleted()) {
			result = domain.Person{}
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindByAccount(ctx context.Context, accountID int64) (domain.Person, error) {
	return s.findOne(ctx, "find_person_by_account", func(txCtx context.Context) (domain.Person, bool, domain.OperationStats, error) {
		return s.store.FindByAccount(txCtx, accountID)
	})
}

func (s *Service) FindByTag(ctx context.Context, tagID string) (domain.Person, error) {
	return s.findOne(ctx, "find_person_by_tag", func(txCtx context.Context) (domain.Person, bool, domain.OperationStats, error) {
		return s.store.FindByTag(txCtx, tagID)
	})
}

func (s *Service) findOne(ctx context.Context, operation string, query func(context.Context) (domain.Person, bool, domain.OperationStats, error)) (result domain.Person, err error) {
	err = s.runRead(ctx, operation, func(txCtx context.Context, stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = query(txCtx)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) ListByIDs(ctx context.Context, ids []int64) (result []domain.Person, err error) {
	err = s.runRead(ctx, "list_persons_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListByAccounts(ctx context.Context, accountIDs []int64) (result []domain.Person, err error) {
	err = s.runRead(ctx, "list_persons_by_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByAccounts(txCtx, accountIDs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) Search(ctx context.Context, filter domain.Filter) (result []domain.Person, err error) {
	err = s.runRead(ctx, "search_persons", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.Search(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CountByTenant(ctx context.Context) (result map[int64]int, err error) {
	err = s.runRead(ctx, "count_persons_by_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountByTenant(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) LinkAccount(ctx context.Context, personID, accountID int64) error {
	return s.runWrite(ctx, "link_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, personID, stats); err != nil {
			return err
		}
		holder, found, queryStats, err := s.store.FindByAccount(txCtx, accountID)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if found && holder.ID != personID {
			return domain.ErrAccountConflict
		}
		setStats, err := s.store.SetAccount(txCtx, personID, &accountID)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) UnlinkAccount(ctx context.Context, personID int64) error {
	return s.runWrite(ctx, "unlink_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, personID, stats); err != nil {
			return err
		}
		setStats, err := s.store.SetAccount(txCtx, personID, nil)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) LinkTag(ctx context.Context, personID int64, tagID string) error {
	return s.runWrite(ctx, "link_tag", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, personID, stats); err != nil {
			return err
		}
		holder, found, queryStats, err := s.store.FindByTag(txCtx, tagID)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if found && holder.ID != personID {
			return domain.ErrTagConflict
		}
		setStats, err := s.store.SetTag(txCtx, personID, &tagID)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) UnlinkTag(ctx context.Context, personID int64) error {
	return s.runWrite(ctx, "unlink_tag", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, personID, stats); err != nil {
			return err
		}
		setStats, err := s.store.SetTag(txCtx, personID, nil)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) ReleaseTags(ctx context.Context, personIDs []int64) (result []domain.ReleasedTag, err error) {
	err = s.runWrite(ctx, "release_tags", func(txCtx context.Context, stats *domain.OperationStats) error {
		var lockStats domain.OperationStats
		result, lockStats, err = s.store.LockHeldTags(txCtx, personIDs)
		stats.Add(lockStats)
		if err != nil || len(result) == 0 {
			return err
		}
		ids := make([]int64, 0, len(result))
		for _, held := range result {
			ids = append(ids, held.PersonID)
		}
		clearStats, err := s.store.ClearTags(txCtx, ids)
		stats.Add(clearStats)
		return err
	})
	if result == nil {
		result = []domain.ReleasedTag{}
	}
	return result, err
}

func (s *Service) RestoreTag(ctx context.Context, personID int64, tagID string) (restored bool, err error) {
	err = s.runWrite(ctx, "restore_tag", func(txCtx context.Context, stats *domain.OperationStats) error {
		return s.tx.RunSavepoint(txCtx, func(spCtx context.Context) error {
			var restoreStats domain.OperationStats
			restored, restoreStats, err = s.store.RestoreTag(spCtx, personID, tagID)
			stats.Add(restoreStats)
			return err
		})
	})
	if errors.Is(err, domain.ErrTagConflict) {
		// The tag found a new holder between the snapshot read and the write;
		// the savepoint rolled the statement back and the current holder wins.
		return false, nil
	}
	return restored, err
}

func (s *Service) requireLocked(ctx context.Context, id int64, stats *domain.OperationStats) error {
	person, found, queryStats, err := s.store.FindByID(ctx, id, "UPDATE")
	stats.Add(queryStats)
	if err != nil {
		return err
	}
	if !found || person.IsDeleted() {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Service) runWrite(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.observeRun(ctx, operation, s.tx.RunWrite, fn)
}

func (s *Service) runRead(ctx context.Context, operation string, fn func(context.Context, *domain.OperationStats) error) error {
	return s.observeRun(ctx, operation, s.tx.RunRead, fn)
}

func (s *Service) observeRun(ctx context.Context, operation string, run func(context.Context, func(context.Context) error) error, fn func(context.Context, *domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = run(ctx, func(txCtx context.Context) error { return fn(txCtx, &stats) })
	if err != nil {
		stats.Rows = 0
	}
	return err
}
