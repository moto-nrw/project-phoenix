//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewUnobservedCorrectionQueries builds the Audit Platform's correction query
// the request review history reads, for test graphs that record no
// observations.
func NewUnobservedCorrectionQueries(db transactionruntime.Database) (auditlog.CorrectionQuery, error) {
	return auditCompose.NewCorrectionQueries(db.Pool(), func(auditlog.Observation) {})
}
