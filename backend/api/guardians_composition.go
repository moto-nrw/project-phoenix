package api

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	usersAPI "github.com/moto-nrw/project-phoenix/modules/peopledirectory/http"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The guardian surface under /api/guardians (#2663) is the People Directory
// HTTP adapter bound to the shared renderer, the JWT identity and the
// legacy-service runtime for invitations and document rendering.

// renderPeopleDirectoryFailure writes the shared error shape for a failure
// kind the People Directory adapters classified.
func renderPeopleDirectoryFailure(w http.ResponseWriter, r *http.Request, kind usersAPI.FailureKind, err error) {
	switch kind {
	case usersAPI.FailureInvalidRequest:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case usersAPI.FailureUnauthorized:
		apiCommon.RenderError(w, r, apiCommon.ErrorUnauthorized(err))
	case usersAPI.FailureForbidden:
		apiCommon.RenderError(w, r, apiCommon.ErrorForbidden(err))
	case usersAPI.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	case usersAPI.FailureConflict:
		apiCommon.RenderError(w, r, apiCommon.ErrorConflict(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}

func guardianFailureKind(kind services.GuardianFailureKind) usersAPI.FailureKind {
	if kind == services.GuardianFailureForbidden {
		return usersAPI.FailureForbidden
	}
	return usersAPI.FailureInvalidRequest
}

// newGuardiansResource binds the guardian HTTP adapter over the People
// Directory and the legacy-service runtime. appEnv gates the seed-only raw
// invitation token.
func newGuardiansResource(module peopleModule.Capability, runtime services.GuardianDirectoryRuntime, db *bun.DB, appEnv string, logger *slog.Logger) *usersAPI.GuardianResource {
	return usersAPI.NewGuardianResource(module, usersAPI.GuardianRuntime{
		Protected: func(router chi.Router, register func(chi.Router, usersAPI.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, register)
		},
		Permission: func(permission string) usersAPI.Middleware {
			return apiCommon.RequiresPermission(permission)
		},
		ParsePagination: apiCommon.ParsePagination,
		Success:         apiCommon.Respond,
		SuccessPaginated: func(w http.ResponseWriter, r *http.Request, status int, data any, pagination usersAPI.Pagination, message string) {
			apiCommon.RespondPaginated(w, r, status, data, apiCommon.PaginationParams{Page: pagination.Page, PageSize: pagination.PageSize, Total: pagination.Total}, message)
		},
		Failure:         renderPeopleDirectoryFailure,
		ObserveResponse: observability.ObserveGuardianDirectoryHTTPResponse,

		ActorID: func(r *http.Request) int64 {
			return int64(jwt.ClaimsFromCtx(r.Context()).ID)
		},
		ActorRole: func(r *http.Request) string {
			return strings.Join(jwt.ClaimsFromCtx(r.Context()).Roles, ",")
		},
		HasPermission: func(r *http.Request, permission string) bool {
			return apiCommon.HasPermission(permission, jwt.PermissionsFromCtx(r.Context()))
		},
		IsAdmin: func(r *http.Request) bool {
			return apiCommon.HasAdminWildcard(jwt.PermissionsFromCtx(r.Context()))
		},
		IsVerifiedStaff: runtime.IsVerifiedStaff,
		ExposeInvitationToken: func(r *http.Request) bool {
			return apiCommon.ShouldExposeSeedInvitationToken(r, appEnv)
		},
		MarkRollback: runtime.MarkRollback,

		SendInvitation: func(ctx context.Context, guardianID, actorAccountID int64) (usersAPI.GuardianInvitation, error) {
			summary, err := runtime.SendInvitation(ctx, guardianID, actorAccountID)
			if err != nil {
				return usersAPI.GuardianInvitation{}, err
			}
			return usersAPI.GuardianInvitation{
				ID: summary.ID, GuardianProfileID: summary.GuardianProfileID, ExpiresAt: summary.ExpiresAt,
				EmailSent: summary.EmailSent, Token: summary.Token,
			}, nil
		},
		ListPendingInvitations: func(ctx context.Context) ([]usersAPI.PendingGuardianInvitation, error) {
			invitations, err := runtime.ListPendingInvitations(ctx)
			if err != nil {
				return nil, err
			}
			result := make([]usersAPI.PendingGuardianInvitation, 0, len(invitations))
			for _, invitation := range invitations {
				result = append(result, usersAPI.PendingGuardianInvitation{
					ID: invitation.ID, GuardianProfileID: invitation.GuardianProfileID, CreatedAt: invitation.CreatedAt,
					ExpiresAt: invitation.ExpiresAt, EmailSentAt: invitation.EmailSentAt, EmailError: invitation.EmailError,
					EmailRetryCount: invitation.EmailRetryCount,
				})
			}
			return result, nil
		},
		InviteGuardianToStudent: func(ctx context.Context, input usersAPI.GuardianInvite) (usersAPI.GuardianInviteResult, error) {
			result, err := runtime.InviteGuardianToStudent(ctx, services.GuardianInviteInput{
				StudentID: input.StudentID, Email: input.Email, FirstName: input.FirstName, LastName: input.LastName,
				RelationshipType: input.RelationshipType, ActorAccountID: input.ActorAccountID, ConfirmRoleUpgrade: input.ConfirmRoleUpgrade,
			})
			if err != nil {
				return usersAPI.GuardianInviteResult{}, err
			}
			return usersAPI.GuardianInviteResult{
				Outcome: result.Outcome, GuardianProfileID: result.GuardianProfileID,
				InvitationID: result.InvitationID, ExistingRole: result.ExistingRole,
			}, nil
		},
		InviteFailureKind: func(err error) usersAPI.FailureKind {
			return guardianFailureKind(services.ClassifyGuardianInvitationFailure(err))
		},
		ListPendingApprovals: func(ctx context.Context) ([]usersAPI.GuardianPendingApproval, error) {
			views, err := runtime.ListPendingApprovals(ctx)
			if err != nil {
				return nil, err
			}
			result := make([]usersAPI.GuardianPendingApproval, 0, len(views))
			for _, view := range views {
				result = append(result, usersAPI.GuardianPendingApproval{
					InvitationID: view.InvitationID, GuardianProfileID: view.GuardianProfileID,
					GuardianName: view.GuardianName, GuardianEmail: view.GuardianEmail,
					StudentID: view.StudentID, StudentName: view.StudentName, RequestedByEmail: view.RequestedByEmail,
					CreatedAt: view.CreatedAt, ExpiresAt: view.ExpiresAt, RoleUpgrade: view.RoleUpgrade,
				})
			}
			return result, nil
		},
		PendingInvitationStudentID: runtime.PendingInvitationStudentID,
		ApproveInvitation:          runtime.ApproveInvitation,
		RejectInvitation:           runtime.RejectInvitation,

		RenderPaymentExport: func(rows []peopleModule.GuardianPaymentRow, format string) (usersAPI.ExportFile, error) {
			file, err := runtime.RenderPaymentExport(rows, format)
			if err != nil {
				return usersAPI.ExportFile{}, err
			}
			return usersAPI.ExportFile{ContentType: file.ContentType, Filename: file.Filename, Data: file.Data}, nil
		},

		Log: logger,
	})
}
