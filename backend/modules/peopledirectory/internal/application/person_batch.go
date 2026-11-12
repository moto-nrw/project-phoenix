package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
)

// The data import binds the person batch slice outside Owner (#5552).
var _ peopledirectory.PersonBatches = (*Service)(nil)

// CreatePersons is CreatePerson for many persons in one statement: each
// input is normalized and validated like a single create, then all are
// written or none.
func (s *Service) CreatePersons(ctx context.Context, inputs []peopledirectory.CreatePerson) (result []peopledirectory.Person, err error) {
	if len(inputs) == 0 {
		return []peopledirectory.Person{}, nil
	}
	normalized := make([]peopledirectory.CreatePerson, len(inputs))
	for i, input := range inputs {
		input, err = peopledirectory.NormalizePersonCreate(input)
		if err != nil {
			return nil, err
		}
		normalized[i] = peopledirectory.CreatePerson{
			FirstName: input.FirstName, LastName: input.LastName, Birthday: input.Birthday,
			TagID: input.TagID, AccountID: input.AccountID,
		}
	}
	// A batch command joins its caller's transaction and never opens one
	// (ADR 0067 addendum of 2026-11-08, point 3). The data import observes
	// it (create_persons); the owner observer is a callable the
	// command-effects inventory cannot follow, which a new command may only
	// reach with an approval ADR (ADR 0083, 0098).
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	result, _, err = s.store.CreatePersons(ctx, normalized)
	return result, MapError(err)
}
