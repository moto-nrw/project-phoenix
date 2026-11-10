package api

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
)

// NewFinanceExportHTTP binds the finance exports of the Elternabrechnung
// (#4693) to the Träger right "Finanz-Export" and the history of an invoice
// to the right "Abrechnung".
func NewFinanceExportHTTP(exports parentbilling.FinanceExports) *parentBillingHTTP.FinanceExportResource {
	return parentBillingHTTP.NewFinanceExportResource(exports,
		common.RequiresPermission(identityaccess.TraegerRightFinanceExport),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}
