package api

import (
	"context"
	"log/slog"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetRepositoryAdapter "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose/repositoryadapter"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	timetableOwner "github.com/moto-nrw/project-phoenix/modules/timetable"
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

// newDatabaseStats counts the school's rows for the Datenverwaltung hub
// (#4013): the retained student, staff and group rows, the rooms, the
// activities, the roles and permissions and the devices. It redacts every
// count the caller's permissions do not cover.
func newDatabaseStats(people peopleCompose.Repositories, groups owners.EducationGroupRows, roomOwner facilitiesModule.Capability, timetable timetableCompose.Repositories, identity identityaccess.Owner, deviceOwner devicefleet.Capability, logger *slog.Logger) DatabaseStatsReader {
	rooms := facilitiesCompose.NewRoomRepository(roomOwner)
	devices := devicefleetRepositoryAdapter.NewDeviceRepository(deviceOwner)
	stats := transactionruntime.NewDatabaseStats(statsContract.StatsDependencies{
		Students: func(ctx context.Context) (int, error) {
			return people.Students.CountEnrolled(ctx)
		},
		Teachers: func(ctx context.Context) (int, error) {
			rows, err := people.Staff.List(ctx, nil)
			return len(rows), err
		},
		Rooms: func(ctx context.Context) (int, error) {
			rows, err := rooms.List(ctx, nil)
			return len(rows), err
		},
		Activities: func(ctx context.Context) (int, error) {
			rows, err := timetable.Timetable.ListGroups(ctx, timetableOwner.GroupFilter{})
			return len(rows), err
		},
		Groups: func(ctx context.Context) (int, error) {
			rows, err := groups.List(ctx, nil)
			return len(rows), err
		},
		Roles: func(ctx context.Context) (int, error) {
			rows, err := identity.ListRoles(ctx, identityaccess.RoleFilter{})
			return len(rows), err
		},
		Devices: func(ctx context.Context) (int, error) {
			rows, err := devices.List(ctx, nil)
			return len(rows), err
		},
		PermissionCount: func(ctx context.Context) (int, error) {
			rows, err := identity.ListPermissions(ctx, identityaccess.PermissionFilter{})
			return len(rows), err
		},
	}, logger.With("service", "database"))
	return NewDatabaseStatsReader(stats, func(ctx context.Context) statsContract.StatsCapabilities {
		return securityruntime.DatabaseStatsCapabilities(requestTokenReads.Permissions(ctx))
	})
}
