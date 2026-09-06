package services

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/database"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
	"github.com/uptrace/bun"
)

func NewDatabaseStatsTestReader(db *bun.DB) (DatabaseStatsReader, error) {
	repos, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return nil, err
	}
	auth, err := repositories.NewAuthTestRepositories(db, repositories.NewTestAuditStore(db))
	if err != nil {
		return nil, err
	}
	device, err := repositories.NewDeviceTestRepository(db)
	if err != nil {
		return nil, err
	}
	databaseService := database.NewService(database.StatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			rows, err := repos.Student.List(ctx, nil)
			return len(rows), err
		},
		Teachers: func(ctx context.Context) (int, error) { rows, err := repos.Staff.List(ctx, nil); return len(rows), err },
		Rooms:    func(ctx context.Context) (int, error) { rows, err := repos.Room.List(ctx, nil); return len(rows), err },
		Activities: func(ctx context.Context) (int, error) {
			rows, err := repos.ActivityGroup.List(ctx, nil)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) { rows, err := repos.Group.List(ctx, nil); return len(rows), err },
		Roles:  func(ctx context.Context) (int, error) { rows, err := auth.Role.List(ctx, nil); return len(rows), err },
		Devices: func(ctx context.Context) (int, error) {
			rows, err := device.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := auth.Permission.List(ctx, nil)
			return len(rows), err
		},
	}, slog.Default())

	return NewDatabaseStatsReader(databaseService, func(ctx context.Context) database.StatsCapabilities {
		return usercontext.DatabaseStatsCapabilities(ctx)
	}), nil
}
