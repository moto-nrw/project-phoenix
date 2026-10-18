// Package compose builds the Group Live view for the typed root, which binds
// its ports (ADR 0053, ADR 0058).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/grouplive"
	legacy "github.com/moto-nrw/project-phoenix/modules/grouplive/internal/adapters/nameorder"
)

// New builds the live-group view; it lists groups in German dictionary order.
func New(deps grouplive.Dependencies) grouplive.Query {
	deps.Collate = legacy.CompareNames
	return grouplive.New(deps)
}
