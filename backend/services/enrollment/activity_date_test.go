package enrollment_test

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
)

func activityDatePtr(date *timezone.Date) *activitiesModels.Date {
	if date == nil {
		return nil
	}
	value := activitiesModels.Date(*date)
	return &value
}
