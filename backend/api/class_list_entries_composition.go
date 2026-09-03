package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	schoolMembershipModule "github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	classListHTTP "github.com/moto-nrw/project-phoenix/modules/schoolmembership/http/classlistentries"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

// The class-list entries under /api/class-list-entries are served by the
// School Membership class-list adapter (#2668). It reads through the owner
// capability; the audited write flows and the student-match hint stay with
// the legacy service and are bound here as closures.

// newClassListEntriesResource binds the adapter to the shared renderer, the
// JWT identity and the legacy-service composition.
func newClassListEntriesResource(module schoolMembershipModule.Query, svc *services.Factory, db *bun.DB, logger *slog.Logger) *classListHTTP.Resource {
	runtime := svc.NewClassListEntryRuntime()
	return classListHTTP.NewResource(module, classListHTTP.Runtime{
		Protected: func(router chi.Router, register func(chi.Router, classListHTTP.Middleware)) {
			apiCommon.ProtectedTenantGroup(router, db, func(protected chi.Router, withTx apiCommon.Middleware) {
				register(protected, withTx)
			})
		},
		Permission:      apiCommon.RequiresPermission,
		Success:         apiCommon.Respond,
		Failure:         renderClassListEntryFailure,
		ObserveResponse: observability.ObserveSchoolMembershipHTTPResponse,
		WriteFailure: func(w http.ResponseWriter, r *http.Request, err error) {
			kind, rendered := services.ClassifyClassListEntryFailure(err)
			if kind == services.StaffFailureInternal {
				logger.Error("class list entries: request failed", "error", err.Error())
			}
			renderClassListEntryFailure(w, r, classListEntryFailureKind(kind), rendered)
		},
		CurrentAccountID: func(ctx context.Context) int64 { return int64(jwt.ClaimsFromCtx(ctx).ID) },

		Order: runtime.Order,
		MatchingStudentIDs: func(ctx context.Context, input classListHTTP.EntryInput) ([]int64, error) {
			return runtime.MatchingStudentIDs(ctx, classListEntryInput(input))
		},
		Create: func(ctx context.Context, input classListHTTP.EntryInput, actorID int64) (schoolMembershipModule.ClassListEntry, error) {
			return runtime.Create(ctx, classListEntryInput(input), actorID)
		},
		Update: func(ctx context.Context, id int64, input classListHTTP.EntryInput, actorID int64) (schoolMembershipModule.ClassListEntry, error) {
			return runtime.Update(ctx, id, classListEntryInput(input), actorID)
		},
		Delete: runtime.Delete,
		Assign: runtime.Assign,

		Log: logger,
	})
}

func classListEntryInput(input classListHTTP.EntryInput) services.ClassListEntryInput {
	return services.ClassListEntryInput{FirstName: input.FirstName, LastName: input.LastName, SchoolClass: input.SchoolClass}
}

func classListEntryFailureKind(kind services.StaffFailureKind) classListHTTP.FailureKind {
	switch kind {
	case services.StaffFailureInvalidRequest:
		return classListHTTP.FailureInvalidRequest
	case services.StaffFailureNotFound:
		return classListHTTP.FailureNotFound
	default:
		return classListHTTP.FailureInternal
	}
}

// renderClassListEntryFailure writes the shared error shape for a classified
// failure and records the response like every other membership response.
func renderClassListEntryFailure(w http.ResponseWriter, r *http.Request, kind classListHTTP.FailureKind, err error) {
	observability.ObserveSchoolMembershipHTTPResponse(classListHTTP.StatusOf(kind), string(kind))
	switch kind {
	case classListHTTP.FailureInvalidRequest:
		apiCommon.RenderError(w, r, apiCommon.ErrorInvalidRequest(err))
	case classListHTTP.FailureNotFound:
		apiCommon.RenderError(w, r, apiCommon.ErrorNotFound(err))
	default:
		apiCommon.RenderError(w, r, apiCommon.ErrorInternalServer(err))
	}
}
