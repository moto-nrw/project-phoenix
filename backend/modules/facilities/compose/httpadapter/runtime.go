package httpadapter

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	roomsHTTP "github.com/moto-nrw/project-phoenix/modules/facilities/http/rooms"
	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type Resource = roomsHTTP.Resource

type Dependencies struct {
	Facilities  facilitiesService.Service
	Settings    configService.SettingsService
	UserContext CallerStaff
	Active      studentpresence.Presence
	Users       usersService.PersonService
	Education   educationService.Service
	ListExport  *listexport.RendererService
}

func NewResource(
	rooms facilitiesModule.Capability,
	dependencies Dependencies,
	db *bun.DB,
	logger *slog.Logger,
) *roomsHTTP.Resource {
	if dependencies.Facilities == nil || dependencies.Settings == nil || dependencies.UserContext == nil ||
		dependencies.Active == nil || dependencies.Users == nil || dependencies.Education == nil ||
		dependencies.ListExport == nil || db == nil || logger == nil {
		panic("rooms HTTP composition: all dependencies are required")
	}
	return roomsHTTP.NewResource(rooms, runtime(dependencies, db, logger))
}

func runtime(dependencies Dependencies, db *bun.DB, logger *slog.Logger) roomsHTTP.Runtime {
	return roomsHTTP.Runtime{
		Protected:        protectedRoutes(db),
		Permission:       apiCommon.RequiresPermission,
		ParseID:          apiCommon.ParseID,
		Pagination:       apiCommon.ParsePagination,
		Success:          apiCommon.Respond,
		Paginated:        respondPaginated,
		NoContent:        apiCommon.RespondNoContent,
		Read:             permissions.RoomsRead,
		ReadUsers:        permissions.UsersRead,
		Create:           permissions.RoomsCreate,
		Update:           permissions.RoomsUpdate,
		Delete:           permissions.RoomsDelete,
		Failure:          renderFailure,
		Occupancy:        occupancyProjection(dependencies.Facilities),
		HistoryConfig:    historyConfig(dependencies.Settings, logger),
		HistoryAllowed:   historyAllowed(dependencies.UserContext),
		History:          roomHistory(dependencies.Facilities),
		TodayBounds:      todayBounds,
		ExportSnapshot:   snapshotExporter(dependencies),
		ConstraintFailed: apiCommon.IsConstraintViolation,
		Log:              logger,
	}
}

func protectedRoutes(db *bun.DB) func(chi.Router, func(chi.Router, roomsHTTP.Middleware)) {
	return func(router chi.Router, routes func(chi.Router, roomsHTTP.Middleware)) {
		apiCommon.ProtectedTenantGroup(router, db, routes)
	}
}

func respondPaginated(w http.ResponseWriter, r *http.Request, status int, data any, pagination roomsHTTP.Pagination, message string) {
	apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{
		Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total,
	}, message)
}

func renderFailure(w http.ResponseWriter, r *http.Request, kind roomsHTTP.FailureKind, err error, code string) {
	switch kind {
	case roomsHTTP.FailureInvalid:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case roomsHTTP.FailureForbidden:
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case roomsHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	case roomsHTTP.FailureConflict:
		renderConflict(w, r, err, code)
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}

func renderConflict(w http.ResponseWriter, r *http.Request, err error, code string) {
	if code == "color_already_in_use" {
		apiCommon.RenderError(w, r, apiCommon.ErrorConflictWithCode(err, code))
		return
	}
	apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
}

func occupancyProjection(service facilitiesService.Service) func(context.Context, []facilitiesModule.Room) ([]roomsHTTP.RoomView, error) {
	return func(ctx context.Context, rooms []facilitiesModule.Room) ([]roomsHTTP.RoomView, error) {
		legacyViews, err := service.ProjectRoomOccupancy(ctx, rooms)
		if err != nil {
			return nil, err
		}
		result := make([]roomsHTTP.RoomView, 0, len(legacyViews))
		for index, view := range legacyViews {
			if view.Room == nil || index >= len(rooms) || view.ID != rooms[index].ID {
				return nil, errors.New("room occupancy projection changed the owner-visible room set")
			}
			result = append(result, roomView(*view.Room, view))
		}
		if len(result) != len(rooms) {
			return nil, errors.New("room occupancy projection changed the owner-visible room set")
		}
		return result, nil
	}
}

func roomView(room facilitiesModule.Room, view facilitiesService.RoomWithOccupancy) roomsHTTP.RoomView {
	return roomsHTTP.RoomView{
		Room: room, IsOccupied: view.IsOccupied, GroupName: view.GroupName, CategoryName: view.CategoryName,
		StudentCount: view.StudentCount, SupervisorNames: view.SupervisorNames,
	}
}

func historyConfig(settings configService.SettingsService, logger *slog.Logger) func(context.Context) (context.Context, bool, int) {
	return func(ctx context.Context) (context.Context, bool, int) {
		ctx = apiCommon.PrefetchSettings(ctx, settings,
			configModel.KeyAttendanceLogEnabled, configModel.KeyRoomDetailVisibleDays)
		enabled := configService.ResolveBoolOrDefault(ctx, settings, configModel.KeyAttendanceLogEnabled, false, logger)
		days := configService.ResolveIntOrDefault(ctx, settings, configModel.KeyRoomDetailVisibleDays, 7, logger)
		return ctx, enabled, days
	}
}

// CallerStaff resolves the caller's staff member through the Identity &
// Access caller context. found is false, without an error, for a person who
// is no staff member; an account without a person is an error.
type CallerStaff interface {
	CurrentStaffIDOfPerson(ctx context.Context) (staffID int64, found bool, err error)
}

func historyAllowed(users CallerStaff) func(context.Context) (bool, error) {
	return func(ctx context.Context) (bool, error) {
		if apiCommon.HasAdminWildcard(projectJWT.PermissionsFromCtx(ctx)) {
			return true, nil
		}
		_, found, err := users.CurrentStaffIDOfPerson(ctx)
		if err != nil {
			return false, err
		}
		return found, nil
	}
}

func roomHistory(service facilitiesService.Service) func(context.Context, int64, time.Time, time.Time) ([]roomsHTTP.RoomSessionEntry, error) {
	return func(ctx context.Context, roomID int64, start, end time.Time) ([]roomsHTTP.RoomSessionEntry, error) {
		entries, err := service.GetRoomHistory(ctx, roomID, start, end, nil)
		if err != nil {
			return nil, err
		}
		result := make([]roomsHTTP.RoomSessionEntry, 0, len(entries))
		for _, entry := range entries {
			result = append(result, roomsHTTP.RoomSessionEntry{
				SessionID: entry.SessionID, StartedAt: entry.StartedAt, EndedAt: entry.EndedAt,
				DurationMinutes: entry.DurationMinutes, ActivityName: entry.ActivityName,
				SupervisorName: entry.SupervisorName, StudentCount: entry.StudentCount,
			})
		}
		return result, nil
	}
}

func todayBounds() (time.Time, time.Time) {
	today := timezone.Today()
	return today, timezone.EndOfDay(today)
}

func snapshotExporter(dependencies Dependencies) func(context.Context, roomsHTTP.SnapshotRequest) (roomsHTTP.ExportFile, error) {
	return func(ctx context.Context, request roomsHTTP.SnapshotRequest) (roomsHTTP.ExportFile, error) {
		return exportRoomSnapshot(ctx, dependencies, request)
	}
}
