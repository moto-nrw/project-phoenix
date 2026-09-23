package timetableplanning

import scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"

// indexActivityInstances keys instances by id; the shift-plan-sync workflow
// that first held it keeps its own copy (#3219, #3418).
func indexActivityInstances(instances []*scheduleModel.ActivityInstance) map[int64]*scheduleModel.ActivityInstance {
	indexed := make(map[int64]*scheduleModel.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance != nil {
			indexed[instance.ID] = instance
		}
	}
	return indexed
}
