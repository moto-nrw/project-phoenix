package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// Inbox serves carecontract.Inbox (ADR 0088 section 7, ADR 0126, ADR 0153):
// a contract lies in the inbox of the account its mail went to. The parents
// portal supplies the schools and children its relationship checks permit.
type Inbox struct {
	store Store
}

var _ carecontract.Inbox = (*Inbox)(nil)

// NewInbox builds the contracts of the parents' inbox.
func NewInbox(store Store) *Inbox {
	if store == nil {
		panic("care contract: the store is required")
	}
	return &Inbox{store: store}
}

// InboxContracts lists the sent contracts addressed to the reader and
// records the read of each.
func (in *Inbox) InboxContracts(ctx context.Context, organizationID int64, reader carecontract.InboxReader) ([]carecontract.InboxContract, error) {
	email, err := inboxAddress(organizationID, reader)
	if err != nil {
		return nil, err
	}
	reader.Email = email
	contracts, err := in.store.InboxContracts(ctx, organizationID, reader)
	if err != nil {
		return nil, fmt.Errorf("care contract: read inbox: %w", err)
	}
	if len(contracts) == 0 {
		return contracts, nil
	}
	ids := make([]int64, len(contracts))
	for i, contract := range contracts {
		ids[i] = contract.ID
	}
	if err := in.store.InsertEvents(ctx, organizationID, []Event{{
		AccountID: reader.AccountID, Action: ActionRead, Details: map[string]any{"inbox": "list", "contract_ids": ids},
	}}); err != nil {
		return nil, fmt.Errorf("care contract: record inbox read: %w", err)
	}
	return contracts, nil
}

// InboxContractDocument reads the stored PDF of one sent contract addressed
// to the reader and records the read.
func (in *Inbox) InboxContractDocument(ctx context.Context, organizationID, contractID int64, reader carecontract.InboxReader) (carecontract.File, error) {
	email, err := inboxAddress(organizationID, reader)
	if err != nil {
		return carecontract.File{}, err
	}
	if contractID <= 0 {
		return carecontract.File{}, carecontract.ErrInboxDocumentNotFound
	}
	reader.Email = email
	file, err := in.store.InboxDocument(ctx, organizationID, contractID, reader)
	if err != nil {
		return carecontract.File{}, err
	}
	if err := in.store.InsertEvents(ctx, organizationID, []Event{{
		ContractID: &contractID, AccountID: reader.AccountID, Action: ActionRead, Details: map[string]any{"inbox": "download"},
	}}); err != nil {
		return carecontract.File{}, fmt.Errorf("care contract: record inbox read: %w", err)
	}
	return file, nil
}

func inboxAddress(organizationID int64, reader carecontract.InboxReader) (string, error) {
	email := strings.TrimSpace(reader.Email)
	if organizationID <= 0 || reader.AccountID <= 0 || email == "" {
		return "", carecontract.ErrInboxDocumentNotFound
	}
	return email, nil
}
