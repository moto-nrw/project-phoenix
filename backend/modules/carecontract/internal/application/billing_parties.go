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

// BillingReads serves carecontract.ContractBilling: the versions in force
// and the recipient of each monthly invoice (#4684).
type BillingReads struct {
	*Versions
	parties PartyStore
}

var _ carecontract.ContractBilling = (*BillingReads)(nil)

// BillingStore is a store that reads versions and parties.
type BillingStore interface {
	Store
	PartyStore
}

// NewBillingReads builds the reads of the parents' billing.
func NewBillingReads(store BillingStore) *BillingReads {
	return &BillingReads{Versions: NewVersions(store), parties: store}
}

// BillingRecipients picks per contract the first party with an e-mail
// address, else the first party: the invoice goes where a mail can reach.
func (b *BillingReads) BillingRecipients(ctx context.Context, organizationID int64, contractIDs []int64) (map[int64]carecontract.Party, error) {
	if organizationID <= 0 {
		return nil, &carecontract.ValidationError{Fields: map[string]string{"organization": "required"}}
	}
	recipients := map[int64]carecontract.Party{}
	if len(contractIDs) == 0 {
		return recipients, nil
	}
	parties, err := b.parties.PartiesOf(ctx, organizationID, contractIDs)
	if err != nil {
		return nil, fmt.Errorf("care contract: read contract parties: %w", err)
	}
	for contractID, list := range parties {
		recipients[contractID] = billingRecipient(list)
	}
	return recipients, nil
}

// billingRecipient is the first party with an e-mail address, else the
// first party; none for no party.
func billingRecipient(parties []carecontract.Party) carecontract.Party {
	if len(parties) == 0 {
		return carecontract.Party{}
	}
	for _, party := range parties {
		if party.Email != "" {
			return party
		}
	}
	return parties[0]
}
