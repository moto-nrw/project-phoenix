package compose

import (
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
)

type NativeMirrorRows struct {
	Instances    *ports.MirrorInstanceRecords
	Participants *ports.MirrorParticipantRecords
}

func NewNativeMirrorRows(rows OwnerRows) NativeMirrorRows {
	return NativeMirrorRows{Instances: application.NativeMirrorInstances(rows.Instances), Participants: application.NativeMirrorParticipants(rows.Participants)}
}
