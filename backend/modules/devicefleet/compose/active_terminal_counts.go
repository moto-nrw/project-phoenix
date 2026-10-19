package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/postgres"
)

// NewActiveTerminalCounts composes the cross-school active terminal count of
// the operator billing report (#2791). It reads on the caller's
// administrative transaction and needs nothing else of the owner.
func NewActiveTerminalCounts() devicefleet.ActiveTerminalCounts {
	return postgres.ActiveTerminalCounts{}
}
