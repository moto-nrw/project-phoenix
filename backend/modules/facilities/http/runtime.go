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
	listexport "github.com/moto-nrw/project-phoenix/modules/documentrendering/records"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type RoomReads interface {
	ListRooms(context.Context) ([]facilitiesModule.RoomWithOccupancy, error)
	ProjectRoomOccupancy(context.Context, []facilitiesModule.Room) ([]facilitiesModule.RoomWithOccupancy, error)
	GetRoomHistory(context.Context, int64, time.Time, time.Time, *int64) ([]facilitiesModule.RoomSessionEntry, error)
}

type Dependencies struct {
	Facilities RoomReads
	// RoomHistorySettings resolves whether the room history is shown and how
	// many days it spans; the root binds it to the Settings Platform.
	RoomHistorySettings func(context.Context) (context.Context, bool, int)
	UserContext         CallerStaff
	Active              studentpresence.Presence
	Users               Students
	Education           SchoolGroups
	ListExport          listexport.Renderer
}

func NewResource(
	rooms facilitiesModule.Capability,
	dependencies Dependencies,
	logger *slog.Logger,
	protectedTenantRoutes func(chi.Router, func(chi.Router, Middleware)),
	constraintViolation func(error) bool,
) *Resource {
	if dependencies.Facilities == nil || dependencies.RoomHistorySettings == nil || dependencies.UserContext == nil ||
		dependencies.Active == nil || dependencies.Users == nil || dependencies.Education == nil ||
		dependencies.ListExport == nil || logger == nil || protectedTenantRoutes == nil || constraintViolation == nil {
		panic("rooms HTTP composition: all dependencies are required")
	}
	return newRoomResource(rooms, Runtime{
		Protected:        protectedTenantRoutes,
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
		HistoryConfig:    dependencies.RoomHistorySettings,
		HistoryAllowed:   historyAllowed(dependencies.UserContext),
		History:          roomHistory(dependencies.Facilities),
		TodayBounds:      todayBounds,
		ExportSnapshot:   snapshotExporter(dependencies),
		ConstraintFailed: constraintViolation,
		Log:              logger,
	})
}

func respondPaginated(w http.ResponseWriter, r *http.Request, status int, data any, pagination Pagination, message string) {
	apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{
		Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total,
	}, message)
}

func renderFailure(w http.ResponseWriter, r *http.Request, kind FailureKind, err error, _ string) {
	switch kind {
	case FailureInvalid:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case FailureForbidden:
		if errors.Is(err, ErrHistoryDisabled) {
			apiCommon.RenderError(w, r, apiCommon.ErrorForbiddenWithCode(err, apiCommon.CodeAttendanceLogDisabled))
			return
		}
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case FailureNotFound:
		// Only ErrRoomNotFound maps here: a deleted or unknown room names
		// itself instead of falling back to the input class text.
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFoundWithCode(err, apiCommon.CodeRoomsNotFound))
	case FailureConflict:
		apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}

func occupancyProjection(service RoomReads) func(context.Context, []facilitiesModule.Room) ([]RoomView, error) {
	return func(ctx context.Context, rooms []facilitiesModule.Room) ([]RoomView, error) {
		legacyViews, err := service.ProjectRoomOccupancy(ctx, rooms)
		if err != nil {
			return nil, err
		}
		result := make([]RoomView, 0, len(legacyViews))
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

func roomView(room facilitiesModule.Room, view facilitiesModule.RoomWithOccupancy) RoomView {
	return RoomView{
		Room: room, IsOccupied: view.IsOccupied, GroupName: view.GroupName, CategoryName: view.CategoryName,
		StudentCount: view.StudentCount, SupervisorNames: view.SupervisorNames,
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
		if apiCommon.HasAdminWildcard(sessiontoken.PermissionsFromCtx(ctx)) {
			return true, nil
		}
		_, found, err := users.CurrentStaffIDOfPerson(ctx)
		if err != nil {
			return false, err
		}
		return found, nil
	}
}

func roomHistory(service RoomReads) func(context.Context, int64, time.Time, time.Time) ([]RoomSessionEntry, error) {
	return func(ctx context.Context, roomID int64, start, end time.Time) ([]RoomSessionEntry, error) {
		entries, err := service.GetRoomHistory(ctx, roomID, start, end, nil)
		if err != nil {
			return nil, err
		}
		result := make([]RoomSessionEntry, 0, len(entries))
		for _, entry := range entries {
			result = append(result, RoomSessionEntry{
				SessionID: entry.SessionID, StartedAt: entry.StartedAt, EndedAt: entry.EndedAt,
				DurationMinutes: entry.DurationMinutes, ActivityName: entry.ActivityName,
				SupervisorName: entry.SupervisorName, StudentCount: entry.StudentCount,
			})
		}
		return result, nil
	}
}

func todayBounds() (time.Time, time.Time) {
	today := calendar.Today()
	return today, calendar.EndOfDay(today)
}

func snapshotExporter(dependencies Dependencies) func(context.Context, SnapshotRequest) (ExportFile, error) {
	return func(ctx context.Context, request SnapshotRequest) (ExportFile, error) {
		return exportRoomSnapshot(ctx, dependencies, request)
	}
}
