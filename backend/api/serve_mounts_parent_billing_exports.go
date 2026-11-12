package api

import (
	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingHTTP "github.com/moto-ogs/project-phoenix/modules/parentbilling/http"
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
