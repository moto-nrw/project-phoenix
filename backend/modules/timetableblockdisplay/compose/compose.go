// Package compose names the timetable block display projection for the typed
// root, which binds its reader into the Timetable owner's block metadata
// (ADR 0053 point 2).
package compose

import "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay"

// List reads the display metadata of scheduled timetable blocks.
var List = timetableblockdisplay.List
