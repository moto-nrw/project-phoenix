package compose

import (
	"github.com/uptrace/bun"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
)

func mustPresenceReads(db *bun.DB) PresenceReads {
	reads, err := NewPresenceReads(db)
	if err != nil {
		panic(err)
	}
	return reads
}

func legacyDatabaseError(operation string, err error) error {
	return activitiesModels.WrapDatabaseError(operation, err)
}

func legacyNotFoundError(operation string) error {
	return activitiesModels.WrapNotFoundDatabaseError(operation)
}

func legacyNullInt64(value *int64) activitiesModels.NullInt64 {
	if value == nil {
		return activitiesModels.NullInt64{}
	}
	return activitiesModels.NullInt64{Int64: *value, Valid: true}
}

func legacyNullInt16(value *int16) activitiesModels.NullInt16 {
	if value == nil {
		return activitiesModels.NullInt16{}
	}
	return activitiesModels.NullInt16{Int16: *value, Valid: true}
}

func legacyNullString(value *string) activitiesModels.NullString {
	if value == nil {
		return activitiesModels.NullString{}
	}
	return activitiesModels.NullString{String: *value, Valid: true}
}

func setDate[D ~string](target *D, value string) { *target = D(value) }
