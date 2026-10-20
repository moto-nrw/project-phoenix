package application

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) CreatePerson(ctx context.Context, input peopledirectory.CreatePerson) (peopledirectory.Person, error) {
	var err error
	input, err = peopledirectory.NormalizePersonCreate(input)
	if err != nil {
		return peopledirectory.Person{}, err
	}

	value, err := s.Create(ctx, peopledirectory.CreatePerson{
		FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
		TagID: input.TagID, AccountID: input.AccountID,
	})
	return value, MapError(err)
}

func (s *Service) UpdatePerson(ctx context.Context, input peopledirectory.UpdatePerson) (result peopledirectory.Person, err error) {
	input, err = peopledirectory.NormalizePersonUpdate(input)
	if err != nil {
		return peopledirectory.Person{}, err
	}

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

func (s *Service) DeletePerson(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalid("person ID is required")
	}

	return s.runWrite(ctx, "delete_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, id, stats); err != nil {
			return err
		}
		deleteStats, err := s.store.SoftDelete(txCtx, id)
		stats.Add(deleteStats)
		return err
	})
}

func (s *Service) FindPerson(ctx context.Context, id int64) (peopledirectory.Person, error) {
	if id <= 0 {
		return peopledirectory.Person{}, invalid("person ID is required")
	}

	value, err := s.FindByID(ctx, id, "")
	return value, MapError(err)
}

func (s *Service) FindPersonForMutation(ctx context.Context, id int64) (peopledirectory.Person, error) {
	if id <= 0 {
		return peopledirectory.Person{}, invalid("person ID is required")
	}

	value, err := s.FindByID(ctx, id, "UPDATE")
	return value, MapError(err)
}

func (s *Service) FindPersonByAccount(ctx context.Context, accountID int64) (peopledirectory.Person, error) {
	if accountID <= 0 {
		return peopledirectory.Person{}, invalid("account ID is required")
	}

	value, err := s.FindByAccount(ctx, accountID)
	return value, MapError(err)
}

func (s *Service) FindPersonByTag(ctx context.Context, tagID string) (peopledirectory.Person, error) {
	tagID = peopledirectory.NormalizeTagID(tagID)
	if tagID == "" {
		return peopledirectory.Person{}, invalid("tag ID is required")
	}

	value, err := s.FindByTag(ctx, tagID)
	return value, MapError(err)
}

func (s *Service) ListPersonsByID(ctx context.Context, ids []int64) (result []peopledirectory.Person, err error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.Person{}, nil
	}

	err = s.runRead(ctx, "list_persons_by_id", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return nonNilPersons(result), err
}

func (s *Service) ListPersonsAcrossTenantsByID(ctx context.Context, ids []int64) (result []peopledirectory.Person, err error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.Person{}, nil
	}

	err = observeRun(ctx, s.observe, "list_persons_across_tenants_by_id", s.tx.RunAdminRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByIDs(txCtx, ids)
		stats.Add(queryStats)
		return err
	})
	return nonNilPersons(result), err
}

func (s *Service) ListPersonsByTenantIDs(ctx context.Context, tenantIDs []int64) (result []peopledirectory.Person, err error) {
	tenantIDs = peopledirectory.PositiveIDs(tenantIDs)
	if len(tenantIDs) == 0 {
		return []peopledirectory.Person{}, nil
	}

	err = s.runRead(ctx, "list_persons_by_tenant", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByTenantIDs(txCtx, tenantIDs)
		stats.Add(queryStats)
		return err
	})
	return nonNilPersons(result), err
}

func (s *Service) ListPersonsByAccount(ctx context.Context, accountIDs []int64) (result []peopledirectory.Person, err error) {
	accountIDs = peopledirectory.PositiveIDs(accountIDs)
	if len(accountIDs) == 0 {
		return []peopledirectory.Person{}, nil
	}

	err = s.runRead(ctx, "list_persons_by_account", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListByAccounts(txCtx, accountIDs)
		stats.Add(queryStats)
		return err
	})
	return nonNilPersons(result), err
}

