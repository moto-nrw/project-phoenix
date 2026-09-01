package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffHRPermissionsVersion     = "1.15.360"
	staffHRPermissionsDescription = "Split staff personnel access out of users:update into staff:stammdaten / staff:documents / staff:manage (#2906)"

	staffStammdatenPermissionName = "staff:stammdaten"
	staffDocumentsPermissionName  = "staff:documents"
	staffManagePermissionName     = "staff:manage"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffHRPermissionsVersion,
		Description: staffHRPermissionsDescription,
		DependsOn:   []string{auditCommandViewsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addStaffHRPermissions(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeStaffHRPermissions(ctx, db)
		},
	)
}

// addStaffHRPermissions introduces the three personnel permissions and grants
// them to the admin (OGS-Leitung) role only.
//
// Until now the staff Stammdaten tab, the general personnel documents and the
// write on another person's staff record all gated on users:update. Migration
// 1.9.4 grants users:update to the plain `user` role — the role every Betreuer
// holds, because it is what carries the child-data writes — so by default any
// supervisor could read and change a colleague's private address, emergency
// contact, contract terms and qualifications, and list, download, upload and
// delete their Arbeitsvertrag, Zeugnis and Bewerbung (#2906).
//
// No grant is removed here: users:update keeps every child-data surface it
// gates. The personnel surfaces simply stop accepting it. Bank/tax data
// (staff:financial), AU-Bescheinigungen (staff_documents:health) and the
// working-time history (time_tracking:manage) were already separate and are
// untouched; admins keep everything through the admin:* wildcard anyway, the
// explicit grants exist so the permissions show up in the role management UI.
func addStaffHRPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.360: Adding staff personnel permissions (#2906)...")

	specs := []permissionSpec{
		{
			Name:        staffStammdatenPermissionName,
			Description: "Read and maintain the personnel master data of staff members (birthday, address, emergency contact, contract, qualifications)",
			Resource:    "staff",
			Action:      "stammdaten",
		},
		{
			Name:        staffDocumentsPermissionName,
			Description: "Manage general personnel documents of staff members (employment contract, references, applications, other)",
			Resource:    "staff",
			Action:      "documents",
		},
		{
			Name:        staffManagePermissionName,
			Description: "Change the staff record of other staff members (staff notes, teacher role, vacation quota)",
			Resource:    "staff",
			Action:      "manage",
		},
	}

	for _, spec := range specs {
		if err := grantPermissionToRoles(ctx, db, spec, "admin"); err != nil {
			return err
		}
	}

	fmt.Println("Migration 1.15.360: Granted staff:stammdaten, staff:documents and staff:manage to the admin role")
	return nil
}

func removeStaffHRPermissions(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.360: Removing staff personnel permissions...")

	for _, name := range []string{
		staffStammdatenPermissionName,
		staffDocumentsPermissionName,
		staffManagePermissionName,
	} {
		if err := dropPermission(ctx, db, name); err != nil {
			return err
		}
	}

	return nil
}
