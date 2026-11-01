package api

import (
	"context"
	"log/slog"
	"net/http"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	nativeUploads "github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/realtime"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	filesHTTP "github.com/moto-nrw/project-phoenix/modules/filestorage/http/files"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	staffHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timeTrackingHTTP "github.com/moto-nrw/project-phoenix/modules/workforce/http/timetracking"
	"github.com/moto-nrw/project-phoenix/observability"
)

// The staff surface under /api/staff is composed from two adapters (#2667):
// the School Membership HTTP adapter owns the directory and membership
// routes, the workforce admin resource of the Workforce module owns everything
// about working time, Stammdaten and documents. Both register on one
// protected router so the URL surface stays exactly what the frontend calls.

func staffFailureKind(kind StaffFailureKind) staffHTTP.FailureKind {
	switch kind {
	case StaffFailureInvalidRequest:
		return staffHTTP.FailureInvalidRequest
	case StaffFailureUnauthorized:
		return staffHTTP.FailureUnauthorized
	case StaffFailureForbidden:
		return staffHTTP.FailureForbidden
	case StaffFailureNotFound:
		return staffHTTP.FailureNotFound
	case StaffFailureConflict:
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

func toStaffHTTPPerson(person StaffDirectoryPerson) staffHTTP.Person {
	return staffHTTP.Person{
		ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, TagID: person.TagID,
		AccountID: person.AccountID, CreatedAt: person.CreatedAt, UpdatedAt: person.UpdatedAt,
	}
}

func toStaffHTTPRoleRows(rows []StaffRoleRow) []staffHTTP.StaffWithRoleResponse {
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
// workforce admin resource of the Workforce module and the School Membership
// adapter bound over it.
func newStaffComposition(
	module schoolMembershipModule.Capability, workforce workforceModule.Capability, users peoplePorts.PersonDirectory,
	documents *workforceModule.StaffAdmin, timeTracking *workforceCompose.TimeTracking, payroll workforceModule.PayrollDispatch, absenceWrites timeTrackingHTTP.AdminAbsenceWrites, education schoolstructure.GroupManagement,
	people peopleModule.Capability, settings settingsCompose.SettingsService, hub *realtime.Hub,
	identity identityaccess.Owner, appEnv string, db transactionruntime.Database, logger *slog.Logger,
) (*staffHTTP.Resource, *timeTrackingHTTP.StaffAdminResource, error) {
	exportTransfer, err := owners.BuildExportTransfer(settings, db, logger)
	if err != nil {
		return nil, nil, err
	}
	payrollRules, err := owners.BuildPayrollRules(workforce, db)
	if err != nil {
		return nil, nil, err
	}
	capabilities := owners.NewWorkforceAdminCapabilities(users, documents, timeTracking)
	staffAdmin := newStaffAdminResource(capabilities, absenceWrites, workforce, staffTimeTrackingNotifier(hub), exportTransfer, payroll, payrollRules, db, logger)
	resource, err := newStaffResource(module, people, identity, appEnv, func(hooks StaffMembershipHooks) StaffMembershipRuntime {
		return NewStaffMembershipRuntime(users, education, timeTracking.StaffDay, hub, db, identity, logger, hooks)
	}, staffAdmin, logger)
	if err != nil {
		return nil, nil, err
	}
	return resource, staffAdmin, nil
}

// newStaffResource binds the School Membership HTTP adapter to the shared
// renderer, the JWT identity, the legacy-service composition and the
// external-caregiver workflow (#4573), built here as a local value. appEnv
// gates the seed-only invitation token.
func newStaffResource(
	module schoolMembershipModule.Capability, people peopleModule.Capability, identity identityaccess.Owner, appEnv string,
	buildRuntime func(StaffMembershipHooks) StaffMembershipRuntime, staffAdmin *timeTrackingHTTP.StaffAdminResource, logger *slog.Logger,
) (*staffHTTP.Resource, error) {
	externals, err := newExternalCaregiverBindings(module, people, identity, appEnv)
	if err != nil {
		return nil, err
	}
	runtime := buildStaffMembershipRuntime(buildRuntime, staffAdmin)
	return staffHTTP.NewResource(module, staffHTTP.Runtime{
		Protected:          staffProtectedRoutes(staffAdmin),
		Permission:         staffPermissionGate,
		Success:            apiCommon.Respond,
		Failure:            renderStaffFailure,
		ObserveResponse:    observability.ObserveSchoolMembershipHTTPResponse,
		ParseIDParam:       apiCommon.ParseIDParam,
		ServeAvatar:        filesHTTP.NewStaffAvatarHandler(nativeUploads.ResolveStoredPath, nativeUploads.UploadFiles{}),
		WriteFailure:       renderStaffWriteFailure,
		SchoolClassFailure: delegatedStaffFailure(ClassifyStaffSchoolClassFailure),

		Permissions:      sessiontoken.PermissionsFromCtx,
		HasPermission:    apiCommon.HasPermission,
		CurrentAccountID: currentStaffAccountID,
		CurrentUsername:  currentStaffUsername,

		Person:         staffPersonLookup(runtime),
		PersonNotFound: apiCommon.IsNotFound,
		Persons:        staffPersonsLookup(runtime),

		CreateExternalCaregiver:   externals.create,
		InviteExternalCaregiver:   externals.invite,
		ExternalFailure:           renderExternalCaregiverFailure,
		ExposeSeedInvitationToken: externals.exposeSeedToken,
		PendingInvitationPersons:  externals.pendingPersons,

		PresentStaffIDs:     runtime.PresentStaffIDs,
		WorkStatusMap:       runtime.WorkStatusMap,
		AbsenceMap:          runtime.AbsenceMap,
		AbsenceLabelMap:     runtime.AbsenceLabelMap,
		AccountRoles:        runtime.AccountRoles,
		AccountEmails:       runtime.AccountEmails,
		AccountAvatars:      runtime.AccountAvatars,
		AccountHasRole:      runtime.AccountHasRole,
		AccountsHoldingRole: runtime.AccountsHoldingRole,

		RetryQueuedDocumentCleanups: staffAdmin.RetryQueuedStaffDocumentCleanups,

		TeacherGroups:    staffTeacherGroups(runtime),
		SchoolClasses:    runtime.SchoolClasses,
		SetSchoolClasses: runtime.SetSchoolClasses,
		ActiveCaregivers: staffRoleRows(runtime.ActiveCaregivers),
		StaffByRoles:     staffRoleRowsByRole(runtime.StaffByRoles),

		CreateStaff: staffCreate(runtime),
		UpdateStaff: staffUpdate(runtime),
		Offboard:    runtime.Offboard,
		PlanExit:    runtime.PlanExit, WithdrawExit: runtime.WithdrawExit, ExitFailure: renderStaffExitFailure,

		Log: logger,
	}), nil
}

// buildStaffMembershipRuntime builds the legacy-service runtime with the
// Workforce hooks it calls back into.
func buildStaffMembershipRuntime(buildRuntime func(StaffMembershipHooks) StaffMembershipRuntime, staffAdmin *timeTrackingHTTP.StaffAdminResource) StaffMembershipRuntime {
	return buildRuntime(StaffMembershipHooks{
		ResolveEditorStaffID:           staffAdmin.ResolveEditorStaffID,
		QueueOffboardedDocumentCleanup: staffAdmin.QueueOffboardedStaffDocumentCleanup,
	})
}

// staffProtectedRoutes composes the shared /api/staff router: the membership
// routes register first, then the workforce admin routes from the Workforce
// module, both inside one protected tenant group.
func staffProtectedRoutes(staffAdmin *timeTrackingHTTP.StaffAdminResource) func(chi.Router, func(chi.Router, staffHTTP.Middleware)) {
	return func(router chi.Router, register func(chi.Router, staffHTTP.Middleware)) {
		ProtectedTenantRoutes(router, func(protected chi.Router, withTx apiCommon.Middleware) {
			register(protected, withTx)
			staffAdmin.RegisterStaffRoutes(protected, withTx)
		})
	}
}

func staffPermissionGate(required ...string) staffHTTP.Middleware {
	if len(required) == 1 {
		return apiCommon.RequiresPermission(required[0])
	}
	return apiCommon.RequiresAnyPermission(required...)
}

// renderStaffWriteFailure classifies the create/update/offboard errors: a
// foreign-key violation on offboarding keeps its German conflict message,
// everything else follows the service classification.
func renderStaffWriteFailure(w http.ResponseWriter, r *http.Request, err error) {
	if transactionFailures.IsConstraintViolation(err) {
		renderStaffFailure(w, r, staffHTTP.FailureConflict, schoolMembershipModule.ErrStaffStillReferenced)
		return
	}
	kind, rendered := ClassifyStaffWriteFailure(err)
	renderStaffFailure(w, r, staffFailureKind(kind), rendered)
}

func delegatedStaffFailure(classify func(error) (StaffFailureKind, error)) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		kind, rendered := classify(err)
		renderStaffFailure(w, r, staffFailureKind(kind), rendered)
	}
}

func currentStaffAccountID(ctx context.Context) int64 {
	return int64(sessiontoken.ClaimsFromCtx(ctx).ID)
}

func currentStaffUsername(ctx context.Context) string {
	return sessiontoken.ClaimsFromCtx(ctx).Username
}

func staffPersonLookup(runtime StaffMembershipRuntime) func(context.Context, int64) (staffHTTP.Person, error) {
	return func(ctx context.Context, id int64) (staffHTTP.Person, error) {
		person, err := runtime.Person(ctx, id)
		if err != nil {
			return staffHTTP.Person{}, err
		}
		return toStaffHTTPPerson(person), nil
	}
}

func staffPersonsLookup(runtime StaffMembershipRuntime) func(context.Context, []int64) ([]staffHTTP.Person, error) {
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

func staffTeacherGroups(runtime StaffMembershipRuntime) func(context.Context, int64) ([]staffHTTP.Group, error) {
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

func staffRoleRows(list func(context.Context) ([]StaffRoleRow, error)) func(context.Context) ([]staffHTTP.StaffWithRoleResponse, error) {
	return func(ctx context.Context) ([]staffHTTP.StaffWithRoleResponse, error) {
		rows, err := list(ctx)
		if err != nil {
			return nil, err
		}
		return toStaffHTTPRoleRows(rows), nil
	}
}

func staffRoleRowsByRole(list func(context.Context, []string) ([]StaffRoleRow, error)) func(context.Context, []string) ([]staffHTTP.StaffWithRoleResponse, error) {
	return func(ctx context.Context, roles []string) ([]staffHTTP.StaffWithRoleResponse, error) {
		rows, err := list(ctx, roles)
		if err != nil {
			return nil, err
		}
		return toStaffHTTPRoleRows(rows), nil
	}
}

func staffCreate(runtime StaffMembershipRuntime) func(context.Context, staffHTTP.CreateStaffInput) (staffHTTP.CreateStaffResult, error) {
	return func(ctx context.Context, input staffHTTP.CreateStaffInput) (staffHTTP.CreateStaffResult, error) {
		result, err := runtime.CreateStaff(ctx, StaffCreateInput{
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

func staffUpdate(runtime StaffMembershipRuntime) func(context.Context, staffHTTP.UpdateStaffInput) (staffHTTP.UpdateStaffResult, error) {
	return func(ctx context.Context, input staffHTTP.UpdateStaffInput) (staffHTTP.UpdateStaffResult, error) {
		result, err := runtime.UpdateStaff(ctx, StaffUpdateInput{
			StaffID: input.StaffID, PersonID: input.PersonID, StaffNotes: input.StaffNotes, IsTeacher: input.IsTeacher,
			Specialization: input.Specialization, Role: input.Role, Qualifications: input.Qualifications,
		})
		if err != nil {
			return staffHTTP.UpdateStaffResult{}, err
		}
		return staffHTTP.UpdateStaffResult{Staff: result.Staff, Teacher: result.Teacher, Action: staffHTTP.TeacherAction(result.Action)}, nil
	}
}
