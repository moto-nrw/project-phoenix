package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	supervisionOwnPermissionVersion     = "1.15.330"
	supervisionOwnPermissionDescription = "supervision:own permission for the school portal supervision surface (#2527)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     supervisionOwnPermissionVersion,
		Description: supervisionOwnPermissionDescription,
		DependsOn:   []string{lehrkraftRoleVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return supervisionOwnPermissionUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return supervisionOwnPermissionDown(ctx, db)
		},
	)
}

// supervisionOwnPermissionUp grants supervision:own to the lehrkraft system
// role (#2527), so a Lehrkraft can run the Betreuungsplan blocks she is
// personally assigned to through "moto schule".
//
// The permission opens a door, it does not widen a scope: every route behind
// it re-checks the caller's schedule.instance_staff assignment for the
// concrete block, and a school token never inherits the tenant-wide
// operational overview (#2380). Admins get it too so the tenant portal keeps
// answering the same question with the same permission name.
func supervisionOwnPermissionUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.330: supervision:own for the lehrkraft role...")

	return grantPermissionToRoles(ctx, db, permissionSpec{
		Name:        "supervision:own",
		Description: "Eigene Aufsichten aus dem Betreuungsplan durchführen",
		Resource:    "supervision",
		Action:      "own",
	}, "admin", "lehrkraft")
}

func supervisionOwnPermissionDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.330: Removing supervision:own...")

	return dropPermission(ctx, db, "supervision:own")
}
