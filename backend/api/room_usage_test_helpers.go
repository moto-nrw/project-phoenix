//go:build testhooks

package api

import (
	"context"
	"log/slog"

	facilitiesModule "github.com/moto-ogs/project-phoenix/modules/facilities"
	transactionruntime "github.com/moto-ogs/project-phoenix/modules/transactionruntime/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
)

// NewRoomReferencesForTest binds the Facilities room reference count over
// the same owners the serving root asks (#6845).
func NewRoomReferencesForTest(db transactionruntime.Database) (func(context.Context, int64) (facilitiesModule.RoomReferences, error), error) {
	groups, err := owners.NewSchoolStructure(db)
	if err != nil {
		return nil, err
	}
	devices, err := owners.NewDeviceFleet(db)
	if err != nil {
		return nil, err
	}
	return owners.NewRoomReferences(db, owners.NewLoggedStudentPresence(db, slog.Default()), groups, devices)
}
