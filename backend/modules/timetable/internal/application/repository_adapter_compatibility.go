package application

import (
	activitiesModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
)

func legacyDatabaseError(operation string, err error) error {
	return activitiesModels.WrapDatabaseError(operation, err)
}

func legacyNotFoundError(operation string) error {
	return activitiesModels.WrapNotFoundDatabaseError(operation)
}

func SetDate[D ~string](target *D, value string) { *target = D(value) }
