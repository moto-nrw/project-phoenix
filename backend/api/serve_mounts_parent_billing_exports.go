package api

import (
	"log/slog"

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	exportTransferCompose "github.com/moto-ogs/project-phoenix/modules/exporttransfer/compose"
	exportTransferHTTP "github.com/moto-ogs/project-phoenix/modules/exporttransfer/http"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-ogs/project-phoenix/modules/parentbilling/http"
	"github.com/moto-ogs/project-phoenix/roots/owners"
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
// recipient of the Träger (#6824, ADR 0218) over Export Transfer's Träger
// scope: the Sollstellung and DATEV handovers to the right "Finanz-Export",
// the PAIN.008 handover to "Abrechnung", like the downloads.
func NewExportHandoverHTTP(exports parentbilling.FinanceExports, debits parentbilling.SEPADebits, transfers exporttransfer.TraegerTransfers,
) *parentBillingHTTP.ExportHandoverResource {
	transport := owners.FinanceExportTransfer{Transfers: transfers}
	return parentBillingHTTP.NewExportHandoverResource(parentBillingCompose.NewExportHandovers(exports, debits, transport),
		common.RequiresPermission(identityaccess.TraegerRightFinanceExport),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}

// NewExportRecipientHTTP binds the recipients of the Träger and the format
// each receives (ADR 0218, ADR 0222) to the right "Finanz-Export", like the
// DATEV values and the field mapping.
func NewExportRecipientHTTP(transfers exporttransfer.TraegerTransfers) *exportTransferHTTP.RecipientResource {
	return exportTransferHTTP.NewRecipientResource(transfers, common.RequiresPermission(identityaccess.TraegerRightFinanceExport))
}

// NewTraegerExportTransfers builds Export Transfer's Träger scope over the
// bank cipher of the root, which seals the recipient password (ADR 0218).
func NewTraegerExportTransfers(bankData parentbilling.BankDataCipher, logger *slog.Logger) exporttransfer.TraegerTransfers {
	transfers, err := exportTransferCompose.NewTraegerTransfers(bankData, logger)
	if err != nil {
		panic(err)
	}
	return transfers
}
