package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling/internal/application"
)

// NewFinanceExports builds the finance exports of the Elternabrechnung
// (#4693): field mapping, file numbers, export protocol, Sollstellung and
// the history of an invoice, on the owner's store.
func NewFinanceExports() parentbilling.FinanceExports {
	return application.NewFinanceExports(postgres.NewStore())
}
