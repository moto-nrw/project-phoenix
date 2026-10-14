package api

import (
	"time"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewStaffDocumentCleanup composes the narrow cleanup capability for adapter
// tests without constructing a repository factory.
func NewStaffDocumentCleanup(db transactionruntime.Database, now func() time.Time) (*workforce.DocumentCleanup, error) {
	return workforceCompose.NewDocumentCleanup(db.Pool(), now)
}
