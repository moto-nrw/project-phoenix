package api

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	timeTrackingAPI "github.com/moto-nrw/project-phoenix/api/time-tracking"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The staff surface under /api/staff is composed from two adapters (#2667):
// the School Membership HTTP adapter owns the directory and membership
// routes, the workforce admin resource in api/time-tracking owns everything
// about working time, Stammdaten and documents. Both register on one
// protected router so the URL surface stays exactly what the frontend calls.

func staffFailureKind(kind services.StaffFailureKind) staffHTTP.FailureKind {
	switch kind {
	case services.StaffFailureInvalidRequest:
		return staffHTTP.FailureInvalidRequest
	case services.StaffFailureUnauthorized:
		return staffHTTP.FailureUnauthorized
	case services.StaffFailureForbidden:
		return staffHTTP.FailureForbidden
	case services.StaffFailureNotFound:
		return staffHTTP.FailureNotFound
	case services.StaffFailureConflict:
		return staffHTTP.FailureConflict
	default:
		return staffHTTP.FailureInternal
	}
}

func staffFailureStatus(kind staffHTTP.FailureKind) int {
	switch kind {
	case staffHTTP.FailureInvalidRequest:
		return http.StatusBadRequest
	case staffHTTP.FailureUnauthorized:
		return http.StatusUnauthorized
	case staffHTTP.FailureForbidden:
		return http.StatusForbidden
	case staffHTTP.FailureNotFound:
		return http.StatusNotFound
	case staffHTTP.FailureConflict:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// renderStaffFailure writes the shared error shape for a classified failure
// and records the response the way every other membership response is.
func renderStaffFailure(w http.ResponseWriter, r *http.Request, kind staffHTTP.FailureKind, err error) {
	observability.ObserveSchoolMembershipHTTPResponse(staffFailureStatus(kind), string(kind))
	switch kind {
	case staffHTTP.FailureInvalidRequest:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case staffHTTP.FailureUnauthorized:
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(err))
	case staffHTTP.FailureForbidden:
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case staffHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	case staffHTTP.FailureConflict:
		apiCommon.RenderError(w, r, apiCommon.ErrorConflictMessage(err.Error()))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}

func toStaffHTTPPerson(person services.StaffDirectoryPerson) staffHTTP.Person {
	return staffHTTP.Person{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID,
		AccountID: person.AccountID, CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
	}
}

func toStaffHTTPRoleRows(rows []services.StaffRoleRow) []staffHTTP.StaffWithRoleResponse {
	if rows == nil {
		// A nil result stays nil: the by-role endpoint historically answered
		// "data":null for an empty role match.
		return nil
	}
	result := make([]staffHTTP.StaffWithRoleResponse, 0, len(rows))
	for _, row := range rows {
		result = append(result, staffHTTP.StaffWithRoleResponse{
			ID: row.StaffID, PersonID: row.PersonID, TeacherID: row.TeacherID,
			FirstName: row.FirstName, LastName: row.LastName, FullName: row.FirstName + " " + row.LastName,
			AccountID: row.AccountID, Email: row.Email, IsActiveCaregiver: row.IsActiveCaregiver,
			CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return result
}

// newStaffComposition builds both halves of the /api/staff surface: the
// workforce admin resource from api/time-tracking and the School Membership
// adapter bound over it.
func newStaffComposition(module schoolMembershipModule.Capability, svc *services.Factory, db *bun.DB, logger *slog.Logger) (*staffHTTP.Resource, *timeTrackingAPI.StaffAdminResource) {
	staffAdmin := timeTrackingAPI.NewStaffAdminResource(svc.Users, svc.StaffDocuments, svc.WorkSession, svc.StaffAbsence, svc.WorkTimeMonth, svc.StaffBalanceAdjust, svc.StaffMonthClose, svc.StaffOverview, svc.TimeTrackingAuditLog, svc.StaffTimeExport, db, logger)
	return newStaffResource(module, svc, staffAdmin, db, logger), staffAdmin
}

// newStaffResource binds the School Membership HTTP adapter to the shared
// renderer, the JWT identity and the legacy-service composition.
func newStaffResource(module schoolMembershipModule.Capability, svc *services.Factory, staffAdmin *timeTrackingAPI.StaffAdminResource, db *bun.DB, logger *slog.Logger) *staffHTTP.Resource {
	runtime := svc.NewStaffMembershipRuntime(db, logger, services.StaffMembershipHooks{
		ResolveEditorStaffID:           staffAdmin.ResolveEditorStaffID,
		QueueOffboardedDocumentCleanup: staffAdmin.QueueOffboardedStaffDocumentCleanup,
	})
	return staffHTTP.NewResource(module, staffHTTP.Runtime{
		// Protected composes the shared /api/staff router: the membership
		// routes register first, then the workforce admin routes from
		// api/time-tracking, both inside one protected tenant group.
		Protected: func(router chi.Router, register func(chi.Router, staffHTTP.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, func(protected chi.Router, withTx apiCommon.Middleware) {
				register(protected, withTx)
				staffAdmin.RegisterStaffRoutes(protected, withTx)
			})
		},
		Permission:         staffPermissionGate,
		Success:            apiCommon.Respond,
		Failure:            renderStaffFailure,
		ObserveResponse:    observability.ObserveSchoolMembershipHTTPResponse,
		ServeAvatar:        serveStaffAvatar,
		WriteFailure:       renderStaffWriteFailure,
		SchoolClassFailure: delegatedStaffFailure(services.ClassifyStaffSchoolClassFailure),
		PINFailure:         delegatedStaffFailure(services.ClassifyStaffPINFailure),

		Permissions:      jwt.PermissionsFromCtx,
		HasPermission:    apiCommon.HasPermission,
		CurrentAccountID: currentStaffAccountID,
		CurrentUsername:  currentStaffUsername,

		Person:            staffPersonLookup(runtime),
		Persons:           staffPersonsLookup(runtime),
		PersonIDByAccount: runtime.PersonIDByAccount,

		PresentStaffIDs: runtime.PresentStaffIDs,
		WorkStatusMap:   runtime.WorkStatusMap,
		AbsenceMap:      runtime.AbsenceMap,
		AbsenceLabelMap: runtime.AbsenceLabelMap,
		AccountRoles:    runtime.AccountRoles,
		AccountEmails:   runtime.AccountEmails,
		AccountAvatars:  runtime.AccountAvatars,
		AccountHasRole:  runtime.AccountHasRole,

		GrantDefaultPermissions:     runtime.GrantDefaultPermissions,
		RetryQueuedDocumentCleanups: staffAdmin.RetryQueuedStaffDocumentCleanups,

		TeacherGroups:    staffTeacherGroups(runtime),
		SchoolClasses:    runtime.SchoolClasses,
		SetSchoolClasses: runtime.SetSchoolClasses,
		ActiveCaregivers: staffRoleRows(runtime.ActiveCaregivers),
		StaffByRoles:     staffRoleRowsByRole(runtime.StaffByRoles),

		CreateStaff: staffCreate(runtime),
		UpdateStaff: staffUpdate(runtime),
		Offboard:    runtime.Offboard,

		PINStatus:    runtime.PINStatus,
		PINPreflight: runtime.PINPreflight,
		UpdatePIN:    runtime.UpdatePIN,

		Log: logger,
	})
}

func staffPermissionGate(required ...string) staffHTTP.Middleware {
	if len(required) == 1 {
		return apiCommon.RequiresPermission(required[0])
	}
	return apiCommon.RequiresAnyPermission(required...)
}

func serveStaffAvatar(w http.ResponseWriter, r *http.Request, avatarPath string) {
	filePath, err := apiCommon.ResolveStoredPath("public", avatarPath, "/uploads/avatars/")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	apiCommon.ServeImage(w, r, filepath.Dir(filePath), filepath.Base(filePath), "private, max-age=86400")
}

// renderStaffWriteFailure classifies the create/update/offboard errors: a
// foreign-key violation on offboarding keeps its German conflict message,
// everything else follows the service classification.
func renderStaffWriteFailure(w http.ResponseWriter, r *http.Request, err error) {
	if apiCommon.IsConstraintViolation(err) {
		renderStaffFailure(w, r, staffHTTP.FailureConflict, errStaffStillReferenced)
		return
	}
	kind, rendered := services.ClassifyStaffWriteFailure(err)
	renderStaffFailure(w, r, staffFailureKind(kind), rendered)
}

func delegatedStaffFailure(classify func(error) (services.StaffFailureKind, error)) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		kind, rendered := classify(err)
		renderStaffFailure(w, r, staffFailureKind(kind), rendered)
	}
}

func currentStaffAccountID(ctx context.Context) int64 {
	return int64(jwt.ClaimsFromCtx(ctx).ID)
}

func currentStaffUsername(ctx context.Context) string {
	return jwt.ClaimsFromCtx(ctx).Username
}

func staffPersonLookup(runtime services.StaffMembershipRuntime) func(context.Context, int64) (staffHTTP.Person, error) {
	return func(ctx context.Context, id int64) (staffHTTP.Person, error) {
		person, err := runtime.Person(ctx, id)
		if err != nil {
			return staffHTTP.Person{}, err
		}
		return toStaffHTTPPerson(person), nil
	}
}

func staffPersonsLookup(runtime services.StaffMembershipRuntime) func(context.Context, []int64) ([]staffHTTP.Person, error) {
	return func(ctx context.Context, ids []int64) ([]staffHTTP.Person, error) {
		persons, err := runtime.Persons(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make([]staffHTTP.Person, 0, len(persons))
		for _, person := range persons {
			result = append(result, toStaffHTTPPerson(person))
		}
		return result, nil
	}
}

func staffTeacherGroups(runtime services.StaffMembershipRuntime) func(context.Context, int64) ([]staffHTTP.Group, error) {
	return func(ctx context.Context, teacherID int64) ([]staffHTTP.Group, error) {
		groups, err := runtime.TeacherGroups(ctx, teacherID)
		if err != nil {
			return nil, err
		}
		result := make([]staffHTTP.Group, 0, len(groups))
		for _, group := range groups {
			result = append(result, staffHTTP.Group{ID: group.ID, Name: group.Name})
		}
		return result, nil
	}
}

func staffRoleRows(list func(context.Context) ([]services.StaffRoleRow, error)) func(context.Context) ([]staffHTTP.StaffWithRoleResponse, error) {
	return func(ctx context.Context) ([]staffHTTP.StaffWithRoleResponse, error) {
		rows, err := list(ctx)
		if err != nil {
			return nil, err
		}
		return toStaffHTTPRoleRows(rows), nil
	}
}

func staffRoleRowsByRole(list func(context.Context, []string) ([]services.StaffRoleRow, error)) func(context.Context, []string) ([]staffHTTP.StaffWithRoleResponse, error) {
	return func(ctx context.Context, roles []string) ([]staffHTTP.StaffWithRoleResponse, error) {
		rows, err := list(ctx, roles)
		if err != nil {
			return nil, err
		}
		return toStaffHTTPRoleRows(rows), nil
	}
}

func staffCreate(runtime services.StaffMembershipRuntime) func(context.Context, staffHTTP.CreateStaffInput) (staffHTTP.CreateStaffResult, error) {
	return func(ctx context.Context, input staffHTTP.CreateStaffInput) (staffHTTP.CreateStaffResult, error) {
		result, err := runtime.CreateStaff(ctx, services.StaffCreateInput{
			PersonID: input.PersonID, StaffNotes: input.StaffNotes, IsTeacher: input.IsTeacher,
			Specialization: input.Specialization, Role: input.Role, Qualifications: input.Qualifications,
			ActorPermissions: input.ActorPermissions,
		})
		if err != nil {
			return staffHTTP.CreateStaffResult{}, err
		}
		return staffHTTP.CreateStaffResult{Staff: result.Staff, Teacher: result.Teacher, TeacherCreationFailed: result.TeacherCreationFailed}, nil
	}
}

func staffUpdate(runtime services.StaffMembershipRuntime) func(context.Context, staffHTTP.UpdateStaffInput) (staffHTTP.UpdateStaffResult, error) {
	return func(ctx context.Context, input staffHTTP.UpdateStaffInput) (staffHTTP.UpdateStaffResult, error) {
		result, err := runtime.UpdateStaff(ctx, services.StaffUpdateInput{
			StaffID: input.StaffID, PersonID: input.PersonID, StaffNotes: input.StaffNotes, IsTeacher: input.IsTeacher,
			Specialization: input.Specialization, Role: input.Role, Qualifications: input.Qualifications,
		})
		if err != nil {
			return staffHTTP.UpdateStaffResult{}, err
		}
		return staffHTTP.UpdateStaffResult{Staff: result.Staff, Teacher: result.Teacher, Action: staffHTTP.TeacherAction(result.Action)}, nil
	}
}

// errStaffStillReferenced is the German conflict message the offboarding
// route produced for a foreign-key violation; it is user-facing copy, so it
// keeps its capital letter.
//
//nolint:staticcheck // ST1005: user-facing German message, not a Go error string
var errStaffStillReferenced = errors.New("Personal kann nicht gelöscht werden: Mitarbeiter/in wird noch in anderen Bereichen referenziert")
