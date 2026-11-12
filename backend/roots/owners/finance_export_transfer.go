package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// FinanceExportTransfer binds the Elternabrechnung's handover port to the
// Träger scope of Export Transfer (#6824, ADR 0217). It translates the two
// vocabularies and decides nothing.
type FinanceExportTransfer struct {
	Transfers exporttransfer.TraegerTransfers
}

var _ parentbilling.FinanceExportTransfer = FinanceExportTransfer{}

// Handover carries one finance export. An outcome whose journal could not
// be completed is uncertain, not an error: the file may have arrived.
func (t FinanceExportTransfer) Handover(ctx context.Context, request parentbilling.FinanceHandoverRequest,
) (parentbilling.FinanceHandoverOutcome, error) {
	outcome, err := t.Transfers.Transfer(ctx, exporttransfer.TraegerTransferRequest{
		OrganizationID: request.OrganizationID, Kind: exporttransfer.KindParentBillingFinance, Format: request.Format,
		ActorAccountID: request.AccountID,
		PrepareFile: func(ctx context.Context, destination exporttransfer.Destination) (string, []byte, error) {
			return request.Prepare(ctx, destination.Host, destination.Directory)
		},
		Settle: func(ctx context.Context, outcome exporttransfer.Outcome) error {
			return request.Settle(ctx, outcome.Transferred, outcome.Reason)
		},
	})
	if errors.Is(err, exporttransfer.ErrUnknownSettlement) {
		return parentbilling.FinanceHandoverOutcome{Uncertain: true}, nil
	}
	if err != nil {
		return parentbilling.FinanceHandoverOutcome{}, err
	}
	if outcome.Reason == exporttransfer.ReasonNotConfigured {
		return parentbilling.FinanceHandoverOutcome{NotReady: true}, nil
	}
	return parentbilling.FinanceHandoverOutcome{Transferred: outcome.Transferred, Reason: outcome.Reason,
		TargetHost: outcome.TargetHost, TargetDirectory: outcome.TargetDirectory}, nil
}

// ExportRecipient reads the recipient without its password.
func (t FinanceExportTransfer) ExportRecipient(ctx context.Context, organizationID int64) (parentbilling.ExportRecipient, error) {
	recipient, err := t.Transfers.Recipient(ctx, organizationID)
	return parentbilling.ExportRecipient(recipient), err
}

// SaveExportRecipient saves the recipient; invalid fields keep their names.
func (t FinanceExportTransfer) SaveExportRecipient(ctx context.Context, organizationID, accountID int64, input parentbilling.ExportRecipientInput,
) (parentbilling.ExportRecipient, error) {
	recipient, err := t.Transfers.SaveRecipient(ctx, organizationID, accountID, exporttransfer.TraegerRecipientInput(input))
	var invalid *exporttransfer.RecipientError
	if errors.As(err, &invalid) {
		return parentbilling.ExportRecipient{}, &parentbilling.ExportRecipientError{Fields: invalid.Fields}
	}
	return parentbilling.ExportRecipient(recipient), err
}

// DeleteExportRecipient removes the recipient.
func (t FinanceExportTransfer) DeleteExportRecipient(ctx context.Context, organizationID int64) error {
	return t.Transfers.DeleteRecipient(ctx, organizationID)
}

// ExportHandovers reads the newest attempts of the journal.
func (t FinanceExportTransfer) ExportHandovers(ctx context.Context, organizationID int64, limit int) ([]parentbilling.ExportHandoverEntry, error) {
	entries, err := t.Transfers.Journal(ctx, organizationID, limit)
	if err != nil {
		return nil, err
	}
	handovers := make([]parentbilling.ExportHandoverEntry, len(entries))
	for i, entry := range entries {
		handovers[i] = parentbilling.ExportHandoverEntry{ID: entry.ID, CreatedAt: entry.CreatedAt, Format: entry.Format,
			FileName: entry.Filename, ByteSize: entry.ByteSize, TargetHost: entry.TargetHost, TargetDirectory: entry.TargetDirectory,
			State: entry.State, Reason: entry.Reason}
	}
	return handovers, nil
}
