// Package compose builds the emergency snapshot for the root (ADR 0053).
package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
	legacy "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/internal/adapters/nameorder"
)

// New builds the Notfallliste; it lists children in German dictionary order.
func New(deps emergencysnapshot.Dependencies) (*emergencysnapshot.Projection, error) {
	deps.Collate = legacy.CompareNames
	return emergencysnapshot.New(deps)
}
