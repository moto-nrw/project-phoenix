//go:build testhooks

package application

import (
	models "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureGroup struct {
	fixtureRecord[timetable.Group, models.Group]
}

func NativeGroupRecords(rows models.GroupRepository) ports.NativeGroupRecords {
	return nativeFixtureGroup{fixtureRecord: fixtureRecord[timetable.Group, models.Group]{rows: rows, encode: encodeFixtureGroup, decode: decodeFixtureGroup}}
}
func encodeFixtureGroup(value *timetable.Group) (*models.Group, error) {
	if value == nil {
		return nil, nil
	}
	return legacyGroup(*value), nil
}
func decodeFixtureGroup(value *models.Group) *timetable.Group {
	if value == nil {
		return nil
	}
	row := CatalogGroup(value)
	return &row
}