func (s *Service) SearchPersons(ctx context.Context, filter peopledirectory.PersonFilter) (result []peopledirectory.Person, err error) {
	filter.FirstNamePrefix = strings.TrimSpace(filter.FirstNamePrefix)
	filter.LastNamePrefix = strings.TrimSpace(filter.LastNamePrefix)
	filter.FirstNameEquals = strings.TrimSpace(filter.FirstNameEquals)
	filter.LastNameEquals = strings.TrimSpace(filter.LastNameEquals)
	filter.FullNameContains = strings.TrimSpace(filter.FullNameContains)
	filter.TagID = peopledirectory.NormalizeTagID(filter.TagID)
	filter.AccountIDs = peopledirectory.PositiveIDs(filter.AccountIDs)
	if filter.Page < 0 || filter.PageSize < 0 {
		return nil, invalid("page and page size must not be negative")
	}
	if filter.PageSize > peopledirectory.MaxPageSize {
		filter.PageSize = peopledirectory.MaxPageSize
	}

	err = s.runRead(ctx, "search_persons", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.Search(txCtx, filter)
		stats.Add(queryStats)
		return err
	})
	return nonNilPersons(result), err
}

func (s *Service) CountPersonsByTenant(ctx context.Context) (result map[int64]int, err error) {
	err = observeRun(ctx, s.observe, "count_persons_by_tenant", s.tx.RunAdminRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CountByTenant(txCtx)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) LinkAccount(ctx context.Context, personID, accountID int64) error {
	if personID <= 0 || accountID <= 0 {
		return invalid("person ID and account ID are required")
	}

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
			return peopledirectory.ErrAccountConflict
		}
		setStats, err := s.store.SetAccount(txCtx, personID, &accountID)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) UnlinkAccount(ctx context.Context, personID int64) error {
	if personID <= 0 {
		return invalid("person ID is required")
	}

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
	tagID = peopledirectory.NormalizeTagID(tagID)
	if personID <= 0 || tagID == "" {
		return invalid("person ID and tag ID are required")
	}

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
			return peopledirectory.ErrTagConflict
		}
		setStats, err := s.store.SetTag(txCtx, personID, &tagID)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) UnlinkTag(ctx context.Context, personID int64) error {
	if personID <= 0 {
		return invalid("person ID is required")
	}

	return s.runWrite(ctx, "unlink_tag", func(txCtx context.Context, stats *domain.OperationStats) error {
		if err := s.requireLocked(txCtx, personID, stats); err != nil {
			return err
		}
		setStats, err := s.store.SetTag(txCtx, personID, nil)
		stats.Add(setStats)
		return err
	})
}

func (s *Service) ReleaseTags(ctx context.Context, personIDs []int64) (result []peopledirectory.ReleasedTag, err error) {
	personIDs = peopledirectory.PositiveIDs(personIDs)
	if len(personIDs) == 0 {
		return []peopledirectory.ReleasedTag{}, nil
	}

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
		result = []peopledirectory.ReleasedTag{}
	}
	return result, err
}

func (s *Service) RestoreTag(ctx context.Context, personID int64, tagID string) (restored bool, err error) {
	tagID = peopledirectory.NormalizeTagID(tagID)
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if tagID == "" {
		return false, nil
	}

	err = s.runWrite(ctx, "restore_tag", func(txCtx context.Context, stats *domain.OperationStats) error {
		return s.tx.RunSavepoint(txCtx, func(spCtx context.Context) error {
			var restoreStats domain.OperationStats
			restored, restoreStats, err = s.store.RestoreTag(spCtx, personID, tagID)
			stats.Add(restoreStats)
			return err
		})
	})
	if errors.Is(err, peopledirectory.ErrTagConflict) {
		// The tag found a new holder between the snapshot read and the write;
		// the savepoint rolled the statement back and the current holder wins.
		return false, nil
	}
	return restored, err
}

func invalid(reason string) error { return &peopledirectory.InvalidPersonError{Reason: reason} }
