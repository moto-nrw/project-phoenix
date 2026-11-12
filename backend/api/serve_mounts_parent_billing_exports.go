package api

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/api/common"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
	"github.com/moto-nrw/project-phoenix/roots/owners"
)

// NewFinanceExportHTTP binds the finance exports of the Elternabrechnung
// (#4693) to the Träger right "Finanz-Export" and the history of an invoice
// to the right "Abrechnung"; the revenue accounts and cost centers (#4821)
// belong to "Finanz-Export" too.
func NewFinanceExportHTTP(exports parentbilling.FinanceExports, accounts parentbilling.BookingAccountService) *parentBillingHTTP.FinanceExportResource {
	return parentBillingHTTP.NewFinanceExportResource(exports, accounts,
		common.RequiresPermission(identityaccess.TraegerRightFinanceExport),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewExportHandoverHTTP binds the handover of the finance exports to the
// recipient of the Träger (#6824, ADR 0217): Export Transfer's Träger scope
// with the bank cipher of the root for the recipient password, the
// recipient and the Sollstellung and DATEV handovers to the right
// "Finanz-Export", the PAIN.008 handover to "Abrechnung", like the downloads.
func NewExportHandoverHTTP(exports parentbilling.FinanceExports, debits parentbilling.SEPADebits, bankData parentbilling.BankDataCipher,
	logger *slog.Logger,
) *parentBillingHTTP.ExportHandoverResource {
	transfers, err := exportTransferCompose.NewTraegerTransfers(bankData, logger)
	if err != nil {
		panic(err)
	}
	transport := owners.FinanceExportTransfer{Transfers: transfers}
	return parentBillingHTTP.NewExportHandoverResource(parentBillingCompose.NewExportHandovers(exports, debits, transport), transport,
		common.RequiresPermission(identityaccess.TraegerRightFinanceExport),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}
