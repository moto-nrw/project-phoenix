package repositories

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/uptrace/bun"
)

// NewStaffDocumentCleanup composes the narrow cleanup capability for adapter
// tests without constructing a repository factory.
func NewStaffDocumentCleanup(db *bun.DB, now func() time.Time) (*workforce.DocumentCleanup, error) {
	return workforceCompose.NewDocumentCleanup(db, now)
}
