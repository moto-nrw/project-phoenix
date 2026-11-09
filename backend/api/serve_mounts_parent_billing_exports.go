package api

import (
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	parentBillingCompose "github.com/moto-nrw/project-phoenix/modules/parentbilling/compose"
	parentBillingHTTP "github.com/moto-nrw/project-phoenix/modules/parentbilling/http"
)

// NewFinanceExportHTTP binds the finance exports of the Elternabrechnung
// (#4693) to the Träger right "Finanz-Export" and the history of an invoice
// to the right "Abrechnung".
func NewFinanceExportHTTP() *parentBillingHTTP.FinanceExportResource {
	return parentBillingHTTP.NewFinanceExportResource(parentBillingCompose.NewFinanceExports(),
		common.RequiresPermission(identityaccess.TraegerRightFinanceExport),
		common.RequiresPermission(identityaccess.TraegerRightBillingManage))
}
