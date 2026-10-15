// Package compose names the emergency snapshot projection for the typed root:
// the sources it reads and its constructor. The projection's adapter stays
// inside the owner (ADR 0053 point 2).
package compose

import emergencysnapshotlegacy "github.com/moto-nrw/project-phoenix/modules/emergencysnapshot/legacy"

// Sources are the owner facades and compatibility bindings the Notfallliste
// reads.
type Sources = emergencysnapshotlegacy.Sources

// New builds the Notfallliste over its sources.
var New = emergencysnapshotlegacy.New
