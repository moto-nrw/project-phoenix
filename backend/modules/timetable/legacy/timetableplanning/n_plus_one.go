package timetableplanning

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
)

// int64FilterArgs widens IDs for the persistence-neutral Filter.In API.
func int64FilterArgs(ids []int64) []any {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

func dateFilterArgs(dates []timezone.Date) []any {
	args := make([]any, len(dates))
	for i, date := range dates {
		args[i] = date
	}
	return args
}

func activityInstanceIDs(instances []*scheduleModel.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func indexInstanceStaffRows(rows []*scheduleModel.InstanceStaff) map[int64][]*scheduleModel.InstanceStaff {
	byInstance := make(map[int64][]*scheduleModel.InstanceStaff)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

func indexInstanceStudentRows(rows []*scheduleModel.InstanceStudent) map[int64][]*scheduleModel.InstanceStudent {
	byInstance := make(map[int64][]*scheduleModel.InstanceStudent)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}
