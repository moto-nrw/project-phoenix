package timetableplanning

import scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"

// indexActivityInstances keys instances by id. The shift-plan sync service
// that first held it moved to modules/workforce/legacy/shiftplanning (#3219);
// the timetable data service keeps its own copy.
func indexActivityInstances(instances []*scheduleModel.ActivityInstance) map[int64]*scheduleModel.ActivityInstance {
	indexed := make(map[int64]*scheduleModel.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance != nil {
			indexed[instance.ID] = instance
		}
	}
	return indexed
}
