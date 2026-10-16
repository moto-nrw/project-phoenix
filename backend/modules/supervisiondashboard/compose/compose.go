// Package compose names the supervision dashboard for the typed root: the
// sources it reads and its constructor. The dashboard's adapter stays inside
// the owner (ADR 0053 point 2).
package compose

import supervisiondashboardlegacy "github.com/moto-nrw/project-phoenix/modules/supervisiondashboard/legacy"

// Sources are the owner capabilities the dashboard reads.
type Sources = supervisiondashboardlegacy.Sources

// CallerGroup is one of the caller's educational groups, as the root serves
// the dashboard's caller port (#4006).
type CallerGroup = supervisiondashboardlegacy.CallerGroup

// OpenRoomSessions are the running sessions the dashboard shows (#4010).
type OpenRoomSessions = supervisiondashboardlegacy.OpenRoomSessions

// New builds the supervision dashboard over its sources.
var New = supervisiondashboardlegacy.New
