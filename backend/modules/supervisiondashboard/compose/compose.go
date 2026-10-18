// Package compose names the supervision dashboard's room order for the typed
// root, which binds the dashboard's ports (ADR 0053, ADR 0058).
package compose

import legacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/internal/adapters/nameorder"

// CompareRoomNames orders two room names in German dictionary order
// (DIN 5007-1), the order the session port delivers its rows in.
func CompareRoomNames(a, b string) int {
	return legacy.CompareNames(a, b)
}
