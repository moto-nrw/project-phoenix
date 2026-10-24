package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	schoolPortalClassTripsVersion     = "1.15.493"
	schoolPortalClassTripsDescription = "class_day:class_trip_write for the lehrkraft role (#4799)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolPortalClassTripsVersion,
		Description: schoolPortalClassTripsDescription,
		DependsOn:   []string{schoolPortalArrivalExceptionsVersion, schoolPortalWriteScopeMultiSelectVersion},
	})

	Migrations.MustRegister(schoolPortalClassTripsUp, schoolPortalClassTripsDown)
}

// schoolPortalClassTripsUp lets a Lehrkraft report a class trip or day outing
// of her class through "moto schule" (#4799). class_day:class_trip_write goes
// to the lehrkraft system role like class_day:arrival_exception_write (#2970):
// the routes behind it re-check the education.class_teachers assignment per
// class, and the school's setting operations.school_portal_write_scope
// decides whether the door is unlocked at all. The trip writes the OGS's own
// rows (status days, per-child arrival exceptions); no table changes.
func schoolPortalClassTripsUp(ctx context.Context, db *bun.DB) error {
	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "class_day:class_trip_write",
		Description: "Ausflug oder Klassenfahrt einer zugewiesenen Klasse melden (moto schule)",
		Resource:    "class_day",
		Action:      "class_trip_write",
	}, "lehrkraft")
}

func schoolPortalClassTripsDown(ctx context.Context, db *bun.DB) error {
	return dropPermission(ctx, db, "class_day:class_trip_write")
}
