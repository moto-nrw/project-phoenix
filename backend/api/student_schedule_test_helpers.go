//go:build testhooks

package api

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
)

func NewPickupNoteRepository(capability careplan.Capability) timetablePorts.CarePickupNoteRows {
	return timetableCompose.NewCarePickupNoteRows(owners.NewCarePlanRows(capability))
}
