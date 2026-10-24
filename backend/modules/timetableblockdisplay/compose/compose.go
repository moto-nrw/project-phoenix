// Package compose names the timetable block display projection for the typed
// root, which binds its reader into the Timetable owner's block metadata
// (ADR 0053 point 2).
package compose

import timetableblockdisplay "github.com/moto-nrw/project-phoenix/modules/timetableblockdisplay/internal/adapters/postgres"

// List reads the display metadata of scheduled timetable blocks.
var List = timetableblockdisplay.List
