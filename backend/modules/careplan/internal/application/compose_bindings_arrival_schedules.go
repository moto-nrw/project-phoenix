package application

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

func ArrivalScheduleRecords(r carePlanBindingPorts.ArrivalScheduleRecords) EffectiveRecords[careplan.ArrivalSchedule, *careplan.ArrivalSchedule] {
	return NewEffectiveRecords[careplan.ArrivalSchedule, *careplan.ArrivalSchedule](RecordFuncs[careplan.ArrivalSchedule]{Find: Unlocked(r.FindArrivalSchedule), List: r.ListArrivalSchedules,
		Create: r.CreateArrivalSchedule, Upsert: r.UpsertArrivalSchedule, Remove: r.DeleteArrivalSchedule, RemoveByStudent: r.DeleteArrivalSchedulesByStudent})
}

func ArrivalExceptionRecords(r carePlanBindingPorts.ArrivalScheduleRecords) ExceptionRecords[careplan.ArrivalException, *careplan.ArrivalException] {
	return NewExceptionRecords[careplan.ArrivalException, *careplan.ArrivalException](RecordFuncs[careplan.ArrivalException]{Find: r.FindArrivalException, List: r.ListArrivalExceptions,
		Create: r.CreateArrivalException, Update: r.UpdateArrivalException, Remove: r.DeleteArrivalException, RemoveByStudent: r.DeleteArrivalExceptionsByStudent})
}

func ArrivalNoteRecords(r carePlanBindingPorts.ArrivalScheduleRecords) NoteRecords[careplan.ArrivalNote, *careplan.ArrivalNote] {
	return NewNoteRecords[careplan.ArrivalNote, *careplan.ArrivalNote](RecordFuncs[careplan.ArrivalNote]{Find: Unlocked(r.FindArrivalNote), List: r.ListArrivalNotes,
		Create: r.CreateArrivalNote, Update: r.UpdateArrivalNote, Remove: r.DeleteArrivalNote, RemoveByStudent: r.DeleteArrivalNotesByStudent})
}
