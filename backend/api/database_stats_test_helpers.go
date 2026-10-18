//go:build testhooks

package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	databasestats "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

func NewDatabaseStatsTestReader(db transactionruntime.Database) (DatabaseStatsReader, error) {
	repos, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	members := MustNewMembershipTestRepositories(db)
	rooms, err := NewTimetableRoomRows(db)
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
	databaseService := databasestats.NewDatabaseStats(databasestats.DatabaseStatsDependencies{
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
			rows, err := repos.ActivityGroup.List(ctx, nil)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) {
			rows, err := members.Group.List(ctx, nil)
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

	return NewDatabaseStatsReader(databaseService, func(ctx context.Context) databasestats.DatabaseStatsCapabilities {
		return securityruntime.DatabaseStatsCapabilities(requestTokenReads.Permissions(ctx))
	}), nil
}
