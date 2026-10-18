//go:build testhooks

package application

import (
	models "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type nativeFixtureGroup struct {
	fixtureRecord[timetable.Group, activityContract.Group]
}

func NativeGroupRecords(rows models.GroupRepository) ports.NativeGroupRecords {
	return nativeFixtureGroup{fixtureRecord: fixtureRecord[timetable.Group, activityContract.Group]{rows: rows, encode: encodeFixtureGroup, decode: decodeFixtureGroup}}
}
func encodeFixtureGroup(value *timetable.Group) (*activityContract.Group, error) {
	if value == nil {
		return nil, nil
	}
	return legacyGroup(*value), nil
}
func decodeFixtureGroup(value *activityContract.Group) *timetable.Group {
	if value == nil {
		return nil
	}
	row := CatalogGroup(value)
	return &row
}
