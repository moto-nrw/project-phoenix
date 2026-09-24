package timetableplanning

import (
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// SubstituteTimeConflict keeps the retained name of the owner's warning type;
// the detection itself lives in modules/timetable since #3594.
type SubstituteTimeConflict = timetable.SubstituteTimeConflict

// toConflictInstance converts an ActivityInstance's TIME columns into the
// minutes-since-midnight form expected by the conflict helper.
func toConflictInstance(inst *scheduleModel.ActivityInstance) timetable.SubstituteConflictInstance {
	return timetable.SubstituteConflictInstance{
		ID:        inst.ID,
		StartMin:  timetable.MinutesOfTime(inst.StartTime.Hour(), inst.StartTime.Minute()),
		EndMin:    timetable.MinutesOfTime(inst.EndTime.Hour(), inst.EndTime.Minute()),
		StartHHMM: inst.StartTime.Format("15:04"),
	}
}
