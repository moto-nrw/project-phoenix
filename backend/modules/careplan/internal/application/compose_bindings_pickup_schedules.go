package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

func PickupScheduleRecords(r carePlanBindingPorts.PickupScheduleRecords) EffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule] {
	return NewEffectiveRecords[careplan.PickupSchedule, *careplan.PickupSchedule](RecordFuncs[careplan.PickupSchedule]{Find: Unlocked(r.FindPickupSchedule), List: r.ListPickupSchedules,
		Create: r.CreatePickupSchedule, Upsert: r.UpsertPickupSchedule, Remove: r.DeletePickupSchedule, RemoveByStudent: r.DeletePickupSchedulesByStudent})
}

func PickupExceptionRecords(r carePlanBindingPorts.PickupScheduleRecords) ExceptionRecords[careplan.PickupException, *careplan.PickupException] {
	return NewExceptionRecords[careplan.PickupException, *careplan.PickupException](RecordFuncs[careplan.PickupException]{Find: r.FindPickupException, List: r.ListPickupExceptions,
		Create: r.CreatePickupException, Update: r.UpdatePickupException, Remove: r.DeletePickupException, RemoveByStudent: r.DeletePickupExceptionsByStudent})
}

func PickupNoteRecords(r carePlanBindingPorts.PickupScheduleRecords) NoteRecords[careplan.PickupNote, *careplan.PickupNote] {
	return NewNoteRecords[careplan.PickupNote, *careplan.PickupNote](RecordFuncs[careplan.PickupNote]{Find: Unlocked(r.FindPickupNote), List: r.ListPickupNotes,
		Create: r.CreatePickupNote, Update: r.UpdatePickupNote, Remove: r.DeletePickupNote, RemoveByStudent: r.DeletePickupNotesByStudent})
}
