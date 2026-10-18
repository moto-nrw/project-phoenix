package api

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	statsContract "github.com/moto-nrw/project-phoenix/modules/transactionruntime/stats"
)

type DatabaseStatsReader func(context.Context) (*statsContract.StatsResponse, error)

func NewDatabaseStatsReader(service statsContract.Service, capabilities func(context.Context) statsContract.StatsCapabilities) DatabaseStatsReader {
	return func(ctx context.Context) (*statsContract.StatsResponse, error) {
		return service.GetStats(ctx, capabilities(ctx))
	}
}

// DatabaseStatsPermissions lists every permission the stats capabilities
// honour; the stats route opens for any of them (#3469).
func DatabaseStatsPermissions() []string {
	return securityruntime.DatabaseStatsPermissions()
}

// databaseStatsSources are the rows and owners the Datenverwaltung hub
// counts (#4013).
type databaseStatsSources struct {
	people    PeopleWiring
	rooms     facilitiesModule.Capability
	timetable timetableCompose.Repositories
	identity  *identityaccess.Module
	devices   devicefleet.Capability
	logger    *slog.Logger
}

// newDatabaseStats counts the school's rows for the Datenverwaltung hub and
// redacts every count the caller's permissions do not cover.
func newDatabaseStats(sources databaseStatsSources) DatabaseStatsReader {
	rooms := facilitiesCompose.NewRoomRepository(sources.rooms)
	devices := devicefleetRepositoryAdapter.NewDeviceRepository(sources.devices)
	stats := transactionruntime.NewDatabaseStats(statsContract.StatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			return sources.people.Students.CountEnrolled(ctx)
		},
		Teachers: func(ctx context.Context) (int, error) {
			rows, err := sources.people.Staff.List(ctx, nil)
			return len(rows), err
		},
		Rooms: func(ctx context.Context) (int, error) {
			rows, err := rooms.List(ctx, nil)
			return len(rows), err
		},
		Activities: func(ctx context.Context) (int, error) {
			rows, err := sources.timetable.ActivityGroup.List(ctx, nil)
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) {
			rows, err := sources.people.Groups.List(ctx, nil)
			return len(rows), err
		},
		Roles: func(ctx context.Context) (int, error) {
			rows, err := sources.identity.ListRoles(ctx, identityaccess.RoleFilter{})
			return len(rows), err
		},
		Devices: func(ctx context.Context) (int, error) {
			rows, err := devices.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := sources.identity.ListPermissions(ctx, identityaccess.PermissionFilter{})
			return len(rows), err
		},
	}, sources.logger.With("service", "database"))
	return NewDatabaseStatsReader(stats, func(ctx context.Context) statsContract.StatsCapabilities {
		return securityruntime.DatabaseStatsCapabilities(requestTokenReads.Permissions(ctx))
	})
}
