package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// These are defined workflow DTOs, not aliases to another owner's model.
type PersonSnapshot care.Person
type PersonAccountSnapshot care.PersonAccount
type PersonRFIDSnapshot care.PersonRFIDCard

type PersonReads interface{ care.PersonReads }

// WithPersonSnapshotRelations preserves both nullable, already-loaded facts.
func WithPersonSnapshotRelations(value *PersonSnapshot, account *PersonAccountSnapshot, card *PersonRFIDSnapshot) *PersonSnapshot {
	if value == nil {
		return nil
	}
	value.Account = (*care.PersonAccount)(account)
	value.RFIDCard = (*care.PersonRFIDCard)(card)
	return value
}

// NewPersonReads binds a root callback to the application's native read port.
func NewPersonReads(find func(context.Context, int64) (*PersonSnapshot, error)) PersonReads {
	return personReads{find}
}

type personReads struct {
	find func(context.Context, int64) (*PersonSnapshot, error)
}

func (r personReads) FindByID(ctx context.Context, id int64) (*care.Person, error) {
	value, err := r.find(ctx, id)
	return (*care.Person)(value), err
}
