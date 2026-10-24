package compose

import (
	postgresNative "github.com/moto-nrw/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/uptrace/bun"
)

// BlockDisplayProjection lists a tenant's block display metadata over the
// handle the caller's tenant transaction holds. The legacy composition binds
// the block display projection, which this composition may not name.

// NewBlockDisplayMetadata serves the planner's BlockMetadata port from the
// block display projection over db, inside the ambient tenant transaction
// supplied by the application workflow.
func NewBlockDisplayMetadata(db *bun.DB, project postgresNative.BlockDisplayProjection) ports.BlockDisplayMetadata {
	if db == nil {
		panic("timetable block display metadata: database is required")
	}
	if project == nil {
		panic("timetable block display metadata: projection is required")
	}
	return postgresNative.BlockDisplayMetadata{Db: db, Project: project}
}
