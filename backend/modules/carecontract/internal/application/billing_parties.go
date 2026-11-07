package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// PartyStore reads the parties of contracts in the caller's Träger unit.
type PartyStore interface {
	PartiesOf(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64][]carecontract.Party, error)
}

// Parties serves carecontract.ContractParties for the parents' billing.
type Parties struct {
	store PartyStore
}

var _ carecontract.ContractParties = (*Parties)(nil)

// NewParties builds the read of the contract parties.
func NewParties(store PartyStore) *Parties {
	if store == nil {
		panic("care contract: the store is required")
	}
	return &Parties{store: store}
}

// PartiesOf checks the Träger and reads the parties; no contract reads
// nothing.
func (p *Parties) PartiesOf(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64][]carecontract.Party, error) {
	if organizationID <= 0 {
		return nil, &carecontract.ValidationError{Fields: map[string]string{"organization": "required"}}
	}
	if len(contractIDs) == 0 {
		return map[int64][]carecontract.Party{}, nil
	}
	parties, err := p.store.PartiesOf(ctx, organizationID, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("care contract: read contract parties: %w", err)
	}
	return parties, nil
}
