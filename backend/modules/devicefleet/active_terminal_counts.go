package devicefleet

import "context"

// ActiveTerminalCounts answers the operator billing report (#2791): how many
// terminals each school runs right now. Device Fleet owns the rule of what
// counts: a device that is not archived (a transfer archives the source row),
// is not the virtual web check-in device, and is in status active. Whether
// the terminal was online recently does not matter. Schools without an active
// terminal are missing from the map.
//
// The caller must run it in the administrative transaction, which sees every
// school; inside a tenant transaction it fails instead of counting one school.
type ActiveTerminalCounts interface {
	CountActiveTerminalsByTenant(context.Context) (map[int64]int, error)
}
