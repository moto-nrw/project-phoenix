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
	projectJWT "github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	roomsHTTP "github.com/moto-nrw/project-phoenix/modules/facilities/http/rooms"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	educationService "github.com/moto-nrw/project-phoenix/services/education"
	facilitiesService "github.com/moto-nrw/project-phoenix/services/facilities"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	userContextService "github.com/moto-nrw/project-phoenix/services/usercontext"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type Resource = roomsHTTP.Resource

type Dependencies struct {
	Facilities  facilitiesService.Service
	Settings    configService.SettingsService
	UserContext userContextService.UserContextService
	Active      activeService.Service
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
		Protected: func(router chi.Router, routes func(chi.Router, roomsHTTP.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, routes)
		},
		Permission: apiCommon.RequiresPermission,
		ParseID:    apiCommon.ParseID,
		Pagination: apiCommon.ParsePagination,
		Success:    apiCommon.Respond,
		Paginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination roomsHTTP.Pagination, message string) {
			apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{
				Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total,
			}, message)
		},
		NoContent: apiCommon.RespondNoContent,
		Read:      permissions.RoomsRead,
		ReadUsers: permissions.UsersRead,
		Create:    permissions.RoomsCreate,
		Update:    permissions.RoomsUpdate,
		Delete:    permissions.RoomsDelete,
		Failure: func(w http.ResponseWriter, r *http.Request, kind roomsHTTP.FailureKind, err error, code string) {
			switch kind {
			case roomsHTTP.FailureInvalid:
				apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
			case roomsHTTP.FailureForbidden:
				apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
			case roomsHTTP.FailureNotFound:
				apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
			case roomsHTTP.FailureConflict:
				if code == "color_already_in_use" {
					apiCommon.RenderError(w, r, apiCommon.ErrorConflictWithCode(err, code))
				} else {
					apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
				}
			default:
				apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
			}
		},
		Occupancy: func(ctx context.Context, rooms []facilitiesModule.Room) ([]roomsHTTP.RoomView, error) {
			legacyViews, err := dependencies.Facilities.ProjectRoomOccupancy(ctx, rooms)
			if err != nil {
				return nil, err
			}
			result := make([]roomsHTTP.RoomView, 0, len(legacyViews))
			for index, view := range legacyViews {
				if view.Room == nil || index >= len(rooms) || view.ID != rooms[index].ID {
					return nil, errors.New("room occupancy projection changed the owner-visible room set")
				}
				result = append(result, roomsHTTP.RoomView{
					Room:       *view.Room,
					IsOccupied: view.IsOccupied, GroupName: view.GroupName, CategoryName: view.CategoryName,
					StudentCount: view.StudentCount, SupervisorNames: view.SupervisorNames,
				})
			}
			if len(result) != len(rooms) {
				return nil, errors.New("room occupancy projection changed the owner-visible room set")
			}
			return result, nil
		},
		HistoryConfig: func(ctx context.Context) (context.Context, bool, int) {
			ctx = apiCommon.PrefetchSettings(ctx, dependencies.Settings,
				configModel.KeyAttendanceLogEnabled, configModel.KeyRoomDetailVisibleDays)
			enabled := configService.ResolveBoolOrDefault(ctx, dependencies.Settings, configModel.KeyAttendanceLogEnabled, false, logger)
			days := configService.ResolveIntOrDefault(ctx, dependencies.Settings, configModel.KeyRoomDetailVisibleDays, 7, logger)
			return ctx, enabled, days
		},
		HistoryAllowed: func(ctx context.Context) (bool, error) {
			if apiCommon.HasAdminWildcard(projectJWT.PermissionsFromCtx(ctx)) {
				return true, nil
			}
			staff, err := dependencies.UserContext.GetCurrentStaff(ctx)
			if errors.Is(err, userContextService.ErrUserNotLinkedToStaff) {
				return false, nil
			}
			if err != nil {
				return false, err
			}
			if staff == nil {
				return false, errors.New("unexpected nil staff")
			}
			return true, nil
		},
		History: func(ctx context.Context, roomID int64, start, end time.Time) ([]roomsHTTP.RoomSessionEntry, error) {
			entries, err := dependencies.Facilities.GetRoomHistory(ctx, roomID, start, end, nil)
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
		},
		TodayBounds: func() (time.Time, time.Time) {
			today := timezone.Today()
			return today, timezone.EndOfDay(today)
		},
		ExportSnapshot: func(ctx context.Context, request roomsHTTP.SnapshotRequest) (roomsHTTP.ExportFile, error) {
			return exportRoomSnapshot(ctx, dependencies, request)
		},
		ConstraintFailed: apiCommon.IsConstraintViolation,
		Log:              logger,
	}
}
