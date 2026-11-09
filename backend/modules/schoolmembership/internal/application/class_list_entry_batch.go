package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// The data import binds the class-list batch slice outside Owner (#5827).
var _ schoolmembership.ClassListEntryBatches = (*Service)(nil)

// CreateClassListEntries is CreateClassListEntry for many entries in one
// statement: each input is validated and normalized like a single create,
// then all are written or none.
func (s *Service) CreateClassListEntries(ctx context.Context, inputs []schoolmembership.CreateClassListEntry) (result []schoolmembership.ClassListEntry, err error) {
	if len(inputs) == 0 {
		return []schoolmembership.ClassListEntry{}, nil
	}
	fields := make([]schoolmembership.ClassListEntryFields, len(inputs))
	createdBy := make([]*int64, len(inputs))
	for i, input := range inputs {
		if err := validateClassListEntryFields(&input.ClassListEntryFields); err != nil {
			return nil, err
		}
		if input.CreatedBy != nil && *input.CreatedBy <= 0 {
			input.CreatedBy = nil
		}
		fields[i], createdBy[i] = input.ClassListEntryFields, input.CreatedBy
	}
	err = s.runWrite(ctx, "create_class_list_entries", func(txCtx context.Context, stats *domain.OperationStats) error {
		var createStats domain.OperationStats
		result, createStats, err = s.store.CreateClassListEntries(txCtx, fields, createdBy)
		stats.Add(createStats)
		return err
	})
	return result, err
}

// MatchClassListEntries runs the name-and-class filter of
// ListClassListEntries for many keys in one statement.
func (s *Service) MatchClassListEntries(ctx context.Context, keys []schoolmembership.ClassListEntryFields) (result [][]schoolmembership.ClassListEntry, err error) {
	if len(keys) == 0 {
		return [][]schoolmembership.ClassListEntry{}, nil
	}
	normalized := make([]schoolmembership.ClassListEntryFields, len(keys))
	for i, key := range keys {
		key.FirstName, key.LastName, key.SchoolClass = strings.TrimSpace(key.FirstName), strings.TrimSpace(key.LastName), strings.TrimSpace(key.SchoolClass)
		if key.FirstName == "" || key.LastName == "" || key.SchoolClass == "" {
			return nil, invalid("first name, last name and school class are required to match class list entries")
		}
		normalized[i] = key
	}
	err = s.runRead(ctx, "match_class_list_entries", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.MatchClassListEntries(txCtx, normalized)
		stats.Add(queryStats)
		return err
	})
	return result, err
}
