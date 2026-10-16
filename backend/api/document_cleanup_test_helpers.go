//go:build testhooks

package api

import (
	"time"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewStaffDocumentCleanup composes the narrow cleanup capability for adapter
// tests without constructing a repository factory. Workforce's own suites
// call its composition themselves; the People Directory offboarding suite may
// not import it and builds the owner through the root (ADR 0053 point 7.2,
// #4005).
func NewStaffDocumentCleanup(db transactionruntime.Database, now func() time.Time) (*workforce.DocumentCleanup, error) {
	return workforceCompose.NewDocumentCleanup(db.Pool(), now)
}
