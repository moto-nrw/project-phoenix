package enrollment_test

import (
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
)

func activityDatePtr[D ~string](date *D) *activitiesModels.Date {
	if date == nil {
		return nil
	}
	value := activitiesModels.Date(*date)
	return &value
}
