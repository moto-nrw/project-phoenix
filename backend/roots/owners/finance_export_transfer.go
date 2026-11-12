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
