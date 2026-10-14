// Package compose names the supervision dashboard for the typed root: the
// sources it reads, its constructor and the yard view. The dashboard's
// adapter stays inside the owner (ADR 0053 point 2).
package compose

import supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"

// Sources are the owner capabilities the dashboard reads.
type Sources = supervisiondashboardlegacy.Sources

// CallerGroup is one of the caller's educational groups, as the root serves
// the dashboard's caller port (#4006).
type CallerGroup = supervisiondashboardlegacy.CallerGroup

var (
	// New builds the supervision dashboard over its sources.
	New = supervisiondashboardlegacy.New
	// NewYard adapts the yard service to the dashboard's yard view.
	NewYard = supervisiondashboardlegacy.NewYard
)
