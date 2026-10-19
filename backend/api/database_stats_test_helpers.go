//go:build testhooks

package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	databasestats "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	stats "github.com/moto-nrw/project-phoenix/modules/transactionruntime/stats"
)

func NewDatabaseStatsTestReader(db transactionruntime.Database) (DatabaseStatsReader, error) {
	repos, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	members := owners.MustNewMembershipTestRepositories(db)
	rooms, err := owners.NewTimetableRoomRows(db)
	if err != nil {
		return nil, err
	}
	auth, err := NewIdentityAccessForTests(db)
	if err != nil {
		return nil, err
	}
	device, err := NewDeviceRepository(db)
	if err != nil {
		return nil, err
	}
	databaseService := databasestats.NewDatabaseStats(stats.StatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			rows, err := members.Student.List(ctx, nil)
			return len(rows), err
		},
		Teachers: func(ctx context.Context) (int, error) {
			rows, err := members.Staff.List(ctx, nil)
			return len(rows), err
		},
		Rooms: func(ctx context.Context) (int, error) { rows, err := rooms.Room.List(ctx, nil); return len(rows), err },
		Activities: func(ctx context.Context) (int, error) {
			rows, err := repos.ActivityGroup.ListAll(ctx)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) {
			rows, err := owners.NewSchoolStructureSuiteRepositories(db).Group.List(ctx, nil)
			return len(rows), err
		},
		Roles: func(ctx context.Context) (int, error) {
			rows, err := auth.ListRoles(ctx, identityaccess.RoleFilter{})
			return len(rows), err
		},
		Devices: func(ctx context.Context) (int, error) {
			rows, err := device.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := auth.ListPermissions(ctx, identityaccess.PermissionFilter{})
			return len(rows), err
		},
	}, slog.Default())

	return NewDatabaseStatsReader(databaseService, func(ctx context.Context) stats.StatsCapabilities {
		return securityruntime.DatabaseStatsCapabilities(requestTokenReads.Permissions(ctx))
	}), nil
}
